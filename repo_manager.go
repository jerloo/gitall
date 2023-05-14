package repos

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	cssh "golang.org/x/crypto/ssh"
)

type CommandLogger struct {
	verbose bool
}

func (l *CommandLogger) Info(msg string, args ...interface{}) {
	if l.verbose {
		fmt.Printf(msg+"\n", args...)
	}
}

var logger *CommandLogger = &CommandLogger{}

type RepoManager struct {
	workspace string
	verbose   bool

	auth   *ssh.PublicKeys
	config *ReposConfig
}

type NewRepoManagerClientOptions func(*RepoManager)

func WithVerbose(verbose bool) NewRepoManagerClientOptions {
	return func(client *RepoManager) {
		logger.verbose = verbose
		client.verbose = verbose
	}
}

func WithConfig(config *ReposConfig) NewRepoManagerClientOptions {
	return func(client *RepoManager) {
		client.config = config
	}
}

func IfRepoIsClean(r *git.Repository) bool {
	w, err := r.Worktree()
	cobra.CheckErr(err)

	status, err := w.Status()
	cobra.CheckErr(err)

	return status.IsClean()
}

func newAuth() (*ssh.PublicKeys, error) {
	var publicKey *ssh.PublicKeys
	sshPath := filepath.Join(os.Getenv("HOME"), ".ssh/id_rsa")
	publicKey, keyError := ssh.NewPublicKeysFromFile(ssh.DefaultUsername, sshPath, "")
	if keyError != nil {
		return nil, keyError
	}
	publicKey.HostKeyCallbackHelper = ssh.HostKeyCallbackHelper{
		HostKeyCallback: cssh.InsecureIgnoreHostKey(),
	}
	return publicKey, nil
}

func NewRepoManager(workspace string, options ...NewRepoManagerClientOptions) (*RepoManager, error) {
	dir, err := os.Stat(workspace)
	if err != nil {
		return nil, err
	}
	if !dir.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", workspace)
	}
	auth, err := newAuth()
	if err != nil {
		return nil, err
	}

	client := &RepoManager{
		auth:      auth,
		workspace: workspace,
	}

	for _, opt := range options {
		opt(client)
	}
	return client, nil
}

func (client *RepoManager) openRepo(repoConfig *RepoConfig) (*git.Repository, error) {
	repoPath := filepath.Join(client.workspace, repoConfig.Dir)
	repo, err := git.PlainOpen(repoPath)
	logger.Info("Opening %s", repoPath)
	if err != nil {
		return nil, err
	}
	if !IfRepoIsClean(repo) {
		return nil, fmt.Errorf("%s is not clean", repoPath)
	}
	return repo, nil
}

func (client *RepoManager) progeess() io.Writer {
	if client.verbose {
		return os.Stdout
	}
	return nil
}

func (client *RepoManager) pullSingleRepo(repo *git.Repository) error {
	w, err := repo.Worktree()
	if err != nil {
		return err
	}

	err = w.Pull(&git.PullOptions{RemoteName: "origin", Auth: client.auth, Progress: client.progeess()})
	if errors.Is(err, git.NoErrAlreadyUpToDate) {
		return nil
	}
	return err
}

func (client *RepoManager) Pull() error {
	logger.Info("Pulling all in workspace %s", client.workspace)
	fn := func(repoConfig *RepoConfig) error {
		logger.Info("Pulling %s %s", repoConfig.Name, repoConfig.Dir)
		repo, err := client.openRepo(repoConfig)
		if err != nil {
			return err
		}
		err = client.pullSingleRepo(repo)
		if err != nil {
			return err
		}
		return nil
	}

	for _, repoConfig := range client.config.Repos {
		err := fn(repoConfig)
		if err != nil {
			return err
		}
	}
	return nil
}

func (client *RepoManager) pushSingleRepo(repo *git.Repository) error {
	err := repo.Push(&git.PushOptions{RemoteName: "origin", Auth: client.auth, Progress: client.progeess()})
	if errors.Is(err, git.NoErrAlreadyUpToDate) {
		return nil
	}
	return err
}

func (client *RepoManager) Push() error {
	logger.Info("Pushing all in workspace %s", client.workspace)
	wg := sync.WaitGroup{}
	for _, repoConfig := range client.config.Repos {
		wg.Add(1)
		go func(repoConfig *RepoConfig) error {
			logger.Info("Pushing %s", repoConfig.Name)
			repo, err := client.openRepo(repoConfig)
			if err != nil {
				wg.Done()
				return err
			}
			err = client.pushSingleRepo(repo)
			wg.Done()
			return err
		}(repoConfig)
	}
	wg.Wait()
	return nil
}

func (client *RepoManager) Sync() error {
	logger.Info("Syncing all in workspace %s", client.workspace)
	wg := sync.WaitGroup{}
	for _, repoDir := range client.config.Repos {
		wg.Add(1)
		go func(repoConfig *RepoConfig) error {
			logger.Info("Syncing %s", repoConfig)
			repo, err := client.openRepo(repoConfig)
			if err != nil {
				wg.Done()
				return err
			}
			err = client.pullSingleRepo(repo)
			if err != nil {
				wg.Done()
				return err
			}
			err = client.pushSingleRepo(repo)
			if err != nil {
				wg.Done()
				return err
			}
			wg.Done()
			return nil
		}(repoDir)
	}
	wg.Wait()
	return nil
}

func (client *RepoManager) Status() error {
	logger.Info("Statusing all in workspace %s", client.workspace)
	for _, repoConfig := range client.config.Repos {
		logger.Info("Statusing %s", repoConfig.Name)
		repo, err := client.openRepo(repoConfig)
		if err != nil {
			return err
		}
		w, err := repo.Worktree()
		if err != nil {
			return err
		}
		status, err := w.Status()
		if err != nil {
			return err
		}
		fmt.Println(status)
	}
	return nil
}

func (client *RepoManager) Add(repoPath string) error {
	logger.Info("Adding %s to workspace %s", repoPath, client.workspace)
	repoConfig := &RepoConfig{
		Name: filepath.Base(repoPath),
		Dir:  repoPath,
	}
	repo, err := client.openRepo(repoConfig)
	if errors.Is(err, git.ErrRepositoryNotExists) {
		files, err := ioutil.ReadDir(repoPath)
		if err != nil {
			return err
		}
		for _, file := range files {
			if file.IsDir() {
				if err := client.Add(filepath.Join(repoPath, file.Name())); err != nil {
					return err
				}
			}
		}
	} else {
		if _, err := repo.Branch("main"); err == nil {
			repoConfig.Branch = "main"
		} else if _, err := repo.Branch("master"); err == nil {
			repoConfig.Branch = "master"
		}
		client.config.Repos = append(client.config.Repos, repoConfig)
		viper.Set("repos", client.config.Repos)
	}

	return viper.WriteConfig()
}

func (client *RepoManager) Remove(repoPath string) error {
	logger.Info("Removing %s from workspace %s", repoPath, client.workspace)
	for i, repoConfig := range client.config.Repos {
		if repoConfig.Dir == repoPath {
			client.config.Repos = append(client.config.Repos[:i], client.config.Repos[i+1:]...)
			viper.Set("repos", client.config.Repos)
			return viper.WriteConfig()
		}
	}
	return nil
}

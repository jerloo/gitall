package gitall

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/spf13/cobra"
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

type GitAllClient struct {
	workspace string

	auth *ssh.PublicKeys
}

type NewGitAllClientOptions func(*GitAllClient)

func WithVerbose(verbose bool) NewGitAllClientOptions {
	return func(client *GitAllClient) {
		logger.verbose = verbose
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
	sshPath := os.Getenv("HOME") + "/.ssh/id_rsa"
	publicKey, keyError := ssh.NewPublicKeysFromFile(ssh.DefaultUsername, sshPath, "")
	if keyError != nil {
		return nil, keyError
	}
	publicKey.HostKeyCallbackHelper = ssh.HostKeyCallbackHelper{
		HostKeyCallback: cssh.InsecureIgnoreHostKey(),
	}
	return publicKey, nil
}

func NewGitAllClient(workspace string, options ...NewGitAllClientOptions) (*GitAllClient, error) {
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

	client := &GitAllClient{
		workspace: workspace,
		auth:      auth,
	}

	for _, opt := range options {
		opt(client) //opt是个方法，入参是*Client，内部会修改client的值
	}
	return client, nil
}

func (client *GitAllClient) fetchAllDirs() ([]string, error) {
	items, err := os.ReadDir(client.workspace)
	if err != nil {
		return nil, err
	}
	var dirs []string
	for _, item := range items {
		if item.IsDir() {
			dirs = append(dirs, filepath.Join(client.workspace, item.Name()))
		}
	}
	sort.SliceStable(dirs, func(i, j int) bool {
		return dirs[i] < dirs[j]
	})
	return dirs, nil
}

func (client *GitAllClient) openRepo(dir string) (*git.Repository, error) {
	repo, err := git.PlainOpen(dir)
	if err != nil {
		return nil, err
	}
	if !IfRepoIsClean(repo) {
		return nil, fmt.Errorf("%s is not clean", dir)
	}
	return repo, nil
}

func (client *GitAllClient) pullSingleRepo(repo *git.Repository) error {
	w, err := repo.Worktree()
	if err != nil {
		return err
	}

	err = w.Pull(&git.PullOptions{RemoteName: "origin", Auth: client.auth})
	if errors.Is(err, git.NoErrAlreadyUpToDate) {
		// fmt.Println("已是最新")
		return nil
	}
	return err
}

func (client *GitAllClient) Pull() error {
	logger.Info("Pulling all in workspace %s", client.workspace)
	repoDirs, err := client.fetchAllDirs()
	if err != nil {
		return err
	}
	for _, repoDir := range repoDirs {
		logger.Info("Pulling %s", repoDir)
		repo, err := client.openRepo(repoDir)
		if err != nil {
			return err
		}
		err = client.pullSingleRepo(repo)
		if err != nil {
			return err
		}
	}
	return nil
}

func (client *GitAllClient) pushSingleRepo(repo *git.Repository) error {
	err := repo.Push(&git.PushOptions{RemoteName: "origin", Auth: client.auth})
	if errors.Is(err, git.NoErrAlreadyUpToDate) {
		// fmt.Println("已是最新")
		return nil
	}
	return err
}

func (client *GitAllClient) Push() error {
	logger.Info("Pushing all in workspace %s", client.workspace)
	repoDirs, err := client.fetchAllDirs()
	if err != nil {
		return err
	}
	for _, repoDir := range repoDirs {
		logger.Info("Pushing %s", repoDir)
		repo, err := client.openRepo(repoDir)
		if err != nil {
			return err
		}
		err = client.pushSingleRepo(repo)
		if err != nil {
			return err
		}
	}
	return nil
}

func (client *GitAllClient) Sync() error {
	logger.Info("Syncing all workspace %s", client.workspace)
	repoDirs, err := client.fetchAllDirs()
	if err != nil {
		return err
	}
	for _, repoDir := range repoDirs {
		repo, err := client.openRepo(repoDir)
		if err != nil {
			return err
		}
		err = client.pullSingleRepo(repo)
		if err != nil {
			return err
		}
		err = client.pushSingleRepo(repo)
		if err != nil {
			return err
		}
	}
	return nil
}

package gitall

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"sync"

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

type GitAllStatus struct {
	Pull *OperationStatus `json:"pull"`
	Push *OperationStatus `json:"push"`
	Sync *OperationStatus `json:"sync"`

	statusfile string
}

func NewGitAllStatus(workspace string) *GitAllStatus {
	return &GitAllStatus{
		statusfile: filepath.Join(workspace, ".status.json"),
	}
}

type OperationStatus struct {
	DoneDirs   []string `json:"doneDirs"`
	UndoneDirs []string `json:"undoneDirs"`
}

func (opStatus *OperationStatus) IsClear() bool {
	return len(opStatus.UndoneDirs) == 0
}

func (status *GitAllStatus) Save() error {
	if status.IsClear() {
		return nil
	}
	jsonStr, err := json.Marshal(status)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(status.statusfile, jsonStr, 0644)
}

func (status *GitAllStatus) Load() error {
	if status.IsClear() {
		return nil
	}
	bts, err := ioutil.ReadFile(status.statusfile)
	if err != nil {
		return err
	}
	return json.Unmarshal(bts, status)
}

func (status *GitAllStatus) IsClear() bool {
	_, err := os.Stat(status.statusfile)
	if err != nil {
		return true
	}
	if !status.Pull.IsClear() {
		return false
	}
	if !status.Push.IsClear() {
		return false
	}
	if !status.Sync.IsClear() {
		return false
	}
	return true
}

func (status *GitAllStatus) Clear() error {
	return os.Remove(status.statusfile)
}

type GitAllClient struct {
	workspace string
	verbose   bool

	auth *ssh.PublicKeys
}

type NewGitAllClientOptions func(*GitAllClient)

func WithVerbose(verbose bool) NewGitAllClientOptions {
	return func(client *GitAllClient) {
		logger.verbose = verbose
		client.verbose = verbose
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
		opt(client)
	}
	return client, nil
}

type Mode string

const (
	ModePull Mode = "pull"
	ModePush Mode = "push"
	ModeSync Mode = "sync"
)

func (client *GitAllClient) fetchAllDirs(mode Mode) ([]string, error) {
	status := NewGitAllStatus(client.workspace)
	if !status.IsClear() {
		switch mode {
		case ModePull:
			return status.Pull.UndoneDirs, nil
		case ModePush:
			return status.Push.UndoneDirs, nil
		case ModeSync:
			return status.Sync.UndoneDirs, nil
		default:
			return nil, fmt.Errorf("unknown mode: %s", mode)
		}
	}
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

func (client *GitAllClient) progeess() io.Writer {
	if client.verbose {
		return os.Stdout
	}
	return nil
}

func (client *GitAllClient) pullSingleRepo(repo *git.Repository) error {
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

func (client *GitAllClient) Pull() error {
	logger.Info("Pulling all in workspace %s", client.workspace)
	repoDirs, err := client.fetchAllDirs(ModePull)
	if err != nil {
		return err
	}
	wg := sync.WaitGroup{}
	for _, repoDir := range repoDirs {
		wg.Add(1)
		go func(rd string) error {
			logger.Info("Pulling %s", rd)
			repo, err := client.openRepo(rd)
			if err != nil {
				wg.Done()
				return err
			}
			err = client.pullSingleRepo(repo)
			if err != nil {
				wg.Done()
				return err
			}
			wg.Done()
			return nil
		}(repoDir)
	}
	return nil
}

func (client *GitAllClient) pushSingleRepo(repo *git.Repository) error {
	err := repo.Push(&git.PushOptions{RemoteName: "origin", Auth: client.auth, Progress: client.progeess()})
	if errors.Is(err, git.NoErrAlreadyUpToDate) {
		return nil
	}
	return err
}

func (client *GitAllClient) Push() error {
	logger.Info("Pushing all in workspace %s", client.workspace)
	repoDirs, err := client.fetchAllDirs(ModePush)
	if err != nil {
		return err
	}
	wg := sync.WaitGroup{}
	for _, repoDir := range repoDirs {
		wg.Add(1)
		go func(rd string) error {
			logger.Info("Pushing %s", rd)
			repo, err := client.openRepo(rd)
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

func (client *GitAllClient) Sync() error {
	logger.Info("Syncing all in workspace %s", client.workspace)
	repoDirs, err := client.fetchAllDirs(ModeSync)
	if err != nil {
		return err
	}
	wg := sync.WaitGroup{}
	for _, repoDir := range repoDirs {
		wg.Add(1)
		go func(rd string) error {
			logger.Info("Syncing %s", rd)
			repo, err := client.openRepo(rd)
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

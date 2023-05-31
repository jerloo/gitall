package repos

import "path/filepath"

type ReposConfig struct {
	CfgFile string                 `yaml:"-"`
	Version string                 `yaml:"version"`
	Repos   map[string]*RepoConfig `yaml:"repos"`
}

type RepoConfig struct {
	Name   string `yaml:"name"`
	Dir    string `yaml:"dir"`
	Url    string `yaml:"url"`
	Branch string `yaml:"branch"`
}

func (config *RepoConfig) FullDir(workspace string) string {
	return filepath.Join(workspace, config.Dir)
}

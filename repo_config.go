package repos

type ReposConfig struct {
	Version string        `yaml:"version"`
	Repos   []*RepoConfig `yaml:"repos"`
}

type RepoConfig struct {
	Name   string `yaml:"name"`
	Dir    string `yaml:"dir"`
	Url    string `yaml:"url"`
	Branch string `yaml:"branch"`
}

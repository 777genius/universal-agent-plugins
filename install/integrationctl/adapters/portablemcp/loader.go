package portablemcp

import "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"

type Loader struct {
	FS ports.FileSystem
}

type File struct {
	APIVersion string            `yaml:"api_version"`
	Servers    map[string]Server `yaml:"servers"`
}

type Server struct {
	Type    string   `yaml:"type"`
	Targets []string `yaml:"targets"`
	Stdio   *Stdio   `yaml:"stdio,omitempty"`
	Remote  *Remote  `yaml:"remote,omitempty"`
}

type Stdio struct {
	Command string            `yaml:"command"`
	Args    []string          `yaml:"args,omitempty"`
	Env     map[string]string `yaml:"env,omitempty"`
}

type Remote struct {
	Protocol string            `yaml:"protocol,omitempty"`
	URL      string            `yaml:"url"`
	Headers  map[string]string `yaml:"headers,omitempty"`
}

type Loaded struct {
	Path    string
	File    File
	Servers map[string]Server
}

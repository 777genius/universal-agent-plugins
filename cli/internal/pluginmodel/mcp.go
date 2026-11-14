package pluginmodel

import (
	"regexp"
)

const PortableMCPLegacyFormatMarker = "plugin-kit-ai/mcp"

var portableMCPAliasRe = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

type PortableMCPFile struct {
	APIVersion string                       `yaml:"api_version,omitempty" json:"api_version,omitempty"`
	Format     string                       `yaml:"format,omitempty" json:"format,omitempty"`
	Version    int                          `yaml:"version,omitempty" json:"version,omitempty"`
	Servers    map[string]PortableMCPServer `yaml:"servers" json:"servers"`
}

type PortableMCPServer struct {
	Description string                    `yaml:"description,omitempty" json:"description,omitempty"`
	Type        string                    `yaml:"type,omitempty" json:"type,omitempty"`
	Stdio       *PortableMCPStdio         `yaml:"stdio,omitempty" json:"stdio,omitempty"`
	Remote      *PortableMCPRemote        `yaml:"remote,omitempty" json:"remote,omitempty"`
	Targets     []string                  `yaml:"targets,omitempty" json:"targets,omitempty"`
	Overrides   map[string]map[string]any `yaml:"overrides,omitempty" json:"overrides,omitempty"`
	Passthrough map[string]map[string]any `yaml:"passthrough,omitempty" json:"passthrough,omitempty"`
}

type PortableMCPStdio struct {
	Command string            `yaml:"command,omitempty" json:"command,omitempty"`
	Args    []string          `yaml:"args,omitempty" json:"args,omitempty"`
	Env     map[string]string `yaml:"env,omitempty" json:"env,omitempty"`
}

type PortableMCPRemote struct {
	Protocol string            `yaml:"protocol,omitempty" json:"protocol,omitempty"`
	URL      string            `yaml:"url,omitempty" json:"url,omitempty"`
	Headers  map[string]string `yaml:"headers,omitempty" json:"headers,omitempty"`
}

type ParsedPortableMCP struct {
	Servers map[string]any
	File    *PortableMCPFile
}

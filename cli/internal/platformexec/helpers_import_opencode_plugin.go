package platformexec

import (
	"gopkg.in/yaml.v3"
)

type opencodePackageMeta struct {
	Plugins []opencodePluginRef `yaml:"plugins,omitempty"`
}

type opencodePluginRef struct {
	Name    string
	Options map[string]any
}

func (r *opencodePluginRef) UnmarshalYAML(node *yaml.Node) error {
	return unmarshalOpenCodePluginRefYAML(node, r)
}

func (r opencodePluginRef) MarshalYAML() (any, error) {
	return marshalOpenCodePluginRefYAML(r), nil
}

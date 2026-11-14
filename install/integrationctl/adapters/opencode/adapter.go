package opencode

import (
	"context"
	"errors"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
	"gopkg.in/yaml.v3"
)

type Adapter struct {
	FS                   ports.FileSystem
	SafeMutator          ports.SafeFileMutator
	ProjectRoot          string
	UserHome             string
	managedConfigPathsFn func(string) []string
}

type packageMeta struct {
	Plugins []pluginRef `yaml:"plugins,omitempty"`
}

type pluginRef struct {
	Name    string
	Options map[string]any
}

func (r *pluginRef) UnmarshalYAML(node *yaml.Node) error {
	if node == nil {
		*r = pluginRef{}
		return nil
	}
	switch node.Kind {
	case yaml.ScalarNode:
		var name string
		if err := node.Decode(&name); err != nil {
			return err
		}
		r.Name = strings.TrimSpace(name)
		r.Options = nil
		return nil
	case yaml.MappingNode:
		var raw map[string]any
		if err := node.Decode(&raw); err != nil {
			return err
		}
		for key := range raw {
			switch key {
			case "name", "options":
			default:
				return errors.New("unsupported OpenCode plugin metadata field " + key)
			}
		}
		name, _ := raw["name"].(string)
		r.Name = strings.TrimSpace(name)
		if options, ok := raw["options"]; ok {
			typed, ok := options.(map[string]any)
			if !ok {
				return errors.New("OpenCode plugin metadata options must be a mapping")
			}
			r.Options = typed
		}
		return nil
	default:
		return errors.New("OpenCode plugin metadata entries must be strings or mappings")
	}
}

func (r pluginRef) jsonValue() any {
	if len(r.Options) == 0 {
		return r.Name
	}
	return []any{r.Name, r.Options}
}

type sourceMaterial struct {
	WholeFields map[string]any
	Plugins     []pluginRef
	MCP         map[string]any
	CopyFiles   []copyFile
}

type configMutation struct {
	WholeSet      map[string]any
	WholeRemove   []string
	PluginsSet    []pluginRef
	PluginsRemove []string
	MCPSet        map[string]any
	MCPRemove     []string
}

type configPatchResult struct {
	Body            []byte
	ConfigPath      string
	ManagedKeys     []string
	OwnedPluginRefs []string
	OwnedMCPAliases []string
}

type copyFile struct {
	Source      string
	Destination string
}

type inspectSurface struct {
	ConfigPath              string
	SettingsFiles           []string
	ConfigPrecedenceContext []string
	EnvironmentRestrictions []domain.EnvironmentRestrictionCode
	VolatileOverride        bool
	SourceAccessState       string
}

func (Adapter) ID() domain.TargetID { return domain.TargetOpenCode }

func (Adapter) Capabilities(context.Context) (ports.Capabilities, error) {
	return ports.Capabilities{
		InstallMode:          "config_projection",
		SupportsNativeUpdate: false,
		SupportsNativeRemove: true,
		SupportsScopeUser:    true,
		SupportsScopeProject: true,
		SupportsRepair:       true,
		SupportedSourceKinds: []string{"local_path", "github_repo_path", "git_url"},
		EvidenceKey:          "target.opencode.native_surface",
	}, nil
}

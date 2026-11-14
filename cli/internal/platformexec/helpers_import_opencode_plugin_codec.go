package platformexec

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

func unmarshalOpenCodePluginRefYAML(node *yaml.Node, ref *opencodePluginRef) error {
	if node == nil {
		*ref = opencodePluginRef{}
		return nil
	}
	switch node.Kind {
	case yaml.ScalarNode:
		var name string
		if err := node.Decode(&name); err != nil {
			return err
		}
		ref.Name = strings.TrimSpace(name)
		ref.Options = nil
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
				return fmt.Errorf("unsupported OpenCode plugin metadata field %q", key)
			}
		}
		name, _ := raw["name"].(string)
		ref.Name = strings.TrimSpace(name)
		if options, ok := raw["options"]; ok {
			typed, ok := options.(map[string]any)
			if !ok {
				return fmt.Errorf("OpenCode plugin metadata options must be a mapping")
			}
			ref.Options = typed
		} else {
			ref.Options = nil
		}
		return nil
	default:
		return fmt.Errorf("OpenCode plugin metadata entries must be strings or mappings")
	}
}

func marshalOpenCodePluginRefYAML(ref opencodePluginRef) any {
	if len(ref.Options) == 0 {
		return strings.TrimSpace(ref.Name)
	}
	return map[string]any{
		"name":    strings.TrimSpace(ref.Name),
		"options": ref.Options,
	}
}

func isOpenCodePermissionValue(value any) bool {
	if _, ok := value.(string); ok {
		return true
	}
	_, ok := value.(map[string]any)
	return ok
}

package platformexec

import (
	"fmt"
	"strings"
)

func (r opencodePluginRef) jsonValue() any {
	name := strings.TrimSpace(r.Name)
	if len(r.Options) == 0 {
		return name
	}
	return []any{name, r.Options}
}

func normalizeImportedOpenCodePluginRef(value any) (opencodePluginRef, error) {
	switch typed := value.(type) {
	case string:
		text := strings.TrimSpace(typed)
		if text == "" {
			return opencodePluginRef{}, fmt.Errorf("plugin ref must be a non-empty string")
		}
		return opencodePluginRef{Name: text}, nil
	case []any:
		if len(typed) != 2 {
			return opencodePluginRef{}, fmt.Errorf("tuple plugin ref must have exactly 2 items")
		}
		name, ok := typed[0].(string)
		if !ok || strings.TrimSpace(name) == "" {
			return opencodePluginRef{}, fmt.Errorf("tuple plugin ref name must be a non-empty string")
		}
		options, ok := typed[1].(map[string]any)
		if !ok {
			return opencodePluginRef{}, fmt.Errorf("tuple plugin ref options must be an object")
		}
		return opencodePluginRef{Name: strings.TrimSpace(name), Options: options}, nil
	default:
		return opencodePluginRef{}, fmt.Errorf("plugin ref must be a string or [name, options] tuple")
	}
}

func jsonValuesForOpenCodePlugins(refs []opencodePluginRef) []any {
	out := make([]any, 0, len(refs))
	for _, ref := range refs {
		out = append(out, ref.jsonValue())
	}
	return out
}

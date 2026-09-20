package platformexec

import "fmt"

func mergeOpenCodeLegacyModeField(raw map[string]any, out *importedOpenCodeConfig) error {
	deprecatedMode, ok := raw["mode"]
	if !ok {
		return nil
	}
	values, ok := deprecatedMode.(map[string]any)
	if !ok {
		return fmt.Errorf("OpenCode config field %q must be a JSON object", "mode")
	}
	if !out.AgentsProvided {
		out.Agents = map[string]any{}
		out.AgentsProvided = true
	}
	for name, value := range values {
		if _, exists := out.Agents[name]; exists {
			continue
		}
		out.Agents[name] = value
	}
	return nil
}

func extractOpenCodeExtra(raw map[string]any) map[string]any {
	for _, key := range []string{"$schema", "plugin", "mcp", "command", "agent", "default_agent", "instructions", "permission", "mode"} {
		delete(raw, key)
	}
	if len(raw) == 0 {
		return nil
	}
	return raw
}

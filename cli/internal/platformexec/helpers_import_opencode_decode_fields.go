package platformexec

import (
	"fmt"
	"strings"
)

func decodeOpenCodePluginField(raw map[string]any, out *importedOpenCodeConfig) error {
	pluginsRaw, ok := raw["plugin"]
	if !ok {
		return nil
	}
	out.PluginsProvided = true
	values, ok := pluginsRaw.([]any)
	if !ok {
		return fmt.Errorf("OpenCode config field %q must be an array of strings or [name, options] tuples", "plugin")
	}
	out.Plugins = make([]opencodePluginRef, 0, len(values))
	for i, value := range values {
		ref, err := normalizeImportedOpenCodePluginRef(value)
		if err != nil {
			return fmt.Errorf("OpenCode config field %q has invalid entry at index %d: %w", "plugin", i, err)
		}
		out.Plugins = append(out.Plugins, ref)
	}
	return nil
}

func decodeOpenCodeObjectField(raw map[string]any, key string, provided *bool, dst *map[string]any) error {
	value, ok := raw[key]
	if !ok {
		return nil
	}
	*provided = true
	typed, ok := value.(map[string]any)
	if !ok {
		return fmt.Errorf("OpenCode config field %q must be a JSON object", key)
	}
	*dst = typed
	return nil
}

func decodeOpenCodeDefaultAgentField(raw map[string]any, out *importedOpenCodeConfig) error {
	defaultAgent, ok := raw["default_agent"]
	if !ok {
		return nil
	}
	text, ok := defaultAgent.(string)
	if !ok || strings.TrimSpace(text) == "" {
		return fmt.Errorf("OpenCode config field %q must be a non-empty string", "default_agent")
	}
	out.DefaultAgent = strings.TrimSpace(text)
	out.DefaultAgentSet = true
	return nil
}

func decodeOpenCodeInstructionsField(raw map[string]any, out *importedOpenCodeConfig) error {
	instructions, ok := raw["instructions"]
	if !ok {
		return nil
	}
	values, ok := instructions.([]any)
	if !ok {
		return fmt.Errorf("OpenCode config field %q must be an array of strings", "instructions")
	}
	out.Instructions = make([]string, 0, len(values))
	for i, value := range values {
		text, ok := value.(string)
		if !ok || strings.TrimSpace(text) == "" {
			return fmt.Errorf("OpenCode config field %q must contain non-empty strings (invalid entry at index %d)", "instructions", i)
		}
		out.Instructions = append(out.Instructions, strings.TrimSpace(text))
	}
	out.InstructionsSet = true
	return nil
}

func decodeOpenCodePermissionField(raw map[string]any, out *importedOpenCodeConfig) error {
	permission, ok := raw["permission"]
	if !ok {
		return nil
	}
	if !isOpenCodePermissionValue(permission) {
		return fmt.Errorf("OpenCode config field %q must be a string or JSON object", "permission")
	}
	out.Permission = permission
	out.PermissionSet = true
	return nil
}

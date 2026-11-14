package platformexec

import "strings"

func normalizeInlineOpenCodeAgent(name string, spec map[string]any) (map[string]any, string, bool) {
	description, ok := spec["description"].(string)
	if !ok || strings.TrimSpace(description) == "" {
		return nil, "", false
	}
	_, _ = name, spec
	for key := range spec {
		switch key {
		case "description", "mode", "model", "variant", "temperature", "top_p", "tools", "permission", "disable", "hidden", "options", "color", "steps", "maxSteps", "prompt":
		default:
			return nil, "", false
		}
	}
	frontmatter := map[string]any{
		"description": strings.TrimSpace(description),
	}
	if mode, ok := spec["mode"]; ok {
		text, ok := mode.(string)
		if !ok || strings.TrimSpace(text) == "" {
			return nil, "", false
		}
		frontmatter["mode"] = strings.TrimSpace(text)
	}
	if model, ok := spec["model"]; ok {
		text, ok := model.(string)
		if !ok || strings.TrimSpace(text) == "" {
			return nil, "", false
		}
		frontmatter["model"] = strings.TrimSpace(text)
	}
	if variant, ok := spec["variant"]; ok {
		text, ok := variant.(string)
		if !ok || strings.TrimSpace(text) == "" {
			return nil, "", false
		}
		frontmatter["variant"] = strings.TrimSpace(text)
	}
	if temperature, ok := spec["temperature"]; ok {
		switch value := temperature.(type) {
		case float64:
			frontmatter["temperature"] = value
		default:
			return nil, "", false
		}
	}
	if topP, ok := spec["top_p"]; ok {
		switch value := topP.(type) {
		case float64:
			frontmatter["top_p"] = value
		default:
			return nil, "", false
		}
	}
	if tools, ok := spec["tools"]; ok {
		toolMap, ok := tools.(map[string]any)
		if !ok {
			return nil, "", false
		}
		normalizedTools := map[string]any{}
		for key, value := range toolMap {
			flag, ok := value.(bool)
			if !ok || strings.TrimSpace(key) == "" {
				return nil, "", false
			}
			normalizedTools[key] = flag
		}
		if _, exists := frontmatter["permission"]; !exists && len(normalizedTools) > 0 {
			frontmatter["permission"] = map[string]any{"tools": normalizedTools}
		}
	}
	if permission, ok := spec["permission"]; ok {
		if !isOpenCodePermissionValue(permission) {
			return nil, "", false
		}
		frontmatter["permission"] = permission
	}
	if disable, ok := spec["disable"]; ok {
		flag, ok := disable.(bool)
		if !ok {
			return nil, "", false
		}
		frontmatter["disable"] = flag
	}
	if hidden, ok := spec["hidden"]; ok {
		flag, ok := hidden.(bool)
		if !ok {
			return nil, "", false
		}
		frontmatter["hidden"] = flag
	}
	if options, ok := spec["options"]; ok {
		typed, ok := options.(map[string]any)
		if !ok {
			return nil, "", false
		}
		frontmatter["options"] = typed
	}
	if color, ok := spec["color"]; ok {
		text, ok := color.(string)
		if !ok || strings.TrimSpace(text) == "" {
			return nil, "", false
		}
		frontmatter["color"] = strings.TrimSpace(text)
	}
	if steps, ok := spec["steps"]; ok {
		value, ok := steps.(float64)
		if !ok || value != float64(int(value)) {
			return nil, "", false
		}
		frontmatter["steps"] = int(value)
	}
	if maxSteps, ok := spec["maxSteps"]; ok {
		if _, exists := frontmatter["steps"]; !exists {
			value, ok := maxSteps.(float64)
			if !ok || value != float64(int(value)) {
				return nil, "", false
			}
			frontmatter["steps"] = int(value)
		}
	}
	body := ""
	if prompt, ok := spec["prompt"]; ok {
		text, ok := prompt.(string)
		if !ok {
			return nil, "", false
		}
		if strings.Contains(text, "{file:") {
			return nil, "", false
		}
		body = strings.TrimSpace(text)
	}
	return frontmatter, body, true
}

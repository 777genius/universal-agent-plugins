package opencode

import (
	"encoding/json"
	"errors"
	"sort"
	"strings"
)

func existingObjectMap(raw any, field string) (map[string]any, error) {
	if raw == nil {
		return map[string]any{}, nil
	}
	typed, ok := raw.(map[string]any)
	if !ok {
		return nil, errors.New("OpenCode field " + field + " must be an object")
	}
	return typed, nil
}

func existingPluginRefs(raw any) (map[string]pluginRef, error) {
	if raw == nil {
		return map[string]pluginRef{}, nil
	}
	values, ok := raw.([]any)
	if !ok {
		return nil, errors.New("OpenCode field plugin must be an array")
	}
	out := make(map[string]pluginRef, len(values))
	for i, value := range values {
		ref, err := normalizePluginRef(value)
		if err != nil {
			return nil, errors.New("OpenCode plugin ref at index " + strconvI(i) + " is invalid: " + err.Error())
		}
		out[ref.Name] = ref
	}
	return out, nil
}

func normalizePluginRef(value any) (pluginRef, error) {
	switch typed := value.(type) {
	case string:
		name := strings.TrimSpace(typed)
		if name == "" {
			return pluginRef{}, errors.New("plugin ref must be a non-empty string")
		}
		return pluginRef{Name: name}, nil
	case []any:
		if len(typed) != 2 {
			return pluginRef{}, errors.New("tuple plugin ref must have exactly 2 items")
		}
		name, ok := typed[0].(string)
		if !ok || strings.TrimSpace(name) == "" {
			return pluginRef{}, errors.New("tuple plugin ref name must be a non-empty string")
		}
		options, ok := typed[1].(map[string]any)
		if !ok {
			return pluginRef{}, errors.New("tuple plugin ref options must be an object")
		}
		return pluginRef{Name: strings.TrimSpace(name), Options: options}, nil
	default:
		return pluginRef{}, errors.New("plugin ref must be a string or [name, options] tuple")
	}
}

func mergePluginRefs(existing map[string]pluginRef, remove []string, set []pluginRef) []pluginRef {
	removeSet := mapFromSlice(remove, func(value string) string { return value })
	setMap := make(map[string]pluginRef, len(set))
	for _, ref := range set {
		if strings.TrimSpace(ref.Name) == "" {
			continue
		}
		setMap[ref.Name] = ref
	}
	for name := range setMap {
		removeSet[name] = true
	}
	out := make([]pluginRef, 0, len(existing)+len(setMap))
	for name, ref := range existing {
		if !removeSet[name] {
			out = append(out, ref)
		}
	}
	for _, ref := range setMap {
		out = append(out, ref)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func mergeNamedObject(existing map[string]any, remove []string, set map[string]any) map[string]any {
	out := make(map[string]any, len(existing)+len(set))
	removeSet := mapFromSlice(remove, func(value string) string { return value })
	for key, value := range existing {
		if !removeSet[key] {
			out[key] = value
		}
	}
	for key, value := range set {
		out[key] = value
	}
	return out
}

func pluginRefsToJSON(refs []pluginRef) []any {
	out := make([]any, 0, len(refs))
	for _, ref := range refs {
		out = append(out, ref.jsonValue())
	}
	return out
}

func pluginRefNames(refs []pluginRef) []string {
	out := make([]string, 0, len(refs))
	for _, ref := range refs {
		if strings.TrimSpace(ref.Name) != "" {
			out = append(out, ref.Name)
		}
	}
	sort.Strings(out)
	return out
}

func pluginRefsEqual(left, right pluginRef) bool {
	if strings.TrimSpace(left.Name) != strings.TrimSpace(right.Name) {
		return false
	}
	return jsonValuesEqual(left.Options, right.Options)
}

func jsonValuesEqual(left, right any) bool {
	leftBody, err := json.Marshal(left)
	if err != nil {
		return false
	}
	rightBody, err := json.Marshal(right)
	if err != nil {
		return false
	}
	return string(leftBody) == string(rightBody)
}

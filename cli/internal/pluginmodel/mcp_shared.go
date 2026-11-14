package pluginmodel

import (
	"fmt"
	"strings"
)

func stringMapToAny(values map[string]string) map[string]any {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]any, len(values))
	for key, value := range values {
		out[key] = value
	}
	return out
}

func cloneAnyMap(values map[string]any) map[string]any {
	if len(values) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(values))
	for key, value := range values {
		out[key] = cloneAnyValue(value)
	}
	return out
}

func cloneAnyValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneAnyMap(typed)
	case []any:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, cloneAnyValue(item))
		}
		return out
	default:
		return typed
	}
}

func optionalAnyString(value any) string {
	text, _ := value.(string)
	return strings.TrimSpace(text)
}

func anyToStringMap(value any) map[string]string {
	switch typed := value.(type) {
	case map[string]string:
		return normalizeStringMap(typed)
	case map[string]any:
		out := map[string]string{}
		for key, raw := range typed {
			text := optionalAnyString(raw)
			if strings.TrimSpace(key) == "" || text == "" {
				continue
			}
			out[key] = text
		}
		return normalizeStringMap(out)
	default:
		return nil
	}
}

func anyStringSlice(value any) ([]string, error) {
	switch typed := value.(type) {
	case []string:
		return normalizeStringSlice(typed), nil
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			text := optionalAnyString(item)
			if text == "" {
				return nil, fmt.Errorf("string array contains an empty or non-string item")
			}
			out = append(out, text)
		}
		return normalizeStringSlice(out), nil
	default:
		return nil, fmt.Errorf("expected string array")
	}
}

func mustStringSlice(value any) []string {
	out, err := anyStringSlice(value)
	if err != nil {
		return nil
	}
	return out
}

func filepathExt(path string) string {
	idx := strings.LastIndex(path, ".")
	if idx < 0 {
		return ""
	}
	return path[idx:]
}

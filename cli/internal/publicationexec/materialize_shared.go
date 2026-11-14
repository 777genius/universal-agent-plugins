package publicationexec

import (
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
)

func decodePluginEntries(value any) ([]map[string]any, error) {
	if value == nil {
		return nil, nil
	}
	raw, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("marketplace artifact plugins field must be an array")
	}
	out := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		entry, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("marketplace artifact plugin entries must be objects")
		}
		out = append(out, entry)
	}
	return out, nil
}

func encodePluginEntries(items []map[string]any) []any {
	out := make([]any, 0, len(items))
	for _, item := range items {
		out = append(out, item)
	}
	return out
}

func stringValue(v any) string {
	s, _ := v.(string)
	return s
}

func normalizeMaterializedPackageRoot(path string) string {
	path = filepath.ToSlash(filepath.Clean(strings.TrimSpace(path)))
	if path == "." || path == "" {
		return ""
	}
	return path
}

func jsonDocumentsEqual(left, right any) bool {
	return reflect.DeepEqual(normalizeJSONValue(left), normalizeJSONValue(right))
}

func normalizeJSONValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, child := range typed {
			out[key] = normalizeJSONValue(child)
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for i, child := range typed {
			out[i] = normalizeJSONValue(child)
		}
		return out
	default:
		return typed
	}
}

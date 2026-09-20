package platformexec

import (
	"fmt"
	"strings"
)

func decodeClaudePathField(value any) ([]string, map[string]any, bool, string) {
	switch typed := value.(type) {
	case string:
		text := strings.TrimSpace(typed)
		if text == "" {
			return nil, nil, true, ""
		}
		return []string{text}, nil, true, ""
	case []any:
		refs := jsonStringArray(typed)
		if len(refs) == len(typed) {
			return refs, nil, true, ""
		}
		return nil, nil, false, "uses an unsupported mixed array shape"
	case map[string]any:
		return nil, typed, true, ""
	default:
		return nil, nil, false, "uses an unsupported value shape"
	}
}

func decodeClaudeUserConfig(value any) (map[string]any, bool) {
	switch typed := value.(type) {
	case map[string]any:
		return typed, true
	default:
		return nil, false
	}
}

func consumeClaudePathField(raw map[string]any, field string, override *bool, refs *[]string, inline *map[string]any, warnings *[]string) {
	value, ok := raw[field]
	if !ok {
		return
	}
	*override = true
	decodedRefs, decodedInline, handled, warning := decodeClaudePathField(value)
	if handled {
		*refs = decodedRefs
		if inline != nil {
			*inline = decodedInline
		}
	} else if warning != "" {
		*warnings = append(*warnings, fmt.Sprintf("Claude manifest field %q %s; skipped during import normalization", field, warning))
	}
	delete(raw, field)
}

func consumeClaudeObjectField(raw map[string]any, field string, provided *bool, target *map[string]any, warnings *[]string) {
	value, ok := raw[field]
	if !ok {
		return
	}
	if decoded, ok := decodeClaudeUserConfig(value); ok {
		*provided = true
		*target = decoded
	} else {
		*warnings = append(*warnings, fmt.Sprintf("Claude manifest field %q must be a JSON object for package-standard normalization; skipped during import normalization", field))
	}
	delete(raw, field)
}

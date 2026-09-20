package geminimanifest

import (
	"fmt"
	"strings"
)

func optionalNonEmptyStringField(raw map[string]any, key string) (string, error) {
	value, ok := raw[key]
	if !ok || value == nil {
		return "", nil
	}
	text, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("Gemini extension field %q must be a string", key)
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return "", nil
	}
	return text, nil
}

func stringArrayField(value any, key string) ([]string, error) {
	raw, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("Gemini extension field %q must be an array of strings", key)
	}
	out := make([]string, 0, len(raw))
	for i, item := range raw {
		text, ok := item.(string)
		if !ok || strings.TrimSpace(text) == "" {
			return nil, fmt.Errorf("Gemini extension field %q must contain non-empty strings (invalid entry at index %d)", key, i)
		}
		out = append(out, strings.TrimSpace(text))
	}
	return out, nil
}

func objectArrayField(value any, key string) ([]any, error) {
	raw, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("Gemini extension field %q must be an array of JSON objects", key)
	}
	out := make([]any, 0, len(raw))
	for i, item := range raw {
		if _, ok := item.(map[string]any); !ok {
			return nil, fmt.Errorf("Gemini extension field %q must contain JSON objects (invalid entry at index %d)", key, i)
		}
		out = append(out, item)
	}
	return out, nil
}

func stringsTrimSpace(value string) string {
	return strings.TrimSpace(value)
}

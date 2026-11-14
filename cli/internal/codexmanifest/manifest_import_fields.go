package codexmanifest

import (
	"fmt"
	"strings"
)

func decodeAuthorField(raw map[string]any) (*Author, bool, error) {
	value, ok := raw["author"]
	if !ok || value == nil {
		return nil, false, nil
	}
	typed, ok := value.(map[string]any)
	if !ok {
		return nil, false, fmt.Errorf("Codex plugin author must be a JSON object")
	}
	author := &Author{}
	if item, ok, err := decodeJSONStringMapField(typed, "name"); err != nil {
		return nil, false, fmt.Errorf("Codex plugin author.name must be a string")
	} else if ok {
		author.Name = item
	}
	if item, ok, err := decodeJSONStringMapField(typed, "email"); err != nil {
		return nil, false, fmt.Errorf("Codex plugin author.email must be a string")
	} else if ok {
		author.Email = item
	}
	if item, ok, err := decodeJSONStringMapField(typed, "url"); err != nil {
		return nil, false, fmt.Errorf("Codex plugin author.url must be a string")
	} else if ok {
		author.URL = item
	}
	author.Normalize()
	if author.Empty() {
		return nil, false, nil
	}
	return author, true, nil
}

func decodeJSONStringField(raw map[string]any, field string) (string, bool, error) {
	value, ok := raw[field]
	if !ok || value == nil {
		return "", false, nil
	}
	typed, ok := value.(string)
	if !ok {
		return "", false, fmt.Errorf("Codex plugin %s must be a string", field)
	}
	return strings.TrimSpace(typed), true, nil
}

func decodeJSONStringArrayField(raw map[string]any, field string) ([]string, bool, error) {
	value, ok := raw[field]
	if !ok || value == nil {
		return nil, false, nil
	}
	items, ok := value.([]any)
	if !ok {
		return nil, false, fmt.Errorf("Codex plugin %s must be an array of strings", field)
	}
	out := make([]string, 0, len(items))
	for i, item := range items {
		text, ok := item.(string)
		if !ok {
			return nil, false, fmt.Errorf("Codex plugin %s[%d] must be a string", field, i)
		}
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}
		out = append(out, text)
	}
	return out, true, nil
}

func decodeJSONObjectField(raw map[string]any, field string) (map[string]any, bool, error) {
	value, ok := raw[field]
	if !ok || value == nil {
		return nil, false, nil
	}
	doc, ok := value.(map[string]any)
	if !ok {
		return nil, false, fmt.Errorf("Codex plugin %s must be a JSON object", field)
	}
	return doc, true, nil
}

func decodeJSONStringMapField(raw map[string]any, field string) (string, bool, error) {
	value, ok := raw[field]
	if !ok || value == nil {
		return "", false, nil
	}
	typed, ok := value.(string)
	if !ok {
		return "", false, fmt.Errorf("%s must be a string", field)
	}
	return strings.TrimSpace(typed), true, nil
}

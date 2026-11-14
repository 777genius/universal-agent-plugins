package codexmanifest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

func ValidateInterfaceDoc(doc map[string]any) error {
	if doc == nil {
		return nil
	}
	value, ok := doc["defaultPrompt"]
	if !ok {
		return nil
	}
	items, ok := value.([]any)
	if !ok {
		return fmt.Errorf("interface.defaultPrompt must be an array of strings")
	}
	for i, item := range items {
		text, ok := item.(string)
		if !ok {
			return fmt.Errorf("interface.defaultPrompt[%d] must be a string", i)
		}
		if strings.TrimSpace(text) == "" {
			return fmt.Errorf("interface.defaultPrompt[%d] must not be empty", i)
		}
	}
	return nil
}

func parseJSONObjectDoc(body []byte, label string) (map[string]any, error) {
	dec := json.NewDecoder(bytes.NewReader(body))
	var raw any
	if err := dec.Decode(&raw); err != nil {
		return nil, fmt.Errorf("%s must be valid JSON: %w", label, err)
	}
	var trailing any
	if err := dec.Decode(&trailing); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("%s must contain a single JSON object", label)
		}
		return nil, fmt.Errorf("%s must be valid JSON: %w", label, err)
	}
	doc, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s must be a JSON object", label)
	}
	if doc == nil {
		doc = map[string]any{}
	}
	return doc, nil
}

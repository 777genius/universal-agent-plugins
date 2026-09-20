package platformexec

import (
	"fmt"
	"path/filepath"
	"strings"
)

func validateGeminiThemeMap(rel string, raw map[string]any) string {
	name, _ := raw["name"].(string)
	if strings.TrimSpace(name) == "" {
		return "Gemini themes require name"
	}
	if len(raw) <= 1 {
		return "Gemini themes require at least one theme token besides name"
	}
	for key, value := range raw {
		key = strings.TrimSpace(key)
		if key == "" || key == "name" || key == "type" {
			continue
		}
		switch {
		case geminiThemeRequiresObject(key):
			if _, ok := value.(map[string]any); !ok {
				return fmt.Sprintf("Gemini theme key %q must be a YAML object", key)
			}
			if message := validateGeminiThemeValue(filepath.ToSlash(filepath.Join(rel, key)), value); message != "" {
				return message
			}
		case geminiThemeRequiresStringArray(key):
			if _, ok := geminiStringSlice(value); !ok {
				return fmt.Sprintf("Gemini theme key %q must be an array of non-empty strings", key)
			}
		default:
			if message := validateGeminiThemeValue(filepath.ToSlash(filepath.Join(rel, key)), value); message != "" {
				return message
			}
		}
	}
	return ""
}

func validateGeminiThemeValue(path string, value any) string {
	switch typed := value.(type) {
	case string:
		if strings.TrimSpace(typed) == "" {
			return fmt.Sprintf("Gemini theme token %q must be a non-empty string", path)
		}
		return ""
	case []any:
		if _, ok := geminiStringSlice(typed); !ok {
			return fmt.Sprintf("Gemini theme token %q must be an array of non-empty strings", path)
		}
		return ""
	case map[string]any:
		if len(typed) == 0 {
			return fmt.Sprintf("Gemini theme object %q may not be empty", path)
		}
		for childKey, childValue := range typed {
			childKey = strings.TrimSpace(childKey)
			if childKey == "" {
				continue
			}
			if geminiThemeRequiresStringArray(childKey) {
				if _, ok := geminiStringSlice(childValue); !ok {
					return fmt.Sprintf("Gemini theme key %q must be an array of non-empty strings", filepath.ToSlash(filepath.Join(path, childKey)))
				}
				continue
			}
			if message := validateGeminiThemeValue(filepath.ToSlash(filepath.Join(path, childKey)), childValue); message != "" {
				return message
			}
		}
		return ""
	default:
		return fmt.Sprintf("Gemini theme token %q must be a non-empty string, string array, or object", path)
	}
}

func geminiThemeRequiresObject(key string) bool {
	_, ok := geminiThemeObjectKeys[key]
	return ok
}

func geminiThemeRequiresStringArray(key string) bool {
	_, ok := geminiThemeStringArrayKeys[key]
	return ok
}

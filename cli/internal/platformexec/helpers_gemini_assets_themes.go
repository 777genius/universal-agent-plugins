package platformexec

import (
	"fmt"
	"strings"
)

func loadGeminiThemes(root string, rels []string) ([]map[string]any, error) {
	if len(rels) == 0 {
		return nil, nil
	}
	seenNames := map[string]string{}
	var themes []map[string]any
	for _, rel := range rels {
		raw, err := loadGeminiThemeRaw(root, rel)
		if err != nil {
			return nil, err
		}
		if err := validateUniqueGeminiTheme(seenNames, rel, raw); err != nil {
			return nil, err
		}
		themes = append(themes, normalizeGeminiThemeMap(raw))
	}
	return themes, nil
}

func loadGeminiThemeRaw(root, rel string) (map[string]any, error) {
	_, raw, err := readGeminiYAMLMap(root, rel)
	if err != nil {
		return nil, err
	}
	if raw == nil {
		raw = map[string]any{}
	}
	if message := validateGeminiThemeMap(rel, raw); message != "" {
		return nil, fmt.Errorf("invalid %s: %s", rel, message)
	}
	return raw, nil
}

func validateUniqueGeminiTheme(seenNames map[string]string, rel string, raw map[string]any) error {
	name, _ := raw["name"].(string)
	name = strings.TrimSpace(name)
	nameKey := strings.ToLower(name)
	if prev, ok := seenNames[nameKey]; ok {
		return fmt.Errorf("invalid %s: Gemini theme name %q duplicates %s", rel, name, prev)
	}
	seenNames[nameKey] = rel
	return nil
}

func normalizeGeminiThemeMap(raw map[string]any) map[string]any {
	theme := map[string]any{}
	for key, value := range raw {
		switch strings.TrimSpace(key) {
		case "":
			continue
		case "name":
			theme["name"] = value
		default:
			theme[key] = value
		}
	}
	return theme
}

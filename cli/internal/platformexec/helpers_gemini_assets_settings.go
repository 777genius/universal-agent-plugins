package platformexec

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

func loadGeminiSettings(root string, rels []string) ([]map[string]any, error) {
	if len(rels) == 0 {
		return nil, nil
	}
	seenNames := map[string]string{}
	seenEnvVars := map[string]string{}
	var settings []map[string]any
	for _, rel := range rels {
		setting, err := loadGeminiSetting(root, rel)
		if err != nil {
			return nil, err
		}
		if err := validateUniqueGeminiSetting(seenNames, seenEnvVars, rel, setting); err != nil {
			return nil, err
		}
		settings = append(settings, map[string]any{
			"name":        setting.Name,
			"description": setting.Description,
			"envVar":      setting.EnvVar,
			"sensitive":   setting.Sensitive,
		})
	}
	return settings, nil
}

func loadGeminiSetting(root, rel string) (geminiSetting, error) {
	body, raw, err := readGeminiYAMLMap(root, rel)
	if err != nil {
		return geminiSetting{}, err
	}
	var setting geminiSetting
	if err := yaml.Unmarshal(body, &setting); err != nil {
		return geminiSetting{}, fmt.Errorf("parse %s: %w", rel, err)
	}
	if message := validateGeminiSettingMap(raw, setting); message != "" {
		return geminiSetting{}, fmt.Errorf("invalid %s: %s", rel, message)
	}
	return setting, nil
}

func validateUniqueGeminiSetting(seenNames, seenEnvVars map[string]string, rel string, setting geminiSetting) error {
	nameKey := strings.ToLower(strings.TrimSpace(setting.Name))
	if prev, ok := seenNames[nameKey]; ok {
		return fmt.Errorf("invalid %s: Gemini setting name %q duplicates %s", rel, setting.Name, prev)
	}
	seenNames[nameKey] = rel
	envKey := strings.ToLower(strings.TrimSpace(setting.EnvVar))
	if prev, ok := seenEnvVars[envKey]; ok {
		return fmt.Errorf("invalid %s: Gemini setting env_var %q duplicates %s", rel, setting.EnvVar, prev)
	}
	seenEnvVars[envKey] = rel
	return nil
}

func validateGeminiSettingMap(raw map[string]any, setting geminiSetting) string {
	_, hasSensitive := raw["sensitive"]
	_, sensitiveIsBool := raw["sensitive"].(bool)
	if strings.TrimSpace(setting.Name) == "" || strings.TrimSpace(setting.Description) == "" || strings.TrimSpace(setting.EnvVar) == "" || !hasSensitive || !sensitiveIsBool {
		return "Gemini settings require string name, description, env_var, and boolean sensitive"
	}
	if !geminiSettingEnvVarRe.MatchString(strings.TrimSpace(setting.EnvVar)) {
		return fmt.Sprintf("Gemini settings require env_var %q to be a valid environment variable name", setting.EnvVar)
	}
	return ""
}

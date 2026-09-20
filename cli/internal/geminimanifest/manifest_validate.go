package geminimanifest

import (
	"fmt"
	"regexp"
	"strings"
)

var importedSettingEnvVarRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func validateImportedSettingObject(doc map[string]any) error {
	name, _ := doc["name"].(string)
	description, _ := doc["description"].(string)
	envVar, _ := doc["envVar"].(string)
	_, hasSensitive := doc["sensitive"]
	if strings.TrimSpace(name) == "" || strings.TrimSpace(description) == "" || strings.TrimSpace(envVar) == "" || !hasSensitive {
		return fmt.Errorf("settings objects require non-empty string name, description, envVar, and boolean sensitive")
	}
	if _, ok := doc["sensitive"].(bool); !ok {
		return fmt.Errorf("settings objects require non-empty string name, description, envVar, and boolean sensitive")
	}
	if !importedSettingEnvVarRe.MatchString(strings.TrimSpace(envVar)) {
		return fmt.Errorf("settings envVar %q must be a valid environment variable name", envVar)
	}
	return nil
}

func validateImportedThemeObject(doc map[string]any) error {
	name, _ := doc["name"].(string)
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("themes objects require non-empty string name")
	}
	if len(doc) <= 1 {
		return fmt.Errorf("themes objects require at least one theme token besides name")
	}
	return nil
}

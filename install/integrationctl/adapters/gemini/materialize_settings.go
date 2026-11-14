package gemini

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"gopkg.in/yaml.v3"
)

func loadSettings(root string) ([]map[string]any, error) {
	rels, err := discoverFiles(root)
	if err != nil {
		return nil, domain.NewError(domain.ErrMutationApply, "discover Gemini settings", err)
	}
	if len(rels) == 0 {
		return nil, nil
	}
	seenNames := map[string]string{}
	seenEnv := map[string]string{}
	out := make([]map[string]any, 0, len(rels))
	for _, rel := range rels {
		if !isYAMLPath(rel) {
			continue
		}
		setting, err := loadSettingDoc(filepath.Join(root, rel))
		if err != nil {
			return nil, err
		}
		if err := validateUniqueSetting(setting, rel, seenNames, seenEnv); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{
			"name":        setting.Name,
			"description": setting.Description,
			"envVar":      setting.EnvVar,
			"sensitive":   setting.Sensitive,
		})
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

func loadSettingDoc(path string) (settingDoc, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return settingDoc{}, domain.NewError(domain.ErrMutationApply, "read Gemini setting", err)
	}
	var doc settingDoc
	if err := yaml.Unmarshal(body, &doc); err != nil {
		return settingDoc{}, domain.NewError(domain.ErrMutationApply, "parse Gemini setting", err)
	}
	doc.Name = strings.TrimSpace(doc.Name)
	doc.Description = strings.TrimSpace(doc.Description)
	doc.EnvVar = strings.TrimSpace(doc.EnvVar)
	if doc.Name == "" || doc.Description == "" || doc.EnvVar == "" {
		return settingDoc{}, domain.NewError(domain.ErrMutationApply, "Gemini settings require non-empty name, description, and env_var", nil)
	}
	return doc, nil
}

func validateUniqueSetting(doc settingDoc, rel string, seenNames, seenEnv map[string]string) error {
	nameKey := strings.ToLower(doc.Name)
	if prev, ok := seenNames[nameKey]; ok {
		return domain.NewError(domain.ErrMutationApply, fmt.Sprintf("Gemini setting name %q duplicates %s", doc.Name, prev), nil)
	}
	envKey := strings.ToLower(doc.EnvVar)
	if prev, ok := seenEnv[envKey]; ok {
		return domain.NewError(domain.ErrMutationApply, fmt.Sprintf("Gemini setting env_var %q duplicates %s", doc.EnvVar, prev), nil)
	}
	seenNames[nameKey] = rel
	seenEnv[envKey] = rel
	return nil
}

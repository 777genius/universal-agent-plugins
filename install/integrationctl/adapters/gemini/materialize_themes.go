package gemini

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"gopkg.in/yaml.v3"
)

func loadThemes(root string) ([]map[string]any, error) {
	rels, err := discoverFiles(root)
	if err != nil {
		return nil, domain.NewError(domain.ErrMutationApply, "discover Gemini themes", err)
	}
	if len(rels) == 0 {
		return nil, nil
	}
	seenNames := map[string]string{}
	out := make([]map[string]any, 0, len(rels))
	for _, rel := range rels {
		if !isYAMLPath(rel) {
			continue
		}
		doc, name, err := loadThemeDoc(filepath.Join(root, rel))
		if err != nil {
			return nil, err
		}
		if prev, ok := seenNames[strings.ToLower(name)]; ok {
			return nil, domain.NewError(domain.ErrMutationApply, fmt.Sprintf("Gemini theme name %q duplicates %s", name, prev), nil)
		}
		seenNames[strings.ToLower(name)] = rel
		out = append(out, doc)
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

func loadThemeDoc(path string) (map[string]any, string, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, "", domain.NewError(domain.ErrMutationApply, "read Gemini theme", err)
	}
	var doc map[string]any
	if err := yaml.Unmarshal(body, &doc); err != nil {
		return nil, "", domain.NewError(domain.ErrMutationApply, "parse Gemini theme", err)
	}
	if doc == nil {
		doc = map[string]any{}
	}
	name, _ := doc["name"].(string)
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, "", domain.NewError(domain.ErrMutationApply, "Gemini themes require a non-empty name", nil)
	}
	return doc, name, nil
}

package gemini

import (
	"context"
	"encoding/json"
	"os"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/authoredpath"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"gopkg.in/yaml.v3"
)

func (a Adapter) materializedGeminiManifest(ctx context.Context, manifest domain.IntegrationManifest, sourceRoot string, meta packageMeta) (map[string]any, error) {
	doc := map[string]any{
		"name":        manifest.IntegrationID,
		"version":     manifest.Version,
		"description": manifest.Description,
	}
	applyPackageMeta(doc, meta)
	if settings, err := loadSettings(authoredpath.Join(sourceRoot, "targets", "gemini", "settings")); err != nil {
		return nil, err
	} else if len(settings) > 0 {
		doc["settings"] = settings
	}
	if themes, err := loadThemes(authoredpath.Join(sourceRoot, "targets", "gemini", "themes")); err != nil {
		return nil, err
	} else if len(themes) > 0 {
		doc["themes"] = themes
	}
	if mcp, err := a.loadProjectedMCP(ctx, sourceRoot); err != nil {
		return nil, err
	} else if len(mcp) > 0 {
		doc["mcpServers"] = mcp
	}
	if err := mergeManifestExtra(doc, authoredpath.Join(sourceRoot, "targets", "gemini", "manifest.extra.json")); err != nil {
		return nil, err
	}
	return doc, nil
}

func loadPackageMeta(path string) (packageMeta, error) {
	var meta packageMeta
	body, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return meta, nil
		}
		return meta, domain.NewError(domain.ErrMutationApply, "read Gemini package metadata", err)
	}
	if err := yaml.Unmarshal(body, &meta); err != nil {
		return meta, domain.NewError(domain.ErrMutationApply, "parse Gemini package metadata", err)
	}
	meta.ContextFileName = strings.TrimSpace(meta.ContextFileName)
	meta.PlanDirectory = strings.TrimSpace(meta.PlanDirectory)
	return meta, nil
}

func applyPackageMeta(doc map[string]any, meta packageMeta) {
	if len(meta.ExcludeTools) > 0 {
		doc["excludeTools"] = append([]string(nil), meta.ExcludeTools...)
	}
	if strings.TrimSpace(meta.PlanDirectory) != "" {
		doc["plan"] = map[string]any{"directory": strings.TrimSpace(meta.PlanDirectory)}
	}
}

func mergeManifestExtra(doc map[string]any, path string) error {
	body, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return domain.NewError(domain.ErrMutationApply, "read Gemini manifest.extra.json", err)
	}
	var extra map[string]any
	if err := json.Unmarshal(body, &extra); err != nil {
		return domain.NewError(domain.ErrMutationApply, "parse Gemini manifest.extra.json", err)
	}
	for key, value := range extra {
		switch strings.TrimSpace(key) {
		case "", "name", "version", "description", "mcpServers", "contextFileName", "excludeTools", "settings", "themes":
			return domain.NewError(domain.ErrMutationApply, "Gemini manifest.extra.json may not override managed key "+key, nil)
		case "plan":
			if err := mergeManifestExtraPlan(doc, value); err != nil {
				return err
			}
		default:
			doc[key] = value
		}
	}
	return nil
}

func mergeManifestExtraPlan(doc map[string]any, value any) error {
	raw, ok := value.(map[string]any)
	if !ok {
		return domain.NewError(domain.ErrMutationApply, "Gemini manifest.extra.json field plan must be an object", nil)
	}
	plan, _ := doc["plan"].(map[string]any)
	if plan == nil {
		plan = map[string]any{}
	}
	for childKey, childValue := range raw {
		if strings.TrimSpace(childKey) == "directory" {
			return domain.NewError(domain.ErrMutationApply, "Gemini manifest.extra.json may not override managed key plan.directory", nil)
		}
		plan[childKey] = childValue
	}
	doc["plan"] = plan
	return nil
}

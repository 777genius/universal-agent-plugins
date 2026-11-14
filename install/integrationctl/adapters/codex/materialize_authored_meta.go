package codex

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"gopkg.in/yaml.v3"
)

func applyCodexPackageMeta(doc map[string]any, path string) error {
	body, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return domain.NewError(domain.ErrMutationApply, "read Codex package.yaml", err)
	}
	var meta packageMeta
	if err := yaml.Unmarshal(body, &meta); err != nil {
		return domain.NewError(domain.ErrMutationApply, "parse Codex package.yaml", err)
	}
	if authorDoc := codexAuthorDoc(meta.Author); len(authorDoc) > 0 {
		doc["author"] = authorDoc
	}
	if strings.TrimSpace(meta.Homepage) != "" {
		doc["homepage"] = strings.TrimSpace(meta.Homepage)
	}
	if strings.TrimSpace(meta.Repository) != "" {
		doc["repository"] = strings.TrimSpace(meta.Repository)
	}
	if strings.TrimSpace(meta.License) != "" {
		doc["license"] = strings.TrimSpace(meta.License)
	}
	if keywords := codexKeywords(meta.Keywords); len(keywords) > 0 {
		doc["keywords"] = keywords
	}
	return nil
}

func codexAuthorDoc(meta *author) map[string]any {
	if meta == nil {
		return nil
	}
	authorDoc := map[string]any{}
	if strings.TrimSpace(meta.Name) != "" {
		authorDoc["name"] = strings.TrimSpace(meta.Name)
	}
	if strings.TrimSpace(meta.Email) != "" {
		authorDoc["email"] = strings.TrimSpace(meta.Email)
	}
	if strings.TrimSpace(meta.URL) != "" {
		authorDoc["url"] = strings.TrimSpace(meta.URL)
	}
	return authorDoc
}

func codexKeywords(items []string) []string {
	keywords := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		keywords = append(keywords, item)
	}
	return keywords
}

func mergeCodexManifestExtra(doc map[string]any, path string) error {
	body, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return domain.NewError(domain.ErrMutationApply, "read Codex manifest.extra.json", err)
	}
	var extra map[string]any
	if err := json.Unmarshal(body, &extra); err != nil {
		return domain.NewError(domain.ErrMutationApply, "parse Codex manifest.extra.json", err)
	}
	for key, value := range extra {
		switch strings.TrimSpace(key) {
		case "", "name", "version", "description", "author", "homepage", "repository", "license", "keywords", "interface", "apps", "mcpServers", "skills":
			return domain.NewError(domain.ErrMutationApply, "Codex manifest.extra.json may not override managed key "+key, nil)
		default:
			doc[key] = value
		}
	}
	return nil
}

func readAnyJSON(path string) (any, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, domain.NewError(domain.ErrMutationApply, "read JSON document", err)
	}
	var doc any
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, domain.NewError(domain.ErrMutationApply, "parse JSON document", err)
	}
	return doc, nil
}

func marshalJSON(doc any) ([]byte, error) {
	body, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(body, '\n'), nil
}

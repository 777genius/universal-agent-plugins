package codex

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
)

func readMarketplace(path string) (marketplaceDoc, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return marketplaceDoc{Extra: map[string]any{}, Plugins: []map[string]any{}}, nil
		}
		return marketplaceDoc{}, domain.NewError(domain.ErrMutationApply, "read Codex marketplace catalog", err)
	}
	var doc marketplaceDoc
	if err := json.Unmarshal(body, &doc); err != nil {
		return marketplaceDoc{}, domain.NewError(domain.ErrMutationApply, "parse Codex marketplace catalog", err)
	}
	if doc.Extra == nil {
		doc.Extra = map[string]any{}
	}
	if doc.Plugins == nil {
		doc.Plugins = []map[string]any{}
	}
	return doc, nil
}

func writeMarketplace(path string, doc marketplaceDoc) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return domain.NewError(domain.ErrMutationApply, "prepare Codex marketplace dir", err)
	}
	body, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return domain.NewError(domain.ErrMutationApply, "marshal Codex marketplace catalog", err)
	}
	body = append(body, '\n')
	if err := os.WriteFile(path, body, 0o644); err != nil {
		return domain.NewError(domain.ErrMutationApply, "write Codex marketplace catalog", err)
	}
	return nil
}

func defaultMarketplaceName(path string) string {
	_ = path
	return "integrationctl-managed"
}

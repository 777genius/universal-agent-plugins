package publishschema

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

func loadCodexMarketplace(root, authoredRoot string) (*CodexMarketplace, bool, error) {
	body, ok, err := readOptional(root, authoredRoot, CodexMarketplaceRel)
	if err != nil || !ok {
		return nil, ok, err
	}
	var out CodexMarketplace
	if err := yaml.Unmarshal(body, &out); err != nil {
		return nil, true, fmt.Errorf("parse %s: %w", CodexMarketplaceRel, err)
	}
	normalizeCodexMarketplace(&out)
	out.Path = prefixedRel(authoredRoot, CodexMarketplaceRel)
	if err := out.Validate(); err != nil {
		return nil, true, fmt.Errorf("parse %s: %w", CodexMarketplaceRel, err)
	}
	return &out, true, nil
}

func loadClaudeMarketplace(root, authoredRoot string) (*ClaudeMarketplace, bool, error) {
	body, ok, err := readOptional(root, authoredRoot, ClaudeMarketplaceRel)
	if err != nil || !ok {
		return nil, ok, err
	}
	var out ClaudeMarketplace
	if err := yaml.Unmarshal(body, &out); err != nil {
		return nil, true, fmt.Errorf("parse %s: %w", ClaudeMarketplaceRel, err)
	}
	normalizeClaudeMarketplace(&out)
	out.Path = prefixedRel(authoredRoot, ClaudeMarketplaceRel)
	if err := out.Validate(); err != nil {
		return nil, true, fmt.Errorf("parse %s: %w", ClaudeMarketplaceRel, err)
	}
	return &out, true, nil
}

func loadGeminiGallery(root, authoredRoot string) (*GeminiGallery, bool, error) {
	body, ok, err := readOptional(root, authoredRoot, GeminiGalleryRel)
	if err != nil || !ok {
		return nil, ok, err
	}
	var out GeminiGallery
	if err := yaml.Unmarshal(body, &out); err != nil {
		return nil, true, fmt.Errorf("parse %s: %w", GeminiGalleryRel, err)
	}
	normalizeGeminiGallery(&out)
	out.Path = prefixedRel(authoredRoot, GeminiGalleryRel)
	if err := out.Validate(); err != nil {
		return nil, true, fmt.Errorf("parse %s: %w", GeminiGalleryRel, err)
	}
	return &out, true, nil
}

func readOptional(root, authoredRoot, rel string) ([]byte, bool, error) {
	body, err := os.ReadFile(filepath.Join(root, prefixedRel(authoredRoot, rel)))
	if err == nil {
		return body, true, nil
	}
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	return nil, false, err
}

func prefixedRel(rootRel, rel string) string {
	rootRel = filepath.ToSlash(strings.TrimSpace(rootRel))
	rel = filepath.ToSlash(strings.TrimSpace(rel))
	if rootRel == "" {
		return rel
	}
	return filepath.ToSlash(filepath.Join(rootRel, rel))
}

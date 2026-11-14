package codex

import (
	"slices"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
)

func marketplaceEntryDoc(manifest domain.IntegrationManifest, pluginRoot string) map[string]any {
	_ = pluginRoot
	return map[string]any{
		"name": manifest.IntegrationID,
		"source": map[string]any{
			"source": "local",
			"path":   "./plugins/" + manifest.IntegrationID,
		},
		"policy": map[string]any{
			"installation":   "AVAILABLE",
			"authentication": "ON_INSTALL",
		},
		"category": "Productivity",
	}
}

func mergeMarketplaceEntry(path string, entry map[string]any) (string, error) {
	doc, err := readMarketplace(path)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(doc.Name) == "" {
		doc.Name = defaultMarketplaceName(path)
	}
	name, _ := entry["name"].(string)
	if strings.TrimSpace(name) == "" {
		return "", domain.NewError(domain.ErrMutationApply, "Codex marketplace entry is missing plugin name", nil)
	}
	replaced := false
	for i, existing := range doc.Plugins {
		existingName, _ := existing["name"].(string)
		if strings.TrimSpace(existingName) == strings.TrimSpace(name) {
			doc.Plugins[i] = entry
			replaced = true
			break
		}
	}
	if !replaced {
		doc.Plugins = append(doc.Plugins, entry)
	}
	slices.SortFunc(doc.Plugins, func(a, b map[string]any) int {
		return strings.Compare(pluginName(a), pluginName(b))
	})
	if err := writeMarketplace(path, doc); err != nil {
		return "", err
	}
	return doc.Name, nil
}

func removeMarketplaceEntry(path, pluginName string) error {
	doc, err := readMarketplace(path)
	if err != nil {
		return err
	}
	filtered := make([]map[string]any, 0, len(doc.Plugins))
	for _, item := range doc.Plugins {
		if strings.TrimSpace(pluginNameFromEntry(item)) == strings.TrimSpace(pluginName) {
			continue
		}
		filtered = append(filtered, item)
	}
	doc.Plugins = filtered
	return writeMarketplace(path, doc)
}

func readMarketplaceEntry(path, pluginName string) (map[string]any, bool, error) {
	doc, err := readMarketplace(path)
	if err != nil {
		return nil, false, err
	}
	for _, item := range doc.Plugins {
		if strings.TrimSpace(pluginNameFromEntry(item)) == strings.TrimSpace(pluginName) {
			return item, true, nil
		}
	}
	return nil, false, nil
}

func pluginName(item map[string]any) string {
	return strings.TrimSpace(pluginNameFromEntry(item))
}

func pluginNameFromEntry(item map[string]any) string {
	value, _ := item["name"].(string)
	return strings.TrimSpace(value)
}

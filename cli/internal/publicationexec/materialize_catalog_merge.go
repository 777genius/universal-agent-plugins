package publicationexec

import (
	"encoding/json"
	"fmt"
	"strings"
)

func mergeCatalogDocument(target string, existing, generated []byte, requiredTopLevelKeys ...string) ([]byte, error) {
	if len(existing) == 0 {
		return append([]byte(nil), generated...), nil
	}
	current, currentPlugins, next, nextPlugins, err := loadCatalogDocuments(target, existing, generated)
	if err != nil {
		return nil, err
	}
	for _, key := range requiredTopLevelKeys {
		if currentValue, ok := current[key]; ok {
			if !jsonDocumentsEqual(currentValue, next[key]) {
				return nil, fmt.Errorf("existing marketplace artifact sets %s differently; materialize requires a matching %s across the marketplace root", key, key)
			}
		}
	}
	generatedPlugin, generatedName, err := generatedCatalogPlugin(nextPlugins)
	if err != nil {
		return nil, err
	}
	next["plugins"] = encodePluginEntries(upsertCatalogPlugin(currentPlugins, generatedName, generatedPlugin))
	return json.MarshalIndent(next, "", "  ")
}

func upsertCatalogPlugin(currentPlugins []map[string]any, generatedName string, generatedPlugin map[string]any) []map[string]any {
	replaced := false
	for i, plugin := range currentPlugins {
		if strings.TrimSpace(stringValue(plugin["name"])) == generatedName {
			currentPlugins[i] = generatedPlugin
			replaced = true
			break
		}
	}
	if !replaced {
		currentPlugins = append(currentPlugins, generatedPlugin)
	}
	return sortedCatalogPlugins(currentPlugins)
}

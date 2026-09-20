package publicationexec

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

func loadCurrentCatalogDocument(target string, existing []byte) (map[string]any, []map[string]any, error) {
	if err := validateCatalogTarget(target); err != nil {
		return nil, nil, err
	}
	current, err := decodeCatalogDocument(existing, "existing")
	if err != nil {
		return nil, nil, err
	}
	currentPlugins, err := decodePluginEntries(current["plugins"])
	if err != nil {
		return nil, nil, err
	}
	return current, currentPlugins, nil
}

func loadCatalogDocuments(target string, existing, generated []byte) (map[string]any, []map[string]any, map[string]any, []map[string]any, error) {
	current, currentPlugins, err := loadCurrentCatalogDocument(target, existing)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	next, err := decodeCatalogDocument(generated, "generated")
	if err != nil {
		return nil, nil, nil, nil, err
	}
	nextPlugins, err := decodePluginEntries(next["plugins"])
	if err != nil {
		return nil, nil, nil, nil, err
	}
	return current, currentPlugins, next, nextPlugins, nil
}

func decodeCatalogDocument(body []byte, label string) (map[string]any, error) {
	var doc map[string]any
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("parse %s marketplace artifact: %w", label, err)
	}
	return doc, nil
}

func generatedCatalogPlugin(nextPlugins []map[string]any) (map[string]any, string, error) {
	if len(nextPlugins) != 1 {
		return nil, "", fmt.Errorf("generated marketplace artifact must contain exactly one plugin entry")
	}
	generatedPlugin := nextPlugins[0]
	generatedName := strings.TrimSpace(stringValue(generatedPlugin["name"]))
	if generatedName == "" {
		return nil, "", fmt.Errorf("generated marketplace artifact plugin entry is missing name")
	}
	return generatedPlugin, generatedName, nil
}

func sortedCatalogPlugins(items []map[string]any) []map[string]any {
	slices.SortFunc(items, func(a, b map[string]any) int {
		return strings.Compare(strings.TrimSpace(stringValue(a["name"])), strings.TrimSpace(stringValue(b["name"])))
	})
	return items
}

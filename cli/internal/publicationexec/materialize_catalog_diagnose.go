package publicationexec

import (
	"fmt"
	"strings"
)

func diagnoseCatalogArtifact(target string, existing, generated []byte, pluginName string) ([]CatalogIssue, error) {
	current, currentPlugins, next, nextPlugins, err := loadCatalogDocuments(target, existing, generated)
	if err != nil {
		return nil, err
	}
	issues := diagnoseCatalogIdentityDrift(current, next, catalogIdentityKeys(target))
	generatedPlugin, _, err := generatedCatalogPlugin(nextPlugins)
	if err != nil {
		return nil, err
	}
	for _, plugin := range currentPlugins {
		if !sameCatalogPluginName(plugin, pluginName) {
			continue
		}
		if !jsonDocumentsEqual(plugin, generatedPlugin) {
			issues = append(issues, CatalogIssue{
				Code:    "drifted_materialized_catalog_entry",
				Path:    "plugins",
				Message: fmt.Sprintf("catalog entry for plugin %s is out of sync with current authored publication data", pluginName),
			})
		}
		return issues, nil
	}
	return append(issues, CatalogIssue{
		Code:    "missing_materialized_catalog_entry",
		Path:    "plugins",
		Message: fmt.Sprintf("catalog entry for plugin %s is missing", pluginName),
	}), nil
}

func diagnoseCatalogIdentityDrift(current, next map[string]any, identityKeys []string) []CatalogIssue {
	var issues []CatalogIssue
	for _, key := range identityKeys {
		if currentValue, ok := current[key]; ok && !jsonDocumentsEqual(currentValue, next[key]) {
			issues = append(issues, CatalogIssue{
				Code:    "drifted_materialized_catalog_identity",
				Path:    key,
				Message: fmt.Sprintf("catalog field %s does not match the authored publication identity", key),
			})
		}
	}
	return issues
}

func sameCatalogPluginName(plugin map[string]any, pluginName string) bool {
	return strings.TrimSpace(stringValue(plugin["name"])) == strings.TrimSpace(pluginName)
}

package publicationexec

import "encoding/json"

func removeCatalogArtifact(target string, existing []byte, pluginName string) ([]byte, bool, error) {
	current, currentPlugins, err := loadCurrentCatalogDocument(target, existing)
	if err != nil {
		return nil, false, err
	}
	filtered, removed := filterCatalogPlugin(currentPlugins, pluginName)
	if !removed {
		return append([]byte(nil), existing...), false, nil
	}
	current["plugins"] = encodePluginEntries(sortedCatalogPlugins(filtered))
	body, err := json.MarshalIndent(current, "", "  ")
	if err != nil {
		return nil, false, err
	}
	return body, true, nil
}

func filterCatalogPlugin(currentPlugins []map[string]any, pluginName string) ([]map[string]any, bool) {
	filtered := make([]map[string]any, 0, len(currentPlugins))
	removed := false
	for _, plugin := range currentPlugins {
		if sameCatalogPluginName(plugin, pluginName) {
			removed = true
			continue
		}
		filtered = append(filtered, plugin)
	}
	return filtered, removed
}

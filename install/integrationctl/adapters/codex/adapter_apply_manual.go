package codex

func manualInstallSteps(location, pluginName string) []string {
	return []string{
		"open Codex Plugin Directory and install " + pluginName + " from the prepared " + location + " marketplace",
		"after installation, start a new Codex thread before using the plugin",
	}
}

func manualUpdateSteps(pluginName string) []string {
	return []string{
		"restart Codex so it re-reads the updated local marketplace source",
		"refresh or reinstall " + pluginName + " from the Codex Plugin Directory if the installed cache copy is stale",
		"open a new Codex thread before using the refreshed plugin",
	}
}

func manualRemoveSteps(pluginName string) []string {
	return []string{
		"if " + pluginName + " was already installed in Codex, uninstall it from the Codex Plugin Directory",
		"bundled apps stay managed separately in ChatGPT even after the plugin bundle is removed from Codex",
		"restart Codex after removing the plugin bundle",
	}
}

func marketplaceLocationLabel(scope string) string {
	if normalizedScope(scope) == "project" {
		return "project"
	}
	return "personal"
}

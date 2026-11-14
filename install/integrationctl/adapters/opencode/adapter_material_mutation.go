package opencode

import "github.com/777genius/plugin-kit-ai/install/integrationctl/domain"

func buildUpdateMutation(material sourceMaterial, target domain.TargetInstallation) configMutation {
	currentKeys := ownedConfigKeys(target)
	currentPlugins := ownedPluginRefs(target)
	currentMCP := ownedMCPAliases(target)
	return configMutation{
		WholeSet:      material.WholeFields,
		WholeRemove:   subtractStrings(currentKeys, sortedManagedKeys(material.WholeFields)),
		PluginsSet:    material.Plugins,
		PluginsRemove: subtractStrings(currentPlugins, pluginRefNames(material.Plugins)),
		MCPSet:        material.MCP,
		MCPRemove:     subtractStrings(currentMCP, sortedMapKeys(material.MCP)),
	}
}

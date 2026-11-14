package opencode

import (
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
)

func ensureConfigMutationCompatible(state configPatchState, mutation configMutation, target *domain.TargetInstallation) error {
	oldPluginRefs := mapFromSlice(ownedPluginRefsOrMetadata(target), func(value string) string { return value })
	oldMCPAliases := mapFromSlice(ownedMCPAliasesOrMetadata(target), func(value string) string { return value })

	for _, ref := range mutation.PluginsSet {
		name := strings.TrimSpace(ref.Name)
		if name == "" {
			continue
		}
		if existing, ok := state.currentPlugins[name]; ok && !oldPluginRefs[name] && !pluginRefsEqual(existing, ref) {
			return domain.NewError(domain.ErrStateConflict, "OpenCode plugin ref conflict for "+name, nil)
		}
	}
	for alias, desired := range mutation.MCPSet {
		if existing, ok := state.currentMCP[alias]; ok && !oldMCPAliases[alias] && !jsonValuesEqual(existing, desired) {
			return domain.NewError(domain.ErrStateConflict, "OpenCode MCP alias conflict for "+alias, nil)
		}
	}
	return nil
}

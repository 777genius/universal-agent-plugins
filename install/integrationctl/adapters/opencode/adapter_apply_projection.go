package opencode

import (
	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func applyProjectionResult(targetID domain.TargetID, scope string, patch configPatchResult, copiedPaths []string) ports.ApplyResult {
	return ports.ApplyResult{
		TargetID:           targetID,
		State:              domain.InstallInstalled,
		ActivationState:    domain.ActivationRestartPending,
		OwnedNativeObjects: ownedObjects(patch.ConfigPath, patch.ManagedKeys, patch.OwnedPluginRefs, patch.OwnedMCPAliases, copiedPaths, protectionForScope(scope)),
		EvidenceClass:      domain.EvidenceConfirmed,
		ManualSteps:        []string{"restart OpenCode to pick up updated config and projected files"},
		AdapterMetadata: map[string]any{
			"config_path":          patch.ConfigPath,
			"managed_config_keys":  patch.ManagedKeys,
			"owned_plugin_refs":    patch.OwnedPluginRefs,
			"owned_mcp_aliases":    patch.OwnedMCPAliases,
			"copied_paths":         copiedPaths,
			"config_body_checksum": len(patch.Body),
		},
	}
}

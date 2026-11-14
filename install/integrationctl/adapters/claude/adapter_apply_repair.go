package claude

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func (a Adapter) Repair(ctx context.Context, in ports.RepairInput) (ports.ApplyResult, error) {
	if in.Manifest == nil {
		return ports.ApplyResult{}, domain.NewError(domain.ErrRepairApply, "Claude repair requires resolved manifest", nil)
	}
	if in.ResolvedSource == nil {
		return ports.ApplyResult{}, domain.NewError(domain.ErrRepairApply, "Claude repair requires resolved source", nil)
	}
	if seedPath, ok := a.seedManagedMarketplacePath(in.Record.IntegrationID, &in.Record); ok {
		return ports.ApplyResult{}, domain.NewError(domain.ErrRepairApply, "Claude marketplace is seed-managed and read-only: "+seedPath, nil)
	}
	commandDir := a.commandDirForScope(in.Record.Policy.Scope, workspaceRootFromRecord(in.Record))
	materializedRoot, marketplaceName, pluginRef, err := a.syncManagedMarketplace(ctx, *in.Manifest, in.ResolvedSource.LocalPath)
	if err != nil {
		return ports.ApplyResult{}, err
	}
	refreshMarketplaceArgv := []string{"claude", "plugin", "marketplace", "update", marketplaceName}
	if err := a.runClaude(ctx, refreshMarketplaceArgv, commandDir); err != nil {
		addMarketplaceArgv := []string{"claude", "plugin", "marketplace", "add", materializedRoot}
		if addErr := a.runClaude(ctx, addMarketplaceArgv, commandDir); addErr != nil {
			return ports.ApplyResult{}, addErr
		}
		refreshMarketplaceArgv = addMarketplaceArgv
	}
	uninstallArgv := scopedPluginArgv([]string{"claude", "plugin", "uninstall", pluginRef}, in.Record.Policy.Scope)
	_ = a.runClaude(ctx, uninstallArgv, commandDir)
	installArgv := scopedPluginArgv([]string{"claude", "plugin", "install", pluginRef}, in.Record.Policy.Scope)
	if err := a.runClaude(ctx, installArgv, commandDir); err != nil {
		return ports.ApplyResult{}, err
	}
	return ports.ApplyResult{
		TargetID:           a.ID(),
		State:              domain.InstallInstalled,
		ActivationState:    domain.ActivationReloadPending,
		OwnedNativeObjects: a.ownedObjects(in.Manifest.IntegrationID, in.Record.Policy.Scope, workspaceRootFromRecord(in.Record), materializedRoot),
		EvidenceClass:      domain.EvidenceConfirmed,
		ReloadRequired:     true,
		ManualSteps:        []string{"run /reload-plugins in Claude Code so the repaired plugin package is reloaded in the current session"},
		AdapterMetadata: map[string]any{
			"marketplace_name":         marketplaceName,
			"plugin_ref":               pluginRef,
			"materialized_source_root": materializedRoot,
			"marketplace_refresh_argv": refreshMarketplaceArgv,
			"plugin_uninstall_argv":    uninstallArgv,
			"plugin_install_argv":      installArgv,
		},
	}, nil
}

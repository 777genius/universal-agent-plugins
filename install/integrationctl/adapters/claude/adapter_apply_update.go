package claude

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func (a Adapter) PlanUpdate(_ context.Context, in ports.PlanUpdateInput) (ports.AdapterPlan, error) {
	manual, blocking := blockingSteps(in.Inspect)
	return ports.AdapterPlan{
		TargetID:       a.ID(),
		ActionClass:    "update_version",
		Summary:        "Refresh managed Claude marketplace and reinstall the plugin",
		ReloadRequired: true,
		PathsTouched: []string{
			managedMarketplaceRoot(a.userHome(), in.CurrentRecord.IntegrationID),
			a.settingsPath(in.CurrentRecord.Policy.Scope, workspaceRootFromRecord(in.CurrentRecord)),
		},
		ManualSteps: manual,
		Blocking:    blocking,
		EvidenceKey: "target.claude.native_surface",
	}, nil
}

func (a Adapter) ApplyUpdate(ctx context.Context, in ports.ApplyInput) (ports.ApplyResult, error) {
	if in.Record == nil {
		return ports.ApplyResult{}, domain.NewError(domain.ErrMutationApply, "Claude update requires current record", nil)
	}
	if in.ResolvedSource == nil {
		return ports.ApplyResult{}, domain.NewError(domain.ErrMutationApply, "Claude update requires resolved source", nil)
	}
	if seedPath, ok := a.seedManagedMarketplacePath(in.Record.IntegrationID, in.Record); ok {
		return ports.ApplyResult{}, domain.NewError(domain.ErrMutationApply, "Claude marketplace is seed-managed and read-only: "+seedPath, nil)
	}
	commandDir := a.commandDirForScope(in.Record.Policy.Scope, workspaceRootFromRecord(*in.Record))
	materializedRoot, marketplaceName, pluginRef, err := a.syncManagedMarketplace(ctx, in.Manifest, in.ResolvedSource.LocalPath)
	if err != nil {
		return ports.ApplyResult{}, err
	}
	updateArgv := []string{"claude", "plugin", "marketplace", "update", marketplaceName}
	if err := a.runClaude(ctx, updateArgv, commandDir); err != nil {
		return ports.ApplyResult{}, err
	}
	uninstallArgv := scopedPluginArgv([]string{"claude", "plugin", "uninstall", pluginRef}, in.Record.Policy.Scope)
	if err := a.runClaude(ctx, uninstallArgv, commandDir); err != nil {
		return ports.ApplyResult{}, err
	}
	installArgv := scopedPluginArgv([]string{"claude", "plugin", "install", pluginRef}, in.Record.Policy.Scope)
	if err := a.runClaude(ctx, installArgv, commandDir); err != nil {
		return ports.ApplyResult{}, err
	}
	return ports.ApplyResult{
		TargetID:           a.ID(),
		State:              domain.InstallInstalled,
		ActivationState:    domain.ActivationReloadPending,
		OwnedNativeObjects: a.ownedObjects(in.Manifest.IntegrationID, in.Record.Policy.Scope, workspaceRootFromRecord(*in.Record), materializedRoot),
		EvidenceClass:      domain.EvidenceConfirmed,
		ReloadRequired:     true,
		ManualSteps:        []string{"run /reload-plugins in Claude Code so the updated plugin package is reloaded in the current session"},
		AdapterMetadata: map[string]any{
			"marketplace_name":         marketplaceName,
			"plugin_ref":               pluginRef,
			"materialized_source_root": materializedRoot,
			"marketplace_update_argv":  updateArgv,
			"plugin_uninstall_argv":    uninstallArgv,
			"plugin_install_argv":      installArgv,
		},
	}, nil
}

package claude

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func (a Adapter) PlanInstall(_ context.Context, in ports.PlanInstallInput) (ports.AdapterPlan, error) {
	marketplaceRoot := managedMarketplaceRoot(a.userHome(), in.Manifest.IntegrationID)
	settings := a.settingsPath(in.Policy.Scope, "")
	manual, blocking := blockingSteps(in.Inspect)
	if blocked, message := a.marketplaceAddBlocked(in.Policy.Scope, "", in.Manifest.IntegrationID); blocked {
		manual = append(manual, message)
		blocking = true
	}
	return ports.AdapterPlan{
		TargetID:       a.ID(),
		ActionClass:    "install_missing",
		Summary:        "Install Claude plugin through a managed local marketplace",
		ReloadRequired: true,
		PathsTouched:   []string{marketplaceRoot, settings},
		ManualSteps:    manual,
		Blocking:       blocking,
		EvidenceKey:    "target.claude.native_surface",
	}, nil
}

func (a Adapter) ApplyInstall(ctx context.Context, in ports.ApplyInput) (ports.ApplyResult, error) {
	if in.ResolvedSource == nil {
		return ports.ApplyResult{}, domain.NewError(domain.ErrMutationApply, "Claude install requires resolved source", nil)
	}
	commandDir := a.commandDirForScope(in.Policy.Scope, workspaceRootFromApplyInput(in))
	materializedRoot, marketplaceName, pluginRef, err := a.syncManagedMarketplace(ctx, in.Manifest, in.ResolvedSource.LocalPath)
	if err != nil {
		return ports.ApplyResult{}, err
	}
	addMarketplaceArgv := []string{"claude", "plugin", "marketplace", "add", materializedRoot}
	if err := a.runClaude(ctx, addMarketplaceArgv, commandDir); err != nil {
		return ports.ApplyResult{}, err
	}
	installArgv := scopedPluginArgv([]string{"claude", "plugin", "install", pluginRef}, in.Policy.Scope)
	if err := a.runClaude(ctx, installArgv, commandDir); err != nil {
		_ = a.runClaude(ctx, []string{"claude", "plugin", "marketplace", "remove", marketplaceName}, commandDir)
		return ports.ApplyResult{}, err
	}
	return ports.ApplyResult{
		TargetID:           a.ID(),
		State:              domain.InstallInstalled,
		ActivationState:    domain.ActivationReloadPending,
		OwnedNativeObjects: a.ownedObjects(in.Manifest.IntegrationID, in.Policy.Scope, workspaceRootFromApplyInput(in), materializedRoot),
		EvidenceClass:      domain.EvidenceConfirmed,
		ReloadRequired:     true,
		ManualSteps:        []string{"run /reload-plugins in Claude Code if the current session should pick up the new plugin immediately"},
		AdapterMetadata: map[string]any{
			"marketplace_name":         marketplaceName,
			"plugin_ref":               pluginRef,
			"materialized_source_root": materializedRoot,
			"marketplace_add_argv":     addMarketplaceArgv,
			"plugin_install_argv":      installArgv,
		},
	}, nil
}

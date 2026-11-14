package codex

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func (a Adapter) applyInstall(ctx context.Context, in ports.ApplyInput) (ports.ApplyResult, error) {
	if in.ResolvedSource == nil {
		return ports.ApplyResult{}, domain.NewError(domain.ErrMutationApply, "Codex install requires resolved source", nil)
	}
	paths := a.pathsForScope(in.Policy.Scope, workspaceRootFromApplyInput(in), in.Manifest.IntegrationID)
	return a.applyMarketplaceMaterialization(ctx, in.Manifest, in.Manifest.IntegrationID, in.ResolvedSource.LocalPath, paths, installActivationMetadata(paths, in.Manifest.IntegrationID))
}

func (a Adapter) applyUpdate(ctx context.Context, in ports.ApplyInput) (ports.ApplyResult, error) {
	if in.Record == nil {
		return ports.ApplyResult{}, domain.NewError(domain.ErrMutationApply, "Codex update requires current record", nil)
	}
	if in.ResolvedSource == nil {
		return ports.ApplyResult{}, domain.NewError(domain.ErrMutationApply, "Codex update requires resolved source", nil)
	}
	if _, ok := in.Record.Targets[domain.TargetCodex]; !ok {
		return ports.ApplyResult{}, domain.NewError(domain.ErrStateConflict, "Codex target is missing from installation record", nil)
	}
	paths := a.pathsForRecord(*in.Record)
	return a.applyMarketplaceMaterialization(ctx, in.Manifest, in.Record.IntegrationID, in.ResolvedSource.LocalPath, paths, updateActivationMetadata(paths, in.Record.IntegrationID))
}

func (a Adapter) applyMarketplaceMaterialization(ctx context.Context, manifest domain.IntegrationManifest, integrationID, sourceRoot string, paths codexSurfacePaths, meta codexApplyMetadata) (ports.ApplyResult, error) {
	if err := a.validateManagedPaths(integrationID, paths); err != nil {
		return ports.ApplyResult{}, domain.NewError(domain.ErrMutationApply, "validate Codex managed paths", err)
	}
	if err := a.syncManagedPlugin(ctx, manifest, sourceRoot, paths.PluginRoot); err != nil {
		return ports.ApplyResult{}, err
	}
	catalogName, err := mergeMarketplaceEntry(paths.CatalogPath, marketplaceEntryDoc(manifest, paths.PluginRoot))
	if err != nil {
		return ports.ApplyResult{}, err
	}
	result := meta.result(a.ID(), paths, integrationID, catalogName)
	result.OwnedNativeObjects = ownedObjects(paths.Scope, paths.CatalogPath, paths.PluginRoot, integrationID)
	return result, nil
}

type codexApplyMetadata struct {
	restartRequired   bool
	newThreadRequired bool
	manualSteps       []string
	activationMethod  string
}

func installActivationMetadata(paths codexSurfacePaths, integrationID string) codexApplyMetadata {
	return codexApplyMetadata{
		newThreadRequired: true,
		manualSteps:       manualInstallSteps(marketplaceLocationLabel(paths.Scope), integrationID),
		activationMethod:  "plugin_directory_install",
	}
}

func updateActivationMetadata(paths codexSurfacePaths, integrationID string) codexApplyMetadata {
	return codexApplyMetadata{
		restartRequired:   true,
		newThreadRequired: true,
		manualSteps:       manualUpdateSteps(integrationID),
		activationMethod:  "plugin_directory_refresh",
	}
}

func (m codexApplyMetadata) result(targetID domain.TargetID, paths codexSurfacePaths, integrationID, catalogName string) ports.ApplyResult {
	restrictions := []domain.EnvironmentRestrictionCode{domain.RestrictionNativeActivation}
	if m.restartRequired {
		restrictions = append(restrictions, domain.RestrictionRestartRequired)
	}
	if m.newThreadRequired {
		restrictions = append(restrictions, domain.RestrictionNewThreadRequired)
	}
	return ports.ApplyResult{
		TargetID:                targetID,
		State:                   domain.InstallActivationPending,
		ActivationState:         domain.ActivationNativePending,
		EvidenceClass:           domain.EvidenceConfirmed,
		RestartRequired:         m.restartRequired,
		NewThreadRequired:       m.newThreadRequired,
		EnvironmentRestrictions: restrictions,
		ManualSteps:             m.manualSteps,
		AdapterMetadata: map[string]any{
			"catalog_path":      paths.CatalogPath,
			"catalog_name":      catalogName,
			"plugin_root":       paths.PluginRoot,
			"plugin_name":       integrationID,
			"activation_method": m.activationMethod,
		},
	}
}

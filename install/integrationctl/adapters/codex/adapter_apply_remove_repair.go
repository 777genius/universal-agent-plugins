package codex

import (
	"context"
	"os"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func (a Adapter) applyRemove(in ports.ApplyInput) (ports.ApplyResult, error) {
	if in.Record == nil {
		return ports.ApplyResult{}, domain.NewError(domain.ErrMutationApply, "Codex remove requires current record", nil)
	}
	if _, ok := in.Record.Targets[domain.TargetCodex]; !ok {
		return ports.ApplyResult{}, domain.NewError(domain.ErrStateConflict, "Codex target is missing from installation record", nil)
	}
	paths := a.pathsForRecord(*in.Record)
	if err := a.validateManagedPaths(in.Record.IntegrationID, paths); err != nil {
		return ports.ApplyResult{}, domain.NewError(domain.ErrMutationApply, "validate Codex managed paths", err)
	}
	if err := removeMarketplaceEntry(paths.CatalogPath, in.Record.IntegrationID); err != nil {
		return ports.ApplyResult{}, err
	}
	if err := os.RemoveAll(paths.PluginRoot); err != nil && !os.IsNotExist(err) {
		return ports.ApplyResult{}, domain.NewError(domain.ErrMutationApply, "remove Codex managed plugin root", err)
	}
	return ports.ApplyResult{
		TargetID:        a.ID(),
		State:           domain.InstallRemoved,
		ActivationState: domain.ActivationRestartPending,
		EvidenceClass:   domain.EvidenceConfirmed,
		RestartRequired: true,
		ManualSteps:     manualRemoveSteps(in.Record.IntegrationID),
		AdapterMetadata: map[string]any{
			"catalog_path": paths.CatalogPath,
			"plugin_root":  paths.PluginRoot,
			"plugin_name":  in.Record.IntegrationID,
		},
	}, nil
}

func (a Adapter) repair(ctx context.Context, in ports.RepairInput) (ports.ApplyResult, error) {
	if in.Manifest == nil || in.ResolvedSource == nil {
		return ports.ApplyResult{}, domain.NewError(domain.ErrRepairApply, "Codex repair requires resolved source and manifest", nil)
	}
	record := in.Record
	result, err := a.applyUpdate(ctx, ports.ApplyInput{
		Plan:           ports.AdapterPlan{TargetID: a.ID(), ActionClass: "repair_drift", EvidenceKey: "target.codex.native_surface"},
		Manifest:       *in.Manifest,
		ResolvedSource: in.ResolvedSource,
		Policy:         record.Policy,
		Inspect:        in.Inspect,
		Record:         &record,
	})
	if err != nil {
		return ports.ApplyResult{}, domain.NewError(domain.ErrRepairApply, "Codex repair refresh failed", err)
	}
	if len(result.ManualSteps) == 0 {
		result.ManualSteps = append(result.ManualSteps, "restart Codex, then use the Plugin Directory to refresh the prepared local plugin and open a new thread")
	}
	return result, nil
}

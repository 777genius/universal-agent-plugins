package codex

import "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"

func (a Adapter) planInstall(in ports.PlanInstallInput) ports.AdapterPlan {
	paths := a.pathsForScope(in.Policy.Scope, "", in.Manifest.IntegrationID)
	return ports.AdapterPlan{
		TargetID:          a.ID(),
		ActionClass:       "install_missing",
		Summary:           "Materialize a Codex local marketplace entry and plugin bundle, then wait for native activation",
		PathsTouched:      []string{paths.CatalogPath, paths.PluginRoot, paths.ConfigPath},
		NewThreadRequired: true,
		ManualSteps:       manualInstallSteps(marketplaceLocationLabel(paths.Scope), in.Manifest.IntegrationID),
		EvidenceKey:       "target.codex.native_surface",
	}
}

func (a Adapter) planUpdate(in ports.PlanUpdateInput) ports.AdapterPlan {
	paths := a.pathsForRecord(in.CurrentRecord)
	return ports.AdapterPlan{
		TargetID:          a.ID(),
		ActionClass:       "update_version",
		Summary:           "Refresh the Codex plugin bundle and local marketplace entry",
		PathsTouched:      []string{paths.CatalogPath, paths.PluginRoot, paths.ConfigPath},
		RestartRequired:   true,
		NewThreadRequired: true,
		ManualSteps:       manualUpdateSteps(in.CurrentRecord.IntegrationID),
		EvidenceKey:       "target.codex.native_surface",
	}
}

func (a Adapter) planRemove(in ports.PlanRemoveInput) ports.AdapterPlan {
	paths := a.pathsForRecord(in.Record)
	return ports.AdapterPlan{
		TargetID:        a.ID(),
		ActionClass:     "remove_orphaned_target",
		Summary:         "Remove the managed Codex marketplace entry and plugin bundle",
		PathsTouched:    []string{paths.CatalogPath, paths.PluginRoot, paths.ConfigPath},
		ManualSteps:     manualRemoveSteps(in.Record.IntegrationID),
		RestartRequired: true,
		EvidenceKey:     "target.codex.native_surface",
	}
}

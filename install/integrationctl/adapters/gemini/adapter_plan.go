package gemini

import (
	"context"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func (a Adapter) PlanInstall(_ context.Context, in ports.PlanInstallInput) (ports.AdapterPlan, error) {
	paths := []string{filepath.Join(a.userHome(), ".gemini", "extensions", in.Manifest.IntegrationID)}
	if in.Manifest.RequestedRef.Kind == "local_path" {
		paths = append(paths, in.Manifest.RequestedRef.Value)
	}
	manualSteps, blocking := a.securityBlockers(in.Manifest, in.Policy.Scope, "")
	return ports.AdapterPlan{
		TargetID:        a.ID(),
		ActionClass:     "install_missing",
		Summary:         "Install Gemini extension using native extension workflow",
		RestartRequired: true,
		PathsTouched:    paths,
		ManualSteps:     manualSteps,
		Blocking:        blocking,
		EvidenceKey:     "target.gemini.native_surface",
	}, nil
}

func (a Adapter) PlanUpdate(_ context.Context, in ports.PlanUpdateInput) (ports.AdapterPlan, error) {
	manualSteps, blocking := a.securityBlockers(in.NextManifest, in.CurrentRecord.Policy.Scope, in.CurrentRecord.WorkspaceRoot)
	return ports.AdapterPlan{
		TargetID:        a.ID(),
		ActionClass:     "update_version",
		Summary:         "Update Gemini extension",
		RestartRequired: true,
		PathsTouched:    []string{filepath.Join(a.userHome(), ".gemini", "extensions", in.CurrentRecord.IntegrationID)},
		ManualSteps:     manualSteps,
		Blocking:        blocking,
		EvidenceKey:     "target.gemini.native_surface",
	}, nil
}

func (a Adapter) PlanRemove(_ context.Context, in ports.PlanRemoveInput) (ports.AdapterPlan, error) {
	return ports.AdapterPlan{
		TargetID:     a.ID(),
		ActionClass:  "remove_orphaned_target",
		Summary:      "Remove Gemini extension",
		PathsTouched: []string{filepath.Join(a.userHome(), ".gemini", "extensions", in.Record.IntegrationID)},
		EvidenceKey:  "target.gemini.native_surface",
	}, nil
}

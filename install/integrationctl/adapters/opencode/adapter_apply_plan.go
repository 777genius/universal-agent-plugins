package opencode

import (
	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func (a Adapter) planInstall(in ports.PlanInstallInput) ports.AdapterPlan {
	configPath := a.configPath(in.Policy.Scope, "")
	paths := []string{configPath}
	if root := a.assetsRoot(in.Policy.Scope, ""); root != "" {
		paths = append(paths, root)
	}
	manualSteps, blocking := planBlockingManualSteps(in.Inspect)
	return ports.AdapterPlan{
		TargetID:        a.ID(),
		ActionClass:     "install_missing",
		Summary:         "Project or global OpenCode projection",
		RestartRequired: true,
		PathsTouched:    paths,
		ManualSteps:     manualSteps,
		Blocking:        blocking,
		EvidenceKey:     "target.opencode.native_surface",
	}
}

func (a Adapter) planUpdate(in ports.PlanUpdateInput) ports.AdapterPlan {
	configPath := a.configPath("user", "")
	if target, ok := in.CurrentRecord.Targets[domain.TargetOpenCode]; ok {
		configPath = configPathFromTarget(target, a.configPath(in.CurrentRecord.Policy.Scope, workspaceRootFromRecord(in.CurrentRecord)))
	}
	manualSteps, blocking := planBlockingManualSteps(in.Inspect)
	return ports.AdapterPlan{
		TargetID:        a.ID(),
		ActionClass:     "update_version",
		Summary:         "Owned projection refresh for OpenCode",
		RestartRequired: true,
		PathsTouched:    []string{configPath, a.assetsRoot(in.CurrentRecord.Policy.Scope, workspaceRootFromRecord(in.CurrentRecord))},
		ManualSteps:     manualSteps,
		Blocking:        blocking,
		EvidenceKey:     "target.opencode.native_surface",
	}
}

func (a Adapter) planRemove(in ports.PlanRemoveInput) ports.AdapterPlan {
	configPath := a.configPath("user", "")
	if target, ok := in.Record.Targets[domain.TargetOpenCode]; ok {
		configPath = configPathFromTarget(target, a.configPath(in.Record.Policy.Scope, workspaceRootFromRecord(in.Record)))
	}
	manualSteps, blocking := planBlockingManualSteps(in.Inspect)
	return ports.AdapterPlan{
		TargetID:     a.ID(),
		ActionClass:  "remove_orphaned_target",
		Summary:      "Remove owned OpenCode projection",
		PathsTouched: []string{configPath, a.assetsRoot(in.Record.Policy.Scope, workspaceRootFromRecord(in.Record))},
		ManualSteps:  manualSteps,
		Blocking:     blocking,
		EvidenceKey:  "target.opencode.native_surface",
	}
}

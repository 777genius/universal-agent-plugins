package cursor

import (
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func (a Adapter) planInstall(in ports.PlanInstallInput) ports.AdapterPlan {
	if shouldUsePluginPackage(in.Manifest, "") {
		pluginRoot := a.targetPluginRoot(in.Manifest.IntegrationID)
		return ports.AdapterPlan{
			TargetID:          a.ID(),
			ActionClass:       "install_missing",
			Summary:           "Install Cursor local plugin package",
			PathsTouched:      []string{pluginRoot},
			ReloadRequired:    true,
			NewThreadRequired: true,
			EvidenceKey:       "target.cursor.native_surface",
		}
	}
	configPath := a.targetConfigPath("user", "")
	if strings.EqualFold(strings.TrimSpace(in.Policy.Scope), "project") {
		configPath = a.targetConfigPath("project", "")
	}
	return ports.AdapterPlan{
		TargetID:     a.ID(),
		ActionClass:  "install_missing",
		Summary:      "Project or global MCP reconciliation for Cursor",
		PathsTouched: []string{configPath},
		EvidenceKey:  "target.cursor.native_surface",
	}
}

func (a Adapter) planUpdate(in ports.PlanUpdateInput) ports.AdapterPlan {
	if shouldUsePluginPackage(in.NextManifest, "") {
		pluginRoot := a.targetPluginRoot(in.NextManifest.IntegrationID)
		if target, ok := in.CurrentRecord.Targets[domain.TargetCursor]; ok {
			pluginRoot = pluginRootFromTarget(target, pluginRoot)
		}
		return ports.AdapterPlan{
			TargetID:          a.ID(),
			ActionClass:       "update_version",
			Summary:           "Refresh Cursor local plugin package",
			PathsTouched:      []string{pluginRoot},
			ReloadRequired:    true,
			NewThreadRequired: true,
			EvidenceKey:       "target.cursor.native_surface",
		}
	}
	configPath := a.targetConfigPath("user", "")
	if target, ok := in.CurrentRecord.Targets[domain.TargetCursor]; ok {
		configPath = configPathFromTarget(target, a.targetConfigPath(in.CurrentRecord.Policy.Scope, workspaceRootFromRecord(in.CurrentRecord)))
	}
	return ports.AdapterPlan{
		TargetID:     a.ID(),
		ActionClass:  "update_version",
		Summary:      "Owned-entry reconciliation for Cursor MCP",
		PathsTouched: []string{configPath},
		EvidenceKey:  "target.cursor.native_surface",
	}
}

func (a Adapter) planRemove(in ports.PlanRemoveInput) ports.AdapterPlan {
	if target, ok := in.Record.Targets[domain.TargetCursor]; ok {
		if pluginRoot := pluginRootFromTarget(target, ""); pluginRoot != "" {
			return ports.AdapterPlan{
				TargetID:       a.ID(),
				ActionClass:    "remove_orphaned_target",
				Summary:        "Remove Cursor local plugin package",
				PathsTouched:   []string{pluginRoot},
				ReloadRequired: true,
				EvidenceKey:    "target.cursor.native_surface",
			}
		}
	}
	configPath := a.targetConfigPath("user", "")
	if target, ok := in.Record.Targets[domain.TargetCursor]; ok {
		configPath = configPathFromTarget(target, a.targetConfigPath(in.Record.Policy.Scope, workspaceRootFromRecord(in.Record)))
	}
	return ports.AdapterPlan{
		TargetID:     a.ID(),
		ActionClass:  "remove_orphaned_target",
		Summary:      "Remove owned Cursor MCP entries",
		PathsTouched: []string{configPath},
		EvidenceKey:  "target.cursor.native_surface",
	}
}

package opencode

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func (a Adapter) applyRemove(ctx context.Context, in ports.ApplyInput) (ports.ApplyResult, error) {
	if in.Record == nil {
		return ports.ApplyResult{}, domain.NewError(domain.ErrMutationApply, "OpenCode remove requires current record", nil)
	}
	target, ok := in.Record.Targets[domain.TargetOpenCode]
	if !ok {
		return ports.ApplyResult{}, domain.NewError(domain.ErrStateConflict, "OpenCode target is missing from installation record", nil)
	}
	configPath := configPathFromTarget(target, a.configPath(in.Record.Policy.Scope, workspaceRootFromRecord(*in.Record)))
	patch, err := a.patchConfig(ctx, configPath, configMutation{
		WholeRemove:   ownedConfigKeys(target),
		PluginsRemove: ownedPluginRefs(target),
		MCPRemove:     ownedMCPAliases(target),
	}, &target)
	if err != nil {
		return ports.ApplyResult{}, err
	}
	if err := a.removeStaleFiles(ctx, copiedFilePaths(target), nil); err != nil {
		return ports.ApplyResult{}, err
	}
	return ports.ApplyResult{
		TargetID:        a.ID(),
		State:           domain.InstallRemoved,
		ActivationState: domain.ActivationRestartPending,
		EvidenceClass:   domain.EvidenceConfirmed,
		ManualSteps:     []string{"restart OpenCode to unload removed managed config and projected files"},
		AdapterMetadata: map[string]any{
			"config_path":          patch.ConfigPath,
			"managed_config_keys":  nil,
			"owned_plugin_refs":    nil,
			"owned_mcp_aliases":    nil,
			"copied_paths":         nil,
			"config_body_checksum": len(patch.Body),
		},
	}, nil
}

func (a Adapter) repair(ctx context.Context, in ports.RepairInput) (ports.ApplyResult, error) {
	if in.Manifest == nil || in.ResolvedSource == nil {
		return ports.ApplyResult{}, domain.NewError(domain.ErrRepairApply, "OpenCode repair requires resolved source and manifest", nil)
	}
	result, err := a.applyUpdate(ctx, ports.ApplyInput{
		Manifest:       *in.Manifest,
		ResolvedSource: in.ResolvedSource,
		Policy:         in.Record.Policy,
		Inspect:        in.Inspect,
		Record:         &in.Record,
	})
	if err != nil {
		return ports.ApplyResult{}, err
	}
	if len(result.ManualSteps) == 0 {
		result.ManualSteps = append(result.ManualSteps, "repair reconciled managed OpenCode config and projected files")
	}
	return result, nil
}

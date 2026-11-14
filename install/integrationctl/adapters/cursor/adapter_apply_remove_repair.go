package cursor

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func (a Adapter) applyRemove(ctx context.Context, in ports.ApplyInput) (ports.ApplyResult, error) {
	if in.Record == nil {
		return ports.ApplyResult{}, domain.NewError(domain.ErrMutationApply, "Cursor remove requires current record", nil)
	}
	target, ok := in.Record.Targets[domain.TargetCursor]
	if !ok {
		return ports.ApplyResult{}, domain.NewError(domain.ErrStateConflict, "Cursor target is missing from installation record", nil)
	}
	docPath := configPathFromTarget(target, a.targetConfigPath(in.Record.Policy.Scope, workspaceRootFromRecord(*in.Record)))
	if err := a.validateConfigPath(in.Record.Policy.Scope, workspaceRootFromRecord(*in.Record), docPath); err != nil {
		return ports.ApplyResult{}, domain.NewError(domain.ErrMutationApply, "validate Cursor MCP config path", err)
	}
	pluginRoot := ownedPluginRoot(target.OwnedNativeObjects)
	removedPluginRoot := false
	if pluginRoot != "" {
		if err := a.removePluginRoot(in.Record.IntegrationID, pluginRoot); err != nil {
			return ports.ApplyResult{}, err
		}
		removedPluginRoot = true
	}
	aliases := ownedAliases(target.OwnedNativeObjects)
	if len(aliases) == 0 {
		meta := map[string]any{}
		if removedPluginRoot {
			meta["removed_plugin_root"] = pluginRoot
		}
		return ports.ApplyResult{
			TargetID:        a.ID(),
			State:           domain.InstallRemoved,
			ActivationState: removeActivationState(removedPluginRoot),
			EvidenceClass:   domain.EvidenceConfirmed,
			ReloadRequired:  removedPluginRoot,
			ManualSteps:     removeManualSteps(removedPluginRoot),
			AdapterMetadata: meta,
		}, nil
	}
	doc, wrapped, originalBody, err := a.readDocument(ctx, docPath)
	if err != nil {
		return ports.ApplyResult{}, err
	}
	for _, alias := range aliases {
		delete(doc, alias)
	}
	body, err := marshalCursorDocument(doc, wrapped)
	if err != nil {
		return ports.ApplyResult{}, domain.NewError(domain.ErrMutationApply, "marshal Cursor MCP config", err)
	}
	if _, err := a.mutator().MutateFile(ctx, ports.SafeFileMutationInput{
		Path: docPath,
		Mode: 0o644,
		Build: func(_ []byte, _ bool) ([]byte, error) {
			return body, nil
		},
		ValidateBefore: func(next []byte) error {
			_, _, err := a.readDocumentBytes(next)
			return err
		},
		ValidateAfter: func(ctx context.Context, path string, _ []byte) error {
			return a.verifyMissingAliases(ctx, path, aliases)
		},
	}); err != nil {
		if len(originalBody) > 0 {
			_ = a.fs().WriteFileAtomic(ctx, docPath, originalBody, 0o644)
		}
		return ports.ApplyResult{}, err
	}
	return ports.ApplyResult{
		TargetID:        a.ID(),
		State:           domain.InstallRemoved,
		ActivationState: removeActivationState(removedPluginRoot),
		EvidenceClass:   domain.EvidenceConfirmed,
		ReloadRequired:  removedPluginRoot,
		ManualSteps:     removeManualSteps(removedPluginRoot),
		AdapterMetadata: map[string]any{
			"config_path":         docPath,
			"removed_aliases":     aliases,
			"removed_plugin_root": pluginRoot,
			"wrapped_style":       wrapped,
		},
	}, nil
}

func (a Adapter) repair(ctx context.Context, in ports.RepairInput) (ports.ApplyResult, error) {
	if in.Manifest == nil || in.ResolvedSource == nil {
		return ports.ApplyResult{}, domain.NewError(domain.ErrRepairApply, "Cursor repair requires resolved source and manifest", nil)
	}
	result, err := a.applyInstall(ctx, ports.ApplyInput{
		Manifest:       *in.Manifest,
		ResolvedSource: in.ResolvedSource,
		Policy:         in.Record.Policy,
		Inspect:        in.Inspect,
		Record:         &in.Record,
	})
	if err != nil {
		return ports.ApplyResult{}, err
	}
	result.State = domain.InstallInstalled
	if len(result.ManualSteps) == 0 {
		result.ManualSteps = append(result.ManualSteps, "repair reconciled managed Cursor MCP entries into the effective config layer")
	}
	return result, nil
}

func removeActivationState(pluginRemoved bool) domain.ActivationState {
	if pluginRemoved {
		return domain.ActivationNativePending
	}
	return domain.ActivationNotRequired
}

func removeManualSteps(pluginRemoved bool) []string {
	if !pluginRemoved {
		return nil
	}
	return []string{"reload Cursor with Developer: Reload Window or restart Cursor"}
}

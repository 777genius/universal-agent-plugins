package opencode

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func (a Adapter) applyUpdate(ctx context.Context, in ports.ApplyInput) (ports.ApplyResult, error) {
	target, err := validateUpdateTarget(in)
	if err != nil {
		return ports.ApplyResult{}, err
	}
	material, err := a.loadSourceMaterial(ctx, in.ResolvedSource.LocalPath, in.Record.Policy.Scope, workspaceRootFromRecord(*in.Record))
	if err != nil {
		return ports.ApplyResult{}, err
	}
	patch, err := a.patchUpdateConfig(ctx, target, in.Record.Policy.Scope, workspaceRootFromRecord(*in.Record), material)
	if err != nil {
		return ports.ApplyResult{}, err
	}
	nextCopiedPaths, err := a.copyOwnedFiles(material.CopyFiles)
	if err != nil {
		return ports.ApplyResult{}, err
	}
	if err := a.removeStaleFiles(ctx, copiedFilePaths(target), nextCopiedPaths); err != nil {
		return ports.ApplyResult{}, err
	}
	return applyProjectionResult(a.ID(), in.Record.Policy.Scope, patch, nextCopiedPaths), nil
}

func validateUpdateTarget(in ports.ApplyInput) (domain.TargetInstallation, error) {
	if in.Record == nil || in.ResolvedSource == nil {
		return domain.TargetInstallation{}, domain.NewError(domain.ErrMutationApply, "OpenCode update requires current record and resolved source", nil)
	}
	target, ok := in.Record.Targets[domain.TargetOpenCode]
	if !ok {
		return domain.TargetInstallation{}, domain.NewError(domain.ErrStateConflict, "OpenCode target is missing from installation record", nil)
	}
	return target, nil
}

func (a Adapter) patchUpdateConfig(ctx context.Context, target domain.TargetInstallation, scope, workspaceRoot string, material sourceMaterial) (configPatchResult, error) {
	configPath := configPathFromTarget(target, a.configPath(scope, workspaceRoot))
	return a.patchConfig(ctx, configPath, material.mutationForUpdate(target), &target)
}

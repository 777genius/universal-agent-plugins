package opencode

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func (a Adapter) applyInstall(ctx context.Context, in ports.ApplyInput) (ports.ApplyResult, error) {
	if in.ResolvedSource == nil {
		return ports.ApplyResult{}, domain.NewError(domain.ErrMutationApply, "OpenCode apply requires resolved source", nil)
	}
	material, err := a.loadSourceMaterial(ctx, in.ResolvedSource.LocalPath, in.Policy.Scope, workspaceRootFromApplyInput(in))
	if err != nil {
		return ports.ApplyResult{}, err
	}
	patch, err := a.patchInstallConfig(ctx, in.Policy.Scope, workspaceRootFromApplyInput(in), material)
	if err != nil {
		return ports.ApplyResult{}, err
	}
	copiedPaths, err := a.copyOwnedFiles(material.CopyFiles)
	if err != nil {
		return ports.ApplyResult{}, err
	}
	return applyProjectionResult(a.ID(), in.Policy.Scope, patch, copiedPaths), nil
}

func (a Adapter) patchInstallConfig(ctx context.Context, scope, workspaceRoot string, material sourceMaterial) (configPatchResult, error) {
	configPath := a.configPath(scope, workspaceRoot)
	return a.patchConfig(ctx, configPath, configMutation{
		WholeSet:   material.WholeFields,
		PluginsSet: material.Plugins,
		MCPSet:     material.MCP,
	}, nil)
}

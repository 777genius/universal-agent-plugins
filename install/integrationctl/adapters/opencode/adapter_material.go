package opencode

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
)

func (a Adapter) loadSourceMaterial(ctx context.Context, sourceRoot, scope string, workspaceRoot string) (sourceMaterial, error) {
	material, err := a.loadBaseSourceMaterial(ctx, sourceRoot)
	if err != nil {
		return sourceMaterial{}, err
	}
	return a.completeSourceMaterial(sourceRoot, scope, workspaceRoot, material)
}

func (m sourceMaterial) mutationForUpdate(target domain.TargetInstallation) configMutation {
	return buildUpdateMutation(m, target)
}

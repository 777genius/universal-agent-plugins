package opencode

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
)

func (a Adapter) patchConfig(ctx context.Context, path string, mutation configMutation, target *domain.TargetInstallation) (configPatchResult, error) {
	state, err := a.loadConfigPatchState(ctx, path)
	if err != nil {
		return configPatchResult{}, err
	}
	if err := ensureConfigMutationCompatible(state, mutation, target); err != nil {
		return configPatchResult{}, err
	}
	if err := applyConfigMutation(state, mutation); err != nil {
		return configPatchResult{}, err
	}
	return a.writeConfigPatch(ctx, path, state, mutation)
}

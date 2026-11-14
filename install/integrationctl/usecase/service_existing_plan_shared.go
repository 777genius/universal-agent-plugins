package usecase

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func shouldPlanExistingAdoptedTargets(action string, sharedResolved *ports.ResolvedSource, sharedManifest *domain.IntegrationManifest) bool {
	return action == "update_version" && sharedResolved != nil && sharedManifest != nil
}

func (s Service) planAdoptedExistingTargets(
	ctx context.Context,
	record domain.InstallationRecord,
	action string,
	sharedResolved *ports.ResolvedSource,
	sharedManifest *domain.IntegrationManifest,
) ([]plannedExistingTarget, []string, error) {
	if !shouldPlanExistingAdoptedTargets(action, sharedResolved, sharedManifest) {
		return nil, nil, nil
	}
	return s.planAdoptedUpdateTargets(ctx, record, *sharedManifest, *sharedResolved)
}

func shouldResolveExistingSharedSource(action string, dryRun bool) bool {
	if action == "update_version" {
		return true
	}
	if !dryRun && (action == "remove_orphaned_target" || action == "repair_drift") {
		return true
	}
	return false
}

func (s Service) resolveExistingSharedSource(
	ctx context.Context,
	record domain.InstallationRecord,
	action string,
	dryRun bool,
) (*ports.ResolvedSource, *domain.IntegrationManifest, func(), error) {
	if !shouldResolveExistingSharedSource(action, dryRun) {
		return nil, nil, func() {}, nil
	}
	resolved, manifest, err := s.resolveCurrentSourceManifest(ctx, record)
	if err != nil {
		return nil, nil, nil, err
	}
	return &resolved, &manifest, func() { cleanupResolvedSource(resolved) }, nil
}

package usecase

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func (s Service) persistExistingToggleState(ctx context.Context, record domain.InstallationRecord, action string, target plannedExistingTarget, applyResult ports.ApplyResult, verified ports.InspectResult, startedAt string) error {
	state, err := s.StateStore.Load(ctx)
	if err != nil {
		return err
	}
	items, found := findInstallationMutable(state.Installations, record.IntegrationID)
	if !found {
		return domain.NewError(domain.ErrStateConflict, "integration disappeared from state during apply: "+record.IntegrationID, nil)
	}
	nextRecord := items
	if target.Manifest != nil {
		nextRecord.ResolvedVersion = target.Manifest.Version
		nextRecord.ResolvedSourceRef = target.Manifest.ResolvedRef
		nextRecord.SourceDigest = target.Manifest.SourceDigest
		nextRecord.ManifestDigest = target.Manifest.ManifestDigest
	}
	nextRecord.LastCheckedAt = startedAt
	nextRecord.LastUpdatedAt = startedAt
	nextRecord.Targets[target.TargetID] = targetInstallationFromExisting(target, applyResult, verified)
	state.Installations = upsertInstallation(state.Installations, nextRecord)
	return s.StateStore.Save(ctx, state)
}

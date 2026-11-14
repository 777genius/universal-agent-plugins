package usecase

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
)

type desiredSyncPlan struct {
	IntegrationID string
	Source        string
	DesiredPolicy domain.InstallPolicy
	Targets       []domain.TargetID
	Record        domain.InstallationRecord
	Action        desiredSyncAction
}

func (s Service) prepareDesiredSyncPlan(
	ctx context.Context,
	item domain.WorkspaceLockIntegration,
	current map[string]domain.InstallationRecord,
) (desiredSyncPlan, error) {
	source, manifest, err := s.resolveDesiredSyncManifest(ctx, item)
	if err != nil {
		return desiredSyncPlan{}, err
	}
	desiredPolicy, targets, err := resolveDesiredSyncTargets(manifest, item)
	if err != nil {
		return desiredSyncPlan{}, desiredSyncManifestErr{
			integrationID: manifest.IntegrationID,
			err:           err,
		}
	}
	record, exists := current[manifest.IntegrationID]
	return desiredSyncPlan{
		IntegrationID: manifest.IntegrationID,
		Source:        source,
		DesiredPolicy: desiredPolicy,
		Targets:       targets,
		Record:        record,
		Action:        classifyDesiredSyncAction(record, exists, source, desiredPolicy, targets, item.Version),
	}, nil
}

type desiredSyncManifestErr struct {
	integrationID string
	err           error
}

func (e desiredSyncManifestErr) Error() string { return e.err.Error() }
func (e desiredSyncManifestErr) Unwrap() error { return e.err }

package usecase

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
)

func (s Service) syncDesiredIntegration(
	ctx context.Context,
	dryRun bool,
	item domain.WorkspaceLockIntegration,
	current map[string]domain.InstallationRecord,
	desiredIDs map[string]struct{},
	report *domain.Report,
) {
	plan, err := s.prepareDesiredSyncPlan(ctx, item, current)
	if err != nil {
		report.Warnings = append(report.Warnings, desiredSyncWarning(item, err))
		return
	}
	desiredIDs[plan.IntegrationID] = struct{}{}
	s.applyDesiredSyncPlan(ctx, dryRun, plan, report)
}

func (s Service) syncDesiredAdd(ctx context.Context, dryRun bool, integrationID, source string, desiredPolicy domain.InstallPolicy, targets []domain.TargetID, report *domain.Report) {
	itemReport, err := s.Add(ctx, AddInput{
		Source:          source,
		Targets:         targetIDsToStrings(targets),
		Scope:           desiredPolicy.Scope,
		AutoUpdate:      boolPtr(desiredPolicy.AutoUpdate),
		AdoptNewTargets: desiredPolicy.AdoptNewTargets,
		AllowPrerelease: boolPtr(desiredPolicy.AllowPrerelease),
		DryRun:          dryRun,
	})
	if err != nil {
		report.Warnings = append(report.Warnings, syncDesiredAddFailedWarning(integrationID, err))
		return
	}
	report.Targets = append(report.Targets, itemReport.Targets...)
}

func (s Service) syncDesiredReplace(ctx context.Context, dryRun bool, record domain.InstallationRecord, integrationID, source string, desiredPolicy domain.InstallPolicy, targets []domain.TargetID, report *domain.Report) {
	if !canReplaceDesiredSync(record, desiredPolicy) {
		report.Warnings = append(report.Warnings, syncDesiredReplaceBlockedWarning(integrationID))
		return
	}
	removeReport, err := s.Remove(ctx, NamedDryRunInput{Name: record.IntegrationID, DryRun: dryRun})
	if err != nil {
		report.Warnings = append(report.Warnings, syncDesiredRemoveBeforeAddWarning(integrationID, err))
		return
	}
	addReport, err := s.Add(ctx, AddInput{
		Source:          source,
		Targets:         targetIDsToStrings(targets),
		Scope:           desiredPolicy.Scope,
		AutoUpdate:      boolPtr(desiredPolicy.AutoUpdate),
		AdoptNewTargets: desiredPolicy.AdoptNewTargets,
		AllowPrerelease: boolPtr(desiredPolicy.AllowPrerelease),
		DryRun:          dryRun,
	})
	if err != nil {
		report.Warnings = append(report.Warnings, syncDesiredReAddFailedWarning(integrationID, err))
		report.Targets = append(report.Targets, removeReport.Targets...)
		return
	}
	report.Targets = append(report.Targets, removeReport.Targets...)
	report.Targets = append(report.Targets, addReport.Targets...)
}

func (s Service) syncDesiredUpdate(ctx context.Context, dryRun bool, integrationID string, report *domain.Report) {
	itemReport, err := s.Update(ctx, NamedDryRunInput{Name: integrationID, DryRun: dryRun})
	if err != nil {
		report.Warnings = append(report.Warnings, syncDesiredUpdateFailedWarning(integrationID, err))
		return
	}
	report.Targets = append(report.Targets, itemReport.Targets...)
}

type desiredSyncAction string

const (
	desiredSyncActionAdd     desiredSyncAction = "add"
	desiredSyncActionReplace desiredSyncAction = "replace"
	desiredSyncActionUpdate  desiredSyncAction = "update"
	desiredSyncActionNoop    desiredSyncAction = "noop"
)

func (s Service) resolveDesiredSyncManifest(ctx context.Context, item domain.WorkspaceLockIntegration) (string, domain.IntegrationManifest, error) {
	source := resolveWorkspaceLockSource(s.WorkspaceLock.Path(), item.Source)
	resolved, manifest, err := s.resolveDesiredSourceManifest(ctx, source)
	if err != nil {
		return source, domain.IntegrationManifest{}, err
	}
	cleanupResolvedSource(resolved)
	return source, manifest, nil
}

func resolveDesiredSyncTargets(manifest domain.IntegrationManifest, item domain.WorkspaceLockIntegration) (domain.InstallPolicy, []domain.TargetID, error) {
	desiredPolicy := desiredPolicyFromLock(item.Policy)
	targets, err := resolveRequestedTargets(manifest, item.Targets)
	if err != nil {
		return domain.InstallPolicy{}, nil, err
	}
	return desiredPolicy, targets, nil
}

func classifyDesiredSyncAction(record domain.InstallationRecord, exists bool, source string, desiredPolicy domain.InstallPolicy, targets []domain.TargetID, desiredVersion string) desiredSyncAction {
	switch {
	case !exists:
		return desiredSyncActionAdd
	case syncNeedsReplace(record, source, desiredPolicy, targets, desiredVersion):
		return desiredSyncActionReplace
	case syncNeedsUpdate(record, source, desiredVersion):
		return desiredSyncActionUpdate
	default:
		return desiredSyncActionNoop
	}
}

func syncDesiredSourceWarning(source string, err error) string {
	return "Sync skipped for source " + source + ": " + err.Error()
}

func syncDesiredManifestWarning(integrationID string, err error) string {
	return "Sync skipped for " + integrationID + ": " + err.Error()
}

func syncDesiredNoopWarning(integrationID string) string {
	return "Sync no-op for " + integrationID + ": desired state already matches workspace intent"
}

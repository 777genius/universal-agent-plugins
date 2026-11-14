package usecase

import (
	"context"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func ensureExistingTargetsNotBlocking(planned []plannedExistingTarget) error {
	for _, target := range planned {
		if target.Plan.Blocking {
			return domain.NewError(domain.ErrMutationApply, "planned mutation is blocked for target "+string(target.TargetID)+"; rerun with --dry-run to inspect manual steps", nil)
		}
	}
	return nil
}

func ensureExistingTargetsResolved(planned []plannedExistingTarget, action string) error {
	for _, target := range planned {
		if target.Manifest == nil || target.Resolved == nil {
			return domain.NewError(domain.ErrMutationApply, action+" requires resolved source and manifest for target "+string(target.TargetID), nil)
		}
	}
	return nil
}

func (s Service) appendExistingInspectPlanSteps(ctx context.Context, operationID string, targetID domain.TargetID) error {
	if err := s.Journal.AppendStep(ctx, operationID, domain.JournalStep{Target: string(targetID), Action: "inspect", Status: "ok"}); err != nil {
		return err
	}
	return s.Journal.AppendStep(ctx, operationID, domain.JournalStep{Target: string(targetID), Action: "plan", Status: "ok"})
}

func targetInstallationFromExisting(item plannedExistingTarget, result ports.ApplyResult, verified ports.InspectResult) domain.TargetInstallation {
	state := result.State
	if verified.State != "" {
		state = verified.State
	}
	activationState := result.ActivationState
	if verified.ActivationState != "" {
		activationState = verified.ActivationState
	}
	interactiveAuthState := result.InteractiveAuthState
	if strings.TrimSpace(verified.InteractiveAuthState) != "" {
		interactiveAuthState = verified.InteractiveAuthState
	}
	environmentRestrictions := append([]domain.EnvironmentRestrictionCode(nil), result.EnvironmentRestrictions...)
	if len(verified.EnvironmentRestrictions) > 0 {
		environmentRestrictions = append([]domain.EnvironmentRestrictionCode(nil), verified.EnvironmentRestrictions...)
	}
	out := domain.TargetInstallation{
		TargetID:                item.TargetID,
		DeliveryKind:            item.Delivery.DeliveryKind,
		CapabilitySurface:       append([]string(nil), item.Delivery.CapabilitySurface...),
		State:                   state,
		NativeRef:               firstNonEmpty(item.Delivery.NativeRefHint, item.Current.NativeRef),
		ActivationState:         activationState,
		InteractiveAuthState:    interactiveAuthState,
		CatalogPolicy:           cloneCatalogPolicy(firstNonNilCatalogPolicy(verified.CatalogPolicy, item.Inspect.CatalogPolicy)),
		EnvironmentRestrictions: environmentRestrictions,
		SourceAccessState:       firstNonEmpty(verified.SourceAccessState, result.SourceAccessState, item.Inspect.SourceAccessState),
		OwnedNativeObjects:      append([]domain.NativeObjectRef(nil), result.OwnedNativeObjects...),
		AdapterMetadata:         cloneMetadata(result.AdapterMetadata),
	}
	if out.State == "" {
		out.State = domain.InstallInstalled
	}
	return out
}

func applyManifestMetadata(record *domain.InstallationRecord, manifest domain.IntegrationManifest, at string) {
	record.ResolvedVersion = manifest.Version
	record.ResolvedSourceRef = manifest.ResolvedRef
	record.SourceDigest = manifest.SourceDigest
	record.ManifestDigest = manifest.ManifestDigest
	record.LastCheckedAt = at
	record.LastUpdatedAt = at
}

func markTargetDegraded(record *domain.InstallationRecord, targetID domain.TargetID) {
	target, ok := record.Targets[targetID]
	if !ok {
		return
	}
	target.State = domain.InstallDegraded
	record.Targets[targetID] = target
}

package usecase

import (
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func installationRecordFromApplied(manifest domain.IntegrationManifest, policy domain.InstallPolicy, workspaceRoot string, startedAt string, applied []appliedTargetInstall) domain.InstallationRecord {
	targets := make(map[domain.TargetID]domain.TargetInstallation, len(applied))
	for _, item := range applied {
		targets[item.Planned.TargetID] = targetInstallationFromApplied(item)
	}
	return domain.InstallationRecord{
		IntegrationID:      manifest.IntegrationID,
		RequestedSourceRef: manifest.RequestedRef,
		ResolvedSourceRef:  manifest.ResolvedRef,
		ResolvedVersion:    manifest.Version,
		SourceDigest:       manifest.SourceDigest,
		ManifestDigest:     manifest.ManifestDigest,
		Policy:             policy,
		WorkspaceRoot:      workspaceRoot,
		Targets:            targets,
		LastCheckedAt:      startedAt,
		LastUpdatedAt:      startedAt,
	}
}

func degradedRecordFromApplied(manifest domain.IntegrationManifest, policy domain.InstallPolicy, workspaceRoot string, startedAt string, applied []appliedTargetInstall) domain.InstallationRecord {
	record := installationRecordFromApplied(manifest, policy, workspaceRoot, startedAt, applied)
	for key, target := range record.Targets {
		target.State = domain.InstallDegraded
		record.Targets[key] = target
	}
	return record
}

func targetInstallationFromApplied(item appliedTargetInstall) domain.TargetInstallation {
	state := item.Result.State
	if item.Verify.State != "" {
		state = item.Verify.State
	}
	activationState := item.Result.ActivationState
	if item.Verify.ActivationState != "" {
		activationState = item.Verify.ActivationState
	}
	interactiveAuthState := item.Result.InteractiveAuthState
	if strings.TrimSpace(item.Verify.InteractiveAuthState) != "" {
		interactiveAuthState = item.Verify.InteractiveAuthState
	}
	environmentRestrictions := append([]domain.EnvironmentRestrictionCode(nil), item.Result.EnvironmentRestrictions...)
	if len(item.Verify.EnvironmentRestrictions) > 0 {
		environmentRestrictions = append([]domain.EnvironmentRestrictionCode(nil), item.Verify.EnvironmentRestrictions...)
	}
	return domain.TargetInstallation{
		TargetID:                item.Planned.TargetID,
		DeliveryKind:            item.Planned.Delivery.DeliveryKind,
		CapabilitySurface:       append([]string(nil), item.Planned.Delivery.CapabilitySurface...),
		State:                   state,
		NativeRef:               item.Planned.Delivery.NativeRefHint,
		ActivationState:         activationState,
		InteractiveAuthState:    interactiveAuthState,
		CatalogPolicy:           cloneCatalogPolicy(firstNonNilCatalogPolicy(item.Verify.CatalogPolicy, item.Planned.Inspect.CatalogPolicy)),
		EnvironmentRestrictions: environmentRestrictions,
		SourceAccessState:       firstNonEmpty(item.Verify.SourceAccessState, item.Result.SourceAccessState, item.Planned.Inspect.SourceAccessState),
		OwnedNativeObjects:      append([]domain.NativeObjectRef(nil), item.Result.OwnedNativeObjects...),
		AdapterMetadata:         cloneMetadata(item.Result.AdapterMetadata),
	}
}

func provisionalRecordForAdd(manifest domain.IntegrationManifest, policy domain.InstallPolicy, workspaceRoot string, target plannedTargetInstall, result ports.ApplyResult) domain.InstallationRecord {
	return domain.InstallationRecord{
		IntegrationID:      manifest.IntegrationID,
		RequestedSourceRef: manifest.RequestedRef,
		ResolvedSourceRef:  manifest.ResolvedRef,
		ResolvedVersion:    manifest.Version,
		SourceDigest:       manifest.SourceDigest,
		ManifestDigest:     manifest.ManifestDigest,
		Policy:             policy,
		WorkspaceRoot:      workspaceRoot,
		Targets: map[domain.TargetID]domain.TargetInstallation{
			target.TargetID: {
				TargetID:           target.TargetID,
				DeliveryKind:       target.Delivery.DeliveryKind,
				CapabilitySurface:  append([]string(nil), target.Delivery.CapabilitySurface...),
				NativeRef:          target.Delivery.NativeRefHint,
				OwnedNativeObjects: append([]domain.NativeObjectRef(nil), result.OwnedNativeObjects...),
				AdapterMetadata:    cloneMetadata(result.AdapterMetadata),
			},
		},
	}
}

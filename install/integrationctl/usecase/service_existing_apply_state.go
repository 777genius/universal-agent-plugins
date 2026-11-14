package usecase

import (
	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func markPlannedTargetDegraded(record *domain.InstallationRecord, target plannedExistingTarget) {
	if _, ok := record.Targets[target.TargetID]; ok {
		markTargetDegraded(record, target.TargetID)
		return
	}
	record.Targets[target.TargetID] = domain.TargetInstallation{
		TargetID:          target.TargetID,
		DeliveryKind:      target.Delivery.DeliveryKind,
		CapabilitySurface: append([]string(nil), target.Delivery.CapabilitySurface...),
		State:             domain.InstallDegraded,
		NativeRef:         target.Delivery.NativeRefHint,
		ActivationState:   target.Inspect.ActivationState,
		CatalogPolicy:     cloneCatalogPolicy(target.Inspect.CatalogPolicy),
		EnvironmentRestrictions: append([]domain.EnvironmentRestrictionCode(nil),
			target.Inspect.EnvironmentRestrictions...,
		),
		SourceAccessState: target.Inspect.SourceAccessState,
	}
}

func provisionalRecordForExisting(record domain.InstallationRecord, target plannedExistingTarget, result ports.ApplyResult) domain.InstallationRecord {
	next := cloneInstallationRecord(record)
	if next.Targets == nil {
		next.Targets = map[domain.TargetID]domain.TargetInstallation{}
	}
	next.Targets[target.TargetID] = targetInstallationFromExisting(target, result, ports.InspectResult{})
	if target.Manifest != nil {
		applyManifestMetadata(&next, *target.Manifest, record.LastUpdatedAt)
	}
	return next
}

package usecase

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func (s Service) loadExistingTargetBase(ctx context.Context, record domain.InstallationRecord, targetID domain.TargetID) (plannedExistingTarget, error) {
	target, err := loadExistingTargetRecord(record, targetID)
	if err != nil {
		return plannedExistingTarget{}, err
	}
	adapter, err := s.loadExistingTargetAdapter(targetID)
	if err != nil {
		return plannedExistingTarget{}, err
	}
	inspect, err := inspectExistingTargetAdapter(ctx, adapter, record)
	if err != nil {
		return plannedExistingTarget{}, err
	}
	return plannedExistingTarget{
		TargetID: targetID,
		Current:  target,
		Delivery: existingTargetDelivery(targetID, target),
		Adapter:  adapter,
		Inspect:  inspect,
	}, nil
}

func loadExistingTargetRecord(record domain.InstallationRecord, targetID domain.TargetID) (domain.TargetInstallation, error) {
	target, ok := record.Targets[targetID]
	if !ok {
		return domain.TargetInstallation{}, domain.NewError(domain.ErrStateConflict, "target missing from installation record: "+string(targetID), nil)
	}
	return target, nil
}

func (s Service) loadExistingTargetAdapter(targetID domain.TargetID) (ports.TargetAdapter, error) {
	adapter, ok := s.Adapters[targetID]
	if !ok {
		return nil, domain.NewError(domain.ErrUnsupportedTarget, "adapter not registered for "+string(targetID), nil)
	}
	return adapter, nil
}

func inspectExistingTargetAdapter(ctx context.Context, adapter ports.TargetAdapter, record domain.InstallationRecord) (ports.InspectResult, error) {
	return adapter.Inspect(ctx, ports.InspectInput{IntegrationID: record.IntegrationID, Record: &record, Scope: record.Policy.Scope})
}

func existingTargetDelivery(targetID domain.TargetID, target domain.TargetInstallation) domain.Delivery {
	return domain.Delivery{
		TargetID:      targetID,
		DeliveryKind:  target.DeliveryKind,
		NativeRefHint: target.NativeRef,
	}
}

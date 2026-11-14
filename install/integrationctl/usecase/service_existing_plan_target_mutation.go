package usecase

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func (s Service) planExistingMutation(ctx context.Context, record domain.InstallationRecord, item plannedExistingTarget, sharedResolved *ports.ResolvedSource, sharedManifest *domain.IntegrationManifest, repair bool) (plannedExistingTarget, error) {
	resolved, manifest, err := s.resolveExistingMutationSource(ctx, record, sharedResolved, sharedManifest)
	if err != nil {
		return plannedExistingTarget{}, err
	}
	nextDelivery, err := requireExistingMutationDelivery(manifest, item.TargetID)
	if err != nil {
		return plannedExistingTarget{}, err
	}
	plan, err := item.Adapter.PlanUpdate(ctx, ports.PlanUpdateInput{
		CurrentRecord: record,
		NextManifest:  manifest,
		Inspect:       item.Inspect,
	})
	if err != nil {
		return plannedExistingTarget{}, err
	}
	if repair {
		plan.ActionClass = "repair_drift"
		plan.Summary = "Repair managed drift for target " + string(item.TargetID)
	}
	if _, err := s.validateEvidence(ctx, item.TargetID, plan.EvidenceKey); err != nil {
		return plannedExistingTarget{}, err
	}
	return finalizeExistingMutationPlan(record, item, *nextDelivery, plan, resolved, manifest), nil
}

func (s Service) resolveExistingMutationSource(ctx context.Context, record domain.InstallationRecord, sharedResolved *ports.ResolvedSource, sharedManifest *domain.IntegrationManifest) (ports.ResolvedSource, domain.IntegrationManifest, error) {
	if sharedResolved != nil && sharedManifest != nil {
		return *sharedResolved, *sharedManifest, nil
	}
	return s.resolveCurrentSourceManifest(ctx, record)
}

func requireExistingMutationDelivery(manifest domain.IntegrationManifest, targetID domain.TargetID) (*domain.Delivery, error) {
	nextDelivery := findDelivery(manifest.Deliveries, targetID)
	if nextDelivery == nil {
		return nil, domain.NewError(domain.ErrUnsupportedTarget, "updated manifest no longer exposes target "+string(targetID), nil)
	}
	return nextDelivery, nil
}

func finalizeExistingMutationPlan(record domain.InstallationRecord, item plannedExistingTarget, delivery domain.Delivery, plan ports.AdapterPlan, resolved ports.ResolvedSource, manifest domain.IntegrationManifest) plannedExistingTarget {
	item.Delivery = delivery
	item.Plan = plan
	item.Manifest = &manifest
	item.Resolved = &resolved
	item.Report = toTargetReport(record.IntegrationID, delivery, item.Inspect, plan)
	return item
}

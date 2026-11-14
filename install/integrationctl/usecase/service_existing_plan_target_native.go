package usecase

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func (s Service) planExistingRemoval(ctx context.Context, record domain.InstallationRecord, item plannedExistingTarget, sharedResolved *ports.ResolvedSource, sharedManifest *domain.IntegrationManifest) (plannedExistingTarget, error) {
	plan, err := item.Adapter.PlanRemove(ctx, ports.PlanRemoveInput{Record: record, Inspect: item.Inspect})
	if err != nil {
		return plannedExistingTarget{}, err
	}
	if _, err := s.validateEvidence(ctx, item.TargetID, plan.EvidenceKey); err != nil {
		return plannedExistingTarget{}, err
	}
	return finalizeExistingRemovalPlan(record, item, plan, sharedResolved, sharedManifest), nil
}

func finalizeExistingRemovalPlan(record domain.InstallationRecord, item plannedExistingTarget, plan ports.AdapterPlan, sharedResolved *ports.ResolvedSource, sharedManifest *domain.IntegrationManifest) plannedExistingTarget {
	item.Plan = plan
	item.Manifest = cloneManifestPtr(sharedManifest)
	item.Resolved = cloneResolvedPtr(sharedResolved)
	item.Report = toTargetReport(record.IntegrationID, item.Delivery, item.Inspect, plan)
	return item
}

func (s Service) planExistingToggle(ctx context.Context, record domain.InstallationRecord, item plannedExistingTarget, enable bool) (plannedExistingTarget, error) {
	toggle, err := existingToggleAdapter(item, enable)
	if err != nil {
		return plannedExistingTarget{}, err
	}
	var (
		plan    ports.AdapterPlan
		planErr error
	)
	if enable {
		plan, planErr = toggle.PlanEnable(ctx, ports.PlanToggleInput{Record: record, Inspect: item.Inspect})
	} else {
		plan, planErr = toggle.PlanDisable(ctx, ports.PlanToggleInput{Record: record, Inspect: item.Inspect})
	}
	if planErr != nil {
		return plannedExistingTarget{}, planErr
	}
	if _, err := s.validateEvidence(ctx, item.TargetID, plan.EvidenceKey); err != nil {
		return plannedExistingTarget{}, err
	}
	return finalizeExistingTogglePlan(record, item, plan), nil
}

func existingToggleAdapter(item plannedExistingTarget, enable bool) (ports.ToggleTargetAdapter, error) {
	toggle, ok := item.Adapter.(ports.ToggleTargetAdapter)
	if !ok {
		return nil, domain.NewError(domain.ErrUnsupportedTarget, "target "+string(item.TargetID)+" does not support "+existingToggleAction(enable), nil)
	}
	return toggle, nil
}

func existingToggleAction(enable bool) string {
	if enable {
		return "enable"
	}
	return "disable"
}

func finalizeExistingTogglePlan(record domain.InstallationRecord, item plannedExistingTarget, plan ports.AdapterPlan) plannedExistingTarget {
	item.Plan = plan
	item.Report = toTargetReport(record.IntegrationID, item.Delivery, item.Inspect, plan)
	return item
}

func cloneManifestPtr(manifest *domain.IntegrationManifest) *domain.IntegrationManifest {
	if manifest == nil {
		return nil
	}
	manifestCopy := *manifest
	return &manifestCopy
}

func cloneResolvedPtr(resolved *ports.ResolvedSource) *ports.ResolvedSource {
	if resolved == nil {
		return nil
	}
	resolvedCopy := *resolved
	return &resolvedCopy
}

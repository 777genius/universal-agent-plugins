package usecase

import (
	"context"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func (s Service) planTargetInstall(ctx context.Context, manifest domain.IntegrationManifest, policy domain.InstallPolicy, target domain.TargetID) (plannedTargetInstall, error) {
	adapter, ok := s.Adapters[target]
	if !ok {
		return plannedTargetInstall{}, domain.NewError(domain.ErrUnsupportedTarget, "adapter not registered for "+string(target), nil)
	}
	delivery := findDelivery(manifest.Deliveries, target)
	if delivery == nil {
		return plannedTargetInstall{}, domain.NewError(domain.ErrUnsupportedTarget, "delivery not available for "+string(target), nil)
	}
	inspect, err := adapter.Inspect(ctx, ports.InspectInput{IntegrationID: manifest.IntegrationID, Scope: policy.Scope})
	if err != nil {
		return plannedTargetInstall{}, err
	}
	plan, err := adapter.PlanInstall(ctx, ports.PlanInstallInput{Manifest: manifest, Policy: policy, Inspect: inspect})
	if err != nil {
		return plannedTargetInstall{}, err
	}
	if _, err := s.validateEvidence(ctx, target, plan.EvidenceKey); err != nil {
		return plannedTargetInstall{}, err
	}
	return plannedTargetInstall{
		TargetID: target,
		Delivery: *delivery,
		Adapter:  adapter,
		Inspect:  inspect,
		Plan:     plan,
	}, nil
}

func (s Service) validateEvidence(ctx context.Context, target domain.TargetID, key string) (ports.EvidenceEntry, error) {
	if strings.TrimSpace(key) == "" {
		return ports.EvidenceEntry{}, domain.NewError(domain.ErrEvidenceViolation, "adapter plan missing evidence key for "+string(target), nil)
	}
	entry, err := s.Evidence.Get(ctx, key)
	if err != nil {
		return ports.EvidenceEntry{}, domain.NewError(domain.ErrEvidenceViolation, "unknown evidence key "+key, err)
	}
	return entry, nil
}

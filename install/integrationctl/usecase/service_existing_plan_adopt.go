package usecase

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func (s Service) planAdoptedUpdateTargets(ctx context.Context, record domain.InstallationRecord, manifest domain.IntegrationManifest, resolved ports.ResolvedSource) ([]plannedExistingTarget, []string, error) {
	autoAdopt := autoAdoptNewTargets(record)
	out := []plannedExistingTarget{}
	warnings := []string{}
	for _, delivery := range missingAdoptedDeliveries(record, manifest) {
		item, warning, err := s.planAdoptedUpdateTarget(ctx, record, manifest, resolved, delivery, autoAdopt)
		if err != nil {
			return nil, nil, err
		}
		if strings.TrimSpace(warning) != "" {
			warnings = append(warnings, warning)
			continue
		}
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].TargetID < out[j].TargetID })
	return out, warnings, nil
}

func missingAdoptedDeliveries(record domain.InstallationRecord, manifest domain.IntegrationManifest) []domain.Delivery {
	existing := make(map[domain.TargetID]struct{}, len(record.Targets))
	for targetID := range record.Targets {
		existing[targetID] = struct{}{}
	}
	var out []domain.Delivery
	for _, delivery := range manifest.Deliveries {
		if _, ok := existing[delivery.TargetID]; ok {
			continue
		}
		out = append(out, delivery)
	}
	return out
}

func autoAdoptNewTargets(record domain.InstallationRecord) bool {
	return strings.EqualFold(strings.TrimSpace(record.Policy.AdoptNewTargets), "auto")
}

func (s Service) planAdoptedUpdateTarget(ctx context.Context, record domain.InstallationRecord, manifest domain.IntegrationManifest, resolved ports.ResolvedSource, delivery domain.Delivery, autoAdopt bool) (plannedExistingTarget, string, error) {
	if !autoAdopt {
		return plannedExistingTarget{}, adoptedUpdateManualWarning(record, delivery), nil
	}
	return s.planAutomaticAdoptedUpdateTarget(ctx, record, manifest, resolved, delivery)
}

func adoptedUpdateManualWarning(record domain.InstallationRecord, delivery domain.Delivery) string {
	return fmt.Sprintf("New target support is available for %s on %s, but adopt_new_targets=%s.", record.IntegrationID, delivery.TargetID, defaultString(record.Policy.AdoptNewTargets, "manual"))
}

func adoptedUpdateMissingAdapterWarning(record domain.InstallationRecord, targetID domain.TargetID) string {
	return fmt.Sprintf("Automatic adoption skipped for %s on %s: no adapter is registered.", record.IntegrationID, targetID)
}

func adoptedUpdateBlockedWarning(record domain.InstallationRecord, targetID domain.TargetID) string {
	return fmt.Sprintf("Automatic adoption skipped for %s on %s: native environment blocks installation.", record.IntegrationID, targetID)
}

func (s Service) planAutomaticAdoptedUpdateTarget(ctx context.Context, record domain.InstallationRecord, manifest domain.IntegrationManifest, resolved ports.ResolvedSource, delivery domain.Delivery) (plannedExistingTarget, string, error) {
	adapter, ok := s.adoptedUpdateAdapter(delivery.TargetID)
	if !ok {
		return plannedExistingTarget{}, adoptedUpdateMissingAdapterWarning(record, delivery.TargetID), nil
	}
	inspect, err := inspectAdoptedUpdateTarget(ctx, adapter, record)
	if err != nil {
		return plannedExistingTarget{}, "", err
	}
	plan, err := s.planAdoptedInstall(ctx, adapter, record, manifest, inspect, delivery.TargetID)
	if err != nil {
		return plannedExistingTarget{}, "", err
	}
	if adoptedUpdateShouldWarnBlocked(plan) {
		return plannedExistingTarget{}, adoptedUpdateBlockedWarning(record, delivery.TargetID), nil
	}
	return finalizeAdoptedExistingTarget(record, delivery, adapter, inspect, plan, manifest, resolved), "", nil
}

func (s Service) adoptedUpdateAdapter(targetID domain.TargetID) (ports.TargetAdapter, bool) {
	adapter, ok := s.Adapters[targetID]
	return adapter, ok
}

func inspectAdoptedUpdateTarget(ctx context.Context, adapter ports.TargetAdapter, record domain.InstallationRecord) (ports.InspectResult, error) {
	return adapter.Inspect(ctx, ports.InspectInput{IntegrationID: record.IntegrationID, Record: &record, Scope: record.Policy.Scope})
}

func (s Service) planAdoptedInstall(
	ctx context.Context,
	adapter ports.TargetAdapter,
	record domain.InstallationRecord,
	manifest domain.IntegrationManifest,
	inspect ports.InspectResult,
	targetID domain.TargetID,
) (ports.AdapterPlan, error) {
	plan, err := adapter.PlanInstall(ctx, ports.PlanInstallInput{
		Manifest: manifest,
		Policy:   record.Policy,
		Inspect:  inspect,
	})
	if err != nil {
		return ports.AdapterPlan{}, err
	}
	plan = normalizeAdoptedInstallPlan(plan, targetID)
	if _, err := s.validateEvidence(ctx, targetID, plan.EvidenceKey); err != nil {
		return ports.AdapterPlan{}, err
	}
	return plan, nil
}

func normalizeAdoptedInstallPlan(plan ports.AdapterPlan, targetID domain.TargetID) ports.AdapterPlan {
	plan.ActionClass = "adopt_new_target"
	plan.Summary = "Adopt newly supported target " + string(targetID)
	return plan
}

func adoptedUpdateShouldWarnBlocked(plan ports.AdapterPlan) bool {
	return plan.Blocking
}

func finalizeAdoptedExistingTarget(record domain.InstallationRecord, delivery domain.Delivery, adapter ports.TargetAdapter, inspect ports.InspectResult, plan ports.AdapterPlan, manifest domain.IntegrationManifest, resolved ports.ResolvedSource) plannedExistingTarget {
	resolvedCopy := resolved
	manifestCopy := manifest
	return newAdoptedExistingTarget(record.IntegrationID, delivery, adapter, inspect, plan, &manifestCopy, &resolvedCopy)
}

func newAdoptedExistingTarget(integrationID string, delivery domain.Delivery, adapter ports.TargetAdapter, inspect ports.InspectResult, plan ports.AdapterPlan, manifest *domain.IntegrationManifest, resolved *ports.ResolvedSource) plannedExistingTarget {
	return plannedExistingTarget{
		TargetID: delivery.TargetID,
		Delivery: delivery,
		Adapter:  adapter,
		Inspect:  inspect,
		Plan:     plan,
		Manifest: manifest,
		Resolved: resolved,
		Report:   toTargetReport(integrationID, delivery, inspect, plan),
		Adopted:  true,
	}
}

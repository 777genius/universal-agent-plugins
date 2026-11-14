package usecase

import (
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func appendListTargets(report *domain.Report, inst domain.InstallationRecord) {
	for _, targetID := range sortedTargets(inst.Targets) {
		ti := inst.Targets[targetID]
		report.Targets = append(report.Targets, domain.TargetReport{
			IntegrationID:     inst.IntegrationID,
			TargetID:          string(targetID),
			DeliveryKind:      string(ti.DeliveryKind),
			CapabilitySurface: append([]string(nil), ti.CapabilitySurface...),
			State:             string(ti.State),
			ActivationState:   string(ti.ActivationState),
			CatalogPolicy:     cloneCatalogPolicy(ti.CatalogPolicy),
			SourceAccessState: ti.SourceAccessState,
		})
	}
}

func toTargetReport(integrationID string, delivery domain.Delivery, inspect ports.InspectResult, plan ports.AdapterPlan) domain.TargetReport {
	report := domain.TargetReport{
		IntegrationID:            integrationID,
		TargetID:                 string(delivery.TargetID),
		DeliveryKind:             string(delivery.DeliveryKind),
		CapabilitySurface:        append([]string(nil), delivery.CapabilitySurface...),
		ActionClass:              plan.ActionClass,
		State:                    string(inspect.State),
		ActivationState:          string(inspect.ActivationState),
		InteractiveAuthState:     inspect.InteractiveAuthState,
		RestartRequired:          plan.RestartRequired,
		ReloadRequired:           plan.ReloadRequired,
		NewThreadRequired:        plan.NewThreadRequired,
		CatalogPolicy:            cloneCatalogPolicy(inspect.CatalogPolicy),
		VolatileOverrideDetected: inspect.VolatileOverrideDetected,
		TrustResolutionSource:    inspect.TrustResolutionSource,
		SourceAccessState:        inspect.SourceAccessState,
		EvidenceKey:              plan.EvidenceKey,
		ManualSteps:              append([]string(nil), plan.ManualSteps...),
	}
	for _, restriction := range inspect.EnvironmentRestrictions {
		report.EnvironmentRestrictions = append(report.EnvironmentRestrictions, string(restriction))
	}
	return report
}

func toAppliedTargetReport(integrationID string, delivery domain.Delivery, inspect ports.InspectResult, verified ports.InspectResult, plan ports.AdapterPlan, result ports.ApplyResult) domain.TargetReport {
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
	sourceAccessState := firstNonEmpty(verified.SourceAccessState, result.SourceAccessState)
	environmentRestrictions := append([]domain.EnvironmentRestrictionCode(nil), result.EnvironmentRestrictions...)
	if len(verified.EnvironmentRestrictions) > 0 {
		environmentRestrictions = append([]domain.EnvironmentRestrictionCode(nil), verified.EnvironmentRestrictions...)
	}
	report := domain.TargetReport{
		IntegrationID:        integrationID,
		TargetID:             string(delivery.TargetID),
		DeliveryKind:         string(delivery.DeliveryKind),
		CapabilitySurface:    append([]string(nil), delivery.CapabilitySurface...),
		ActionClass:          plan.ActionClass,
		State:                string(state),
		ActivationState:      string(activationState),
		InteractiveAuthState: interactiveAuthState,
		RestartRequired:      result.RestartRequired,
		ReloadRequired:       result.ReloadRequired,
		NewThreadRequired:    result.NewThreadRequired,
		CatalogPolicy:        cloneCatalogPolicy(firstNonNilCatalogPolicy(verified.CatalogPolicy, inspect.CatalogPolicy)),
		SourceAccessState:    sourceAccessState,
		EvidenceKey:          plan.EvidenceKey,
		ManualSteps:          append([]string(nil), result.ManualSteps...),
	}
	for _, restriction := range environmentRestrictions {
		report.EnvironmentRestrictions = append(report.EnvironmentRestrictions, string(restriction))
	}
	return report
}

func restrictionsToStrings(in []domain.EnvironmentRestrictionCode) []string {
	out := make([]string, 0, len(in))
	for _, restriction := range in {
		out = append(out, string(restriction))
	}
	return out
}

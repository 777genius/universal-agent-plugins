package usecase

import (
	"sort"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
)

func buildDoctorReport(installations []domain.InstallationRecord) (domain.Report, int, int, int) {
	var degradedCount, activationPendingCount, authPendingCount int
	report := domain.Report{}
	for _, inst := range installations {
		for _, targetID := range sortedTargets(inst.Targets) {
			ti := inst.Targets[targetID]
			if !doctorTargetNeedsAttention(ti) {
				continue
			}
			switch ti.State {
			case domain.InstallDegraded:
				degradedCount++
			case domain.InstallActivationPending:
				activationPendingCount++
			case domain.InstallAuthPending:
				authPendingCount++
			}
			report.Targets = append(report.Targets, domain.TargetReport{
				IntegrationID:           inst.IntegrationID,
				TargetID:                string(targetID),
				DeliveryKind:            string(ti.DeliveryKind),
				CapabilitySurface:       append([]string(nil), ti.CapabilitySurface...),
				ActionClass:             "doctor_attention",
				State:                   string(ti.State),
				ActivationState:         string(ti.ActivationState),
				InteractiveAuthState:    ti.InteractiveAuthState,
				CatalogPolicy:           cloneCatalogPolicy(ti.CatalogPolicy),
				EnvironmentRestrictions: restrictionsToStrings(ti.EnvironmentRestrictions),
				SourceAccessState:       ti.SourceAccessState,
				ManualSteps:             doctorManualSteps(inst.IntegrationID, ti),
			})
		}
	}
	return report, degradedCount, activationPendingCount, authPendingCount
}

func sortDoctorTargets(targets []domain.TargetReport) {
	sort.Slice(targets, func(i, j int) bool {
		if targets[i].IntegrationID == targets[j].IntegrationID {
			if targets[i].TargetID == targets[j].TargetID {
				return targets[i].DeliveryKind < targets[j].DeliveryKind
			}
			return targets[i].TargetID < targets[j].TargetID
		}
		return targets[i].IntegrationID < targets[j].IntegrationID
	})
}

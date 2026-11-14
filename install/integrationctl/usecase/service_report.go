package usecase

import (
	"context"
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
)

func (s Service) list(ctx context.Context) (domain.Report, error) {
	state, err := s.StateStore.Load(ctx)
	if err != nil {
		return domain.Report{}, err
	}
	if len(state.Installations) == 0 {
		return domain.Report{Summary: "No managed integrations are installed yet."}, nil
	}
	report := domain.Report{Summary: fmt.Sprintf("%d managed integration(s) in state.", len(state.Installations))}
	for _, inst := range state.Installations {
		appendListTargets(&report, inst)
	}
	return report, nil
}

func (s Service) doctor(ctx context.Context) (domain.Report, error) {
	state, err := s.StateStore.Load(ctx)
	if err != nil {
		return domain.Report{}, err
	}
	openOps, err := s.Journal.ListOpen(ctx)
	if err != nil {
		return domain.Report{}, err
	}
	report, degradedCount, activationPendingCount, authPendingCount := buildDoctorReport(state.Installations)
	report.Summary = fmt.Sprintf("Doctor: %d installation(s), %d open operation journal(s), %d degraded target(s), %d activation-pending target(s), %d auth-pending target(s).", len(state.Installations), len(openOps), degradedCount, activationPendingCount, authPendingCount)
	for _, op := range openOps {
		report.Warnings = append(report.Warnings, doctorWarningForOperation(op))
	}
	sortDoctorTargets(report.Targets)
	return report, nil
}

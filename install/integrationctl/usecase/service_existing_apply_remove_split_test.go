package usecase

import (
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func TestValidateExistingRemoveTargetsRejectsEmptyPlans(t *testing.T) {
	t.Parallel()

	err := validateExistingRemoveTargets(nil)
	if err == nil || !strings.Contains(err.Error(), "remove requires at least one planned target") {
		t.Fatalf("error = %v", err)
	}
}

func TestFinalizeExistingRemoveStateRemovesInstallationWhenNoTargetsRemain(t *testing.T) {
	t.Parallel()

	state := ports.StateFile{
		Installations: []domain.InstallationRecord{{IntegrationID: "demo"}},
	}
	got := finalizeExistingRemoveState(state, domain.InstallationRecord{IntegrationID: "demo", Targets: map[domain.TargetID]domain.TargetInstallation{}}, "2026-01-01T00:00:00Z")
	if len(got.Installations) != 0 {
		t.Fatalf("state = %+v", got)
	}
}

func TestExistingRemoveReportSortsTargets(t *testing.T) {
	t.Parallel()

	report := existingRemoveReport("op", "demo", []domain.TargetReport{
		{TargetID: "gemini"},
		{TargetID: "claude"},
	})
	if report.Targets[0].TargetID != "claude" || report.Targets[1].TargetID != "gemini" {
		t.Fatalf("report = %+v", report)
	}
}

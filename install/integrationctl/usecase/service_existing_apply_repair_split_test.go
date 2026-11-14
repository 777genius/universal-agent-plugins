package usecase

import (
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func TestValidateExistingRepairTargetsRejectsEmptyPlans(t *testing.T) {
	t.Parallel()

	err := validateExistingRepairTargets(nil)
	if err == nil || !strings.Contains(err.Error(), "repair requires at least one planned target") {
		t.Fatalf("error = %v", err)
	}
}

func TestFinalizeExistingRepairStateUpsertsRecord(t *testing.T) {
	t.Parallel()

	state := ports.StateFile{}
	record := domain.InstallationRecord{IntegrationID: "demo"}
	got := finalizeExistingRepairState(state, record)
	if len(got.Installations) != 1 || got.Installations[0].IntegrationID != "demo" {
		t.Fatalf("state = %+v", got)
	}
}

func TestExistingRepairReportSortsTargets(t *testing.T) {
	t.Parallel()

	report := existingRepairReport("op", "demo", []domain.TargetReport{
		{TargetID: "gemini"},
		{TargetID: "claude"},
	})
	if report.Targets[0].TargetID != "claude" || report.Targets[1].TargetID != "gemini" {
		t.Fatalf("report = %+v", report)
	}
}

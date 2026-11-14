package usecase

import (
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func TestAutoAdoptNewTargetsTreatsTrimmedAutoAsEnabled(t *testing.T) {
	t.Parallel()

	record := domain.InstallationRecord{
		Policy: domain.InstallPolicy{AdoptNewTargets: " Auto "},
	}
	if !autoAdoptNewTargets(record) {
		t.Fatal("expected auto adopt policy to be enabled")
	}
}

func TestMissingAdoptedDeliveriesSkipsExistingTargets(t *testing.T) {
	t.Parallel()

	record := domain.InstallationRecord{
		Targets: map[domain.TargetID]domain.TargetInstallation{
			domain.TargetCursor: {TargetID: domain.TargetCursor},
		},
	}
	manifest := domain.IntegrationManifest{
		Deliveries: []domain.Delivery{
			{TargetID: domain.TargetCursor},
			{TargetID: domain.TargetGemini},
			{TargetID: domain.TargetCodex},
		},
	}
	got := missingAdoptedDeliveries(record, manifest)
	if len(got) != 2 || got[0].TargetID != domain.TargetGemini || got[1].TargetID != domain.TargetCodex {
		t.Fatalf("deliveries = %+v", got)
	}
}

func TestAdoptedUpdateManualWarningPreservesPolicyMessage(t *testing.T) {
	t.Parallel()

	record := domain.InstallationRecord{
		IntegrationID: "demo",
		Policy:        domain.InstallPolicy{AdoptNewTargets: "manual"},
	}
	got := adoptedUpdateManualWarning(record, domain.Delivery{TargetID: domain.TargetGemini})
	want := "New target support is available for demo on gemini, but adopt_new_targets=manual."
	if got != want {
		t.Fatalf("warning = %q, want %q", got, want)
	}
}

func TestAdoptedUpdateBlockedWarningPreservesTargetMessage(t *testing.T) {
	t.Parallel()

	record := domain.InstallationRecord{IntegrationID: "demo"}
	got := adoptedUpdateBlockedWarning(record, domain.TargetGemini)
	want := "Automatic adoption skipped for demo on gemini: native environment blocks installation."
	if got != want {
		t.Fatalf("warning = %q, want %q", got, want)
	}
}

func TestAdoptedUpdateMissingAdapterWarningPreservesTargetMessage(t *testing.T) {
	t.Parallel()

	record := domain.InstallationRecord{IntegrationID: "demo"}
	got := adoptedUpdateMissingAdapterWarning(record, domain.TargetCodex)
	want := "Automatic adoption skipped for demo on codex: no adapter is registered."
	if got != want {
		t.Fatalf("warning = %q, want %q", got, want)
	}
}

func TestNormalizeAdoptedInstallPlanProjectsActionAndSummary(t *testing.T) {
	t.Parallel()

	got := normalizeAdoptedInstallPlan(ports.AdapterPlan{}, domain.TargetGemini)
	if got.ActionClass != "adopt_new_target" || got.Summary != "Adopt newly supported target gemini" {
		t.Fatalf("plan = %+v", got)
	}
}

func TestAdoptedUpdateShouldWarnBlockedMatchesPlanState(t *testing.T) {
	t.Parallel()

	if !adoptedUpdateShouldWarnBlocked(ports.AdapterPlan{Blocking: true}) {
		t.Fatal("expected blocking plan to warn")
	}
	if adoptedUpdateShouldWarnBlocked(ports.AdapterPlan{Blocking: false}) {
		t.Fatal("expected non-blocking plan to skip warning")
	}
}

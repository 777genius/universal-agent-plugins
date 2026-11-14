package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func TestShouldResolveExistingSharedSourceMatchesLifecyclePolicy(t *testing.T) {
	t.Parallel()

	cases := []struct {
		action string
		dryRun bool
		want   bool
	}{
		{action: "update_version", dryRun: true, want: true},
		{action: "update_version", dryRun: false, want: true},
		{action: "remove_orphaned_target", dryRun: true, want: false},
		{action: "remove_orphaned_target", dryRun: false, want: true},
		{action: "repair_drift", dryRun: true, want: false},
		{action: "repair_drift", dryRun: false, want: true},
		{action: "enable_target", dryRun: false, want: false},
	}
	for _, tc := range cases {
		if got := shouldResolveExistingSharedSource(tc.action, tc.dryRun); got != tc.want {
			t.Fatalf("shouldResolveExistingSharedSource(%q, %v) = %v, want %v", tc.action, tc.dryRun, got, tc.want)
		}
	}
}

func TestNewExistingPlanReportPreservesSummaryContract(t *testing.T) {
	t.Parallel()

	report := newExistingPlanReport("update_version", "demo", time.Unix(0, 0).UTC())
	if report.Summary != `Dry-run update_version plan for "demo".` {
		t.Fatalf("summary = %q", report.Summary)
	}
	if report.OperationID == "" {
		t.Fatal("expected operation id")
	}
}

func TestLoadExistingPlanRecordRejectsUnknownIntegration(t *testing.T) {
	t.Parallel()

	svc := Service{StateStore: existingPlanTestStateStore{
		load: func() (ports.StateFile, error) {
			return ports.StateFile{Installations: []domain.InstallationRecord{}}, nil
		},
	}}
	_, err := svc.loadExistingPlanRecord(context.Background(), "missing")
	if err == nil || err.Error() != "integration not found in state: missing" {
		t.Fatalf("error = %v", err)
	}
}

func TestShouldPlanExistingAdoptedTargetsRequiresUpdateAndSharedSource(t *testing.T) {
	t.Parallel()

	resolved := &ports.ResolvedSource{}
	manifest := &domain.IntegrationManifest{}
	if !shouldPlanExistingAdoptedTargets("update_version", resolved, manifest) {
		t.Fatal("expected adopted update planning for update_version with shared source")
	}
	if shouldPlanExistingAdoptedTargets("repair_drift", resolved, manifest) {
		t.Fatal("did not expect adopted update planning for repair_drift")
	}
	if shouldPlanExistingAdoptedTargets("update_version", nil, manifest) {
		t.Fatal("did not expect adopted update planning without resolved source")
	}
}

func TestFinalizeExistingPlanReportSortsTargetReports(t *testing.T) {
	t.Parallel()

	report := finalizeExistingPlanReport(domain.Report{
		Targets: []domain.TargetReport{
			{TargetID: "gemini"},
			{TargetID: "claude"},
		},
	})
	if report.Targets[0].TargetID != "claude" || report.Targets[1].TargetID != "gemini" {
		t.Fatalf("targets = %+v", report.Targets)
	}
}

type existingPlanTestStateStore struct {
	load func() (ports.StateFile, error)
}

func (s existingPlanTestStateStore) Load(context.Context) (ports.StateFile, error) {
	return s.load()
}

func (s existingPlanTestStateStore) Save(context.Context, ports.StateFile) error {
	return nil
}

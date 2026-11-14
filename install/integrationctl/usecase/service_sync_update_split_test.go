package usecase

import (
	"errors"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
)

func TestBatchUpdateSkipWarningPreservesMessageContract(t *testing.T) {
	t.Parallel()

	if got := batchUpdateSkipWarning("demo", errors.New("boom")); got != "Update skipped for demo: boom" {
		t.Fatalf("warning = %q", got)
	}
}

func TestLessSyncTargetReportOrdersByTargetThenDelivery(t *testing.T) {
	t.Parallel()

	if !lessSyncTargetReport(domain.TargetReport{TargetID: "claude", DeliveryKind: "b"}, domain.TargetReport{TargetID: "gemini", DeliveryKind: "a"}) {
		t.Fatal("expected claude before gemini")
	}
	if !lessSyncTargetReport(domain.TargetReport{TargetID: "gemini", DeliveryKind: "a"}, domain.TargetReport{TargetID: "gemini", DeliveryKind: "b"}) {
		t.Fatal("expected delivery-kind ordering for same target")
	}
}

func TestAppendBatchUpdateTargetsPreservesExistingTargets(t *testing.T) {
	t.Parallel()

	report := domain.Report{
		Targets: []domain.TargetReport{{TargetID: "claude"}},
	}
	appendBatchUpdateTargets(&report, domain.Report{
		Targets: []domain.TargetReport{{TargetID: "gemini"}, {TargetID: "cursor"}},
	})
	if len(report.Targets) != 3 {
		t.Fatalf("targets = %+v", report.Targets)
	}
	if report.Targets[0].TargetID != "claude" || report.Targets[1].TargetID != "gemini" || report.Targets[2].TargetID != "cursor" {
		t.Fatalf("targets = %+v", report.Targets)
	}
}

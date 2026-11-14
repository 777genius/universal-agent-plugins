package usecase

import (
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
)

func TestSortedSyncInstallationsOrdersByIntegrationID(t *testing.T) {
	t.Parallel()

	got := sortedSyncInstallations([]domain.InstallationRecord{
		{IntegrationID: "cursor"},
		{IntegrationID: "claude"},
		{IntegrationID: "gemini"},
	})

	if len(got) != 3 || got[0].IntegrationID != "claude" || got[1].IntegrationID != "cursor" || got[2].IntegrationID != "gemini" {
		t.Fatalf("sorted installations = %+v", got)
	}
}

func TestFinalizeBatchUpdateReportUsesNoSuccessSummaryWhenWarningsExist(t *testing.T) {
	t.Parallel()

	report := domain.Report{
		Summary:  "Processed update for 2 managed integration(s).",
		Warnings: []string{"first failure"},
	}
	finalizeBatchUpdateReport(&report, 0)

	if report.Summary != "No managed integrations were updated successfully." {
		t.Fatalf("summary = %q", report.Summary)
	}
}

func TestFinalizeWorkspaceSyncReportUsesNoChangesSummary(t *testing.T) {
	t.Parallel()

	report := domain.Report{Summary: "Processed workspace sync for 0 desired integration(s)."}
	finalizeWorkspaceSyncReport(&report)

	if report.Summary != "Workspace sync found no changes." {
		t.Fatalf("summary = %q", report.Summary)
	}
}

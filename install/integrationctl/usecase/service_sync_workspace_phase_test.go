package usecase

import (
	"context"
	"testing"
)

func TestRunWorkspaceSyncPhasesReturnsNoChangesSummaryWhenNothingToDo(t *testing.T) {
	t.Parallel()

	var svc Service
	report := svc.runWorkspaceSyncPhases(context.Background(), true, nil, nil, nil)
	if report.Summary != "Workspace sync found no changes." {
		t.Fatalf("summary = %q", report.Summary)
	}
}

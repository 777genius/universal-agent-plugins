package usecase

import (
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
)

func TestWorkspaceSyncHasNoChangesOnlyWhenTargetsAndWarningsAreEmpty(t *testing.T) {
	t.Parallel()

	if !workspaceSyncHasNoChanges(domain.Report{}) {
		t.Fatal("expected empty report to count as no changes")
	}
	if workspaceSyncHasNoChanges(domain.Report{Targets: []domain.TargetReport{{TargetID: "claude"}}}) {
		t.Fatal("expected targets to count as changes")
	}
	if workspaceSyncHasNoChanges(domain.Report{Warnings: []string{"warn"}}) {
		t.Fatal("expected warnings to count as changes")
	}
}

func TestNewSyncDesiredIDsStartsEmpty(t *testing.T) {
	t.Parallel()

	got := newSyncDesiredIDs()
	if got == nil || len(got) != 0 {
		t.Fatalf("desired ids = %+v", got)
	}
}

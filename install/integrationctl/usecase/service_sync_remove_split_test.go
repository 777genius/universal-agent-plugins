package usecase

import (
	"errors"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
)

func TestSyncUndesiredScopeWarningSkipsNonProjectTargets(t *testing.T) {
	t.Parallel()
	warning, skip := syncUndesiredScopeWarning(domain.InstallationRecord{
		IntegrationID: "demo",
		Policy:        domain.InstallPolicy{Scope: "user"},
	})
	if !skip {
		t.Fatal("expected skip")
	}
	if warning != "Sync skipped unmanaged-scope removal for demo: scope=user" {
		t.Fatalf("warning = %q", warning)
	}
}

func TestSyncUndesiredRemoveFailureWarningFormatsMessage(t *testing.T) {
	t.Parallel()
	got := syncUndesiredRemoveFailureWarning(domain.InstallationRecord{IntegrationID: "demo"}, errors.New("boom"))
	if got != "Sync remove failed for demo: boom" {
		t.Fatalf("warning = %q", got)
	}
}

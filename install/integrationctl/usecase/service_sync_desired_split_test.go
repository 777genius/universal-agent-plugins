package usecase

import (
	"errors"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
)

func TestClassifyDesiredSyncActionReturnsUpdateForVersionDrift(t *testing.T) {
	t.Parallel()

	action := classifyDesiredSyncAction(domain.InstallationRecord{
		RequestedSourceRef: domain.RequestedSourceRef{Value: "/tmp/demo"},
		ResolvedVersion:    "0.1.0",
		Policy:             domain.InstallPolicy{Scope: "project"},
		Targets: map[domain.TargetID]domain.TargetInstallation{
			domain.TargetCursor: {TargetID: domain.TargetCursor},
		},
	}, true, "/tmp/demo", domain.InstallPolicy{Scope: "project"}, []domain.TargetID{domain.TargetCursor}, "0.2.0")

	if action != desiredSyncActionUpdate {
		t.Fatalf("action = %q", action)
	}
}

func TestSyncDesiredWarningHelpersPreserveContractText(t *testing.T) {
	t.Parallel()

	err := errors.New("boom")
	if got := syncDesiredSourceWarning("./plugins/demo", err); got != "Sync skipped for source ./plugins/demo: boom" {
		t.Fatalf("source warning = %q", got)
	}
	if got := syncDesiredManifestWarning("demo", err); got != "Sync skipped for demo: boom" {
		t.Fatalf("manifest warning = %q", got)
	}
	if got := syncDesiredNoopWarning("demo"); got != "Sync no-op for demo: desired state already matches workspace intent" {
		t.Fatalf("noop warning = %q", got)
	}
	if got := syncDesiredAddFailedWarning("demo", err); got != "Sync add failed for demo: boom" {
		t.Fatalf("add warning = %q", got)
	}
	if got := syncDesiredReplaceBlockedWarning("demo"); got != "Sync skipped replace for demo: replacing non-project scoped integrations is blocked" {
		t.Fatalf("replace warning = %q", got)
	}
	if got := syncDesiredRemoveBeforeAddWarning("demo", err); got != "Sync remove-before-add failed for demo: boom" {
		t.Fatalf("remove-before-add warning = %q", got)
	}
	if got := syncDesiredReAddFailedWarning("demo", err); got != "Sync re-add failed for demo: boom" {
		t.Fatalf("re-add warning = %q", got)
	}
	if got := syncDesiredUpdateFailedWarning("demo", err); got != "Sync update failed for demo: boom" {
		t.Fatalf("update warning = %q", got)
	}
}

func TestCanReplaceDesiredSyncRequiresProjectScopeOnBothSides(t *testing.T) {
	t.Parallel()

	if canReplaceDesiredSync(domain.InstallationRecord{Policy: domain.InstallPolicy{Scope: "user"}}, domain.InstallPolicy{Scope: "project"}) {
		t.Fatal("expected user-scope record to block replace")
	}
	if canReplaceDesiredSync(domain.InstallationRecord{Policy: domain.InstallPolicy{Scope: "project"}}, domain.InstallPolicy{Scope: "user"}) {
		t.Fatal("expected user-scope desired policy to block replace")
	}
	if !canReplaceDesiredSync(domain.InstallationRecord{Policy: domain.InstallPolicy{Scope: "project"}}, domain.InstallPolicy{Scope: "project"}) {
		t.Fatal("expected project scopes to allow replace")
	}
}

func TestDesiredSyncWarningPrefersManifestScopeWhenIntegrationIDKnown(t *testing.T) {
	t.Parallel()

	got := desiredSyncWarning(domain.WorkspaceLockIntegration{Source: "./plugins/demo"}, desiredSyncManifestErr{
		integrationID: "demo",
		err:           errors.New("boom"),
	})
	if got != "Sync skipped for demo: boom" {
		t.Fatalf("warning = %q", got)
	}
}

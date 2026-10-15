package environment

import (
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

func TestLifecycleFactsAuthenticationDoesNotRetry(t *testing.T) {
	for _, test := range []struct{ auth, want string }{{"not_checked", "auth_unverified"}, {"auth_pending", "auth_pending"}, {"failed", "auth_failed"}} {
		binding := installer.InspectedBinding{Materialization: "materialized", Activation: "active", Verification: "installation_verified", Authentication: test.auth}
		retry, facts := LifecycleFacts(binding)
		if retry || len(facts) != 1 || facts[0] != test.want {
			t.Fatalf("%s: %v %v", test.auth, retry, facts)
		}
		binding.Activation = "failed"
		if retry, _ := LifecycleFacts(binding); !retry {
			t.Fatal("failed activation must retry")
		}
		binding.LifecycleConverged = true
		if retry, facts := LifecycleFacts(binding); retry || len(facts) != 0 {
			t.Fatal("converged binding changed")
		}
	}
}
func TestObserveIncludesUnmanagedSiblingsAndRejectsReceiptMismatch(t *testing.T) {
	receipt := domain.EnvironmentReceipt{ReceiptID: "r", EnvironmentID: EnvironmentUserDefault, Alias: "review", ClientID: "codex", InstallationID: "i", ClientBindingID: "b", SourceKey: "github:acme/tools:", PackageDigest: "digest"}
	view := installer.Inspection{EnvironmentReceipts: []domain.EnvironmentReceipt{receipt}, Installations: []installer.InspectedInstallation{{InstallationID: "i", SourceKey: receipt.SourceKey, Bindings: []installer.InspectedBinding{{BindingID: "b", ClientID: "codex", TreeDigest: "digest", Materialization: "materialized", LifecycleConverged: true}, {BindingID: "foreign", ClientID: "claude"}}}}}
	observed, err := Observe(view)
	if err != nil || len(observed) != 2 || !observed[0].Managed || observed[1].Managed {
		t.Fatalf("observe: %+v %v", observed, err)
	}
	pkg := LockedPackage{Alias: "review", SourceKey: receipt.SourceKey}
	if _, err := validateDesiredBinding(pkg, "codex", observed[0], observed); err == nil {
		t.Fatal("unmanaged sibling adopted")
	}
	view.Installations[0].Bindings[0].TreeDigest = "changed"
	if _, err := Observe(view); err == nil {
		t.Fatal("receipt mismatch accepted")
	}
	view.Installations = nil
	if _, err := Observe(view); err == nil {
		t.Fatal("orphan receipt accepted")
	}
}

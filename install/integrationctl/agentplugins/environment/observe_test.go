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

// Prepared artifacts stop automatic retries without claiming native activation;
// failed, absent and unverified materializations must still be retried.
func TestLifecycleFactsManualReadiness(t *testing.T) {
	for _, test := range []struct {
		materialization, activation, verification string
		retry                                     bool
		fact                                      string
	}{
		{"materialized", "manual_activation_required", "package_validated", false, "activation_manual"},
		{"materialized", "prepared", "package_validated", false, "activation_prepared"},
		{"materialized", "manual_activation_required", "failed", true, "partial"},
		{"materialized", "manual_activation_required", "not_run", true, "partial"},
		{"materialized", "manual_activation_required", "", true, "partial"},
		{"materialized", "failed", "package_validated", true, "partial"},
		{"not_materialized", "manual_activation_required", "package_validated", true, "partial"},
	} {
		binding := installer.InspectedBinding{Materialization: test.materialization, Activation: test.activation, Verification: test.verification}
		retry, facts := LifecycleFacts(binding)
		if retry != test.retry || len(facts) != 1 || facts[0] != test.fact {
			t.Fatalf("%+v: %v %v", test, retry, facts)
		}
	}
	binding := installer.InspectedBinding{Materialization: "materialized", Activation: "manual_activation_required", Verification: "package_validated", Authentication: "failed"}
	retry, facts := LifecycleFacts(binding)
	if retry || len(facts) != 2 || facts[1] != "auth_failed" {
		t.Fatalf("manual authentication failure hidden: %v %v", retry, facts)
	}
}

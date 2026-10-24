package agentpluginscli

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

func TestProjectPreparedGuidancePreservesUserSDKAndAuthentication(t *testing.T) {
	for _, activation := range []string{"activation_manual", "activation_prepared"} {
		fact := dependencyStatusFact{Scope: domain.ScopeProject, ClientID: "grok", Facts: []string{activation, "auth_failed"}}
		sdk := installer.NextAction{Kind: "inspect_sdk", Reason: "explicit SDK instruction"}
		actions := dependencyNextActions(fact, sdk)
		if len(actions) != 3 || actions[0].Kind != sdk.Kind || actions[0].Reason != sdk.Reason || actions[1].Kind != "review_project_activation" || actions[2].Kind != "attest_authentication" {
			t.Fatalf("project/SDK/auth guidance %+v", actions)
		}
		if strings.Contains(actions[1].Reason, "--activation-complete") {
			t.Fatal("project review offered unsupported attestation")
		}
		fact.Scope = domain.ScopeUser
		user := dependencyNextActions(fact)
		if len(user) != 2 || user[0].Kind != "attest_activation" || !strings.Contains(user[0].Reason, "--activation-complete") || user[1].Kind != "attest_authentication" {
			t.Fatalf("user behavior changed %+v", user)
		}
	}
	body, err := json.Marshal(dependencyStatusFact{Scope: domain.ScopeProject})
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		t.Fatal(err)
	}
	if _, exists := fields["scope"]; exists {
		t.Fatal("internal guidance scope changed fact wire schema")
	}
}
func TestProjectPreparedGuidanceDesiredAndRetainedScopes(t *testing.T) {
	for _, desired := range []bool{false, true} {
		for _, scopes := range [][2]domain.InstallScope{{domain.ScopeProject, domain.ScopeUser}, {domain.ScopeUser, domain.ScopeProject}, {domain.ScopeProject, domain.ScopeProject}, {domain.ScopeUser, domain.ScopeUser}} {
			root := t.TempDir()
			app := App{Scope: scopes[0], EnvironmentStoreRoot: filepath.Join(root, "content-store")}
			if scopes[0] == domain.ScopeProject {
				app.ProjectRoot = root
			}
			receipt := domain.EnvironmentReceipt{EnvironmentID: app.environmentID(), Alias: "docs", ClientID: "grok", InstallationID: "install", ClientBindingID: "binding", SourceKey: "source", PackageDigest: "sha256:" + strings.Repeat("a", 64)}
			binding := installer.InspectedBinding{ClientID: "grok", BindingID: "binding", Scope: string(scopes[1]), Materialization: string(domain.MaterializationMaterialized), Activation: string(domain.ActivationPrepared), Verification: string(domain.VerificationPackageValid), TreeDigest: receipt.PackageDigest}
			state := installer.Inspection{EnvironmentReceipts: []domain.EnvironmentReceipt{receipt}, Installations: []installer.InspectedInstallation{{InstallationID: "install", SourceKey: "source", Bindings: []installer.InspectedBinding{binding}}}}
			lock := environment.Lock{}
			if desired {
				lock.Packages = []environment.LockedPackage{{Alias: "docs", Targets: []string{"grok"}, SourceKey: "source", TreeDigest: receipt.PackageDigest}}
			}
			facts := inspectDependencyFacts(context.Background(), app, state, lock, desired)
			if len(facts) != 1 {
				t.Fatalf("expected one status fact: %+v", facts)
			}
			project := scopes[0] == domain.ScopeProject || scopes[1] == domain.ScopeProject
			want := "attest_activation"
			if project {
				want = "review_project_activation"
			}
			if len(facts[0].NextActions) != 1 || facts[0].NextActions[0].Kind != want {
				t.Fatalf("desired%t scopes%v actions%+v", desired, scopes, facts[0].NextActions)
			}
		}
	}
}
func TestProjectPreparedGuidanceMissingBindingDoesNotInventActivation(t *testing.T) {
	fact := dependencyStatusFact{Scope: domain.ScopeProject, ClientID: "grok"}
	appendReceiptStatus(&fact, installer.Inspection{}, domain.EnvironmentReceipt{InstallationID: "missing", ClientBindingID: "missing"})
	if len(fact.NextActions) != 0 {
		t.Fatalf("missing identity manufactured activation action %+v", fact.NextActions)
	}
}

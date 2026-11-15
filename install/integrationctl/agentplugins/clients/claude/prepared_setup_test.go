package claude

import (
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestClaudePreparedCompletionRequiresKnownAutomaticIntactScope(t *testing.T) {
	root := filepath.Join(t.TempDir(), "uncreated-profile")
	project := filepath.Join(t.TempDir(), "uncreated-project")
	anchor := filepath.Join(project, ".claude")
	request := domain.ActivationRequest{DeclaredName: "demo", BackendExecutable: "/trusted/claude", Client: domain.DetectedClient{ConfigRoot: root}, Plan: domain.DeliveryPlan{Status: domain.PlanReady, Scope: domain.ScopeProject, PackageMode: domain.PackageProjection, InstallIntent: domain.InstallIntentAutomatic, TargetContext: domain.TargetContext{Scope: domain.ScopeProject, ProjectRoot: project, UserConfigRoot: root}, NativeRegistryRoot: anchor, TargetAnchor: anchor, TargetRoot: filepath.Join(anchor, "skills"), ActivePath: filepath.Join(anchor, "skills", "demo")}}
	outcome := domain.ActivationOutcome{Activation: domain.ActivationPrepared, Verification: domain.VerificationInstalled, Policy: domain.PolicyAllowed}
	if !New().CanCompletePreparedSetup(request, outcome) {
		t.Fatal("known project setup was not completable without filesystem observation")
	}
	for _, change := range []string{"prepare-intent", "unverified", "blocked", "foreign-layout", "unsupported-package", "no-cli", "unknown-scope"} {
		r, o := request, outcome
		switch change {
		case "prepare-intent":
			r.Plan.InstallIntent = domain.InstallIntentPrepare
		case "unverified":
			o.Verification = domain.VerificationPackageValid
		case "blocked":
			o.Policy = domain.PolicyBlocked
		case "foreign-layout":
			r.Plan.ActivePath = filepath.Join(project, "foreign", "demo")
		case "unsupported-package":
			r.Plan.PackageMode = domain.PackagePrepared
		case "no-cli":
			r.BackendExecutable = ""
		case "unknown-scope":
			r.Plan.Scope = domain.ScopeLocal
		}
		if New().CanCompletePreparedSetup(r, o) {
			t.Fatalf("unsupported completion admitted: %s", change)
		}
	}
	for _, test := range []struct {
		name, action string
		allowed      bool
	}{
		{"legacy-user-precedence", (projectListObservation{state: projectListOtherSource}).action(), false},
		{"legacy-disabled", (projectListObservation{state: projectListDisabled}).action(), false},
		{"marketplace-disabled", "the owned Claude Code project plugin is disabled; review its enablement in Claude Code", false},
		{"duplicate-native-name", "Claude Code does not expose one unique enabled plugin named demo; review same-name plugin precedence before using its MCP servers", false},
		{"missing-workspace-trust", "the selected Claude Code project has no recorded workspace trust; retry its confirmed installation to finish exact project setup", true},
		{"pending-mcp-approval", "Claude Code has not connected the selected MCP server plugin:demo:local; review its existing rejection, policy or authentication", true},
	} {
		observed := outcome
		observed.UserActions = []string{test.action}
		if got := New().CanCompletePreparedSetup(request, observed); got != test.allowed {
			t.Fatalf("%s completion=%v, want %v", test.name, got, test.allowed)
		}
	}
	request.Plan.Scope, request.Plan.TargetContext.Scope = domain.ScopeUser, domain.ScopeUser
	request.Plan.TargetAnchor, request.Plan.TargetRoot = root, filepath.Join(root, "skills")
	request.Plan.ActivePath = filepath.Join(root, "skills", "demo")
	if !New().CanCompletePreparedSetup(request, outcome) {
		t.Fatal("known user MCP setup was not completable")
	}
}

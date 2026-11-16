package claude

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/packagesnapshot"
)

func TestClaudeNativeLocalAuthenticationPreservesExplicitAndCatalogDecisions(t *testing.T) {
	base := domain.DeliveryPlan{Warnings: []string{"authentication_not_catalog_verified"}, Components: []domain.ComponentDecision{{Kind: domain.ComponentSkill, Support: domain.SupportProjected}, {Kind: domain.ComponentMCPServer, Support: domain.SupportProjected}}}
	for _, state := range []domain.AuthenticationState{domain.AuthenticationNotChecked, domain.AuthenticationPending, domain.AuthenticationFailed, domain.AuthenticationComplete} {
		initial := domain.ActivationOutcome{Activation: domain.ActivationActive, Policy: domain.PolicyAllowed, Authentication: state}
		got := qualifyClaudeNativeAuthentication(base, initial, true)
		want := state
		if state == domain.AuthenticationNotChecked {
			want = domain.AuthenticationNotRequired
		}
		if got.Authentication != want {
			t.Fatalf("explicit %s became %s", state, got.Authentication)
		}
	}
	for _, change := range []string{"remote", "catalog", "unready", "unsupported-kind"} {
		plan := base
		initial := domain.ActivationOutcome{Activation: domain.ActivationActive, Policy: domain.PolicyAllowed, Authentication: domain.AuthenticationNotChecked}
		local := true
		switch change {
		case "remote":
			local = false
		case "catalog":
			plan.Warnings = nil
		case "unready":
			initial.Activation = domain.ActivationPrepared
		case "unsupported-kind":
			plan.Components = []domain.ComponentDecision{{Kind: domain.ComponentApp, Support: domain.SupportNative}}
		}
		if got := qualifyClaudeNativeAuthentication(plan, initial, local); got.Authentication != domain.AuthenticationNotChecked {
			t.Fatalf("%s manufactured authentication proof: %+v", change, got)
		}
	}
}

func TestClaudeNativeSkillAuthenticationRequiresSuccessfulObservedInstallation(t *testing.T) {
	plan := domain.DeliveryPlan{Authentication: domain.AuthenticationNotChecked, Warnings: []string{"authentication_not_catalog_verified"},
		Components: []domain.ComponentDecision{{Kind: domain.ComponentSkill, Name: "guide", Support: domain.SupportProjected},
			{Kind: domain.ComponentSkill, Name: "echo", Support: domain.SupportProjected, Source: domain.ComponentSourceCommand, SourceName: "echo"}}}
	for _, state := range []string{"observed", "prepared", "failed", "package-only", "blocked", "activation-attested", "authentication-attested", "pending", "complete", "plan-empty", "plan-pending", "plan-complete", "catalog-unknown"} {
		t.Run(state, func(t *testing.T) {
			selected := plan
			initial := domain.ActivationOutcome{Activation: domain.ActivationActive, Verification: domain.VerificationInstalled, Policy: domain.PolicyAllowed, Authentication: domain.AuthenticationNotChecked}
			switch state {
			case "prepared":
				initial.Activation = domain.ActivationPrepared
			case "failed":
				initial.Activation = domain.ActivationFailed
			case "package-only":
				initial.Verification = domain.VerificationPackageValid
			case "blocked":
				initial.Policy = domain.PolicyBlocked
			case "activation-attested":
				initial.ActivationAttested = true
			case "authentication-attested":
				initial.AuthenticationAttested = true
			case "pending":
				initial.Authentication = domain.AuthenticationPending
			case "complete":
				initial.Authentication = domain.AuthenticationComplete
			case "plan-empty":
				selected.Authentication = ""
			case "plan-pending":
				selected.Authentication = domain.AuthenticationPending
			case "plan-complete":
				selected.Authentication = domain.AuthenticationComplete
			case "catalog-unknown":
				selected.Warnings = []string{"authentication_requirement_unknown"}
			}
			got, err := finishClaudeMCPActivation(t.Context(), clients.Env{}, domain.ActivationRequest{Plan: selected}, initial)
			want := initial
			if state == "observed" {
				want.Authentication = domain.AuthenticationNotRequired
			}
			if err != nil || got.Authentication != want.Authentication || got.Activation != initial.Activation || got.Verification != initial.Verification || got.Policy != initial.Policy || got.NativeEffect != initial.NativeEffect {
				t.Fatalf("Skill-only completion changed unrelated or unqualified evidence: %+v %v", got, err)
			}
		})
	}
}

func TestClaudeLegacyMCPAuthenticationRequiresVerifiedSelectedLocalDelivery(t *testing.T) {
	for _, scope := range []domain.InstallScope{domain.ScopeUser, domain.ScopeProject} {
		for _, test := range []struct {
			name, body string
			local      bool
		}{
			{"local", `{"local":{"command":"fixture","args":[]}}`, true},
			{"explicit-local", `{"local":{"type":"stdio","command":"fixture"}}`, true},
			{"remote", `{"local":{"type":"http","url":"https://example.invalid/mcp"}}`, false},
			{"sse", `{"local":{"type":"sse","url":"https://example.invalid/mcp"}}`, false},
			{"mixed", `{"local":{"command":"fixture","url":"https://example.invalid/mcp"}}`, false},
			{"foreign", `{"other":{"command":"fixture"}}`, false},
			{"empty-command", `{"local":{"command":" "}}`, false},
			{"unknown-type", `{"local":{"type":"unknown","command":"fixture"}}`, false},
			{"null-type", `{"local":{"type":null,"command":"fixture"}}`, false},
			{"duplicate-entry", `{"local":{"command":"fixture"},"local":{"command":"other"}}`, false},
			{"digest-drift", `{"local":{"command":"fixture"}}`, false},
			{"no-digest", `{"local":{"command":"fixture"}}`, false},
			{"other-delivery", `{"local":{"command":"fixture"}}`, false},
		} {
			t.Run(string(scope)+"/"+test.name, func(t *testing.T) {
				request, path := claudeLegacyMCPRequest(t, scope, test.body)
				switch test.name {
				case "digest-drift":
					claudeMarketplaceWrite(t, path, `{"local":{"command":"changed-after-verification"}}`)
				case "no-digest":
					request.Delivery.ArtifactDigest = ""
				case "other-delivery":
					request.Delivery.ActivePath += "-foreign"
				}
				observation := claudeMCPObservation{state: claudeMCPHealthConnected}
				if got := claudeMCPIsLocal(t.Context(), request, "local", observation); got != test.local {
					t.Fatalf("local transport proof=%v want %v", got, test.local)
				}
				if test.name != "other-delivery" {
					requireClaudeLegacyMCPAuthentication(t, request, test.local)
				}
			})
		}
	}
}

func claudeLegacyMCPRequest(t *testing.T, scope domain.InstallScope, body string) (domain.ActivationRequest, string) {
	t.Helper()
	request := newClaudeMarketplaceRequest(t)
	if scope == domain.ScopeUser {
		request.Plan.Scope = scope
		request.Plan.TargetContext = domain.TargetContext{Scope: scope}
		request.Plan.TargetAnchor = request.Client.ConfigRoot
		request.Plan.TargetRoot = filepath.Join(request.Client.ConfigRoot, "skills")
		request.Plan.ActivePath = filepath.Join(request.Plan.TargetRoot, request.Plan.PhysicalArtifactID)
		request.Delivery.ActivePath = request.Plan.ActivePath
	}
	request.VerifyOnly, request.BackendExecutable = true, "/trusted/claude"
	request.Plan.Warnings = []string{"authentication_not_catalog_verified"}
	request.Plan.Components = []domain.ComponentDecision{{Kind: domain.ComponentMCPServer, Name: "local", Support: domain.SupportProjected}}
	relative := ".mcp.json"
	if scope == domain.ScopeProject {
		relative = filepath.Join(claudeProjectBundleDirectory, relative)
		// An unrelated outer file cannot replace the selected nested projection.
		claudeMarketplaceWrite(t, filepath.Join(request.Plan.ActivePath, ".mcp.json"), `{"local":{"command":"foreign-outer-entry"}}`)
	}
	path := filepath.Join(request.Plan.ActivePath, relative)
	claudeMarketplaceWrite(t, path, body)
	digest, err := (packagesnapshot.Builder{}).Digest(t.Context(), request.Plan.ActivePath)
	if err != nil {
		t.Fatal(err)
	}
	request.Delivery.ArtifactDigest = digest
	return request, path
}

func requireClaudeLegacyMCPAuthentication(t *testing.T, request domain.ActivationRequest, local bool) {
	t.Helper()
	runner := &claudeMCPTestRunner{t: t, status: "✔ Connected", omitType: true}
	initial := domain.ActivationOutcome{Activation: domain.ActivationActive, Verification: domain.VerificationInstalled, Policy: domain.PolicyAllowed, Authentication: domain.AuthenticationNotChecked}
	outcome, err := finishClaudeMCPActivation(t.Context(), clients.Env{Runner: runner, NativeConfig: nativeconfig.New()}, request, initial)
	want := domain.AuthenticationNotChecked
	if local {
		want = domain.AuthenticationNotRequired
	}
	if err != nil || outcome.Activation != domain.ActivationActive || outcome.Authentication != want || runner.calls != 2 {
		t.Fatalf("legacy connected health or exact transport proof lost: %+v %v calls=%d", outcome, err, runner.calls)
	}
	settings := "settings.json"
	if request.Plan.Scope == domain.ScopeProject {
		settings = "settings.local.json"
	}
	if _, err := os.Lstat(filepath.Join(request.Plan.TargetAnchor, settings)); !os.IsNotExist(err) {
		t.Fatalf("read-only health changed approval: %v", err)
	}
}

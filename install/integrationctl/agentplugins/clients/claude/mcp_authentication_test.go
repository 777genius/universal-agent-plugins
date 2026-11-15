package claude

import (
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
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

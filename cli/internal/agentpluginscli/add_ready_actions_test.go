package agentpluginscli

import (
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

func TestNativeReadyResultDoesNotRepeatPlanningAuthenticationStep(t *testing.T) {
	const stale = "verify the plugin's authentication requirements before using it"
	base := usecase.AddResult{
		Plan: domain.DeliveryPlan{UserActions: []string{stale}},
		Activation: domain.ActivationOutcome{Activation: domain.ActivationActive, Authentication: domain.AuthenticationNotRequired,
			Verification: domain.VerificationInstalled, Policy: domain.PolicyAllowed},
	}
	if got := newAddResultData(domain.PackageEnvelope{}, base, false); got.Readiness != "ready" || got.NextAction != "" || len(got.NextActions) != 0 {
		t.Fatalf("verified result asks for redundant setup: %+v", got)
	}
	for _, kind := range []string{"pending", "attested", "real action", "rolled back"} {
		t.Run(kind, func(t *testing.T) {
			result := base
			switch kind {
			case "pending":
				result.Activation.Authentication = domain.AuthenticationPending
			case "attested":
				result.Activation.AuthenticationAttested = true
			case "real action":
				result.Activation.UserActions = []string{"restart the existing client session"}
			case "rolled back":
				result.GroupPhase = usecase.GroupTargetManagedRolledBack
			}
			if nextLifecycleAction(result) == "" {
				t.Fatal("required or unverified guidance was suppressed")
			}
		})
	}
}

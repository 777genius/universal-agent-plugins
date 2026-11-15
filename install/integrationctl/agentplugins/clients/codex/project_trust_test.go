package codex

import (
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestCodexProjectTrustPreservesKnownAndUncertainNativeEffects(t *testing.T) {
	cause := errors.New("trust write or rollback failed")
	for _, effect := range []domain.NativeEffectState{domain.NativeEffectUnchanged, domain.NativeEffectCommitted, domain.NativeEffectUncertain} {
		t.Run(string(effect), func(t *testing.T) {
			before := domain.ActivationOutcome{Activation: domain.ActivationPrepared, Verification: domain.VerificationInstalled, NativeEffect: domain.NativeEffectUnchanged}
			actual, err := pendingCodexProjectTrust(before, &shared.NativeEffectError{Effect: effect, Err: cause})
			if effect == domain.NativeEffectUncertain {
				if !errors.Is(err, cause) || actual.NativeEffect != domain.NativeEffectUncertain {
					t.Fatalf("uncertain native write became a retryable prepared result: %+v, %v", actual, err)
				}
				return
			}
			if err != nil || actual.NativeEffect != effect || actual.Activation != domain.ActivationPrepared || len(actual.LocalActions) != 1 {
				t.Fatalf("known trust effect was lost: %+v, %v", actual, err)
			}
		})
	}
}

func TestCodexProjectMCPTrustIsDisclosedBeforeInstallation(t *testing.T) {
	for _, withMCP := range []bool{false, true} {
		_, request, _ := projectDiscoveryFixture(t)
		if withMCP {
			request.Plan.Components = append(request.Plan.Components, domain.ComponentDecision{Kind: domain.ComponentMCPServer, Name: "local", Support: domain.SupportNative})
		}
		if err := refineCodexProjectPlan(clients.PlanInput{Client: request.Client}, &request.Plan); err != nil {
			t.Fatal(err)
		}
		disclosure := strings.Join(request.Plan.Warnings, "\n")
		if withMCP {
			if !strings.Contains(disclosure, strconv.Quote(request.Plan.NativeRegistryRoot)) || !strings.Contains(disclosure, "trust") || !strings.Contains(disclosure, "removing") {
				t.Fatalf("persistent exact-project trust was not disclosed: %s", disclosure)
			}
		} else if strings.Contains(disclosure, "trust") {
			t.Fatalf("skills-only plan advertised an unnecessary trust grant: %s", disclosure)
		}
	}
}

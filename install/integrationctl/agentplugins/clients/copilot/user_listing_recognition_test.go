package copilot

import (
	"context"
	"errors"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func TestCopilotUserListingRecognitionPreservesSharedNilFallback(t *testing.T) {
	r := copilotUserLiveRequest(t)
	body := copilotUserLiveDocument(r)
	registry := &copilotUserLiveRunner{t: t, steps: []copilotUserLiveStep{copilotUserLiveListStep(r, body, true)}}
	finding, err := shared.InspectCopilotRegistry(context.Background(), clients.Env{Runner: registry}, r.Plan, nil)
	if err != nil || finding != clients.RegistryIndeterminate || registry.calls != 1 {
		t.Fatal("shared nil callback acquired Copilot-specific authority", finding, err)
	}
	verify := &copilotUserLiveRunner{t: t, steps: []copilotUserLiveStep{copilotUserLiveListStep(r, body, false)}}
	outcome, err := shared.CompleteCopilotActivation(context.Background(), clients.Env{Runner: verify}, r, shared.StartedActivation(r))
	if err != nil || outcome.Activation != domain.ActivationManual || outcome.ActivationAttested || verify.calls != 1 {
		t.Fatal("shared nil callback changed the original manual fallback", outcome, err)
	}
}

func TestCopilotUserListingRecognitionWaitsForSuccessfulCommand(t *testing.T) {
	for _, kind := range []string{"runner", "exit"} {
		t.Run(kind, func(t *testing.T) {
			r := copilotUserLiveRequest(t)
			for _, tree := range []bool{false, true} {
				step := copilotUserLiveListStep(r, copilotUserLiveDocument(r), tree)
				if kind == "runner" {
					step.err = errors.New("inert native command failure")
				} else {
					step.result.ExitCode = 7
				}
				runner := &copilotUserLiveRunner{t: t, steps: []copilotUserLiveStep{step}}
				calls := 0
				var err error
				if tree {
					_, err = shared.InspectCopilotRegistryWithListingRecognition(context.Background(), clients.Env{Runner: runner}, r.Plan, nil,
						func(legacyports.CommandResult, domain.DeliveryPlan, *domain.ClientBinding) (clients.RegistryFinding, bool) {
							calls++
							return clients.RegistryExpected, true
						})
				} else {
					err = shared.VerifyCopilotPluginWithListingRecognition(context.Background(), clients.Env{Runner: runner}, r,
						func(legacyports.CommandResult, domain.ActivationRequest) bool {
							calls++
							return true
						})
				}
				if err == nil || calls != 0 || runner.calls != 1 {
					t.Fatal("failed command reached listing recognition", err, calls, runner.calls)
				}
			}
		})
	}
}

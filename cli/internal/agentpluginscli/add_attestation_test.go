package agentpluginscli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

func TestPreparedInteractiveInstallDoesNotRequestAttestation(t *testing.T) {
	var output bytes.Buffer
	input := strings.NewReader("next-command\n")
	command := &cobra.Command{}
	command.SetIn(input)
	command.SetOut(&output)
	command.SetErr(&output)
	current := usecase.AddResult{Mutated: true, Activation: domain.ActivationOutcome{
		Activation: domain.ActivationPrepared, Verification: domain.VerificationInstalled,
		Authentication: domain.AuthenticationNotChecked,
	}}
	if err := resumeInteractiveLifecycle(context.Background(), command, usecase.Service{}, usecase.AddInput{}, domain.PackageEnvelope{}, current); err != nil {
		t.Fatal(err)
	}
	if output.Len() != 0 || input.Len() != len("next-command\n") {
		t.Fatalf("prepared install requested another answer: output=%q remaining=%d", output.String(), input.Len())
	}
}

// Regression: the planner's "the plugin's" authentication review was repeated
// as "this plugin's" in every Copilot user-scope next action.
func TestLifecycleActionDoesNotRepeatAuthenticationReview(t *testing.T) {
	result := usecase.AddResult{
		Plan: domain.DeliveryPlan{ClientID: domain.ClientCopilot, UserActions: []string{"verify the plugin's authentication requirements before using it"}},
		Activation: domain.ActivationOutcome{Activation: domain.ActivationActive, Verification: domain.VerificationInstalled,
			Authentication: domain.AuthenticationNotChecked},
	}
	if got := nextLifecycleAction(result); got != "verify the plugin's authentication requirements before using it" {
		t.Fatalf("next action = %q", got)
	}
	result.Plan.UserActions = []string{"reload the client"}
	if got := nextLifecycleAction(result); got != "reload the client; verify this plugin's authentication requirements before using it" {
		t.Fatalf("next action = %q", got)
	}
}

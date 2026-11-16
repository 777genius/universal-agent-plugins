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

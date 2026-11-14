package cline

import (
	"context"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestDeactivateWithoutNativeOwnershipRequiresExplicitExternalConfirmation(t *testing.T) {
	request := domain.DeactivationRequest{
		Client:       domain.DetectedClient{ClientID: domain.ClientCline},
		DeclaredName: "empty-plugin",
	}
	outcome, err := New().Deactivate(context.Background(), clients.Env{}, request)
	if err != nil {
		t.Fatalf("preflight failed instead of returning an actionable step: %v", err)
	}
	if outcome.ArtifactRemovalAllowed || outcome.ExternalRemovalComplete ||
		outcome.Activation != domain.ActivationManual {
		t.Fatalf("unconfirmed outcome = %+v", outcome)
	}
	if len(outcome.UserActions) != 1 ||
		!strings.Contains(outcome.UserActions[0], "--external-uninstalled") {
		t.Fatalf("missing exact recovery command: %+v", outcome.UserActions)
	}

	request.ExternalUninstalled = true
	outcome, err = New().Deactivate(context.Background(), clients.Env{}, request)
	if err != nil {
		t.Fatalf("external confirmation failed: %v", err)
	}
	if !outcome.ArtifactRemovalAllowed || !outcome.ExternalRemovalComplete ||
		outcome.Activation != domain.ActivationNotRequired {
		t.Fatalf("confirmed outcome = %+v", outcome)
	}
	if len(outcome.UserActions) != 0 {
		t.Fatalf("confirmed outcome retained manual actions: %+v", outcome.UserActions)
	}
}

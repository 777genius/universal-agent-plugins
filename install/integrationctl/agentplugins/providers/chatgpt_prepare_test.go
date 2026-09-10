package providers

import (
	"context"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"os"
	"path/filepath"
	"testing"
)

func TestPersonalChatGPTPreparationCannotAttestRemoteActivation(t *testing.T) {
	request := domain.ActivationRequest{Client: domain.DetectedClient{ClientID: domain.ClientChatGPT}, Plan: domain.DeliveryPlan{ClientID: domain.ClientChatGPT, Scope: domain.ScopeUser, InstallIntent: domain.InstallIntentPrepare, PersonalChatGPTPreparation: true}, ActivationComplete: true}
	root := t.TempDir()
	active := filepath.Join(root, "context7")
	if err := os.Mkdir(active, 0700); err != nil {
		t.Fatal(err)
	}
	request.DeclaredName = "context7"
	request.Plan.ActivePath = active
	request.Delivery = domain.StagedDelivery{ClientID: domain.ClientChatGPT, OwnedBase: root, ActivePath: active}
	activator := Activator{}
	if err := activator.PreflightActivation(request); err != nil {
		t.Fatal(err)
	}
	outcome, err := activator.Activate(context.Background(), request)
	if err != nil || outcome.Activation != domain.ActivationPrepared || outcome.Authentication != domain.AuthenticationPending || outcome.Verification != domain.VerificationPackageValid || outcome.ActivationAttested {
		t.Fatalf("false remote success: %+v %v", outcome, err)
	}
	request.Plan.PersonalChatGPTPreparation = false
	if err := activator.PreflightActivation(request); err == nil {
		t.Fatal("unvalidated preparation allowed")
	}
}

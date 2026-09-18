package planner

import (
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/chatgpt"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/kiro"
)

func TestFacadeActionsMatchTheOwningAdapters(t *testing.T) {
	t.Parallel()
	if ChatGPTAppBindingAction != chatgpt.AppBindingAction {
		t.Fatalf("ChatGPTAppBindingAction drifted from clients/chatgpt.AppBindingAction")
	}
	if KiroPrepareAction != kiro.PrepareAction {
		t.Fatalf("KiroPrepareAction drifted from clients/kiro.PrepareAction")
	}
}

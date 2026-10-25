package providers_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestProjectOpenCodePreservesExactMCPLogicalKeys(t *testing.T) {
	request, _, _ := projectOpenCodeRequestWithNames(t, []string{"api/server", "other api server"})
	activator := projectOpenCodeActivator(t, nativeconfig.New())
	outcome, err := activator.Activate(context.Background(), request)
	if err != nil || outcome.Verification != domain.VerificationInstalled {
		t.Fatalf("exact project logical keys: %+v, %v", outcome, err)
	}
	body, err := os.ReadFile(filepath.Join(request.Plan.NativeRegistryRoot, "opencode.jsonc"))
	if err != nil || !strings.Contains(string(body), `"api/server"`) || !strings.Contains(string(body), `"other api server"`) {
		t.Fatalf("project normalized logical keys: %s, %v", body, err)
	}
	remove := domain.DeactivationRequest{Client: request.Client, TargetContext: request.Plan.TargetContext, NativeRegistryRoot: request.Plan.NativeRegistryRoot, Confirmed: true, NativeObjects: outcome.NativeObjects}
	if result, err := activator.Deactivate(context.Background(), remove); err != nil || !result.ExternalRemovalComplete {
		t.Fatalf("exact project-key removal: %+v, %v", result, err)
	}
}

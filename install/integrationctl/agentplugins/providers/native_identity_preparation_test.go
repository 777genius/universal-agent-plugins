package providers

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestChatGPTSignedBindingAuthorizesOnlyClearLocalPreparationBoundary(t *testing.T) {
	t.Parallel()
	root := filepath.Join(t.TempDir(), "prepared")
	plan := identityPlan(root)
	plan.LocalPreparationAuthorized = true
	plan.Components = []domain.ComponentDecision{{Kind: domain.ComponentApp, Name: "docs", Support: domain.SupportProjected}}

	observation, err := (testObserver(NativeIdentityObserver{})).ObserveNativeIdentity(context.Background(), domain.DetectedClient{ClientID: domain.ClientChatGPT}, plan, nil)
	if err != nil || observation.State != domain.NativeIdentityAbsent {
		t.Fatalf("signed local preparation = %+v, %v", observation, err)
	}

	writeIdentityFile(t, filepath.Join(root, "foreign", "plugin.json"), `{"name":"demo"}`)
	observation, err = (testObserver(NativeIdentityObserver{})).ObserveNativeIdentity(context.Background(), domain.DetectedClient{ClientID: domain.ClientChatGPT}, plan, nil)
	if err != nil || observation.State != domain.NativeIdentityUnmanaged {
		t.Fatalf("positive local collision was weakened = %+v, %v", observation, err)
	}
}

func TestChatGPTUnknownRemoteRegistryStillBlocksUnsignedNewPreparation(t *testing.T) {
	t.Parallel()
	plan := identityPlan(filepath.Join(t.TempDir(), "prepared"))
	plan.Components = []domain.ComponentDecision{{Kind: domain.ComponentApp, Name: "docs", Support: domain.SupportProjected}}

	observation, err := (testObserver(NativeIdentityObserver{})).ObserveNativeIdentity(context.Background(), domain.DetectedClient{ClientID: domain.ClientChatGPT}, plan, nil)
	if err != nil || observation.State != domain.NativeIdentityIndeterminate {
		t.Fatalf("unsigned remote identity = %+v, %v", observation, err)
	}
}

func TestKiroManualPowerAllowsPreparationButRejectsPositiveLocalCollision(t *testing.T) {
	t.Parallel()
	root := filepath.Join(t.TempDir(), "prepared")
	plan := identityPlan(root)
	plan.NativeRegistryRoot = filepath.Join(t.TempDir(), ".kiro")
	plan.Components = []domain.ComponentDecision{{Kind: domain.ComponentSkill, Name: "docs", Support: domain.SupportNative}}

	observation, err := (testObserver(NativeIdentityObserver{})).ObserveNativeIdentity(context.Background(), domain.DetectedClient{ClientID: domain.ClientKiro}, plan, nil)
	if err != nil || observation.State != domain.NativeIdentityAbsent {
		t.Fatalf("manual Power preparation = %+v, %v", observation, err)
	}

	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	writeIdentityFile(t, filepath.Join(root, "foreign", "plugin.json"), `{"name":"demo"}`)
	observation, err = (testObserver(NativeIdentityObserver{})).ObserveNativeIdentity(context.Background(), domain.DetectedClient{ClientID: domain.ClientKiro}, plan, nil)
	if err != nil || observation.State != domain.NativeIdentityUnmanaged {
		t.Fatalf("Kiro local collision = %+v, %v", observation, err)
	}
}

func TestKiroMixedPowerStillRejectsPositiveGlobalMCPCollision(t *testing.T) {
	t.Parallel()
	configRoot := filepath.Join(t.TempDir(), ".kiro")
	writeIdentityFile(t, filepath.Join(configRoot, "settings", "mcp.json"), `{"mcpServers":{"docs":{"command":"foreign"}}}`)
	plan := identityPlan(filepath.Join(t.TempDir(), "prepared"))
	plan.NativeRegistryRoot = configRoot
	plan.Components = []domain.ComponentDecision{
		{Kind: domain.ComponentSkill, Name: "guide", Support: domain.SupportNative},
		{Kind: domain.ComponentMCPServer, Name: "docs", Support: domain.SupportNative},
	}

	observation, err := (testObserver(NativeIdentityObserver{})).ObserveNativeIdentity(context.Background(), domain.DetectedClient{ClientID: domain.ClientKiro}, plan, nil)
	if err != nil || observation.State != domain.NativeIdentityUnmanaged {
		t.Fatalf("mixed Power MCP collision = %+v, %v", observation, err)
	}
}

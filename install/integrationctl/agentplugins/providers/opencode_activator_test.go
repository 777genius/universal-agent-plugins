package providers

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/opencode"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestOpenCodeActivatorTreatsCommittedUnlockFailureAsSuccessfulLifecycle(t *testing.T) {
	cleanupErr := errors.New("injected native config unlock failure")
	committedKernel := nativeconfig.NewWithLockAcquirer(func(nativeconfig.Paths, nativeconfig.Codec) (func() error, error) {
		return func() error { return cleanupErr }, nil
	})
	fresh := clients.Env{NativeConfig: nativeconfig.New()}

	t.Run("add", func(t *testing.T) {
		configRoot, active, objects, request := openCodeActivationFixture(t, "add")
		outcome, err := (Activator{NativeConfig: &committedKernel}).Activate(context.Background(), request)
		assertOpenCodeCommittedActivation(t, outcome, err)
		if err := opencode.VerifyNativeObjects(configRoot, active, objects); err != nil {
			t.Fatalf("committed add state: %v", err)
		}
		if err := opencode.DeactivateNative(context.Background(), fresh, domain.DeactivationRequest{
			Client: request.Client, NativeObjects: objects, Confirmed: true,
		}); err != nil {
			t.Fatalf("remove after committed add: %v", err)
		}
	})

	t.Run("update", func(t *testing.T) {
		root := t.TempDir()
		configRoot := filepath.Join(root, "xdg", "opencode")
		activeV1 := filepath.Join(root, "managed", "v1")
		firstEnvelope, firstPlan := openCodeProviderTestPackage(t, activeV1, configRoot, "old")
		first, err := opencode.BuildNativeObjects(activeV1, firstEnvelope, firstPlan)
		if err != nil {
			t.Fatal(err)
		}
		if err := opencode.ActivateNative(context.Background(), clients.Env{NativeConfig: nativeconfig.New()}, domain.ActivationRequest{
			Client: domain.DetectedClient{ClientID: domain.ClientOpenCode, Status: domain.DetectionDetected, ConfigRoot: configRoot},
			Plan:   firstPlan, Delivery: domain.StagedDelivery{ClientID: domain.ClientOpenCode, ActivePath: activeV1, NativeObjects: first},
		}); err != nil {
			t.Fatal(err)
		}

		activeV2 := filepath.Join(root, "managed", "v2")
		secondEnvelope, secondPlan := openCodeProviderTestPackage(t, activeV2, configRoot, "new")
		second, err := opencode.BuildNativeObjects(activeV2, secondEnvelope, secondPlan)
		if err != nil {
			t.Fatal(err)
		}
		request := domain.ActivationRequest{
			Client: domain.DetectedClient{ClientID: domain.ClientOpenCode, Status: domain.DetectionDetected, ConfigRoot: configRoot},
			Plan:   secondPlan, Delivery: domain.StagedDelivery{ClientID: domain.ClientOpenCode, OwnedBase: filepath.Dir(activeV2), ActivePath: activeV2, NativeObjects: second},
			DeclaredName: "demo", Replacing: true, PreviousNativeObjects: first,
		}
		outcome, err := (Activator{NativeConfig: &committedKernel}).Activate(context.Background(), request)
		assertOpenCodeCommittedActivation(t, outcome, err)
		if err := opencode.VerifyNativeObjects(configRoot, activeV2, second); err != nil {
			t.Fatalf("committed update state: %v", err)
		}
		if err := opencode.ActivateNative(context.Background(), fresh, domain.ActivationRequest{
			Client: request.Client, Plan: secondPlan,
			Delivery:              domain.StagedDelivery{ClientID: domain.ClientOpenCode, ActivePath: activeV2, NativeObjects: second},
			PreviousNativeObjects: second,
		}); err != nil {
			t.Fatalf("repair after committed update: %v", err)
		}
	})

	t.Run("repair", func(t *testing.T) {
		configRoot, active, objects, request := openCodeActivationFixture(t, "repair")
		if err := opencode.ActivateNative(context.Background(), clients.Env{NativeConfig: nativeconfig.New()}, request); err != nil {
			t.Fatal(err)
		}
		writeTestFile(t, filepath.Join(configRoot, "opencode.json"), `{"mcp":{}}`)
		request.PreviousNativeObjects = objects
		request.Delivery.NativeObjects = objects
		request.Replacing = true
		outcome, err := (Activator{NativeConfig: &committedKernel}).Activate(context.Background(), request)
		assertOpenCodeCommittedActivation(t, outcome, err)
		if err := opencode.VerifyNativeObjects(configRoot, active, objects); err != nil {
			t.Fatalf("committed repair state: %v", err)
		}
		if err := opencode.DeactivateNative(context.Background(), fresh, domain.DeactivationRequest{
			Client: request.Client, NativeObjects: objects, Confirmed: true,
		}); err != nil {
			t.Fatalf("remove after committed repair: %v", err)
		}
	})

	t.Run("remove", func(t *testing.T) {
		configRoot, _, objects, request := openCodeActivationFixture(t, "remove")
		if err := opencode.ActivateNative(context.Background(), clients.Env{NativeConfig: nativeconfig.New()}, request); err != nil {
			t.Fatal(err)
		}
		outcome, err := (Activator{NativeConfig: &committedKernel}).Deactivate(context.Background(), domain.DeactivationRequest{
			Client:       domain.DetectedClient{ClientID: domain.ClientOpenCode, Status: domain.DetectionDetected, ConfigRoot: configRoot},
			DeclaredName: "demo", CurrentActivation: domain.ActivationActive, Confirmed: true, NativeObjects: objects,
		})
		if err != nil {
			t.Fatalf("committed remove returned error: %v", err)
		}
		if !outcome.ExternalRemovalComplete || len(outcome.UserActions) != 1 || !strings.Contains(outcome.UserActions[0], "committed") {
			t.Fatalf("committed remove outcome = %+v", outcome)
		}
		if _, err := os.Lstat(filepath.Join(configRoot, "skills", "docs")); !os.IsNotExist(err) {
			t.Fatalf("committed remove retained skill: %v", err)
		}
		if err := opencode.ActivateNative(context.Background(), clients.Env{NativeConfig: nativeconfig.New()}, request); err != nil {
			t.Fatalf("add after committed remove: %v", err)
		}
	})
}

func openCodeActivationFixture(t *testing.T, skillText string) (string, string, []domain.NativeObjectOwnership, domain.ActivationRequest) {
	t.Helper()
	root := t.TempDir()
	configRoot := filepath.Join(root, "xdg", "opencode")
	active := filepath.Join(root, "managed", "demo")
	envelope, plan := openCodeProviderTestPackage(t, active, configRoot, skillText)
	objects, err := opencode.BuildNativeObjects(active, envelope, plan)
	if err != nil {
		t.Fatal(err)
	}
	request := domain.ActivationRequest{
		Client: domain.DetectedClient{ClientID: domain.ClientOpenCode, Status: domain.DetectionDetected, ConfigRoot: configRoot},
		Plan:   plan, Delivery: domain.StagedDelivery{ClientID: domain.ClientOpenCode, OwnedBase: filepath.Dir(active), ActivePath: active, NativeObjects: objects}, DeclaredName: "demo",
	}
	return configRoot, active, objects, request
}

func assertOpenCodeCommittedActivation(t *testing.T, outcome domain.ActivationOutcome, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("committed activation returned error: %v", err)
	}
	if outcome.Activation != domain.ActivationActive || outcome.Verification != domain.VerificationInstalled || len(outcome.UserActions) < 1 {
		t.Fatalf("committed activation outcome = %+v", outcome)
	}
	if !strings.Contains(outcome.UserActions[0], "committed") {
		t.Fatalf("committed activation cleanup action = %q", outcome.UserActions[0])
	}
}

func openCodeProviderTestPackage(t *testing.T, active, configRoot, skillText string) (domain.PackageEnvelope, domain.DeliveryPlan) {
	t.Helper()
	if err := os.Remove(filepath.Join(active, ".agentplugins-opencode.json")); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(active, "server.js"), "// server")
	writeTestFile(t, filepath.Join(active, "skills", "docs", "SKILL.md"), "# "+skillText)
	envelope := domain.PackageEnvelope{
		Skills: map[string]domain.Skill{"docs": {Name: "docs", RelativePath: "skills/docs/SKILL.md"}},
		MCP: domain.MCPComponent{Servers: map[string]domain.MCPServer{"docs": {Name: "docs", Type: "stdio", Decoded: map[string]any{
			"command": "node", "args": []any{"${PLUGIN_ROOT}/server.js"}, "env": map[string]any{"DATA": "${PLUGIN_DATA}"},
		}}}},
	}
	plan := domain.DeliveryPlan{ClientID: domain.ClientOpenCode, NativeRegistryRoot: configRoot, ActivePath: active, Components: []domain.ComponentDecision{
		{Kind: domain.ComponentSkill, Name: "docs", Support: domain.SupportPrepared},
		{Kind: domain.ComponentMCPServer, Name: "docs", Support: domain.SupportPrepared},
	}}
	if err := opencode.ProjectNative(active, envelope, plan, filepath.Join(filepath.Dir(active), "data")); err != nil {
		t.Fatal(err)
	}
	return envelope, plan
}

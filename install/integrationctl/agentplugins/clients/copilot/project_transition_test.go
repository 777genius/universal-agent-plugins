package copilot

import (
	"bytes"
	"context"
	"errors"
	"sync"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func copilotComponentSwitchFixture(t *testing.T) (clients.Env, domain.ActivationRequest, string, shared.ProjectComponents) {
	t.Helper()
	env, request, _ := trustFixture(t)
	request.Plan.DeclaredName, request.DeclaredName = "review", "review"
	request.Plan.SetNativePluginRegistration(true)
	request.Plan.InstallIntent = domain.InstallIntentAutomatic
	request.Plan.Components = []domain.ComponentDecision{{Kind: domain.ComponentSkill, Name: "review", Support: domain.SupportNative}}
	request.ProjectTrustConsent = true
	request.Delivery = domain.StagedDelivery{ClientID: domain.ClientCopilot, ActivePath: request.Plan.ActivePath}
	previous, err := copilotProjectPluginObject(request.Plan)
	if err != nil {
		t.Fatal(err)
	}
	request.Delivery.NativeObjects = []domain.NativeObjectOwnership{previous}
	active := request.Plan.ActivePath
	if err := os.MkdirAll(active, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(active, "plugin.json"),
		[]byte("{\"$schema\":\"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json\",\"name\":\"review\"}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := shared.ProjectCopilotMarketplace(active, domain.PackageEnvelope{
		Manifest: domain.PluginManifest{Name: "review", Version: "1.0.0"}}, request.Plan); err != nil {
		t.Fatal(err)
	}
	if _, err := New().Activate(t.Context(), env, request); err != nil {
		t.Fatal("original whole plugin registration:", err)
	}
	root := request.Plan.NativeRegistryRoot
	path, err := copilotProjectPluginSettingsPath(root)
	if err != nil {
		t.Fatal(err)
	}
	layout, err := copilotComponents(root)
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(active, "skills", "review")
	if err := os.MkdirAll(source, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "SKILL.md"), []byte("---\nname: review\ndescription: transition\n---\n"), 0600); err != nil {
		t.Fatal(err)
	}
	desired, err := layout.SkillObject("review", "skills/review", active)
	if err != nil {
		t.Fatal(err)
	}
	request.Plan.SetNativePluginRegistration(false)
	request.Plan.PreviousNativeObjects = []domain.NativeObjectOwnership{previous}
	request.PreviousNativeObjects = []domain.NativeObjectOwnership{previous}
	request.Delivery.NativeObjects = []domain.NativeObjectOwnership{desired}
	request.ProjectTrustConsent = false
	request.Replacing = true
	return env, request, path, layout
}

func TestCopilotWholePluginToComponentsUpdateRetiresOnlyOwnedRegistration(t *testing.T) {
	env, request, settings, layout := copilotComponentSwitchFixture(t)
	original := trustRead(t, settings)
	if err := New().PreflightActivation(env, request); err != nil {
		t.Fatalf("owned native-to-components preflight: %v", err)
	}
	outcome, err := New().Activate(t.Context(), env, request)
	if err != nil || outcome.Activation != domain.ActivationPrepared ||
		outcome.Verification != domain.VerificationInstalled {
		t.Fatalf("native-to-components activation: %+v %v", outcome, err)
	}
	now := trustRead(t, settings)
	if bytes.Equal(now, original) || bytes.Contains(now, []byte("agentplugins-")) {
		t.Fatal("old whole-plugin settings remain after successful component migration")
	}
	if err := layout.Verify(copilotNativeObjects(request.Delivery.NativeObjects), false, env.NativeConfig); err != nil {
		t.Fatalf("new native project skill not installed: %v", err)
	}
}

func TestCopilotWholePluginToComponentsCollisionPreservesOldNativeRegistration(t *testing.T) {
	env, request, settings, layout := copilotComponentSwitchFixture(t)
	original := trustRead(t, settings)
	foreign := filepath.Join(layout.SkillsRoot, "review", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(foreign), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(foreign, []byte("foreign skill"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := New().Activate(t.Context(), env, request); err == nil {
		t.Fatal("unowned skill collision was silently adopted")
	}
	if !bytes.Equal(original, trustRead(t, settings)) {
		t.Fatal("foreign skill collision removed previous whole-plugin registration")
	}
	if content, err := os.ReadFile(foreign); err != nil || string(content) != "foreign skill" {
		t.Fatal("foreign skill was changed on refused update")
	}
	registration, _, err := copilotProjectRegistration(request.Plan.NativeRegistryRoot, request.Plan.PhysicalArtifactID, request.DeclaredName)
	if err != nil {
		t.Fatal(err)
	}
	if present, exact, err := nativeconfig.InspectCopilotPluginSettings(trustRead(t, settings), registration); err != nil || !present || !exact {
		t.Fatal("refused transition lost previously owned native registration")
	}
}

func TestCopilotComponentSwitchUnregisterCloseFailureRemainsUncertain(t *testing.T) {
	env, request, settings, layout := copilotComponentSwitchFixture(t)
	failure := errors.New("forced native settings lock cleanup failure")
	env.NativeConfig = nativeconfig.NewWithLockAcquirer(func(paths nativeconfig.Paths, _ nativeconfig.Codec) (func() error, error) {
		if paths.JSON != settings {
			t.Fatalf("unexpected native writer lock: %s", paths.JSON)
		}
		return func() error { return failure }, nil
	})
	outcome, err := New().Activate(t.Context(), env, request)
	if !errors.Is(err, failure) || outcome.NativeEffect != domain.NativeEffectUncertain ||
		outcome.Verification == domain.VerificationInstalled {
		t.Fatalf("partially committed unregister falsely qualified new native components: %+v %v", outcome, err)
	}
	if _, err := os.Lstat(filepath.Join(layout.SkillsRoot, "review")); !os.IsNotExist(err) {
		t.Fatal("component install ran after uncertain native unregister cleanup")
	}
	if len(outcome.NativeObjects) != len(request.PreviousNativeObjects) {
		t.Fatal("new component ownership was published before the component mutation started")
	}
	registration, _, err := copilotProjectRegistration(request.Plan.NativeRegistryRoot,
		request.Plan.PhysicalArtifactID, request.DeclaredName)
	if err != nil {
		t.Fatal(err)
	}
	if present, _, err := nativeconfig.InspectCopilotPluginSettings(trustRead(t, settings), registration); err != nil || present {
		t.Fatal("test did not exercise committed unregister and degraded lock cleanup")
	}
}

func TestCopilotComponentSwitchRestoresPreviousPairAfterCancellation(t *testing.T) {
	env, request, settings, layout := copilotComponentSwitchFixture(t)
	original := trustRead(t, settings)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	var once sync.Once
	env.NativeConfig = nativeconfig.NewWithLockAcquirer(func(paths nativeconfig.Paths, _ nativeconfig.Codec) (func() error, error) {
		if paths.JSON != settings {
			return func() error { return nil }, nil
		}
		return func() error { once.Do(cancel); return nil }, nil
	})
	_, err := New().Activate(ctx, env, request)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation not propagated: %v", err)
	}
	if !bytes.Equal(original, trustRead(t, settings)) {
		t.Fatal("cancelled component switch did not restore exactly owned plugin pair")
	}
	if _, err := os.Lstat(filepath.Join(layout.SkillsRoot, "review")); !os.IsNotExist(err) {
		t.Fatalf("cancelled component switch exposed an uncommitted skill: %v", err)
	}
}

func TestCopilotComponentSwitchDoesNotResurrectPreviouslyAbsentPair(t *testing.T) {
	env, request, settings, layout := copilotComponentSwitchFixture(t)
	if err := copilotMutateProjectPlugin(t.Context(), env.NativeConfig, request.Plan.NativeRegistryRoot,
		request.Plan.PhysicalArtifactID, request.Plan.DeclaredName, true, true); err != nil {
		t.Fatal(err)
	}
	absent := trustRead(t, settings)
	foreign := filepath.Join(layout.SkillsRoot, "review", "SKILL.md")
	var once sync.Once
	env.NativeConfig = nativeconfig.NewWithLockAcquirer(func(paths nativeconfig.Paths, _ nativeconfig.Codec) (func() error, error) {
		if paths.JSON != settings {
			return func() error { return nil }, nil
		}
		return func() error {
			once.Do(func() {
				if err := os.MkdirAll(filepath.Dir(foreign), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(foreign, []byte("foreign concurrent skill"), 0600); err != nil {
					t.Fatal(err)
				}
			})
			return nil
		}, nil
	})
	if _, err := New().Activate(t.Context(), env, request); err == nil {
		t.Fatal("late foreign component collision was not rejected")
	}
	if !bytes.Equal(absent, trustRead(t, settings)) {
		t.Fatal("switch resurrected an already-absent plugin registration")
	}
	if body, err := os.ReadFile(foreign); err != nil || string(body) != "foreign concurrent skill" {
		t.Fatal("switch changed foreign skill after late collision")
	}
}

func TestCopilotComponentSwitchRefusesBareMCPBeforeUnregister(t *testing.T) {
	env, request, settings, layout := copilotComponentSwitchFixture(t)
	request.Plan.Components = append(request.Plan.Components,
		domain.ComponentDecision{Kind: domain.ComponentMCPServer, Name: "probe", Support: domain.SupportNative})
	before := trustRead(t, settings)
	if err := os.MkdirAll(filepath.Dir(layout.ConfigPaths.JSON), 0700); err != nil {
		t.Fatal(err)
	}
	bare := []byte("{\"mcpServers\":true}")
	if err := os.WriteFile(layout.ConfigPaths.JSON, bare, 0600); err != nil {
		t.Fatal(err)
	}
	if err := New().PreflightActivation(env, request); err == nil {
		t.Fatal("bare project MCP config was admitted before retiring plugin")
	}
	if !bytes.Equal(before, trustRead(t, settings)) {
		t.Fatal("preflight changed owned Copilot plugin registration")
	}
	if got, err := os.ReadFile(layout.ConfigPaths.JSON); err != nil || !bytes.Equal(got, bare) {
		t.Fatal("unsafe project MCP configuration changed")
	}
}

func TestCopilotComponentSwitchCommitsMCPWhenLockCleanupFails(t *testing.T) {
	env, request, settings, layout := copilotComponentSwitchFixture(t)
	// Replace the desired loose skill with one real native MCP server.
	mcpSource := filepath.Join(request.Plan.ActivePath, "mcp.json")
	if err := os.WriteFile(mcpSource,
		[]byte("{\"mcpServers\":{\"probe\":{\"type\":\"http\",\"url\":\"http://127.0.0.1:9/mcp\"}}}"), 0600); err != nil {
		t.Fatal(err)
	}
	server, err := readCopilotProjectServer(request.Plan.ActivePath, "probe")
	if err != nil {
		t.Fatal(err)
	}
	mcp, err := layout.MCPObject("probe", server, layout.ConfigPaths.JSON)
	if err != nil {
		t.Fatal(err)
	}
	request.Plan.Components = []domain.ComponentDecision{{Kind: domain.ComponentMCPServer, Name: "probe", Support: domain.SupportNative}}
	request.Delivery.NativeObjects = []domain.NativeObjectOwnership{mcp}
	cleanup := errors.New("deliberate post-commit MCP lock cleanup failure")
	env.NativeConfig = nativeconfig.NewWithLockAcquirer(func(paths nativeconfig.Paths, _ nativeconfig.Codec) (func() error, error) {
		if paths.JSON == settings {
			return func() error { return nil }, nil
		}
		if paths.JSON == layout.ConfigPaths.JSON {
			return func() error { return cleanup }, nil
		}
		t.Fatalf("unrecognized writer target: %s", paths.JSON)
		return nil, nil
	})
	outcome, err := New().Activate(t.Context(), env, request)
	if err != nil || outcome.Verification != domain.VerificationInstalled ||
		outcome.NativeEffect != domain.NativeEffectCommitted ||
		len(outcome.NativeObjects) != len(request.Delivery.NativeObjects) {
		t.Fatalf("committed MCP was misreported after cleanup failure: %+v %v", outcome, err)
	}
	if err := layout.Verify([]domain.NativeObjectOwnership{mcp}, false, env.NativeConfig); err != nil {
		t.Fatalf("new MCP server was not committed: %v", err)
	}
	reg, _, err := copilotProjectRegistration(request.Plan.NativeRegistryRoot, request.Plan.PhysicalArtifactID, request.DeclaredName)
	if err != nil {
		t.Fatal(err)
	}
	if present, _, err := nativeconfig.InspectCopilotPluginSettings(trustRead(t, settings), reg); err != nil || present {
		t.Fatal("old whole-plugin registration still present after committed MCP transition")
	}
}

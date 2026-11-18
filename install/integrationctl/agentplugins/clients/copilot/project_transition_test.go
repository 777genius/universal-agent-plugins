package copilot

import (
	"bytes"
	"errors"
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

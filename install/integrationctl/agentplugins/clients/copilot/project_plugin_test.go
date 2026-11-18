package copilot

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestCopilotProjectWholePluginLifecycleWithoutClientCLI(t *testing.T) {
	env, request, trustPath := trustFixture(t)
	request.Client.ClientID = domain.ClientCopilot
	request.Plan.DeclaredName = "review"
	request.Plan.SetNativePluginRegistration(true)
	request.Plan.InstallIntent = domain.InstallIntentAutomatic
	request.Plan.Components = []domain.ComponentDecision{{Kind: domain.ComponentSkill, Name: "review", Support: domain.SupportNative}}
	request.DeclaredName = "review"
	request.ProjectTrustConsent = true
	request.Delivery = domain.StagedDelivery{ClientID: domain.ClientCopilot, ActivePath: request.Plan.ActivePath}
	object, err := copilotProjectPluginObject(request.Plan)
	if err != nil {
		t.Fatal(err)
	}
	request.Delivery.NativeObjects = []domain.NativeObjectOwnership{object}
	reg, settingsPath, err := copilotProjectRegistration(request.Plan.NativeRegistryRoot, request.Plan.PhysicalArtifactID, request.DeclaredName)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0700); err != nil {
		t.Fatal(err)
	}
	foreign := []byte("// foreign settings\n{\"foreign\":42,\"enabledPlugins\":{\"foreign@other\":false}}\n")
	if err := os.WriteFile(settingsPath, foreign, 0600); err != nil {
		t.Fatal(err)
	}
	staging := filepath.Join(t.TempDir(), "staging")
	if err := os.MkdirAll(staging, 0700); err != nil {
		t.Fatal(err)
	}
	envelope := domain.PackageEnvelope{FormatID: domain.FormatIDAgentPluginsV1,
		Manifest: domain.PluginManifest{Name: "review", Version: "1.0.0"}}
	projected, err := New().Project(t.Context(), clients.ProjectionInput{StagingPath: staging, Envelope: envelope, Plan: request.Plan})
	if err != nil || len(projected) != 1 || projected[0] != object {
		t.Fatalf("whole plugin projection: %v %#v", err, projected)
	}
	if _, err := os.Stat(filepath.Join(staging, ".github", "plugin", "marketplace.json")); err != nil {
		t.Fatal("native marketplace was not staged:", err)
	}
	if _, err := os.Stat(filepath.Join(staging, ".github", "skills")); !os.IsNotExist(err) {
		t.Fatalf("native registration duplicated loose skills: %v", err)
	}
	if _, err := os.Stat(filepath.Join(staging, "mcp.json")); !os.IsNotExist(err) {
		t.Fatalf("native registration duplicated MCP settings: %v", err)
	}
	if err := os.WriteFile(filepath.Join(staging, "plugin.json"), []byte("{\"$schema\":\"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json\",\"name\":\"review\"}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(request.Plan.ActivePath), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(staging, request.Plan.ActivePath); err != nil {
		t.Fatal("publish staged Agent Plugin:", err)
	}
	outcome, err := New().Activate(t.Context(), env, request)
	if err != nil || outcome.Verification != domain.VerificationInstalled || outcome.Activation != domain.ActivationPrepared {
		t.Fatalf("Copilot project automatic registration: %+v %v", outcome, err)
	}
	installed, err := os.ReadFile(settingsPath)
	if err != nil || !bytes.Contains(installed, []byte("foreign@other")) || !bytes.Contains(installed, []byte("// foreign settings")) {
		t.Fatalf("foreign settings lost: %v %s", err, installed)
	}
	if present, exact, err := nativeconfig.InspectCopilotPluginSettings(installed, reg); err != nil || !present || !exact {
		t.Fatalf("native Copilot registration missing: %v %v %v", present, exact, err)
	}
	if _, err := os.Stat(trustPath); err != nil {
		t.Fatalf("project trust not remembered: %v", err)
	}
	request.PreviousNativeObjects = request.Delivery.NativeObjects
	request.ProjectTrustConsent = false
	// Saved bindings outlive transient planner fields. Read-only verification
	// must recognize their native receipt without rewriting settings or trust.
	request.Plan.SetNativePluginRegistration(false)
	request.Plan.PreviousNativeObjects = request.PreviousNativeObjects
	request.VerifyOnly = true
	observed, err := New().Activate(t.Context(), env, request)
	if err != nil || observed.Verification != domain.VerificationInstalled || observed.Activation != domain.ActivationPrepared {
		t.Fatalf("read-only persisted native registration: %+v %v", observed, err)
	}
	request.VerifyOnly = false
	repeat, err := New().Activate(t.Context(), env, request)
	if err != nil || repeat.Activation != domain.ActivationPrepared {
		t.Fatalf("repeat: %+v %v", repeat, err)
	}
	if next, err := os.ReadFile(settingsPath); err != nil || !bytes.Equal(installed, next) {
		t.Fatal("repeat rewrote unchanged project settings")
	}
	// Native users may delete the two UAP entries to disable this plugin.
	// The saved binding proves which *absent* names repair may restore, while
	// all foreign data and persistent trust remain untouched.
	if err := os.WriteFile(settingsPath, foreign, 0600); err != nil {
		t.Fatal(err)
	}
	managed := &domain.ClientBinding{
		ClientID: string(domain.ClientCopilot), Scope: string(domain.ScopeProject),
		PhysicalArtifact: request.Plan.PhysicalArtifactID, TargetLocator: request.Plan.ActivePath,
		NativeProfileRoot: request.Plan.NativeRegistryRoot, NativeObjects: request.Delivery.NativeObjects,
	}
	finding, err := New().InspectNativeRegistry(t.Context(), env, request.Client, request.Plan, managed)
	if err != nil || finding != clients.RegistryClear {
		t.Fatalf("missing owned registration incorrectly classified: %v %v", finding, err)
	}
	request.Plan.PreviousNativeObjects = request.PreviousNativeObjects
	repaired, err := New().Activate(t.Context(), env, request)
	if err != nil || repaired.Activation != domain.ActivationPrepared {
		t.Fatalf("repair of missing Copilot project registration: %+v %v", repaired, err)
	}
	if present, exact, err := nativeconfig.InspectCopilotPluginSettings(trustRead(t, settingsPath), reg); err != nil || !present || !exact {
		t.Fatalf("repaired Copilot native registration not verified: %v %v %v", present, exact, err)
	}
	// A forged or stale declared name must not turn uninstall into a
	// successful no-op that leaves the original plugin dangling.
	beforeWrongRemoval := trustRead(t, settingsPath)
	_, mismatchErr := New().Deactivate(t.Context(), env, domain.DeactivationRequest{
		Client: request.Client, TargetContext: request.Plan.TargetContext,
		NativeRegistryRoot: request.Plan.NativeRegistryRoot, PhysicalArtifactID: request.Plan.PhysicalArtifactID,
		DeclaredName: "another-plugin", NativeObjects: request.Delivery.NativeObjects, Confirmed: true,
	})
	if mismatchErr == nil || !bytes.Equal(beforeWrongRemoval, trustRead(t, settingsPath)) {
		t.Fatal("Copilot removal changed or accepted a registration with the wrong plugin name")
	}
	removed, err := New().Deactivate(t.Context(), env, domain.DeactivationRequest{
		Client: request.Client, TargetContext: request.Plan.TargetContext,
		NativeRegistryRoot: request.Plan.NativeRegistryRoot, PhysicalArtifactID: request.Plan.PhysicalArtifactID,
		DeclaredName: request.DeclaredName, NativeObjects: request.Delivery.NativeObjects, Confirmed: true,
	})
	if err != nil || !removed.ExternalRemovalComplete {
		t.Fatalf("native project removal failed: %+v %v", removed, err)
	}
	finalSettings, err := os.ReadFile(settingsPath)
	if err != nil || !bytes.Contains(finalSettings, []byte("foreign@other")) {
		t.Fatalf("removal damaged foreign settings: %v %s", err, finalSettings)
	}
	if present, _, err := nativeconfig.InspectCopilotPluginSettings(finalSettings, reg); err != nil || present {
		t.Fatal("project marketplace still registered")
	}
	if _, err := os.Stat(trustPath); err != nil {
		t.Fatal("plugin removal revoked persisted project trust")
	}
}

func TestCopilotWholePluginEligibilityKeepsLegacyProjectComponents(t *testing.T) {
	envelope := domain.PackageEnvelope{FormatID: domain.FormatIDAgentPluginsV1,
		Skills: map[string]domain.Skill{"review": {Name: "review"}}}
	in := clients.PlanInput{Envelope: envelope, Client: domain.DetectedClient{ClientID: domain.ClientCopilot, Version: "1.0.82"}}
	plan := domain.DeliveryPlan{Components: []domain.ComponentDecision{{Kind: domain.ComponentSkill, Name: "review", Support: domain.SupportNative}}}
	if !copilotWholePluginEligible(in, plan) {
		t.Fatal("portable v1 plugin was not eligible")
	}
	in.PreviousNativeObjects = []domain.NativeObjectOwnership{{Kind: copilotProjectSkillKind}}
	if copilotWholePluginEligible(in, plan) {
		t.Fatal("legacy component registration migrated without ownership transaction")
	}
	in.PreviousNativeObjects = nil
	plan.Components = append(plan.Components, domain.ComponentDecision{Kind: domain.ComponentMCPServer, Name: "blocked", Support: domain.SupportUnsupported})
	if copilotWholePluginEligible(in, plan) {
		t.Fatal("whole plugin bypassed unsupported component admission")
	}
	plan.Components = plan.Components[:1]
	in.Client.Version = "1.0.80"
	if copilotWholePluginEligible(in, plan) {
		t.Fatal("unsupported host claimed native plugin loading")
	}
	in.Client.Version = "1.0.82"
	in.Envelope.FormatID = domain.FormatIDLegacyV1
	if copilotWholePluginEligible(in, plan) {
		t.Fatal("legacy package incorrectly treated as Agent Plugins 1.0")
	}
}

func TestCopilotWholePluginRegistrationRefusesDriftAndForeignReplacement(t *testing.T) {
	root := filepath.Join(t.TempDir(), "project")
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	reg, path, err := copilotProjectRegistration(root, "fixture", "review")
	if err != nil {
		t.Fatal(err)
	}
	foreign := []byte("{\"extraKnownMarketplaces\":{\"" + reg.Marketplace + "\":{\"source\":{\"source\":\"directory\",\"path\":\"foreign\"}}}}")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, foreign, 0600); err != nil {
		t.Fatal(err)
	}
	if err := copilotMutateProjectPlugin(context.Background(), nativeconfig.New(), root, "fixture", "review", false, false); err == nil {
		t.Fatal("foreign marketplace overwritten")
	}
	got, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(got, foreign) {
		t.Fatal("collision damaged foreign configuration")
	}
	if err := copilotMutateProjectPlugin(context.Background(), nativeconfig.New(), root, "fixture", "review", true, true); err == nil {
		t.Fatal("unowned marketplace removed")
	}
	if got, err := os.ReadFile(path); err != nil || !bytes.Equal(got, foreign) {
		t.Fatal("removal damaged foreign configuration")
	}
}


func TestCopilotWholePluginRepairCannotRestoreRevokedProjectTrust(t *testing.T) {
	env, request, trustPath := trustFixture(t)
	request.Plan.DeclaredName = "review"
	request.Plan.SetNativePluginRegistration(true)
	request.Plan.InstallIntent = domain.InstallIntentAutomatic
	request.Plan.Components = []domain.ComponentDecision{{
		Kind: domain.ComponentSkill, Name: "review", Support: domain.SupportNative,
	}}
	request.DeclaredName = "review"
	request.Delivery = domain.StagedDelivery{
		ClientID: domain.ClientCopilot, ActivePath: request.Plan.ActivePath,
	}
	object, err := copilotProjectPluginObject(request.Plan)
	if err != nil {
		t.Fatal(err)
	}
	request.Delivery.NativeObjects = []domain.NativeObjectOwnership{object}
	request.PreviousNativeObjects = request.Delivery.NativeObjects
	request.ProjectTrustConsent = false
	outcome, err := New().Activate(t.Context(), env, request)
	if err == nil || outcome.Verification == domain.VerificationInstalled {
		t.Fatalf("repair re-granted revoked trust: %+v %v", outcome, err)
	}
	if _, err := os.Lstat(trustPath); !os.IsNotExist(err) {
		t.Fatal("repair wrote project trust without user consent")
	}
	_, settingsPath, err := copilotProjectRegistration(request.Plan.NativeRegistryRoot,
		request.Plan.PhysicalArtifactID, request.Plan.DeclaredName)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(settingsPath); !os.IsNotExist(err) {
		t.Fatal("rejected repair wrote project plugin settings")
	}
}

func TestCopilotWholePluginInspectRejectsDifferentOwnedPluginName(t *testing.T) {
	env, request, _ := trustFixture(t)
	request.Plan.DeclaredName = "review"
	request.Plan.SetNativePluginRegistration(true)
	request.Plan.InstallIntent = domain.InstallIntentAutomatic
	request.Plan.Components = []domain.ComponentDecision{{
		Kind: domain.ComponentSkill, Name: "review", Support: domain.SupportNative,
	}}
	owned, err := copilotProjectPluginObject(request.Plan)
	if err != nil {
		t.Fatal(err)
	}
	binding := &domain.ClientBinding{
		Scope: string(domain.ScopeProject),
		NativeProfileRoot: request.Plan.NativeRegistryRoot,
		PhysicalArtifact: request.Plan.PhysicalArtifactID,
		TargetLocator: request.Plan.ActivePath,
		NativeObjects: []domain.NativeObjectOwnership{owned},
	}
	other := request.Plan
	other.DeclaredName = "different"
	if finding, err := copilotInspectWholePlugin(other, binding, env.NativeConfig); err == nil || finding != clients.RegistryIndeterminate {
		t.Fatalf("foreign logical identity treated as installed: %s %v", finding, err)
	}
}

// The native plugin loader reads its own manifest and marketplace only after
// staging becomes the selected managed package. A valid settings entry alone
// must never make an absent/foreign source look installed.
func TestCopilotWholePluginSourceVerificationRejectsMissingAndSymlinkedDocs(t *testing.T) {
	env, request, _ := trustFixture(t)
	request.Plan.DeclaredName = "review"
	active := request.Plan.ActivePath
	manifest := filepath.Join(active, "plugin.json")
	marketplace := filepath.Join(active, ".github", "plugin", "marketplace.json")
	if err := os.MkdirAll(filepath.Dir(marketplace), 0700); err != nil {
		t.Fatal(err)
	}
	if err := copilotVerifyProjectPluginSource(request.Plan.NativeRegistryRoot, request.Plan, env.NativeConfig); err == nil {
		t.Fatal("absent source counted as a registered plugin")
	}
	plugin := []byte("{\"$schema\":\"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json\",\"name\":\"review\"}\n")
	if err := os.WriteFile(manifest, plugin, 0600); err != nil {
		t.Fatal(err)
	}
	if err := shared.ProjectCopilotMarketplace(active,
		domain.PackageEnvelope{Manifest: domain.PluginManifest{Name: "review", Version: "1.0.0"}}, request.Plan); err != nil {
		t.Fatal(err)
	}
	if err := copilotVerifyProjectPluginSource(request.Plan.NativeRegistryRoot, request.Plan, env.NativeConfig); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifest, []byte("{\"$schema\":\"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json\",\"name\":\"foreign\"}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := copilotVerifyProjectPluginSource(request.Plan.NativeRegistryRoot, request.Plan, env.NativeConfig); err == nil {
		t.Fatal("foreign manifest admitted")
	}
	if err := os.WriteFile(manifest, plugin, 0600); err != nil {
		t.Fatal(err)
	}
	foreign := filepath.Join(t.TempDir(), "outside-marketplace.json")
	body, err := os.ReadFile(marketplace)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(foreign, body, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(marketplace); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(foreign, marketplace); err != nil {
		if runtime.GOOS == "windows" {
			t.Skip("symlink privilege unavailable")
		}
		t.Fatal(err)
	}
	if err := copilotVerifyProjectPluginSource(request.Plan.NativeRegistryRoot, request.Plan, env.NativeConfig); err == nil {
		t.Fatal("native marketplace reader followed a foreign symlink")
	}
}

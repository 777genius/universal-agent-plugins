//go:build linux

package pluginkitairepo_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/loader"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/specregistry"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/statev2"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/copilot"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/managedstdio"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/planner"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func copilotCycleInert(t *testing.T) *copilotProjectCycle {
	t.Helper()
	root := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	f := &copilotProjectAttempt{base: &nativeFixture{Root: root, Home: filepath.Join(root, "home"), Project: filepath.Join(root, "A"), Probe: filepath.Join(root, "probe"), Events: filepath.Join(root, "events"), Nonce: "0123456789abcdef0123456789abcdef"}, other: filepath.Join(root, "B"), outside: filepath.Join(root, "outside"), installer: filepath.Join(root, "installer"), client: filepath.Join(root, "copilot"), git: filepath.Join(root, "git"), started: time.Now(), ctx: ctx, cancel: cancel, immutable: map[string]string{}, protected: map[string]string{}, files: map[string][]byte{}, evidence: map[string]any{}}
	for _, path := range []string{f.base.Home, filepath.Join(f.base.Project, ".git"), filepath.Join(f.other, ".git"), f.outside} {
		if err := os.MkdirAll(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range []string{f.base.Probe, f.installer, f.client, f.git} {
		projectNativeRuntimeWrite(t, path, []byte("inert bytes, never executed\n"), 0700)
		f.immutable[path] = nativeSHA(t, path)
	}
	c := newCopilotProjectCycle(f)
	c.prepare(t)
	return c
}

type copilotCycleInertRegistry struct {
	active, physical string
	calls            int
}

func (r *copilotCycleInertRegistry) Run(_ context.Context, command ports.Command) (ports.CommandResult, error) {
	r.calls++
	if len(command.Argv) < 3 {
		return ports.CommandResult{}, fmt.Errorf("inert unknown command")
	}
	if reflect.DeepEqual(command.Argv[1:], []string{"plugin", "list"}) {
		market := "agentplugins-" + copilotProjectConfigHash([]byte(r.physical))[:12]
		return ports.CommandResult{Stdout: []byte("Built-in plugins:\n  • browser-use (v0.0.4) [disabled]\nLive Plugins (loaded from a local marketplace directory, never copied):\n  • native-proof@" + market + " (v1.0.0) (enabled)\n      from " + r.active + "\n")}, nil
	}
	if command.Argv[1] != "plugin" || command.Argv[2] != "marketplace" && command.Argv[2] != "install" {
		return ports.CommandResult{}, fmt.Errorf("inert unexpected command")
	}
	return ports.CommandResult{}, nil
}

func copilotCycleStageInert(t *testing.T, c *copilotProjectCycle, key string) domain.ActivationRequest {
	t.Helper()
	r := c.recipes[key]
	snapshot, err := (packagedigest.Builder{TempRoot: c.f.base.Root}).Snapshot(c.f.ctx, r.Source, domain.SourceIdentity{RequestedSource: r.Source, CanonicalSource: r.Source})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := packagedigest.Remove(snapshot); err != nil {
			t.Error(err)
		}
	}()
	spec, err := specregistry.New()
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := (loader.Loader{Registry: spec}).LoadSnapshot(c.f.ctx, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	i := domain.Installation{InstallationID: "01234567-89ab-4cde-8fab-0123456789" + map[string]string{"user": "ab", "a": "ac", "b": "ad"}[key], DeclaredName: "native-proof", Source: domain.SourceBinding{SourceBindingID: domain.ComputeSourceBindingID(envelope.Source), RequestedSource: r.Source, CanonicalSource: r.Source, TreeDigest: envelope.TreeDigest}, Package: domain.PackageBinding{LoaderKind: envelope.LoaderKind, FormatID: envelope.FormatID, SchemaURI: envelope.SchemaURI, DeclaredName: envelope.Manifest.Name, Version: envelope.Manifest.Version, ManifestDigest: envelope.ManifestDigest, Inventory: envelope.Inventory}, Clients: map[string]domain.ClientBinding{}}
	registry, err := clients.NewRegistry(copilot.New())
	if err != nil {
		t.Fatal(err)
	}
	client := domain.DetectedClient{ClientID: domain.ClientCopilot, ConfigRoot: filepath.Join(c.f.base.Home, ".copilot"), ExecutablePath: c.f.client, Version: "1.0.103", Status: domain.DetectionDetected}
	project := ""
	if key != "user" {
		project = r.Project
	}
	planning := planner.Planner{ProjectRoot: project, ManagedRoot: filepath.Join(filepath.Dir(c.statePath(r)), "managed"), Paths: pathpolicy.Policy{}, Registry: registry}
	plan, err := planning.Plan(c.f.ctx, domain.PlanRequest{Envelope: envelope, Client: client, Scope: domain.InstallScope(copilotCycleScope(r)), ProjectRoot: project, PhysicalArtifactID: domain.ComputePhysicalArtifactID("native-proof", i.InstallationID)})
	if err != nil || plan.Status != domain.PlanReady {
		t.Fatal("actual inert Copilot planner", err)
	}
	launcher, err := managedstdio.NewSource(c.f.installer, "0.1.0-development")
	if err != nil {
		t.Fatal(err)
	}
	stager := providers.Stager{Registry: registry, Paths: pathpolicy.Policy{}, LauncherSource: launcher}
	if key != "user" {
		stager.PluginDataRoot = filepath.Join(r.Project, ".uap", "plugin-data")
		d, _, err := (providers.PluginDataManager{Base: stager.PluginDataRoot}).EnsureData(c.f.ctx, i.InstallationID, plan.PhysicalArtifactID, "project")
		if err != nil {
			t.Fatal(err)
		}
		i.DataReceipts = map[string]domain.DataReceipt{d.DataReceiptID: d}
	}
	delivery, err := stager.Stage(c.f.ctx, envelope, plan, "inert-copilot-cycle", domain.CompatibilityHints{})
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Rename(delivery.StagingPath, delivery.ActivePath); err != nil {
		t.Fatal(err)
	}
	runner := &copilotCycleInertRegistry{active: plan.ActivePath, physical: plan.PhysicalArtifactID}
	request := domain.ActivationRequest{Client: client, Plan: plan, Delivery: delivery, DeclaredName: "native-proof", BackendExecutable: c.f.client}
	out, err := copilot.New().Activate(c.f.ctx, clients.Env{NativeConfig: nativeconfig.New(), Runner: runner}, request)
	if err != nil || out.Verification != domain.VerificationInstalled {
		t.Fatal("actual inert Copilot activation", err)
	}
	if key == "user" && runner.calls != 3 || key != "user" && runner.calls != 0 {
		t.Fatal("project file activation crossed user native command seam")
	}
	b := domain.ClientBinding{ClientID: "copilot", Scope: copilotCycleScope(r), TargetLocator: plan.ActivePath, PhysicalArtifact: plan.PhysicalArtifactID, NativeProfileRoot: client.ConfigRoot, Materialization: domain.MaterializationMaterialized, Activation: out.Activation, Verification: out.Verification, Authentication: plan.Authentication, Policy: domain.PolicyAllowed, NativeObjects: delivery.NativeObjects, SelectedDelivery: plan.SelectedDelivery, ProfileNamespace: plan.ProfileNamespace(), ProfileAuthority: plan.ProfileAuthority(), PackageRevision: &domain.ClientPackageRevision{Version: envelope.Manifest.Version, TreeDigest: envelope.TreeDigest, ManifestDigest: envelope.ManifestDigest}}
	b.ClientBindingID = domain.ComputeClientBindingID(i.InstallationID, b.ClientID, b.Scope, b.TargetLocator)
	if key != "user" {
		b.NativeProfileRoot = plan.NativeRegistryRoot
		for id := range i.DataReceipts {
			b.DataReceiptID = id
		}
	}
	i.Clients[b.ClientBindingID] = b
	if err = (statev2.Store{Path: c.statePath(r), ProjectRoot: project}).Save(domain.StateFileV2{SchemaVersion: domain.StateSchemaVersion, Installations: []domain.Installation{i}}); err != nil {
		t.Fatal(err)
	}
	c.proof(t, r, "add-"+key)
	return request
}

func TestCopilotCycleOwnedProofMatchesActualStagerAndAdapter(t *testing.T) {
	for _, key := range []string{"user", "a", "b"} {
		t.Run(key, func(t *testing.T) {
			c := copilotCycleInert(t)
			copilotCycleStageInert(t, c, key)
			if err := c.authority(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

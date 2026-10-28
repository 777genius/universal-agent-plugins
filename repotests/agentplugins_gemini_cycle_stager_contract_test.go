//go:build linux

package pluginkitairepo_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/dirswap"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/loader"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/specregistry"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/statev2"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/gemini"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/managedstdio"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/planner"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/transaction"
)

func geminiCycleLoaded(t *testing.T, w *geminiProjectCycle, r *geminiCycleRecipe) domain.PackageEnvelope {
	t.Helper()
	snapshot, err := (packagedigest.Builder{TempRoot: w.f.base.Root}).Snapshot(w.f.ctx, r.Source, domain.SourceIdentity{RequestedSource: r.Source, CanonicalSource: r.Source})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := packagedigest.Remove(snapshot); err != nil {
			t.Error(err)
		}
	})
	registry, err := specregistry.New()
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := (loader.Loader{Registry: registry}).LoadSnapshot(w.f.ctx, snapshot)
	if err != nil || envelope.TreeDigest != r.Digest {
		t.Fatal("actual loader differs from independently authored digest", err)
	}
	return envelope
}
func geminiCycleStageActual(t *testing.T, w *geminiProjectCycle, scope string) []byte {
	t.Helper()
	r := w.recipes[scope]
	w.open[r.Config] = true
	envelope := geminiCycleLoaded(t, w, r)
	id := "01234567-89ab-4cde-8fab-0123456789a1"
	if scope == "a" {
		id = "01234567-89ab-4cde-8fab-0123456789a2"
	}
	if scope == "b" {
		id = "01234567-89ab-4cde-8fab-0123456789a3"
	}
	adapter := gemini.New()
	registry, err := clients.NewRegistry(adapter)
	if err != nil {
		t.Fatal(err)
	}
	client := domain.DetectedClient{ClientID: domain.ClientGemini, ConfigRoot: w.recipes["user"].Profile, ExecutablePath: w.f.client, Version: "0.65.0", Status: domain.DetectionDetected}
	project := ""
	if scope != "user" {
		project = r.Project
	}
	planning := planner.Planner{ProjectRoot: project, ManagedRoot: filepath.Join(r.StateRoot, "managed"), Paths: pathpolicy.Policy{}, Registry: registry}
	plan, err := planning.Plan(w.f.ctx, domain.PlanRequest{Envelope: envelope, Client: client, Scope: domain.InstallScope(r.scopeName()), ProjectRoot: project, PhysicalArtifactID: domain.ComputePhysicalArtifactID(geminiCycleName, id)})
	if err != nil || plan.Status != domain.PlanReady {
		t.Fatal("actual Gemini planner", err)
	}
	launcher, err := managedstdio.NewSource(w.f.installer, "0.1.0-development")
	if err != nil {
		t.Fatal(err)
	}
	manager := providers.PluginDataManager{Base: filepath.Join(r.StateRoot, "plugin-data")}
	data, _, err := manager.EnsureData(w.f.ctx, id, plan.PhysicalArtifactID, r.scopeName())
	if err != nil {
		t.Fatal(err)
	}
	stager := providers.Stager{Registry: registry, Paths: pathpolicy.Policy{}, LauncherSource: launcher, PluginDataRoot: manager.Base}
	delivery, err := stager.Stage(w.f.ctx, envelope, plan, "inert-windsurf-component-"+scope, domain.CompatibilityHints{})
	if err != nil {
		t.Fatal(err)
	}
	i := domain.Installation{InstallationID: id, DeclaredName: geminiCycleName, OriginMode: domain.OriginModeDirect, Source: domain.SourceBinding{SourceBindingID: domain.ComputeSourceBindingID(envelope.Source), RequestedSource: r.Source, CanonicalSource: r.Source, TreeDigest: envelope.TreeDigest}, Package: domain.PackageBinding{LoaderKind: envelope.LoaderKind, FormatID: envelope.FormatID, SchemaURI: envelope.SchemaURI, DeclaredName: geminiCycleName, Version: "1.0.0", ManifestDigest: envelope.ManifestDigest, Inventory: envelope.Inventory}, DataReceipts: map[string]domain.DataReceipt{data.DataReceiptID: data}}
	b := domain.ClientBinding{ClientID: "gemini", Scope: r.scopeName(), TargetLocator: plan.ActivePath, PhysicalArtifact: plan.PhysicalArtifactID, InstallIntent: domain.InstallIntentAutomatic, Materialization: domain.MaterializationStaged, Activation: domain.ActivationPrepared, Authentication: plan.Authentication, Policy: domain.PolicyAllowed, Verification: domain.VerificationPackageValid, DataReceiptID: data.DataReceiptID, AffectedSurfaces: []string{"gemini"}, PackageRevision: &domain.ClientPackageRevision{Version: "1.0.0", TreeDigest: envelope.TreeDigest, ManifestDigest: envelope.ManifestDigest}}
	b.ClientBindingID = domain.ComputeClientBindingID(id, "gemini", r.scopeName(), plan.ActivePath)
	b.NativeProfileRoot = plan.NativeRegistryRoot
	i.Clients = map[string]domain.ClientBinding{b.ClientBindingID: b}
	state := domain.StateFileV2{SchemaVersion: domain.StateSchemaVersion, Installations: []domain.Installation{i}}
	store := statev2.Store{Path: filepath.Join(r.StateRoot, "state-v2.json"), ProjectRoot: project}
	group := "gemini-cycle-" + scope
	kernel := transaction.Kernel{StateStore: store, Directory: dirswap.Manager{JournalDir: filepath.Join(r.StateRoot, "operations-v2")}}
	_, err = kernel.ApplyDirectoryGroup(w.f.ctx, transaction.DirectoryGroup{OperationGroupID: group, DesiredState: state, Mutations: []transaction.DirectoryMutation{{OperationID: group + "-001", InstallationID: id, ClientBindingID: b.ClientBindingID, Sequence: 1, OwnedBase: delivery.OwnedBase, ActivePath: delivery.ActivePath, StagingPath: delivery.StagingPath, AfterDigest: delivery.ArtifactDigest, NativeObjects: delivery.NativeObjects, Activation: domain.ActivationPrepared, Authentication: plan.Authentication, Policy: domain.PolicyAllowed, Verification: domain.VerificationPackageValid, RequireAbsent: true, Verify: func(ctx context.Context, path string) error { return stager.Verify(ctx, path, delivery.ArtifactDigest) }}}})
	if err != nil {
		t.Fatal("actual Gemini transaction", err)
	}
	request := domain.ActivationRequest{Client: client, Plan: plan, Delivery: delivery, DeclaredName: geminiCycleName}
	profileBefore, err := geminiCycleTree(w.f.ctx, r.Profile)
	if err != nil {
		t.Fatal(err)
	}
	w.profileBefore[scope] = profileBefore
	outcome, err := adapter.Activate(w.f.ctx, clients.Env{NativeConfig: nativeconfig.New()}, request)
	if err != nil || outcome.Verification != domain.VerificationInstalled {
		t.Fatal("actual Gemini native file adapter", err)
	}
	state, err = store.Load()
	if err != nil {
		t.Fatal(err)
	}
	i = state.Installations[0]
	b = i.Clients[b.ClientBindingID]
	b.Activation = outcome.Activation
	b.Verification = outcome.Verification
	b.NativeObjects = outcome.NativeObjects
	i.Clients[b.ClientBindingID] = b
	state.Installations[0] = i
	if err = store.Save(state); err != nil {
		t.Fatal(err)
	}
	raw, err := projectNativeRuntimeRead(store.Path, 64<<10)
	if err != nil {
		t.Fatal(err)
	}
	w.proof(t, r, "add-"+scope)
	return raw
}

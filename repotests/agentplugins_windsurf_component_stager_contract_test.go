//go:build linux

package pluginkitairepo_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/dirswap"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/loader"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/specregistry"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/statev2"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/windsurf"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/managedstdio"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/planner"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/transaction"
)

func windComponentLoaded(t *testing.T, w *windComponentFixture, r *windComponentRecipe) domain.PackageEnvelope {
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
func windComponentStageActual(t *testing.T, w *windComponentFixture, scope string) []byte {
	t.Helper()
	r := w.recipes[scope]
	envelope := windComponentLoaded(t, w, r)
	id := "01234567-89ab-4cde-8fab-0123456789a1"
	if scope == "a" {
		id = "01234567-89ab-4cde-8fab-0123456789a2"
	}
	if scope == "b" {
		id = "01234567-89ab-4cde-8fab-0123456789a3"
	}
	adapter := windsurf.New()
	registry, err := clients.NewRegistry(adapter)
	if err != nil {
		t.Fatal(err)
	}
	client := domain.DetectedClient{ClientID: domain.ClientWindsurf, ConfigRoot: w.recipes["user"].Profile, ExecutablePath: w.f.client, Version: "3000.11.205", Status: domain.DetectionDetected}
	project := ""
	if scope != "user" {
		project = r.Project
	}
	planning := planner.Planner{ProjectRoot: project, ManagedRoot: filepath.Join(r.StateRoot, "managed"), Paths: pathpolicy.Policy{}, Registry: registry}
	plan, err := planning.Plan(w.f.ctx, domain.PlanRequest{Envelope: envelope, Client: client, Scope: domain.InstallScope(r.scopeName()), ProjectRoot: project, PhysicalArtifactID: domain.ComputePhysicalArtifactID(windComponentName, id)})
	if err != nil || plan.Status != domain.PlanReady {
		t.Fatal("actual Windsurf planner", err)
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
	i := domain.Installation{InstallationID: id, DeclaredName: windComponentName, OriginMode: domain.OriginModeDirect, Source: domain.SourceBinding{SourceBindingID: domain.ComputeSourceBindingID(envelope.Source), RequestedSource: r.Source, CanonicalSource: r.Source, TreeDigest: envelope.TreeDigest}, Package: domain.PackageBinding{LoaderKind: envelope.LoaderKind, FormatID: envelope.FormatID, SchemaURI: envelope.SchemaURI, DeclaredName: windComponentName, Version: "1.0.0", ManifestDigest: envelope.ManifestDigest, Inventory: envelope.Inventory}, DataReceipts: map[string]domain.DataReceipt{data.DataReceiptID: data}}
	b := domain.ClientBinding{ClientID: "windsurf", Scope: r.scopeName(), TargetLocator: plan.ActivePath, PhysicalArtifact: plan.PhysicalArtifactID, InstallIntent: domain.InstallIntentAutomatic, Materialization: domain.MaterializationStaged, Activation: domain.ActivationPrepared, Authentication: plan.Authentication, Policy: domain.PolicyAllowed, Verification: domain.VerificationPackageValid, DataReceiptID: data.DataReceiptID, AffectedSurfaces: []string{"windsurf"}, PackageRevision: &domain.ClientPackageRevision{Version: "1.0.0", TreeDigest: envelope.TreeDigest, ManifestDigest: envelope.ManifestDigest}}
	b.ClientBindingID = domain.ComputeClientBindingID(id, "windsurf", r.scopeName(), plan.ActivePath)
	if scope != "user" {
		b.NativeProfileRoot = plan.NativeRegistryRoot
	}
	i.Clients = map[string]domain.ClientBinding{b.ClientBindingID: b}
	state := domain.StateFileV2{SchemaVersion: domain.StateSchemaVersion, Installations: []domain.Installation{i}}
	store := statev2.Store{Path: filepath.Join(r.StateRoot, "state-v2.json"), ProjectRoot: project}
	group := "wind-component-" + scope
	kernel := transaction.Kernel{StateStore: store, Directory: dirswap.Manager{JournalDir: filepath.Join(r.StateRoot, "operations-v2")}}
	_, err = kernel.ApplyDirectoryGroup(w.f.ctx, transaction.DirectoryGroup{OperationGroupID: group, DesiredState: state, Mutations: []transaction.DirectoryMutation{{OperationID: group + "-001", InstallationID: id, ClientBindingID: b.ClientBindingID, Sequence: 1, OwnedBase: delivery.OwnedBase, ActivePath: delivery.ActivePath, StagingPath: delivery.StagingPath, AfterDigest: delivery.ArtifactDigest, NativeObjects: delivery.NativeObjects, Activation: domain.ActivationPrepared, Authentication: plan.Authentication, Policy: domain.PolicyAllowed, Verification: domain.VerificationPackageValid, RequireAbsent: true, Verify: func(ctx context.Context, path string) error { return stager.Verify(ctx, path, delivery.ArtifactDigest) }}}})
	if err != nil {
		t.Fatal("actual Windsurf transaction", err)
	}
	request := domain.ActivationRequest{Client: client, Plan: plan, Delivery: delivery, DeclaredName: windComponentName}
	lockBefore := w.trees[r.Profile]["mcp_config.json.agentplugins.lock"]
	outcome, err := adapter.Activate(w.f.ctx, clients.Env{NativeConfig: nativeconfig.New()}, request)
	if err != nil || outcome.Verification != domain.VerificationInstalled {
		t.Fatal("actual Windsurf native file adapter", err)
	}
	observed, readErr := windOwnedTree(w.f.ctx, r.Profile)
	if readErr != nil || observed["mcp_config.json.agentplugins.lock"] != lockBefore {
		t.Fatal("actual adapter changed preauthored full lock identity/body", readErr)
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
	if _, err = w.validateProof(r, raw); err != nil {
		t.Fatal("independent proof against actual Store/Kernel/adapter", err)
	}
	w.freezeProof(t, r)
	return raw
}
func TestWindsurfComponentRecipeMatchesActualUserAndProjectEngine(t *testing.T) {
	w := windComponentInert(t)
	for _, scope := range []string{"user", "a", "b"} {
		windComponentStageActual(t, w, scope)
		w.check(t)
	}
	if w.recipes["user"].Binding.Activation != domain.ActivationActive || w.recipes["a"].Binding.Activation != domain.ActivationPrepared {
		t.Fatal("receipt activations conflated")
	}
	for _, r := range w.recipes {
		if _, err := os.Lstat(filepath.Join(r.Data, "native-marker.txt")); !os.IsNotExist(err) {
			t.Fatal("native data marker fabricated")
		}
	}
}
func TestWindsurfComponentProofRefusesForeignReceiptsAndUnprovedNativeBytes(t *testing.T) {
	for _, kind := range []string{"extra-data", "foreign-source", "binding-key", "wrong-data", "native-field", "foreign-value", "user-skill", "payload", "caller", "lock-body", "lock-mode", "lock-inode", "extra-lock", "cwd-sibling"} {
		t.Run(kind, func(t *testing.T) {
			w := windComponentInert(t)
			r := w.recipes["user"]
			raw := windComponentStageActual(t, w, "user")
			var state domain.StateFileV2
			if err := json.Unmarshal(raw, &state); err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "extra-data":
				state.Installations[0].DataReceipts["foreign"] = domain.DataReceipt{DataReceiptID: "foreign"}
			case "foreign-source":
				state.Installations[0].Source.CanonicalSource = w.recipes["b"].Source
			case "binding-key":
				state.Installations[0].Clients = map[string]domain.ClientBinding{"foreign": r.Binding}
			case "wrong-data":
				b := r.Binding
				b.DataReceiptID = ""
				state.Installations[0].Clients[b.ClientBindingID] = b
			case "native-field", "foreign-value":
				body, err := os.ReadFile(r.Config)
				if err != nil {
					t.Fatal(err)
				}
				var doc map[string]any
				if err = json.Unmarshal(body, &doc); err != nil {
					t.Fatal(err)
				}
				servers := doc["mcpServers"].(map[string]any)
				name := windComponentName
				if kind == "foreign-value" {
					name = "foreign-user"
				}
				servers[name].(map[string]any)["extra"] = "PRIVATE"
				body, err = json.Marshal(doc)
				if err != nil {
					t.Fatal(err)
				}
				if err = os.WriteFile(r.Config, body, 0600); err != nil {
					t.Fatal(err)
				}
			case "user-skill":
				projectNativeRuntimeWrite(t, filepath.Join(r.Profile, "skills", windComponentName, "SKILL.md"), r.Authored["skills/"+windComponentName+"/SKILL.md"], 0600)
			case "payload":
				projectNativeRuntimeWrite(t, filepath.Join(r.Active, "extra"), []byte("PRIVATE"), 0600)
			case "lock-body":
				if err := os.WriteFile(r.Config+".agentplugins.lock", []byte("changed"), 0600); err != nil {
					t.Fatal(err)
				}
			case "lock-mode":
				if err := os.Chmod(r.Config+".agentplugins.lock", 0644); err != nil {
					t.Fatal(err)
				}
			case "lock-inode":
				if err := os.Remove(r.Config + ".agentplugins.lock"); err != nil {
					t.Fatal(err)
				}
				projectNativeRuntimeWrite(t, r.Config+".agentplugins.lock", []byte{}, 0600)
			case "extra-lock":
				projectNativeRuntimeWrite(t, r.Config+".foreign.lock", []byte{}, 0600)
			case "cwd-sibling":
				projectNativeRuntimeWrite(t, filepath.Join(r.Project, "unknown-native.log"), []byte("private"), 0600)
			case "caller":
				if err := os.WriteFile(filepath.Join(r.StateRoot, "plugin-data", "caller-owned", "keep.txt"), []byte("changed"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			candidate, err := json.Marshal(state)
			if err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(filepath.Join(r.StateRoot, "state-v2.json"), candidate, 0600); err != nil {
				t.Fatal(err)
			}
			before := map[string]string{}
			for path, digest := range w.f.protected {
				before[path] = digest
			}
			if _, err = w.validateProof(r, candidate); err == nil {
				t.Fatal("unproved authority admitted", kind)
			}
			if !reflect.DeepEqual(before, w.f.protected) {
				t.Fatal("failed proof adopted protected bytes")
			}
		})
	}
}

//go:build linux

package pluginkitairepo_test

import (
	"context"
	"encoding/json"
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
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/cline"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/managedstdio"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/planner"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
)

func clineCycleInert(t *testing.T) *clineProjectCycle {
	t.Helper()
	root := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	f := &clineProjectAttempt{base: &nativeFixture{Root: root, Home: filepath.Join(root, "home"), Project: filepath.Join(root, "A"), Probe: filepath.Join(root, "probe"), Events: filepath.Join(root, "events"), Nonce: "inert-nonce"}, other: filepath.Join(root, "B"), outside: filepath.Join(root, "outside"), installer: filepath.Join(root, "installer"), key: "uap-cline-private-inert", ctx: ctx, cancel: cancel, totalEnd: clineAttemptMonotonic() + int64(10*time.Second), immutable: map[string]string{}, protected: map[string]string{}, trees: map[string]map[string]string{}, files: map[string][]byte{}, evidence: map[string]any{}}
	for _, path := range []string{f.base.Home, filepath.Join(f.base.Project, ".git"), filepath.Join(f.other, ".git"), f.outside} {
		if err := os.MkdirAll(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range []string{f.base.Probe, f.installer} {
		projectNativeRuntimeWrite(t, path, []byte("inert executable bytes; never invoked\n"), 0700)
		f.immutable[path] = nativeSHA(t, path)
	}
	c := newClineProjectCycle(t, f)
	c.prepare(t)
	return c
}
func clineCycleLoaded(t *testing.T, c *clineProjectCycle, r *clineCycleRecipe) domain.PackageEnvelope {
	t.Helper()
	snapshot, err := (packagedigest.Builder{TempRoot: c.f.base.Root}).Snapshot(c.f.ctx, r.Source, domain.SourceIdentity{RequestedSource: r.Source, CanonicalSource: r.Source})
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
	envelope, err := (loader.Loader{Registry: registry}).LoadSnapshot(c.f.ctx, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	return envelope
}
func clineCycleStageActual(t *testing.T, c *clineProjectCycle, scope string) domain.ActivationRequest {
	t.Helper()
	r := c.recipes[scope]
	envelope := clineCycleLoaded(t, c, r)
	i := domain.Installation{InstallationID: "01234567-89ab-4cde-8fab-0123456789ab", DeclaredName: clineCycleName, Source: domain.SourceBinding{SourceBindingID: domain.ComputeSourceBindingID(envelope.Source), RequestedSource: envelope.Source.RequestedSource, CanonicalSource: envelope.Source.CanonicalSource, TreeDigest: envelope.TreeDigest}, Package: domain.PackageBinding{LoaderKind: envelope.LoaderKind, FormatID: envelope.FormatID, SchemaURI: envelope.SchemaURI, DeclaredName: envelope.Manifest.Name, Version: envelope.Manifest.Version, ManifestDigest: envelope.ManifestDigest, Inventory: envelope.Inventory}, Clients: map[string]domain.ClientBinding{}}
	adapter := cline.New()
	registry, err := clients.NewRegistry(adapter)
	if err != nil {
		t.Fatal(err)
	}
	client := domain.DetectedClient{ClientID: domain.ClientCline, ConfigRoot: filepath.Join(c.f.base.Home, ".cline"), ExecutablePath: filepath.Join(c.f.base.Root, "cline"), Version: "3.0.73", Status: domain.DetectionDetected}
	projectRoot := ""
	if scope != "user" {
		projectRoot = r.Project
	}
	planning := planner.Planner{ProjectRoot: projectRoot, ManagedRoot: filepath.Join(filepath.Dir(c.statePath(r)), "managed"), Paths: pathpolicy.Policy{}, Registry: registry}
	plan, err := planning.Plan(c.f.ctx, domain.PlanRequest{Envelope: envelope, Client: client, Scope: domain.InstallScope(scopeName(r)), ProjectRoot: projectRoot, PhysicalArtifactID: domain.ComputePhysicalArtifactID(clineCycleName, i.InstallationID)})
	if err != nil || plan.Status != domain.PlanReady {
		t.Fatal("actual inert Cline planning", err)
	}
	launcher, err := managedstdio.NewSource(c.f.installer, "0.1.0-development")
	if err != nil {
		t.Fatal(err)
	}
	stager := providers.Stager{Registry: registry, Paths: pathpolicy.Policy{}, LauncherSource: launcher}
	if scope != "user" {
		stager.PluginDataRoot = filepath.Join(r.Project, ".uap", "plugin-data")
		d, _, err := (providers.PluginDataManager{Base: stager.PluginDataRoot}).EnsureData(c.f.ctx, i.InstallationID, plan.PhysicalArtifactID, "project")
		if err != nil {
			t.Fatal(err)
		}
		i.DataReceipts = map[string]domain.DataReceipt{d.DataReceiptID: d}
	}
	delivery, err := stager.Stage(c.f.ctx, envelope, plan, "inert-cline-cycle", domain.CompatibilityHints{})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(delivery.StagingPath, delivery.ActivePath); err != nil {
		t.Fatal(err)
	}
	delete(c.f.trees, filepath.Dir(filepath.Dir(r.SkillPath)))
	delete(c.f.protected, r.Config)
	request := domain.ActivationRequest{Client: client, Plan: plan, Delivery: delivery, DeclaredName: clineCycleName}
	out, err := adapter.Activate(c.f.ctx, clients.Env{NativeConfig: nativeconfig.New()}, request)
	if err != nil || out.Verification != domain.VerificationInstalled {
		t.Fatal("actual inert Cline file activation", err)
	}
	b := domain.ClientBinding{ClientID: "cline", Scope: scopeName(r), TargetLocator: plan.ActivePath, PhysicalArtifact: plan.PhysicalArtifactID, Materialization: domain.MaterializationMaterialized, Activation: out.Activation, Verification: out.Verification, Authentication: plan.Authentication, Policy: domain.PolicyAllowed, NativeObjects: out.NativeObjects, SelectedDelivery: plan.SelectedDelivery, ProfileNamespace: plan.ProfileNamespace(), ProfileAuthority: plan.ProfileAuthority(), PackageRevision: &domain.ClientPackageRevision{Version: envelope.Manifest.Version, TreeDigest: envelope.TreeDigest, ManifestDigest: envelope.ManifestDigest}}
	b.ClientBindingID = domain.ComputeClientBindingID(i.InstallationID, b.ClientID, b.Scope, b.TargetLocator)
	if scope != "user" {
		b.NativeProfileRoot = plan.NativeRegistryRoot
		for id := range i.DataReceipts {
			b.DataReceiptID = id
		}
	}
	i.Clients[b.ClientBindingID] = b
	state := domain.StateFileV2{SchemaVersion: domain.StateSchemaVersion, Installations: []domain.Installation{i}}
	// Persist through the real current writer before the proof consumes raw JSON.
	if err := (statev2.Store{Path: c.statePath(r), ProjectRoot: projectRoot}).Save(state); err != nil {
		t.Fatal(err)
	}
	// Normalize omitempty fields exactly as a public process's state does.
	raw, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := clineCycleState(raw); err != nil {
		t.Fatal(err)
	}
	c.proof(t, r, "add-"+scope)
	return request
}
func TestClineCycleRecipeProofMatchesActualUserAndProjectStager(t *testing.T) {
	for _, scope := range []string{"user", "a"} {
		t.Run(scope, func(t *testing.T) { c := clineCycleInert(t); clineCycleStageActual(t, c, scope) })
	}
}

func TestClineCycleMissingOwnedDirectoryRepairsButPartialTamperingRefuses(t *testing.T) {
	for _, kind := range []string{"absent", "leaf", "extra"} {
		t.Run(kind, func(t *testing.T) {
			c := clineCycleInert(t)
			request := clineCycleStageActual(t, c, "a")
			r := c.recipes["a"]
			request.Replacing = true
			request.PreviousNativeObjects = append([]domain.NativeObjectOwnership(nil), r.Binding.NativeObjects...)
			if kind == "absent" {
				c.missingA(t)
				if _, err := os.Lstat(filepath.Dir(r.SkillPath)); !os.IsNotExist(err) {
					t.Fatal("fault left the exact independently owned directory present")
				}
			} else {
				delete(c.f.protected, r.SkillPath)
				delete(c.f.trees, filepath.Dir(filepath.Dir(r.SkillPath)))
				if kind == "leaf" {
					if err := os.Remove(r.SkillPath); err != nil {
						t.Fatal(err)
					}
				} else {
					projectNativeRuntimeWrite(t, filepath.Join(filepath.Dir(r.SkillPath), "foreign.txt"), []byte("foreign must remain\n"), 0600)
				}
			}
			before, err := clineAttemptTree(filepath.Dir(filepath.Dir(r.SkillPath)))
			if err != nil {
				t.Fatal(err)
			}
			c.f.check(t)
			out, err := cline.New().Activate(c.f.ctx, clients.Env{NativeConfig: nativeconfig.New()}, request)
			c.f.check(t)
			if kind == "absent" {
				if err != nil || out.Verification != domain.VerificationInstalled || out.Activation != domain.ActivationPrepared {
					t.Fatal("actual adapter did not restore exact absent owned directory", err)
				}
				c.proof(t, r, "inert-restored-directory")
				return
			}
			after, treeErr := clineAttemptTree(filepath.Dir(filepath.Dir(r.SkillPath)))
			if err == nil || treeErr != nil || out.Verification != domain.VerificationFailed || out.NativeEffect != domain.NativeEffectUnchanged || !reflect.DeepEqual(out.NativeObjects, request.PreviousNativeObjects) || !reflect.DeepEqual(before, after) {
				t.Fatal("partial skill tampering was adopted, changed or reported installed", err, treeErr)
			}
		})
	}
}

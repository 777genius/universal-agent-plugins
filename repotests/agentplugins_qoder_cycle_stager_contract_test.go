//go:build linux

package pluginkitairepo_test

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/loader"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/specregistry"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/statev2"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/qoder"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/planner"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
)

// Only inert files and actual production planner/stager/adapter APIs. This
// helper never initializes hosted admission, runs an executable or qualifies.
func qoderCycleInert(t *testing.T) *qoderSkillsCycle {
	t.Helper()
	root := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	spec, err := projectNativeCLISpecification("qoder")
	if err != nil {
		t.Fatal(err)
	}
	f := &projectNativeCLIAttempt{spec: spec, started: time.Now(), ctx: ctx, cancel: cancel, files: map[string][]byte{}, immutable: map[string]string{}, protected: map[string]string{}, used: map[string]bool{}, evidence: map[string]any{}, client: filepath.Join(root, "qodercli"), installer: filepath.Join(root, "installer"), git: filepath.Join(root, "git"), bin: filepath.Join(root, "native-bin")}
	f.base = &nativeFixture{Root: root, Home: filepath.Join(root, "home"), Project: filepath.Join(root, "project"), Probe: filepath.Join(root, "probe"), Events: filepath.Join(root, "events.jsonl"), Nonce: "0123456789abcdef0123456789abcdef"}
	f.other, f.outside = filepath.Join(root, "project-b"), filepath.Join(root, "outside")
	for _, path := range []string{f.base.Home, filepath.Join(f.base.Project, ".git"), filepath.Join(f.other, ".git"), filepath.Join(f.outside, ".git"), f.bin} {
		if err := os.MkdirAll(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range []string{f.client, f.installer, f.git, f.base.Probe} {
		projectNativeRuntimeWrite(t, path, []byte("inert bytes, never executed\n"), 0700)
		f.immutable[path] = nativeSHA(t, path)
	}
	if err := os.Symlink(f.client, filepath.Join(f.bin, "qoder")); err != nil {
		t.Fatal(err)
	}
	settings := filepath.Join(f.base.Home, ".qoder", "settings.json")
	_, body := projectNativeCLIUpdater("qoder")
	projectNativeRuntimeWrite(t, settings, body, 0600)
	f.protected[settings] = nativeSHA(t, settings)
	f.qoderMetadata, err = qoderAuthorOriginalMetadata(filepath.Dir(settings), f.protected)
	if err != nil {
		t.Fatal(err)
	}
	f.qoderTelemetry, err = qoderAuthorEmptyTelemetry(ctx, filepath.Dir(settings))
	if err != nil {
		t.Fatal(err)
	}
	c := newQoderSkillsCycle(f)
	c.prepare(t)
	return c
}

func qoderCycleStageInert(t *testing.T, c *qoderSkillsCycle, key string) {
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
	registry, err := clients.NewRegistry(qoder.New())
	if err != nil {
		t.Fatal(err)
	}
	client := domain.DetectedClient{ClientID: domain.ClientQoder, ConfigRoot: filepath.Join(c.f.base.Home, ".qoder"), ExecutablePath: c.f.client, Version: "1.1.78", Status: domain.DetectionDetected}
	planning := planner.Planner{ProjectRoot: r.Project, ManagedRoot: filepath.Join(r.Project, ".uap", "managed"), Paths: pathpolicy.Policy{}, Registry: registry}
	iid := "01234567-89ab-4cde-8fab-0123456789ab"
	if key == "b" {
		iid = "01234567-89ab-4cde-8fab-0123456789ac"
	}
	physical := qoderCycleName + "-" + qoderDeltaHash(iid)[:12]
	plan, err := planning.Plan(c.f.ctx, domain.PlanRequest{Envelope: envelope, Client: client, Scope: domain.ScopeProject, ProjectRoot: r.Project, PhysicalArtifactID: physical})
	if err != nil || plan.Status != domain.PlanReady {
		t.Fatal("actual inert Qoder planner", err)
	}
	stager := providers.Stager{Registry: registry, Paths: pathpolicy.Policy{}}
	delivery, err := stager.Stage(c.f.ctx, envelope, plan, "inert-qoder-cycle", domain.CompatibilityHints{})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(delivery.StagingPath, delivery.ActivePath); err != nil {
		t.Fatal(err)
	}
	request := domain.ActivationRequest{Client: client, Plan: plan, Delivery: delivery, DeclaredName: qoderCycleName, BackendExecutable: c.f.client}
	out, err := qoder.New().Activate(c.f.ctx, clients.Env{}, request)
	if err != nil || out.Activation != domain.ActivationPrepared || out.Verification != domain.VerificationInstalled {
		t.Fatal("actual inert Qoder adapter", err)
	}
	stamp := "2026-10-28T00:00:00Z"
	b := domain.ClientBinding{ClientBindingID: domain.ComputeClientBindingID(iid, "qoder", "project", plan.ActivePath), ClientID: "qoder", Scope: "project", TargetLocator: plan.ActivePath, PhysicalArtifact: physical, Materialization: domain.MaterializationMaterialized, Activation: out.Activation, Authentication: domain.AuthenticationNotChecked, Policy: domain.PolicyAllowed, Verification: out.Verification, PackageRevision: &domain.ClientPackageRevision{Version: "1.0.0", TreeDigest: envelope.TreeDigest, ManifestDigest: envelope.ManifestDigest}, AffectedSurfaces: []string{"qoder"}, NativeProfileRoot: plan.NativeRegistryRoot, NativeObjects: delivery.NativeObjects, UpdatedAt: stamp}
	i := domain.Installation{InstallationID: iid, DeclaredName: qoderCycleName, Source: domain.SourceBinding{SourceBindingID: domain.ComputeSourceBindingID(envelope.Source), RequestedSource: r.Source, CanonicalSource: r.Source, TreeDigest: envelope.TreeDigest}, Package: domain.PackageBinding{LoaderKind: envelope.LoaderKind, FormatID: envelope.FormatID, SchemaURI: envelope.SchemaURI, DeclaredName: envelope.Manifest.Name, Version: envelope.Manifest.Version, ManifestDigest: envelope.ManifestDigest, Inventory: envelope.Inventory}, OriginMode: domain.OriginModeDirect, OperationGroupID: "op-00000000000000000000000000000000", Clients: map[string]domain.ClientBinding{b.ClientBindingID: b}, CreatedAt: stamp, UpdatedAt: stamp}
	c.mutable = key
	if err := (statev2.Store{Path: c.statePath(r), ProjectRoot: r.Project}).Save(domain.StateFileV2{SchemaVersion: domain.StateSchemaVersion, Installations: []domain.Installation{i}}); err != nil {
		t.Fatal(err)
	}
	stored, err := clineCycleState(mustQoderCycleRead(t, c.statePath(r)))
	if err != nil || len(stored.Installations) != 1 {
		t.Fatal("actual inert stored state differs", err)
	}
	i = stored.Installations[0]
	if err := qoderCycleExpectedSource(i, r); err != nil {
		t.Fatal(err)
	}
	c.binding(t, r, i)
	c.delivery(t, r)
	digest, err := qoderCycleExpectedDigest(c.f.ctx, r.Key, r.Revision, r.Nonce)
	if err != nil || r.Binding.NativeObjects[0].ManagedDigest != digest {
		t.Fatal("independently framed authored payload differs from actual stager digest", err)
	}
	c.data(t, r, i)
	r.Installation = i
	c.f.protected[c.statePath(r)] = nativeSHA(t, c.statePath(r))
	c.mutable = ""
	c.check(t)
}

func TestQoderCycleOwnedProofJoinsActualProductionStagerAndPreparedAdapter(t *testing.T) {
	c := qoderCycleInert(t)
	for _, key := range []string{"a", "b"} {
		qoderCycleStageInert(t, c, key)
	}
	if c.recipes["a"].Binding.PhysicalArtifact == c.recipes["b"].Binding.PhysicalArtifact || c.recipes["a"].Binding.ClientBindingID == c.recipes["b"].Binding.ClientBindingID {
		t.Fatal("same-name projects share physical authority")
	}
	before := c.recipes["b"].Installation
	a := c.recipes["a"]
	delete(c.trees, a.Source)
	a.Revision = "a-two"
	c.author(t, a, true)
	delete(c.trees, a.Active)
	delete(c.f.protected, c.statePath(a))
	// Only this helper's previously owned package is removed in the inert setup;
	// the separately tested production fault itself uses nonrecursive removals.
	if err := os.RemoveAll(a.Active); err != nil {
		t.Fatal(err)
	}
	qoderCycleStageInert(t, c, "a")
	c.removeOwnedA(t)
	if !reflect.DeepEqual(before, c.recipes["b"].Installation) {
		t.Fatal("A exact whole-package fault changed B")
	}
}

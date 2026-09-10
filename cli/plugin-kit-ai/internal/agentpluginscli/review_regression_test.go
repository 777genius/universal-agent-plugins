package agentpluginscli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
)

type reviewVersionDetector struct{ client domain.DetectedClient }

func (d reviewVersionDetector) Detect(context.Context) ([]domain.DetectedClient, error) {
	c := d.client
	c.Version = ""
	return []domain.DetectedClient{c}, nil
}
func (d reviewVersionDetector) DetectWithVersionProbe(context.Context) ([]domain.DetectedClient, error) {
	c := d.client
	c.Version = "fixture-client"
	return []domain.DetectedClient{c}, nil
}
func (d reviewVersionDetector) DetectTargetsWithVersionProbe(ctx context.Context, targets []domain.ClientID) ([]domain.DetectedClient, error) {
	for _, target := range targets {
		if target == domain.ClientKiro {
			return d.DetectWithVersionProbe(ctx)
		}
	}
	return d.Detect(ctx)
}

func TestReviewUnrelatedPreparationPreservesAutomaticDirectoryGate(t *testing.T) {
	r := newRolloutDirectoryFixture(t, []domain.ClientID{domain.ClientKiro}, []domain.ClientID{domain.ClientKiro})
	kiro := fixtureClient(t, domain.ClientKiro)
	r.cli.app.Detector = reviewVersionDetector{client: kiro}
	snapshot := &r.directory.bundle.Snapshot
	failed := intendedTrustedDirectoryEvidence(domain.DirectoryEvidence{
		ID: "failed/runtime/kiro", DistributionID: "owner/rollout", ReleaseSequence: 1,
		PackageTreeDigest: r.v1.tree, Level: "runtime", Outcome: "failed", Client: domain.ClientKiro,
		InstallerVersion: r.cli.app.Version,
	})
	snapshot.Evidence = append(snapshot.Evidence, failed)
	snapshot.Distributions[0].ReleasePolicies[0].CurrentEvidence = append(snapshot.Distributions[0].ReleasePolicies[0].CurrentEvidence, failed.ID)
	check := func() error {
		_, clients, err := preflightSelectedTargets(context.Background(), r.cli.app, []domain.ClientID{domain.ClientKiro}, nil, true)
		if err != nil {
			t.Fatal(err)
		}
		loaded, err := r.cli.app.loadPackageFor(context.Background(), "rollout-demo", withDetectedClients(r.cli.app.addResolutionRequest("rollout-demo", []domain.ClientID{domain.ClientKiro}), clients))
		if loaded.cleanup != nil {
			defer loaded.cleanup()
		}
		t.Logf("automatic version=%q resolution=%v", clients[domain.ClientKiro].Version, err)
		if clients[domain.ClientKiro].Version != "fixture-client" {
			t.Fatalf("lost automatic Kiro version: %+v", clients)
		}
		return err
	}
	if err := check(); err == nil || !strings.Contains(err.Error(), "evidence") {
		t.Fatalf("fixture must initially reject current failed evidence: %v", err)
	}
	plugin := writeCLIPlugin(t)
	body, err := os.ReadFile(filepath.Join(plugin, "plugin.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(plugin, "plugin.json"), []byte(strings.ReplaceAll(string(body), `"demo"`, `"unrelated"`)), 0600); err != nil {
		t.Fatal(err)
	}
	writeCLIMCP(t, plugin)
	r.cli.app.Lifecycle.Activator = providers.Activator{}
	if out, _, err := r.cli.execute(false, "add", plugin, "--target", "kiro", "--prepare"); err != nil {
		t.Fatalf("prepare: %s %v", out, err)
	}
	if err := check(); err == nil {
		t.Fatal("unrelated prepared installation suppressed automatic Kiro version and bypassed current failed Directory evidence")
	}
}

func TestUnrelatedRetainedPreparationPreservesAutomaticDirectoryGate(t *testing.T) {
	r := newRolloutDirectoryFixture(t, []domain.ClientID{domain.ClientKiro}, []domain.ClientID{domain.ClientKiro})
	r.cli.app.Detector = reviewVersionDetector{client: fixtureClient(t, domain.ClientKiro)}
	failed := intendedTrustedDirectoryEvidence(domain.DirectoryEvidence{
		ID: "failed/runtime/kiro", DistributionID: "owner/rollout", ReleaseSequence: 1,
		PackageTreeDigest: r.v1.tree, Level: "runtime", Outcome: "failed", Client: domain.ClientKiro,
		InstallerVersion: r.cli.app.Version,
	})
	snapshot := &r.directory.bundle.Snapshot
	snapshot.Evidence = append(snapshot.Evidence, failed)
	snapshot.Distributions[0].ReleasePolicies[0].CurrentEvidence = append(snapshot.Distributions[0].ReleasePolicies[0].CurrentEvidence, failed.ID)
	check := func() {
		t.Helper()
		calls := r.acquirer.verifiedCalls
		out, _, err := r.cli.execute(false, "add", "rollout-demo", "--target", "kiro")
		if err == nil || !strings.Contains(err.Error(), "current trusted runtime evidence failed for kiro") {
			t.Fatalf("automatic Directory evidence gate: %s %v", out, err)
		}
		if r.acquirer.verifiedCalls != calls {
			t.Fatal("failed evidence reached acquisition")
		}
	}
	check()
	plugin := writeCLIPlugin(t)
	body, err := os.ReadFile(filepath.Join(plugin, "plugin.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(plugin, "plugin.json"), []byte(strings.ReplaceAll(string(body), `"demo"`, `"unrelated"`)), 0600); err != nil {
		t.Fatal(err)
	}
	writeCLIMCP(t, plugin)
	r.cli.app.Lifecycle.Activator = providers.Activator{}
	if out, _, err := r.cli.execute(false, "add", plugin, "--target", "kiro", "--prepare"); err != nil {
		t.Fatalf("prepare: %s %v", out, err)
	}
	check()
	if out, _, err := r.cli.execute(false, "remove", "unrelated", "--target", "kiro"); err != nil {
		t.Fatalf("remove: %s %v", out, err)
	}
	state, err := r.cli.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Installations) != 1 || len(state.Installations[0].InstallPreferences) != 1 || state.Installations[0].InstallPreferences[0].InstallIntent != domain.InstallIntentPrepare {
		t.Fatalf("missing retained preparation: %+v", state)
	}
	check()
}

func TestUnrelatedPreparationPreservesAutomaticUpdateAndRepairEvidence(t *testing.T) {
	r := newRolloutDirectoryFixture(t, []domain.ClientID{domain.ClientKiro}, []domain.ClientID{domain.ClientKiro})
	r.cli.app.Detector = reviewVersionDetector{client: fixtureClient(t, domain.ClientKiro)}
	writeCLIMCP(t, r.v1.root)
	loaded, err := r.cli.app.acquireLocal(context.Background(), r.v1.root)
	if err != nil {
		t.Fatal(err)
	}
	r.v1.tree = loaded.envelope.TreeDigest
	release := &r.directory.bundle.Snapshot.Distributions[0].Releases[0]
	release.TreeDigest = r.v1.tree
	release.ManifestDigest = loaded.envelope.ManifestDigest
	for i := range r.directory.bundle.Snapshot.Evidence {
		evidence := &r.directory.bundle.Snapshot.Evidence[i]
		if evidence.ReleaseSequence == 1 {
			evidence.PackageTreeDigest = r.v1.tree
		}
	}
	if err := loaded.cleanup(); err != nil {
		t.Fatal(err)
	}
	// Seed a recorded native binding with disposable preparation, then model a
	// historical automatic binding. Every command below must fail at resolution,
	// before acquisition or activation can execute a client.
	if out, _, err := r.cli.execute(false, "add", "rollout-demo", "--target", "kiro", "--prepare"); err != nil {
		t.Fatalf("seed: %s %v", out, err)
	}
	state, err := r.cli.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	for key, binding := range state.Installations[0].Clients {
		binding.InstallIntent = domain.InstallIntentAutomatic
		state.Installations[0].Clients[key] = binding
	}
	if err := r.cli.store.Save(state); err != nil {
		t.Fatal(err)
	}
	// Keep update on the installed release so both commands evaluate its evidence.
	snapshot := &r.directory.bundle.Snapshot
	snapshot.Distributions[0].Releases = snapshot.Distributions[0].Releases[:1]
	snapshot.Distributions[0].ReleasePolicies = snapshot.Distributions[0].ReleasePolicies[:1]
	failed := intendedTrustedDirectoryEvidence(domain.DirectoryEvidence{
		ID: "failed/runtime/kiro", DistributionID: "owner/rollout", ReleaseSequence: 1,
		PackageTreeDigest: r.v1.tree, Level: "runtime", Outcome: "failed", Client: domain.ClientKiro,
		InstallerVersion: r.cli.app.Version,
	})
	snapshot.Evidence = append(snapshot.Evidence, failed)
	snapshot.Distributions[0].ReleasePolicies[0].CurrentEvidence = append(snapshot.Distributions[0].ReleasePolicies[0].CurrentEvidence, failed.ID)
	check := func() {
		t.Helper()
		for _, operation := range []string{"update", "repair"} {
			calls := r.acquirer.verifiedCalls
			out, _, err := r.cli.execute(false, operation, "demo", "--target", "kiro")
			if err == nil || !strings.Contains(err.Error(), "current trusted runtime evidence failed for kiro") {
				t.Fatalf("%s gate: %s %v", operation, out, err)
			}
			if r.acquirer.verifiedCalls != calls {
				t.Fatal("failed evidence reached acquisition")
			}
		}
	}
	check()
	plugin := writeCLIPlugin(t)
	body, err := os.ReadFile(filepath.Join(plugin, "plugin.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(plugin, "plugin.json"), []byte(strings.ReplaceAll(string(body), `"demo"`, `"unrelated"`)), 0600); err != nil {
		t.Fatal(err)
	}
	writeCLIMCP(t, plugin)
	mcpPath := filepath.Join(plugin, "mcp.json")
	mcp, err := os.ReadFile(mcpPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(mcpPath, []byte(strings.ReplaceAll(string(mcp), `"demo"`, `"unrelated"`)), 0600); err != nil {
		t.Fatal(err)
	}

	if out, _, err := r.cli.execute(false, "add", plugin, "--target", "kiro", "--prepare"); err != nil {
		t.Fatalf("prepare: %s %v", out, err)
	}
	check()
	if out, _, err := r.cli.execute(false, "remove", "unrelated", "--target", "kiro"); err != nil {
		t.Fatalf("remove: %s %v", out, err)
	}
	check()
}

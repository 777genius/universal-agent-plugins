package agentpluginscli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/directoryv1"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
)

const fixturePersonalAppID = "plugin_asdk_app_0123456789abcdef0123456789abcdef"

// This fixture exercises the verified-acquisition interface with immutable local
// bytes. It is deliberately not a live catalog or remote ChatGPT qualification.
func context7GuidedFixture(t *testing.T) (cliFixture, *fixedDirectoryClient, *localBackedSourceAcquirer) {
	t.Helper()
	kiro := fixtureClient(t, domain.ClientKiro)
	f := newCLIFixture(t, []domain.DetectedClient{kiro})
	f.app.Lifecycle.NativeObserver = providers.NativeIdentityObserver{Stager: providers.Stager{}}
	root := writeCLIPlugin(t)
	manifest, err := os.ReadFile(filepath.Join(root, "plugin.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "plugin.json"), []byte(strings.ReplaceAll(string(manifest), `"demo"`, `"context7"`)), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "mcp.json"), []byte(`{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"context7":{"type":"streamable-http","url":"https://mcp.context7.com/mcp/oauth"}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	loaded, err := f.app.acquireLocal(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	defer loaded.cleanup()
	release := domain.DirectoryRelease{Sequence: 1, PackageVersion: "1.0.0", ManifestName: "context7", AgentPluginsSchema: "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json", PackageSource: domain.DirectorySource{Repository: "upstash/context7", Path: "plugins/agent-plugins/context7", Revision: strings.Repeat("a", 40)}, TreeDigestAlgorithm: domain.TreeDigestAlgorithm, TreeDigest: loaded.envelope.TreeDigest, ManifestDigest: loaded.envelope.ManifestDigest, Components: []string{"mcp"}, PublishedAt: "2026-08-21T00:00:00Z"}
	policy := domain.DirectoryReleasePolicy{ReleaseSequence: 1, Status: domain.ReleaseActive, MinimumInstallerVersion: "0.1.0", Targets: []domain.DirectoryTarget{{Client: domain.ClientKiro, Scopes: []domain.InstallScope{domain.ScopeUser}, Delivery: "managed", Authentication: domain.AuthenticationRequirementRequired}}, CurrentEvidence: []string{"materialized/kiro"}}
	snapshot := domain.DirectorySnapshot{SnapshotSchemaVersion: 1, Sequence: 17, SourceCommit: strings.Repeat("b", 40), Products: []domain.DirectoryProduct{{SchemaVersion: 1, ID: "context7", ManifestName: "context7", Aliases: []string{"context7", "context7-alias"}, DefaultDistribution: "upstash/context7", Distributions: []string{"upstash/context7"}}}, Distributions: []domain.DirectoryDistribution{{SchemaVersion: 1, ID: "upstash/context7", ProductID: "context7", Kind: domain.DistributionUpstream, Status: domain.DistributionActive, Releases: []domain.DirectoryRelease{release}, ReleasePolicies: []domain.DirectoryReleasePolicy{policy}}}, Evidence: []domain.DirectoryEvidence{intendedTrustedDirectoryEvidence(domain.DirectoryEvidence{ID: "materialized/kiro", DistributionID: "upstash/context7", ReleaseSequence: 1, PackageTreeDigest: release.TreeDigest, Level: "materialization", Outcome: "passed", Client: domain.ClientKiro})}}
	directory := &fixedDirectoryClient{bundle: directoryv1.VerifiedBundle{Snapshot: snapshot, Digest: "sha256:" + strings.Repeat("c", 64)}}
	acquirer := &localBackedSourceAcquirer{delegate: f.app.SourceAcquirer, root: root}
	f.app.DirectoryClient, f.app.SourceAcquirer = directory, acquirer
	return f, directory, acquirer
}

func TestContext7GuidedMissingRegistrationAndResume(t *testing.T) {
	f, directory, acquirer := context7GuidedFixture(t)
	out, _, err := f.execute(false, "add", "context7", "--target", "kiro,chatgpt", "--prepare", "--format", "json")
	if err == nil {
		t.Fatal("missing registration must require action")
	}
	var response struct {
		Data map[string]any `json:"data"`
	}
	if json.Unmarshal([]byte(out), &response) != nil || response.Data["status"] != "action_required" || response.Data["mcp_url"] != domain.Context7OAuthURL || response.Data["remote_verified"] != false {
		t.Fatalf("action response %s: %v", out, err)
	}
	state, _ := f.store.Load()
	if len(state.Installations) != 0 {
		t.Fatal("missing registration mutated state")
	}
	if acquirer.verifiedCalls != 1 || acquirer.directGitCalls != 0 || acquirer.localCalls != 0 {
		t.Fatalf("acquisition %#v", acquirer)
	}
	before, _ := json.Marshal(directory.bundle)
	out, _, err = f.execute(false, "add", "context7", "--target", "kiro,chatgpt", "--prepare", "--chatgpt-app-id", fixturePersonalAppID, "--format", "json")
	if err != nil {
		t.Fatalf("resume: %s %v", out, err)
	}
	after, _ := json.Marshal(directory.bundle)
	if string(before) != string(after) {
		t.Fatal("mutated signed Directory")
	}
	if acquirer.verifiedCalls != 2 {
		t.Fatal("mixed preparation did not acquire exactly once")
	}
	state, err = f.store.Load()
	if err != nil || len(state.Installations) != 1 {
		t.Fatalf("state %+v %v", state, err)
	}
	i := state.Installations[0]
	if i.LocalChatGPTMapping == nil || i.LocalChatGPTMapping.AppID != fixturePersonalAppID || len(i.Clients) != 2 {
		t.Fatalf("receipt %+v", i)
	}
	for _, b := range i.Clients {
		if b.InstallIntent != domain.InstallIntentPrepare || b.Activation != domain.ActivationPrepared || b.Verification != domain.VerificationPackageValid {
			t.Fatalf("false completion %+v", b)
		}
		if b.PackageRevision != nil && b.PackageRevision.CatalogEvidence != nil {
			if _, ok := b.PackageRevision.CatalogEvidence.Compatibility["chatgpt"]; ok {
				t.Fatal("invented signed ChatGPT compatibility")
			}
		}
		if b.ClientID == "chatgpt" {
			raw, err := os.ReadFile(filepath.Join(b.TargetLocator, ".app.json"))
			if err != nil {
				t.Fatal(err)
			}
			if string(raw) != `{"apps":{"context7":{"id":"`+fixturePersonalAppID+`"}}}` {
				t.Fatalf("projection %s", raw)
			}
			info, _ := os.Stat(filepath.Join(b.TargetLocator, ".app.json"))
			if info.Mode().Perm() != 0600 {
				t.Fatalf("app mode %v", info.Mode())
			}
			m := readCLIObject(t, filepath.Join(b.TargetLocator, ".codex-plugin", "plugin.json"))
			mcp := readCLIObject(t, filepath.Join(b.TargetLocator, ".mcp.json"))
			servers := mcp["mcpServers"].(map[string]any)
			if len(servers) != 1 || servers["context7"].(map[string]any)["url"] != domain.Context7OAuthURL {
				t.Fatalf("OAuth endpoint changed: %+v", mcp)
			}
			marketplace := readCLIObject(t, filepath.Join(b.TargetLocator, ".agents", "plugins", "marketplace.json"))
			plugins := marketplace["plugins"].([]any)
			if len(plugins) != 1 || plugins[0].(map[string]any)["name"] != "context7" {
				t.Fatalf("personal marketplace %+v", marketplace)
			}
			if m["apps"] != "./.app.json" {
				t.Fatalf("manifest %+v", m)
			}
		}
	}
	if _, err := os.Stat(filepath.Join(acquirer.root, ".app.json")); !os.IsNotExist(err) {
		t.Fatal("source package was modified")
	}
	info, _ := os.Stat(f.store.Path)
	if info.Mode().Perm() != 0600 {
		t.Fatal("personal receipt is not private")
	}
}

func TestContext7GuidedLifecycleRetainsReceipt(t *testing.T) {
	for _, purge := range []bool{false, true} {
		t.Run(map[bool]string{false: "retain-data", true: "purge-data"}[purge], func(t *testing.T) {
			f, _, _ := context7GuidedFixture(t)
			run := func(args ...string) {
				t.Helper()
				out, _, err := f.execute(false, args...)
				if err != nil {
					t.Fatalf("%v: %s %v", args, out, err)
				}
			}
			run("add", "context7", "--target", "chatgpt", "--prepare", "--chatgpt-app-id", fixturePersonalAppID)
			run("add", "context7-alias", "--target", "chatgpt")
			run("update", "context7", "--target", "chatgpt")
			run("repair", "context7", "--target", "chatgpt")
			run("add", "context7", "--target", "chatgpt", "--activation-complete", "--auth-complete")
			state, err := f.store.Load()
			if err != nil {
				t.Fatal(err)
			}
			b := onlyCLIClient(state.Installations[0])
			if b.Verification != domain.VerificationPackageValid || b.Authentication != domain.AuthenticationPending {
				t.Fatalf("fabricated remote completion %+v", b)
			}
			if err := os.RemoveAll(b.TargetLocator); err != nil {
				t.Fatal(err)
			}
			run("repair", "context7", "--target", "chatgpt")
			args := []string{"remove", "context7", "--target", "chatgpt", "--external-uninstalled"}
			if purge {
				args = append(args, "--purge-data")
			}
			run(args...)
			state, err = f.store.Load()
			if err != nil || len(state.Installations) != 1 || state.Installations[0].LocalChatGPTMapping == nil {
				t.Fatalf("receipt lost: %+v %v", state, err)
			}
			run("add", "context7", "--target", "chatgpt")
		})
	}
}

func TestContext7GuidedRejectsInvalidIDsAndUnverifiedSource(t *testing.T) {
	for _, id := range []string{"connector_old", "asdk_app_old", "plugin_asdk_app_", "plugin_asdk_app_short", "plugin_asdk_app_bad/id", " plugin_asdk_app_abc"} {
		t.Run(id, func(t *testing.T) {
			f, _, a := context7GuidedFixture(t)
			_, _, err := f.execute(false, "add", "context7", "--target", "chatgpt", "--prepare", "--chatgpt-app-id", id)
			if err == nil || a.verifiedCalls != 0 {
				t.Fatalf("invalid ID reached acquisition: %v", err)
			}
		})
	}
	for _, kind := range []string{"tree", "manifest", "endpoint", "identity", "mixed-peer"} {
		t.Run(kind, func(t *testing.T) {
			f, d, a := context7GuidedFixture(t)
			switch kind {
			case "tree":
				d.bundle.Snapshot.Distributions[0].Releases[0].TreeDigest = "sha256:" + strings.Repeat("d", 64)
			case "manifest":
				d.bundle.Snapshot.Distributions[0].Releases[0].ManifestDigest = "sha256:" + strings.Repeat("d", 64)
			case "endpoint":
				raw, _ := os.ReadFile(filepath.Join(a.root, "mcp.json"))
				os.WriteFile(filepath.Join(a.root, "mcp.json"), []byte(strings.ReplaceAll(string(raw), domain.Context7OAuthURL, "https://mcp.context7.com/mcp")), 0600)
			case "identity":
				d.bundle.Snapshot.Distributions[0].Releases[0].PackageSource.Repository = "other/context7"
			case "mixed-peer":
				d.bundle.Snapshot.Distributions[0].ReleasePolicies[0].Targets[0].Scopes = []domain.InstallScope{domain.ScopeProject}
			}
			_, _, err := f.execute(false, "add", "context7", "--target", "kiro,chatgpt", "--prepare", "--chatgpt-app-id", fixturePersonalAppID)
			if err == nil {
				t.Fatal("accepted invalid source")
			}
			state, _ := f.store.Load()
			if len(state.Installations) != 0 {
				t.Fatal("mutated")
			}
		})
	}
}

func TestContext7GuidedReceiptRejectsChangedRegistrationAndSignedEndpoint(t *testing.T) {
	f, d, a := context7GuidedFixture(t)
	if out, _, err := f.execute(false, "add", "context7", "--target", "chatgpt", "--prepare", "--chatgpt-app-id", fixturePersonalAppID); err != nil {
		t.Fatalf("%s %v", out, err)
	}
	before, _ := os.ReadFile(f.store.Path)
	_, _, err := f.execute(false, "add", "context7", "--target", "chatgpt", "--prepare", "--chatgpt-app-id", "plugin_asdk_app_ffffffffffffffffffffffffffffffff")
	if err == nil || !strings.Contains(err.Error(), "conflicts with retained") {
		t.Fatalf("replaced receipt: %v", err)
	}
	raw, _ := os.ReadFile(filepath.Join(a.root, "mcp.json"))
	if err := os.WriteFile(filepath.Join(a.root, "mcp.json"), []byte(strings.ReplaceAll(string(raw), domain.Context7OAuthURL, "https://mcp.context7.com/mcp")), 0600); err != nil {
		t.Fatal(err)
	}
	loaded, err := f.app.acquireLocal(context.Background(), a.root)
	if err != nil {
		t.Fatal(err)
	}
	defer loaded.cleanup()
	// A newly signed release can change bytes, but cannot silently rebind a
	// personal registration to a different endpoint, even with valid digests.
	dist := &d.bundle.Snapshot.Distributions[0]
	release := dist.Releases[0]
	release.Sequence = 2
	release.PackageSource.Revision = strings.Repeat("d", 40)
	release.TreeDigest = loaded.envelope.TreeDigest
	release.ManifestDigest = loaded.envelope.ManifestDigest
	policy := dist.ReleasePolicies[0]
	policy.ReleaseSequence = 2
	policy.CurrentEvidence = []string{"materialized/kiro/2"}
	dist.Releases = append(dist.Releases, release)
	dist.ReleasePolicies = append(dist.ReleasePolicies, policy)
	d.bundle.Snapshot.Evidence = append(d.bundle.Snapshot.Evidence, intendedTrustedDirectoryEvidence(domain.DirectoryEvidence{ID: policy.CurrentEvidence[0], DistributionID: dist.ID, ReleaseSequence: 2, PackageTreeDigest: release.TreeDigest, Level: "materialization", Outcome: "passed", Client: domain.ClientKiro}))
	_, _, err = f.execute(false, "update", "context7", "--target", "chatgpt")
	if err == nil || !strings.Contains(err.Error(), "OAuth server") {
		t.Fatalf("rebound endpoint: %v", err)
	}
	after, _ := os.ReadFile(f.store.Path)
	if string(before) != string(after) {
		t.Fatal("failed update changed receipt/state")
	}
}

func TestContext7GuidedForeignFilesAndCancellation(t *testing.T) {
	f, _, _ := context7GuidedFixture(t)
	foreign := filepath.Join(f.app.UserHome, ".agents", "plugins", "marketplace.json")
	if err := os.MkdirAll(filepath.Dir(foreign), 0700); err != nil {
		t.Fatal(err)
	}
	content := []byte(`{"name":"foreign-authoring","plugins":[]}`)
	if err := os.WriteFile(foreign, content, 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	command := NewRoot(f.app)
	command.SetArgs([]string{"add", "context7", "--target", "chatgpt", "--prepare", "--chatgpt-app-id", fixturePersonalAppID})
	if err := command.ExecuteContext(ctx); err == nil {
		t.Fatal("cancelled command completed")
	}
	state, err := f.store.Load()
	if err != nil || len(state.Installations) != 0 {
		t.Fatalf("cancel mutated state %+v %v", state, err)
	}
	for _, args := range [][]string{
		{"add", "context7", "--target", "chatgpt", "--prepare", "--chatgpt-app-id", fixturePersonalAppID},
		{"remove", "context7", "--target", "chatgpt", "--external-uninstalled", "--purge-data"},
	} {
		if out, _, err := f.execute(false, args...); err != nil {
			t.Fatalf("%v: %s %v", args, out, err)
		}
	}
	actual, err := os.ReadFile(foreign)
	if err != nil || string(actual) != string(content) {
		t.Fatalf("foreign authoring changed %s %v", actual, err)
	}
}

func TestContext7GuidedMixedUpdateUsesOneNewImmutableRelease(t *testing.T) {
	f, d, a := context7GuidedFixture(t)
	if out, _, err := f.execute(false, "add", "context7", "--target", "kiro,chatgpt", "--prepare", "--chatgpt-app-id", fixturePersonalAppID); err != nil {
		t.Fatalf("%s %v", out, err)
	}
	raw, err := os.ReadFile(filepath.Join(a.root, "plugin.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.root, "plugin.json"), []byte(strings.ReplaceAll(string(raw), "1.0.0\"", "2.0.0\"")), 0600); err != nil {
		t.Fatal(err)
	}
	loaded, err := f.app.acquireLocal(context.Background(), a.root)
	if err != nil {
		t.Fatal(err)
	}
	defer loaded.cleanup()
	dist := &d.bundle.Snapshot.Distributions[0]
	release := dist.Releases[0]
	release.Sequence = 2
	release.PackageVersion = "2.0.0"
	release.PackageSource.Revision = strings.Repeat("d", 40)
	release.TreeDigest = loaded.envelope.TreeDigest
	release.ManifestDigest = loaded.envelope.ManifestDigest
	policy := dist.ReleasePolicies[0]
	policy.ReleaseSequence = 2
	policy.CurrentEvidence = []string{"materialized/kiro/2"}
	dist.Releases = append(dist.Releases, release)
	dist.ReleasePolicies = append(dist.ReleasePolicies, policy)
	d.bundle.Snapshot.Evidence = append(d.bundle.Snapshot.Evidence, intendedTrustedDirectoryEvidence(domain.DirectoryEvidence{ID: policy.CurrentEvidence[0], DistributionID: dist.ID, ReleaseSequence: 2, PackageTreeDigest: release.TreeDigest, Level: "materialization", Outcome: "passed", Client: domain.ClientKiro}))
	beforeCalls := a.verifiedCalls
	if out, _, err := f.execute(false, "update", "context7", "--target", "kiro,chatgpt"); err != nil {
		t.Fatalf("%s %v", out, err)
	}
	state, err := f.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	i := state.Installations[0]
	if a.verifiedCalls != beforeCalls+1 || i.Directory.DesiredReleaseSequence != 2 || i.LocalChatGPTMapping == nil || i.LocalChatGPTMapping.AppID != fixturePersonalAppID {
		t.Fatalf("mixed update receipt %+v calls %d", i, a.verifiedCalls)
	}
	for _, b := range i.Clients {
		if b.PackageRevision == nil || b.PackageRevision.TreeDigest != release.TreeDigest || b.PackageRevision.ResolvedRevision != release.PackageSource.Revision || b.InstallIntent != domain.InstallIntentPrepare || b.Activation != domain.ActivationPrepared {
			t.Fatalf("mixed update not same revision: %+v", b)
		}
	}
}

func TestContext7GuidedDoesNotBypassSignatureOrScope(t *testing.T) {
	f, d, a := context7GuidedFixture(t)
	d.err = fmt.Errorf("signed Directory signature verification failed")
	out, _, err := f.execute(false, "add", "context7", "--target", "chatgpt", "--prepare", "--format", "json")
	if err == nil || a.verifiedCalls != 0 || strings.Contains(out, "action_required") {
		t.Fatalf("signature failure hidden: %s %v", out, err)
	}
	for _, args := range [][]string{
		{"add", a.root, "--target", "chatgpt", "--prepare"},
		{"add", "context7", "--target", "chatgpt", "--prepare", "--scope", "project"},
		{"add", "context7", "--target", "chatgpt", "--chatgpt-app-id", fixturePersonalAppID},
	} {
		if _, _, err := f.execute(false, args...); err == nil {
			t.Fatalf("accepted invalid preparation request %v", args)
		}
	}
}

package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/directoryv1"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/sourceacquisition"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/specregistry"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

const (
	piFullRegistrySourceCommit = "32b6c48d071aceadf90a030c9d69477fe1a6a65a"
	piFullTestFeedID           = "pi-full-real-sources-test-only"
	piFullSnapshotPath         = "testdata/pi-full-modern-v1-test-only-snapshot.json"
	piFullShardCount           = 3
)

// This is NEVER publisher evidence. The snapshot deliberately contains
// synthetically granted Pi targets and no reviewed real-host evidence.
// The test asserts the real pinned 36 package bytes and saved installer
// semantics, but cannot prove protected Ed25519 authority or account login.
func piFullSignedTestDirectory(t *testing.T, cacheRoot string) (*directoryv1.Client, domain.DirectorySnapshot) {
	t.Helper()
	body, err := os.ReadFile(piFullSnapshotPath)
	if err != nil {
		t.Fatal(err)
	}
	var snapshot domain.DirectorySnapshot
	if err := json.Unmarshal(body, &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot.SourceCommit != piFullRegistrySourceCommit || snapshot.Sequence != 41 ||
		len(snapshot.Distributions) != 36 || len(snapshot.Products) != 29 ||
		len(snapshot.Evidence) != 0 || len(snapshot.Revocations) != 0 {
		t.Fatal("pinned synthetic snapshot identity or 36-source coverage changed")
	}
	total := 0
	for _, distribution := range snapshot.Distributions {
		if distribution.Status != domain.DistributionActive || len(distribution.Releases) != 1 ||
			len(distribution.ReleasePolicies) != 1 || len(distribution.ReleasePolicies[0].Targets) != 1 ||
			distribution.ReleasePolicies[0].Targets[0].Client != domain.ClientPi ||
			len(distribution.ReleasePolicies[0].Targets[0].Scopes) != 1 ||
			distribution.ReleasePolicies[0].Targets[0].Scopes[0] != domain.ScopeUser ||
			distribution.ReleasePolicies[0].ReleaseSequence != distribution.Releases[0].Sequence {
			t.Fatalf("unreviewed source or unexpected signed grant: %s", distribution.ID)
		}
		total++
	}
	if total != 36 {
		t.Fatal("not all 36 active source policies were pinned")
	}
	seed := bytes.Repeat([]byte{93}, ed25519.SeedSize)
	privateKey := ed25519.NewKeyFromSeed(seed)
	const keyID = "pi-full-test-signer-only"
	trust := directoryv1.TrustStore{Keys: []directoryv1.TrustedKey{{
		ID: keyID, PublicKey: privateKey.Public().(ed25519.PublicKey), State: directoryv1.KeyCurrent,
	}}}
	digest := sha256.Sum256(body)
	envelope, err := json.Marshal(directoryv1.Envelope{
		EnvelopeSchemaVersion: 1, SnapshotSchemaVersion: 1, Sequence: snapshot.Sequence,
		KeyID: keyID, Algorithm: "Ed25519", SignatureDomain: directoryv1.SignatureDomain,
		SnapshotDigest: "sha256:" + hex.EncodeToString(digest[:]),
		Signature: base64.StdEncoding.EncodeToString(
			ed25519.Sign(privateKey, directoryv1.SnapshotSignatureMessage(body))),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := directoryv1.VerifyBundle(body, envelope, trust); err != nil {
		t.Fatalf("test-only full modern schema-1 source policy failed strict verification: %v", err)
	}
	now := time.Date(2026, 11, 12, 1, 0, 0, 0, time.UTC)
	client := &directoryv1.Client{
		Origin: "https://fixture.invalid/pi-full-test-only/schemas/1/",
		FeedID: piFullTestFeedID,
		HTTPClient: &http.Client{Transport: conformanceFixtureTransport{
			sequence: snapshot.Sequence, snapshot: body, envelope: envelope,
		}},
		Trust: trust, Embedded: directoryv1.EmbeddedBundle{Snapshot: body, Envelope: envelope},
		RequireEmbeddedBootstrap: true,
		Cache: directoryv1.Cache{Path: filepath.Join(cacheRoot, "pi-full-test-only-cache.json")},
		Now: func() time.Time { return now },
	}
	return client, snapshot
}

func piFullRealSavedJourney(t *testing.T) (projectJourney, domain.DirectorySnapshot) {
	t.Helper()
	journey := projectJourney{
		home: projectJourneyTemp(t),
		a: projectJourneyTemp(t),
		b: projectJourneyTemp(t),
	}
	journey.cache = filepath.Join(journey.home, "uap")
	projectJourneyWrite(t, filepath.Join(journey.home, "pi-test-host-never-launched"), "inert Pi host fixture\n")
	for _, root := range []string{"", journey.a, journey.b} {
		native := piSignedProjectNativeRoot(journey, root)
		projectJourneyWrite(t, filepath.Join(native, "mcp.json"), "{\"mcpServers\":{\"foreign\":{\"type\":\"streamable-http\",\"url\":\"https://foreign.example.test/mcp\"}},\"keep\":true}")
		projectJourneyWrite(t, filepath.Join(native, "skills", "foreign", "SKILL.md"), "foreign Pi skill\n")
		if err := os.MkdirAll(filepath.Join(native, "extensions"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	client, snapshot := piFullSignedTestDirectory(t, journey.cache)
	registry, err := specregistry.New()
	if err != nil {
		t.Fatal(err)
	}
	app, err := newAgentpluginsCLIApp(journey.home, journey.cache, nil, nil, nil, registry)
	if err != nil {
		t.Fatal(err)
	}
	newAcquirer := func(root string) agentpluginscli.SourceAcquirer {
		// Only immutable GitHub public commits from the pinned test snapshot.
		// Native OAuth, plugins' MCP binaries and package scripts are never run.
		return sourceacquisition.Acquirer{TempRoot: root}
	}
	var configure func(agentpluginscli.App) agentpluginscli.App
	configure = func(app agentpluginscli.App) agentpluginscli.App {
		app.Version = "1.18.34"
		app.Detector = piSignedProjectDetector{home: journey.home}
		app.DirectoryClient = client
		app.DirectoryClientForDryRun = func(string) (agentpluginscli.DirectoryClient, error) { return client, nil }
		app.SourceAcquirer = newAcquirer(t.TempDir())
		app.SourceAcquirerForDryRun = newAcquirer
		app.SecurityIndex, app.DiscoveryClient = nil, nil
		app.SecurityEvaluator = projectJourneySecurity{}
		app.SecurityEvaluatorForDryRun = func(string) ports.PackageSecurityEvaluator { return projectJourneySecurity{} }
		parent := app.ForScope
		app.ForScope = func(scope domain.InstallScope, root string) (agentpluginscli.App, error) {
			selected, err := parent(scope, root)
			if err != nil {
				return selected, err
			}
			return configure(selected), nil
		}
		return app
	}
	journey.app = configure(app)
	return journey, snapshot
}

// The User and Project commands run against original immutable public Git
// sources and a TEST-ONLY independently signed v1 snapshot. A green result
// is not production key trust or a genuine live Pi/model/MCP/OAuth session.
func TestPiFullModernSignedTestOnlyMarketplace(t *testing.T) {
	if os.Getenv("UAP_PI_FULL_REAL_SOURCE_E2E") != "1" {
		t.Skip("isolated three-OS real public-source matrix only")
	}
	shard, err := strconv.Atoi(os.Getenv("UAP_PI_FULL_SHARD"))
	if err != nil || shard < 0 || shard >= piFullShardCount {
		t.Fatal("a bounded exact source shard 0..2 is required")
	}
	_, snapshot := piFullRealSavedJourney(t)
	selected := 0
	passedCases := make([]string, 0, 24)
	for index, distribution := range snapshot.Distributions {
		if index%piFullShardCount != shard {
			continue
		}
		selected++
		for _, scope := range []domain.InstallScope{domain.ScopeUser, domain.ScopeProject} {
			caseName := fmt.Sprintf("%s@%d/%s", distribution.ID, distribution.Releases[0].Sequence, scope)
			passed := t.Run(caseName, func(t *testing.T) {
				j, _ := piFullRealSavedJourney(t)
				root := ""
				if scope == domain.ScopeProject {
					root = j.a
				}
				originalB := projectJourneyRead(t, filepath.Join(piSignedProjectNativeRoot(j, j.b), "mcp.json"))
				j.run(t, root, "install", distribution.ID, "--target", "pi", "--yes")
				state := j.state(t, root)
				expected := 1
				if distribution.ProductID == "marketing-kit" {
					expected = 4
				}
				if len(state.Installations) != expected || len(state.EnvironmentReceipts) != expected {
					t.Fatalf("signed test-only saved %s %s omitted packages: installations=%d receipts=%d expected=%d", distribution.ID, scope, len(state.Installations), len(state.EnvironmentReceipts), expected)
				}
				for _, installation := range state.Installations {
					if installation.Directory == nil || installation.Directory.FeedID != piFullTestFeedID {
						t.Fatalf("signed test-only release lost feed provenance for %s: %+v", distribution.ID, installation.Directory)
					}
				}
				if got := projectJourneyRead(t, filepath.Join(piSignedProjectNativeRoot(j, j.b), "mcp.json")); got != originalB {
					t.Fatal("Pi install changed unselected Project B foreign native settings")
				}
				j.run(t, root, "deps", "sync", "--locked", "--yes")
				if got := len(j.state(t, root).EnvironmentReceipts); got != expected {
					t.Fatalf("replay changed signed test-only receipt count: got %d want %d", got, expected)
				}
				// A canonical distribution ID selects the source at install time;
				// saved removal resolves the declared root alias, not an imperative
				// mutation of environment-owned bindings.
				j.run(t, root, "remove", distribution.ProductID, "--target", "pi", "--yes")
				if got := len(j.state(t, root).EnvironmentReceipts); got != 0 {
					t.Fatalf("signed test-only %s %s removal left %d owned receipts", distribution.ID, scope, got)
				}
				if got := projectJourneyRead(t, filepath.Join(piSignedProjectNativeRoot(j, j.b), "mcp.json")); got != originalB {
					t.Fatal("Pi remove changed unselected Project B foreign native settings")
				}
			})
			if passed {
				passedCases = append(passedCases, caseName)
			}
		}
	}
	if selected != 12 {
		t.Fatalf("exact shard should cover 12 active distributions, got %d", selected)
	}
	if len(passedCases) != 24 {
		t.Fatalf("missing actual successful saved Pi scopes in shard %d: %d/24", shard, len(passedCases))
	}
	if target := os.Getenv("UAP_PI_FULL_REPORT"); target != "" {
		body, err := os.ReadFile(piFullSnapshotPath)
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(body)
		report, err := json.MarshalIndent(struct {
			Kind string `json:"kind"`
			SourceCommit string `json:"source_commit"`
			SnapshotSHA256 string `json:"snapshot_sha256"`
			RunnerOS string `json:"runner_os"`
			Shard int `json:"shard"`
			Cases []string `json:"passed_cases"`
			ProductionPublished bool `json:"production_published"`
		}{
			Kind: "pi-full-signed-v1-test-only-saved", SourceCommit: piFullRegistrySourceCommit,
			SnapshotSHA256: "sha256:" + hex.EncodeToString(sum[:]), RunnerOS: runtime.GOOS,
			Shard: shard, Cases: passedCases, ProductionPublished: false,
		}, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, append(report, '\n'), 0600); err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("TEST-ONLY Ed25519 full modern v1: %d exact real public sources, %d saved User/Project lifecycles, OS=%s, shard=%d; no live Pi/account/runtime/production key", selected, len(passedCases), runtime.GOOS, shard)
}

package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
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

// Independently reviewed byte identity of the frozen TEST-only fixture.
// The source_commit field alone is not a proof of unchanged fixture bytes.
const piFullExpectedSnapshotSHA256 = "458a5c2809d6d2c443496e18ea3fee309d14f8a12f476f301873f3b9f9769399"

// This is NEVER publisher evidence. The snapshot deliberately contains
// synthetically granted Pi targets and no reviewed real-host evidence.
// The test acquires 34 pinned package sources for saved lifecycles and
// observes two upstream denials, without granting materialization evidence.
func piFullSignedTestDirectory(t *testing.T, cacheRoot string) (*directoryv1.Client, domain.DirectorySnapshot) {
	t.Helper()
	body, err := os.ReadFile(piFullSnapshotPath)
	if err != nil {
		t.Fatal(err)
	}
	if actual := sha256.Sum256(body); hex.EncodeToString(actual[:]) != piFullExpectedSnapshotSHA256 {
		t.Fatal("frozen TEST-only snapshot bytes changed without review")
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
	// Sign the original zero-evidence TEST fixture bytes. Neither source
	// acquisition nor this disposable signature grants upstream materialization.
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
		Cache:                    directoryv1.Cache{Path: filepath.Join(cacheRoot, "pi-full-test-only-cache.json")},
		Now:                      func() time.Time { return now },
	}
	return client, snapshot
}

func piFullRealSavedJourney(t *testing.T) (projectJourney, domain.DirectorySnapshot) {
	t.Helper()
	journey := projectJourney{
		home: projectJourneyTemp(t),
		a:    projectJourneyTemp(t),
		b:    projectJourneyTemp(t),
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

type piFullExpectedDenial struct {
	Case    string `json:"case"`
	Refusal string `json:"refusal"`
}

func piFullUpstreamRefusal(distribution string) string {
	return fmt.Sprintf("%s in %q: upstream release lacks current passed materialization evidence for pi", domain.ErrDirectoryIneligible, distribution)
}

func piFullAssertUpstreamDenied(t *testing.T, j projectJourney, root, distribution string) string {
	t.Helper()
	if distribution != "github/github" && distribution != "upstash/context7" {
		t.Fatalf("unexpected upstream negative case %q", distribution)
	}
	before := make(map[string]map[string]string)
	intentPaths := make([]string, 0, 6)
	for _, scopeRoot := range []string{"", j.a, j.b} {
		state := j.state(t, scopeRoot)
		if len(state.Installations) != 0 || len(state.EnvironmentReceipts) != 0 {
			t.Fatal("upstream denial fixture already owns saved state")
		}
		native := piSignedProjectNativeRoot(j, scopeRoot)
		before[native] = securityFixtureTree(t, native)
		scope := domain.ScopeUser
		if scopeRoot != "" {
			scope = domain.ScopeProject
		}
		selected, err := j.app.ForScope(scope, scopeRoot)
		if err != nil {
			t.Fatal(err)
		}
		intentPaths = append(intentPaths, filepath.Join(selected.DependenciesRoot, "uap.toml"), filepath.Join(selected.DependenciesRoot, "uap.lock"))
	}
	projectJourneyAbsent(t, intentPaths...)
	_, err := j.execute(root, "install", distribution, "--target", "pi", "--yes")
	if !errors.Is(err, domain.ErrDirectoryIneligible) || err.Error() != piFullUpstreamRefusal(distribution) {
		t.Fatalf("expected exact upstream materialization refusal for %s, got %v", distribution, err)
	}
	for _, scopeRoot := range []string{"", j.a, j.b} {
		state := j.state(t, scopeRoot)
		if len(state.Installations) != 0 || len(state.EnvironmentReceipts) != 0 {
			t.Fatal("denied upstream install left saved installations or environment receipts")
		}
		native := piSignedProjectNativeRoot(j, scopeRoot)
		if !reflect.DeepEqual(before[native], securityFixtureTree(t, native)) {
			t.Fatalf("denied upstream install changed native or foreign scope %q", scopeRoot)
		}
	}
	projectJourneyAbsent(t, intentPaths...)
	return err.Error()
}

// The signed zero-evidence fixture proves 68 saved User/Project lifecycles
// and four actual upstream refusals per OS. A refusal is never a successful
// lifecycle, publisher review, production grant or native Pi session.
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
	savedCases := make([]string, 0, 24)
	deniedCases := make([]piFullExpectedDenial, 0, 2)
	wantDenied := 0
	for index, distribution := range snapshot.Distributions {
		if index%piFullShardCount != shard {
			continue
		}
		selected++
		if distribution.Kind == domain.DistributionUpstream {
			wantDenied += 2
		}
		for _, scope := range []domain.InstallScope{domain.ScopeUser, domain.ScopeProject} {
			caseName := fmt.Sprintf("%s@%d/%s", distribution.ID, distribution.Releases[0].Sequence, scope)
			refusal := ""
			completed := false
			passed := t.Run(caseName, func(t *testing.T) {
				j, _ := piFullRealSavedJourney(t)
				root := ""
				if scope == domain.ScopeProject {
					root = j.a
				}
				if distribution.Kind == domain.DistributionUpstream {
					refusal = piFullAssertUpstreamDenied(t, j, root, distribution.ID)
					completed = true
					return
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
				completed = true
			})
			if !passed || !completed {
				t.Fatalf("Pi test-only case %s skipped or failed before completing assertions", caseName)
			}
			if passed {
				if distribution.Kind == domain.DistributionUpstream {
					deniedCases = append(deniedCases, piFullExpectedDenial{Case: caseName, Refusal: refusal})
				} else {
					savedCases = append(savedCases, caseName)
				}
			}
		}
	}
	if selected != 12 {
		t.Fatalf("exact shard should cover 12 active distributions, got %d", selected)
	}
	if len(savedCases) != 24-wantDenied || len(deniedCases) != wantDenied {
		t.Fatalf("missing observed Pi cases in shard %d: saved=%d/%d expected-denied=%d/%d", shard, len(savedCases), 24-wantDenied, len(deniedCases), wantDenied)
	}
	if target := os.Getenv("UAP_PI_FULL_REPORT"); target != "" {
		body, err := os.ReadFile(piFullSnapshotPath)
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(body)
		if hex.EncodeToString(sum[:]) != piFullExpectedSnapshotSHA256 {
			t.Fatal("reviewed Pi snapshot bytes changed before reporting")
		}
		testCommit := os.Getenv("EXPECTED_HEAD")
		decoded, err := hex.DecodeString(testCommit)
		if err != nil || len(decoded) != 20 || hex.EncodeToString(decoded) != testCommit {
			t.Fatal("report requires the exact immutable workflow source commit")
		}
		report, err := json.MarshalIndent(struct {
			Kind                string                 `json:"kind"`
			TestCommit          string                 `json:"test_commit"`
			SourceCommit        string                 `json:"source_commit"`
			SnapshotSHA256      string                 `json:"snapshot_sha256"`
			RunnerOS            string                 `json:"runner_os"`
			Shard               int                    `json:"shard"`
			SavedCases          []string               `json:"saved_cases"`
			ExpectedDeniedCases []piFullExpectedDenial `json:"expected_denied_cases"`
			ProductionPublished bool                   `json:"production_published"`
		}{
			Kind: "pi-signed-v1-test-only-saved-and-denied", TestCommit: testCommit, SourceCommit: piFullRegistrySourceCommit,
			SnapshotSHA256: "sha256:" + hex.EncodeToString(sum[:]), RunnerOS: runtime.GOOS,
			Shard: shard, SavedCases: savedCases, ExpectedDeniedCases: deniedCases, ProductionPublished: false,
		}, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, append(report, '\n'), 0600); err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("TEST-ONLY Ed25519 v1: %d pinned distributions, %d saved User/Project lifecycles, %d observed expected-denied scopes, OS=%s, shard=%d; no upstream evidence or live Pi/account/runtime/production key", selected, len(savedCases), len(deniedCases), runtime.GOOS, shard)
}

// Exercise actual Go filtering: T.Run may return true without executing its
// callback. Every acquisition callback is filtered in this private child;
// its expected failure must not publish a saved/denied product report.
func TestPiFullModernSignedTestOnlyMarketplaceRejectsFilteredCallbacks(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	report := filepath.Join(root, "filtered-report.json")
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, executable,
		"-test.run=^TestPiFullModernSignedTestOnlyMarketplace$/^__no_distribution_matches__$",
		"-test.count=1", "-test.timeout=20s")
	command.WaitDelay = 5 * time.Second
	command.Env = []string{
		"HOME=" + root, "USERPROFILE=" + root,
		"TMPDIR=" + root, "TMP=" + root, "TEMP=" + root,
		"GOMAXPROCS=2", "UAP_PI_FULL_REAL_SOURCE_E2E=1", "UAP_PI_FULL_SHARD=0",
		// Private regression input only: permit the buggy report path to run;
		// this inert head is never a product source claim or uploaded report.
		"EXPECTED_HEAD=" + strings.Repeat("0", 40),
		"UAP_PI_FULL_REPORT=" + report,
	}
	for _, name := range []string{"SystemRoot", "WINDIR"} {
		if value := os.Getenv(name); value != "" {
			command.Env = append(command.Env, name+"="+value)
		}
	}
	output, err := command.CombinedOutput()
	var exit *exec.ExitError
	const refusal = "Pi test-only case 777genius/atlassian@1/user skipped or failed before completing assertions"
	if ctx.Err() != nil || !errors.As(err, &exit) || exit.ExitCode() != 1 || !strings.Contains(string(output), refusal) {
		t.Fatalf("filtered callbacks did not fail at the exact report-completeness guard: %v\n%s", err, output)
	}
	if _, err := os.Lstat(report); !os.IsNotExist(err) {
		t.Fatalf("filtered callbacks published a report or its absence is uncertain: %v", err)
	}
}

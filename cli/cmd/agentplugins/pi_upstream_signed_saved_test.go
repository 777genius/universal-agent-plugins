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
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/directoryv1"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

const piUpstreamTestFeedID = "pi-upstream-positive-test-only"

// This deliberately artificial grant is valid ONLY inside this _test.go file.
// It tests real saved acquisition/materialization of two immutable upstream
// sources. It is NOT evidence reviewed for a production Directory publication.
func piUpstreamTestOnlyClient(t *testing.T, cacheRoot string) (*directoryv1.Client, domain.DirectorySnapshot) {
	t.Helper()
	_, snapshot := piFullSignedTestDirectory(t, cacheRoot)
	if len(snapshot.Evidence) != 0 {
		t.Fatal("frozen negative test fixture unexpectedly has evidence")
	}
	artifactMarker := []byte("synthetic-upstream-fixture-not-a-real-materialization-attestation")
	artifactDigest := sha256.Sum256(artifactMarker)
	upstream := 0
	for i := range snapshot.Distributions {
		distribution := &snapshot.Distributions[i]
		if distribution.Kind != domain.DistributionUpstream {
			continue
		}
		if distribution.ID != "github/github" && distribution.ID != "upstash/context7" {
			t.Fatalf("unreviewed upstream test subject %q", distribution.ID)
		}
		if len(distribution.Releases) != 1 || len(distribution.ReleasePolicies) != 1 ||
			len(distribution.ReleasePolicies[0].CurrentEvidence) != 0 {
			t.Fatal("unexpected upstream fixture release/evidence")
		}
		release := distribution.Releases[0]
		id := "pi-synthetic-" + distribution.ProductID + "-test-only"
		snapshot.Evidence = append(snapshot.Evidence, domain.DirectoryEvidence{
			SchemaVersion: 1, ID: id, DistributionID: distribution.ID,
			ReleaseSequence: release.Sequence, PackageTreeDigest: release.TreeDigest,
			Level: "materialization", Outcome: "passed", Client: domain.ClientPi,
			ClientVersion: "1.5.0", InstallerVersion: "0.1.104",
			OS: "synthetic-fixture", Architecture: "synthetic-fixture",
			ObservedAt: "2026-11-11T00:00:00Z",
			Artifact: domain.DirectoryEvidenceArtifact{
				Repository: "test/isolated", Revision: strings.Repeat("a", 40),
				Path:   "test-only/artificial-grant-do-not-promote.json",
				Digest: "sha256:" + hex.EncodeToString(artifactDigest[:]),
			},
			Trust: &domain.DirectoryEvidenceTrust{Kind: "reviewed_external"},
		})
		distribution.ReleasePolicies[0].CurrentEvidence = []string{id}
		upstream++
	}
	if upstream != 2 || len(snapshot.Evidence) != 2 {
		t.Fatal("test-only positive fixture must contain exactly two synthetic upstream grants")
	}
	body, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	// Different disposable Ed25519 key, feed identity, cache, and origin from
	// both historical production and the frozen zero-evidence negative fixture.
	seed := bytes.Repeat([]byte{94}, ed25519.SeedSize)
	private := ed25519.NewKeyFromSeed(seed)
	const keyID = "pi-upstream-positive-test-only-key"
	trust := directoryv1.TrustStore{Keys: []directoryv1.TrustedKey{{
		ID: keyID, PublicKey: private.Public().(ed25519.PublicKey), State: directoryv1.KeyCurrent,
	}}}
	digest := sha256.Sum256(body)
	envelope, err := json.Marshal(directoryv1.Envelope{
		EnvelopeSchemaVersion: 1, SnapshotSchemaVersion: 1, Sequence: snapshot.Sequence,
		KeyID: keyID, Algorithm: "Ed25519", SignatureDomain: directoryv1.SignatureDomain,
		SnapshotDigest: "sha256:" + hex.EncodeToString(digest[:]),
		Signature: base64.StdEncoding.EncodeToString(
			ed25519.Sign(private, directoryv1.SnapshotSignatureMessage(body))),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := directoryv1.VerifyBundle(body, envelope, trust); err != nil {
		t.Fatalf("synthetic test-only upstream bundle rejected: %v", err)
	}
	negative, _ := piFullSignedTestDirectory(t, t.TempDir())
	if _, err := directoryv1.VerifyBundle(body, envelope, negative.Trust); err == nil {
		t.Fatal("synthetic upstream signed bundle was accepted by another feed key")
	}
	client := &directoryv1.Client{
		Origin: "https://fixture.invalid/pi-upstream-positive-test-only/schemas/1/",
		FeedID: piUpstreamTestFeedID,
		HTTPClient: &http.Client{Transport: conformanceFixtureTransport{
			sequence: snapshot.Sequence, snapshot: body, envelope: envelope,
		}},
		Trust: trust, Embedded: directoryv1.EmbeddedBundle{Snapshot: body, Envelope: envelope},
		RequireEmbeddedBootstrap: true,
		Cache:                    directoryv1.Cache{Path: filepath.Join(cacheRoot, "pi-upstream-positive-test-only-cache.json")},
		Now:                      func() time.Time { return time.Date(2026, 11, 12, 1, 0, 0, 0, time.UTC) },
	}
	return client, snapshot
}

// Assert the selected Pi adapter really materialized an MCP server, not
// merely a saved dependency/receipt. Preserve foreign MCP ownership on remove.
func assertPiUpstreamTestOnlyNativeMCP(t *testing.T, j projectJourney, root, product string, installed bool) {
	t.Helper()
	native := filepath.Join(piSignedProjectNativeRoot(j, root), "mcp.json")
	var config struct {
		Keep       bool                       `json:"keep"`
		MCPServers map[string]json.RawMessage `json:"mcpServers"`
	}
	if err := json.Unmarshal([]byte(projectJourneyRead(t, native)), &config); err != nil {
		t.Fatalf("Pi MCP native config is unreadable: %v", err)
	}
	var foreign struct {
		Type string `json:"type"`
		URL  string `json:"url"`
	}
	if err := json.Unmarshal(config.MCPServers["foreign"], &foreign); err != nil ||
		foreign.Type != "streamable-http" ||
		foreign.URL != "https://foreign.example.test/mcp" ||
		!config.Keep {
		t.Fatalf("foreign Pi MCP config not preserved: %v / %+v", err, foreign)
	}
	if installed {
		if len(config.MCPServers) != 2 {
			t.Fatalf("expected exactly one owned Pi server and foreign server, got %d", len(config.MCPServers))
		}
		raw, ok := config.MCPServers[product]
		if !ok {
			t.Fatalf("expected owned Pi MCP server %q missing", product)
		}
		var owned struct {
			Type    string   `json:"type"`
			URL     string   `json:"url"`
			Command string   `json:"command"`
			Args    []string `json:"args"`
		}
		if err := json.Unmarshal(raw, &owned); err != nil {
			t.Fatalf("owned Pi MCP definition %q is invalid: %v", product, err)
		}
		switch product {
		case "github":
			// Pi's native remote codec omits the portable streamable-http type.
			if owned.URL != "https://api.githubcopilot.com/mcp/" || owned.Command != "" {
				t.Fatalf("GitHub native remote MCP definition is incorrect: %+v", owned)
			}
		case "context7":
			// The upstream source is HTTP, unlike the separate Registry overlay package.
			if owned.URL != "https://mcp.context7.com/mcp/oauth" || owned.Command != "" {
				t.Fatalf("Context7 native remote MCP definition is incorrect: %+v", owned)
			}
		default:
			t.Fatalf("unreviewed owned Pi MCP server %q", product)
		}
	}
	if !installed && len(config.MCPServers) != 1 {
		t.Fatalf("upstream removal left owned Pi MCP servers: %d", len(config.MCPServers)-1)
	}
}

// The real public Git sources are installed with the production saved
// resolver/composer. No Pi process, MCP tool, OAuth, production signing,
// publisher approval, or released modern-routing behavior is exercised.
func TestPiUpstreamSignedTestOnlyPositiveSaved(t *testing.T) {
	if os.Getenv("UAP_PI_UPSTREAM_TEST_ONLY_SAVED_E2E") != "1" {
		t.Skip("networked test-only signed upstream saved journeys run in dedicated 3-OS CI")
	}
	cases := []struct {
		distribution string
		product      string
	}{
		{"github/github", "github"},
		{"upstash/context7", "context7"},
	}
	completed := make([]string, 0, 4)
	for _, subject := range cases {
		for _, scope := range []domain.InstallScope{domain.ScopeUser, domain.ScopeProject} {
			name := fmt.Sprintf("%s/%s", subject.distribution, scope)
			executed := false
			passed := t.Run(name, func(t *testing.T) {
				j, snapshot := piFullRealSavedJourneyWithClient(t, piUpstreamTestOnlyClient)
				seen := false
				var pinned domain.DirectoryRelease
				for _, distribution := range snapshot.Distributions {
					if distribution.ID == subject.distribution && distribution.ProductID == subject.product && len(distribution.Releases) == 1 {
						seen = true
						pinned = distribution.Releases[0]
					}
				}
				if !seen {
					t.Fatal("subject not pinned in original immutable source snapshot")
				}
				root := ""
				if scope == domain.ScopeProject {
					root = j.a
				}
				originalB := securityFixtureTree(t, piSignedProjectNativeRoot(j, j.b))
				originalOther := securityFixtureTree(t, piSignedProjectNativeRoot(j, func() string {
					if root == "" {
						return j.a
					}
					return ""
				}()))
				j.run(t, root, "install", subject.distribution, "--target", "pi", "--yes")
				state := j.state(t, root)
				if len(state.Installations) != 1 || len(state.EnvironmentReceipts) != 1 {
					t.Fatalf("positive saved lifecycle has wrong receipts: installations=%d, receipts=%d",
						len(state.Installations), len(state.EnvironmentReceipts))
				}
				installed := state.Installations[0]
				if installed.Directory == nil || installed.Directory.FeedID != piUpstreamTestFeedID ||
					installed.Directory.DistributionID != subject.distribution ||
					installed.Source.Repository != pinned.PackageSource.Repository ||
					installed.Source.PackageSubpath != pinned.PackageSource.Path ||
					installed.Source.ResolvedRevision != pinned.PackageSource.Revision ||
					installed.Source.TreeDigest != pinned.TreeDigest ||
					installed.Package.ManifestDigest != pinned.ManifestDigest {
					t.Fatal("upstream saved installation was not bound to the exact immutable signed TEST-only source and feed")
				}
				assertPiUpstreamTestOnlyNativeMCP(t, j, root, subject.product, true)
				if !reflect.DeepEqual(originalB, securityFixtureTree(t, piSignedProjectNativeRoot(j, j.b))) {
					t.Fatal("upstream install modified independent project B")
				}
				j.run(t, root, "deps", "sync", "--locked", "--yes")
				if len(j.state(t, root).EnvironmentReceipts) != 1 {
					t.Fatal("locked replay lost saved upstream receipt")
				}
				j.run(t, root, "remove", subject.product, "--target", "pi", "--yes")
				if len(j.state(t, root).EnvironmentReceipts) != 0 {
					t.Fatal("upstream removal left installed environment receipts")
				}
				assertPiUpstreamTestOnlyNativeMCP(t, j, root, subject.product, false)
				if !reflect.DeepEqual(originalB, securityFixtureTree(t, piSignedProjectNativeRoot(j, j.b))) {
					t.Fatal("upstream removal changed independent project B")
				}
				other := j.a
				if root != "" {
					other = ""
				}
				if !reflect.DeepEqual(originalOther, securityFixtureTree(t, piSignedProjectNativeRoot(j, other))) {
					t.Fatal("upstream saved lifecycle modified the unselected user/project scope")
				}
				executed = true
			})
			if !passed || !executed {
				t.Fatalf("test-only positive upstream case %s did not complete", name)
			}
			completed = append(completed, name)
		}
	}
	if len(completed) != 4 {
		t.Fatal("all four test-only upstream saved scopes are required")
	}
	if report := os.Getenv("UAP_PI_UPSTREAM_TEST_ONLY_REPORT"); report != "" {
		head := os.Getenv("EXPECTED_HEAD")
		if len(head) != 40 {
			t.Fatal("exact test source HEAD required for report")
		}
		for _, c := range head {
			if !strings.ContainsRune("0123456789abcdef", c) {
				t.Fatal("invalid immutable workflow head")
			}
		}
		payload, err := json.MarshalIndent(map[string]any{
			"kind":      "pi-upstream-real-source-saved-test-only",
			"test_head": head, "runner_os": os.Getenv("RUNNER_OS"),
			"saved_cases": completed, "production_published": false,
			"production_eligibility": false, "synthetic_evidence": true,
			"native_pi_runtime_or_oauth": false,
		}, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(report, append(payload, '\n'), 0600); err != nil {
			t.Fatal(err)
		}
	}
}

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"runtime"
	"strconv"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// This is one disposable, independently keyed TEST-only signed v1 snapshot.
// Its two upstream grants use deliberately SYNTHETIC materialization evidence.
// Positive saved installs here can never authorize real publisher signatures.
func TestPiAllSignedTestOnlySavedMarketplace(t *testing.T) {
	if os.Getenv("UAP_PI_ALL_SAVED_TEST_ONLY_E2E") != "1" {
		t.Skip("separate nine-shard, 3-OS real-source synthetic signed fixture")
	}
	shard, err := strconv.Atoi(os.Getenv("UAP_PI_ALL_SAVED_SHARD"))
	if err != nil || shard < 0 || shard >= piFullShardCount {
		t.Fatal("exact test-only shard 0..2 is required")
	}
	_, snapshot := piFullRealSavedJourneyWithClient(t, piUpstreamTestOnlyClient)
	if len(snapshot.Distributions) != 36 || len(snapshot.Evidence) != 2 {
		t.Fatal("full immutable 36-source snapshot with exactly two synthetic upstream grants required")
	}
	completed := make([]string, 0, 24)
	selected := 0
	for index, distribution := range snapshot.Distributions {
		if index%piFullShardCount != shard {
			continue
		}
		selected++
		if len(distribution.Releases) != 1 || len(distribution.ReleasePolicies) != 1 {
			t.Fatalf("unexpected source release cardinality: %s", distribution.ID)
		}
		for _, scope := range []domain.InstallScope{domain.ScopeUser, domain.ScopeProject} {
			name := fmt.Sprintf("%s@%d/%s", distribution.ID, distribution.Releases[0].Sequence, scope)
			executed := false
			passed := t.Run(name, func(t *testing.T) {
				j, _ := piFullRealSavedJourneyWithClient(t, piUpstreamTestOnlyClient)
				root := ""
				other := j.a
				if scope == domain.ScopeProject {
					root, other = j.a, ""
				}
				nativeB := piSignedProjectNativeRoot(j, j.b)
				nativeOther := piSignedProjectNativeRoot(j, other)
				beforeB := securityFixtureTree(t, nativeB)
				beforeOther := securityFixtureTree(t, nativeOther)
				j.run(t, root, "install", distribution.ID, "--target", "pi", "--yes")
				state := j.state(t, root)
				expected := 1
				if distribution.ProductID == "marketing-kit" {
					expected = 4
				}
				if len(state.Installations) != expected || len(state.EnvironmentReceipts) != expected {
					t.Fatalf("%s: expected %d persisted signed saved packages, got %d installations / %d receipts", name, expected, len(state.Installations), len(state.EnvironmentReceipts))
				}
				// Verify *every* saved node against the frozen signed source,
				// not merely the root and the number of Marketing Kit children.
				pinnedByID := make(map[string]domain.DirectoryRelease, len(snapshot.Distributions))
				for _, pinnedDistribution := range snapshot.Distributions {
					if len(pinnedDistribution.Releases) != 1 {
						t.Fatal("frozen source has ambiguous release cardinality")
					}
					pinnedByID[pinnedDistribution.ID] = pinnedDistribution.Releases[0]
				}
				installedIDs := make(map[string]bool, expected)
				for _, installed := range state.Installations {
					if installed.Directory == nil || installed.Directory.FeedID != piUpstreamTestFeedID {
						t.Fatalf("%s: saved child lost independent TEST-only feed identity", name)
					}
					id := installed.Directory.DistributionID
					pinned, exists := pinnedByID[id]
					if !exists || installedIDs[id] ||
						installed.Source.Repository != pinned.PackageSource.Repository ||
						installed.Source.PackageSubpath != pinned.PackageSource.Path ||
						installed.Source.ResolvedRevision != pinned.PackageSource.Revision ||
						installed.Source.TreeDigest != pinned.TreeDigest ||
						installed.Package.ManifestDigest != pinned.ManifestDigest {
						t.Fatalf("%s: unexpected, duplicate or unpinned saved child %q", name, id)
					}
					installedIDs[id] = true
				}
				if len(installedIDs) != expected || !installedIDs[distribution.ID] {
					t.Fatalf("%s: exact saved source closure is incomplete", name)
				}
				if distribution.ProductID == "marketing-kit" {
					for _, id := range []string{"777genius/marketing-kit", "777genius/hubspot-crm", "777genius/notion", "777genius/playwright-bridge"} {
						if !installedIDs[id] {
							t.Fatalf("%s: mandatory pinned Marketing Kit dependency %s missing", name, id)
						}
					}
				}
				if distribution.Kind == domain.DistributionUpstream {
					if distribution.ID != "github/github" && distribution.ID != "upstash/context7" {
						t.Fatalf("%s: synthetic evidence was unexpectedly granted to another upstream", name)
					}
					assertPiUpstreamTestOnlyNativeMCP(t, j, root, distribution.ProductID, true)
				}
				if !reflect.DeepEqual(beforeB, securityFixtureTree(t, nativeB)) ||
					!reflect.DeepEqual(beforeOther, securityFixtureTree(t, nativeOther)) {
					t.Fatalf("%s: install changed another Pi user/project scope", name)
				}
				j.run(t, root, "deps", "sync", "--locked", "--yes")
				replayed := j.state(t, root)
				if len(replayed.Installations) != expected || len(replayed.EnvironmentReceipts) != expected {
					t.Fatalf("%s: locked replay lost immutable saved dependencies", name)
				}
				// A receipt count alone cannot detect a changed source, release, or feed.
				// Compare immutable installed identities, not mutable timestamps.
				for _, previous := range state.Installations {
					found := false
					for _, current := range replayed.Installations {
						if reflect.DeepEqual(previous.Directory, current.Directory) &&
							reflect.DeepEqual(previous.Source, current.Source) &&
							previous.Package.ManifestDigest == current.Package.ManifestDigest {
							found = true
							break
						}
					}
					if !found {
						t.Fatalf("%s: locked replay changed a pinned saved dependency", name)
					}
				}
				j.run(t, root, "remove", distribution.ProductID, "--target", "pi", "--yes")
				removed := j.state(t, root)
				if len(removed.EnvironmentReceipts) != 0 {
					t.Fatalf("%s: saved removal left active environment receipts", name)
				}
				// Uninstall intentionally keeps historical records and retained data.
				// It must release every active native client binding.
				for _, installed := range removed.Installations {
					if len(installed.Clients) != 0 {
						t.Fatalf("%s: saved removal retained an active client binding", name)
					}
				}
				if distribution.Kind == domain.DistributionUpstream {
					assertPiUpstreamTestOnlyNativeMCP(t, j, root, distribution.ProductID, false)
				}
				if !reflect.DeepEqual(beforeB, securityFixtureTree(t, nativeB)) ||
					!reflect.DeepEqual(beforeOther, securityFixtureTree(t, nativeOther)) {
					t.Fatalf("%s: removal changed another Pi user/project scope", name)
				}
				executed = true
			})
			if !passed || !executed {
				t.Fatalf("%s: skipped or incomplete signed TEST-only lifecycle", name)
			}
			completed = append(completed, name)
		}
	}
	if selected != 12 || len(completed) != 24 {
		t.Fatalf("shard %d did not execute 12 immutable sources x 2 saved scopes", shard)
	}
	if report := os.Getenv("UAP_PI_ALL_SAVED_REPORT"); report != "" {
		head := os.Getenv("EXPECTED_HEAD")
		decoded, err := hex.DecodeString(head)
		if err != nil || len(decoded) != 20 || hex.EncodeToString(decoded) != head {
			t.Fatal("report requires exact test commit SHA")
		}
		body, err := os.ReadFile(piFullSnapshotPath)
		if err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(body)
		if hex.EncodeToString(digest[:]) != piFullExpectedSnapshotSHA256 {
			t.Fatal("immutable original test-only snapshot digest mismatch")
		}
		payload, err := json.MarshalIndent(struct {
			Kind                  string   `json:"kind"`
			TestCommit            string   `json:"test_commit"`
			SourceCommit          string   `json:"source_commit"`
			SnapshotSHA256        string   `json:"snapshot_sha256"`
			RunnerOS              string   `json:"runner_os"`
			Shard                 int      `json:"shard"`
			SavedCases            []string `json:"saved_cases"`
			ProductionPublished   bool     `json:"production_published"`
			ProductionEligibility bool     `json:"production_eligibility"`
			SyntheticEvidence     bool     `json:"synthetic_evidence"`
			NativePiRuntimeOAuth  bool     `json:"native_pi_runtime_or_oauth"`
		}{
			Kind: "pi-all-72-synthetic-test-only-saved", TestCommit: head,
			SourceCommit:   piFullRegistrySourceCommit,
			SnapshotSHA256: "sha256:" + hex.EncodeToString(digest[:]),
			RunnerOS:       runtime.GOOS, Shard: shard, SavedCases: completed,
			ProductionPublished: false, ProductionEligibility: false,
			SyntheticEvidence: true, NativePiRuntimeOAuth: false,
		}, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(report, append(payload, '\n'), 0600); err != nil {
			t.Fatal(err)
		}
	}
}

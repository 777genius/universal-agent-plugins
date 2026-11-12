package agentpluginscli

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/sourceacquisition"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

// This test requires explicit, disposable hosted CI with GitHub network access.
// The four source tuples below are exact, already-published Registry #451
// inputs. Directory authority/evidence is SYNTHETIC and signed with the
// dedicated test key from pi_modern_saved_closure_test.go, never production.
// No model, account credentials, OAuth, MCP process or native Pi is launched.
const piRealRegistryCommit = "32b6c48d071aceadf90a030c9d69477fe1a6a65a"
const piRealUAPCommit = "2ddbb99dd190c1792b79904f9875e6322bccd243"

type piRealRelease struct {
	distribution string
	product      string
	manifest     string
	version      string
	repository   string
	revision     string
	path         string
	tree         string
	manifestHash string
	components   []string
	sequence     uint64
	kind         domain.DistributionKind
	auth         domain.AuthenticationRequirement
}

var piRealMarketplaceClosure = []piRealRelease{
	{
		distribution: "777genius/marketing-kit", product: "marketing-kit", manifest: "marketing-kit",
		version: "0.1.0", repository: "777genius/universal-agent-plugins-registry",
		revision: piRealRegistryCommit, path: "plugins/marketing-kit",
		tree:         "sha256:8918c0f0222bd735fd0eef3cacf90b5a9351fb213483d1558a65b9911624e68a",
		manifestHash: "sha256:40b18a6b33c6392134aff7beafb9a91eaeecf46ce765e0e888cdc743dd284ea2",
		components: []string{"skills"}, sequence: 1, kind: domain.DistributionCommunity,
		auth: domain.AuthenticationRequirementRequired,
	},
	{
		distribution: "777genius/notion", product: "notion", manifest: "notion",
		version: "0.1.0", repository: "777genius/universal-agent-plugins",
		revision: piRealUAPCommit, path: "plugins/notion",
		tree:         "sha256:860fa6dd1830d0da2b8fed38195fa6ad6307082ac12d708118e5eb1c5d10f9b7",
		manifestHash: "sha256:bbab30a4f062218d7a5fce2f6c818680d3c7adfaebdbaa3e523e15942a2f57e5",
		components: []string{"mcp"}, sequence: 1, kind: domain.DistributionCommunity,
		auth: domain.AuthenticationRequirementRequired,
	},
	{
		distribution: "777genius/hubspot-crm", product: "hubspot-crm", manifest: "hubspot-crm",
		version: "0.1.0", repository: "777genius/universal-agent-plugins",
		revision: piRealUAPCommit, path: "plugins/hubspot-crm",
		tree:         "sha256:09a0b151c384235c3d7b3929398695d24cd7b07c75993a0a73f1a218198ededd",
		manifestHash: "sha256:20334e86a1b599d2759e04bfaef37dc2ddb7f6b0f3994b5d66d4eca12fbec941",
		components: []string{"mcp"}, sequence: 1, kind: domain.DistributionCommunity,
		auth: domain.AuthenticationRequirementRequired,
	},
	{
		distribution: "777genius/playwright-bridge", product: "playwright", manifest: "playwright",
		version: "0.0.80", repository: "777genius/universal-agent-plugins-registry",
		revision: piRealRegistryCommit, path: "plugins/playwright",
		tree:         "sha256:ff3c74bb5760fea47003130f3984d48234892330325a7cad35a7bc973acc9cf6",
		manifestHash: "sha256:8419b5c9e67894c2312048c4fe30e3ea920d00101d02641230183dd7ab4d3b31",
		components: []string{"mcp"}, sequence: 2, kind: domain.DistributionCommunityBridge,
		auth: domain.AuthenticationRequirementNotRequired,
	},
}

// Registry-derived immutable package identities are test data, not a claim of
// independent signed Pi authority. The synthetic policy is deliberately never
// written to the Registry repo or any production publication tree.
func piRealMarketplaceFixture(t *testing.T, scope domain.InstallScope) freshRestoreEnvironment {
	t.Helper()
	fixture, _, directory := dependencyReviewFixture(t)
	pi := fixtureClient(t, domain.ClientPi)
	pi.Version = "1.5.0"
	fixture.client = pi
	fixture.app.Version = "0.1.104"
	fixture.app.Detector = staticDetector{clients: []domain.DetectedClient{pi}}
	fixture.app.Scope = scope
	if scope == domain.ScopeProject {
		fixture.app.ProjectRoot = t.TempDir()
	}
	// Real immutable GitHub acquisition, not a mapped-local mock or moving
	// branch checkout. UAP's isolated Git environment must remain active.
	fixture.app.SourceAcquirer = sourceacquisition.Acquirer{TempRoot: t.TempDir()}
	snapshot := domain.DirectorySnapshot{
		SnapshotSchemaVersion: 1, SourceCommit: piRealRegistryCommit,
		Products: []domain.DirectoryProduct{}, Distributions: []domain.DirectoryDistribution{},
		Evidence: []domain.DirectoryEvidence{}, Revocations: []domain.DirectoryRevocation{},
	}
	for _, item := range piRealMarketplaceClosure {
		release := domain.DirectoryRelease{
			Sequence: item.sequence, PackageVersion: item.version, ManifestName: item.manifest,
			AgentPluginsSchema: domain.PluginSchemaV1,
			PackageSource: domain.DirectorySource{Repository: item.repository, Revision: item.revision, Path: item.path},
			TreeDigestAlgorithm: domain.TreeDigestAlgorithm, TreeDigest: item.tree,
			ManifestDigest: item.manifestHash, Components: item.components,
			PublishedAt: "2026-11-11T00:00:00Z",
		}
		evidenceID := fmt.Sprintf("test/real-source/%s/%d", item.product, item.sequence)
		evidence := intendedTrustedDirectoryEvidence(domain.DirectoryEvidence{
			ID: evidenceID, DistributionID: item.distribution, ReleaseSequence: item.sequence,
			PackageTreeDigest: item.tree, Level: "materialization", Outcome: "passed",
			Client: domain.ClientPi, ClientVersion: pi.Version,
			InstallerVersion: fixture.app.Version, OS: runtime.GOOS, Architecture: runtime.GOARCH,
		})
		snapshot.Evidence = append(snapshot.Evidence, evidence)
		snapshot.Products = append(snapshot.Products, domain.DirectoryProduct{
			SchemaVersion: 1, ID: item.product, DisplayName: item.product,
			Description: "Actual pinned public source in a disposable signed-v1 test",
			ManifestName: item.manifest, Aliases: []string{item.product},
			ReservedAliases: []string{item.product}, Categories: []string{"tools"},
			MinimumCapabilities: domain.DirectoryMinimumCapabilities{Skills: "optional", MCP: "required"},
			DefaultDistribution: item.distribution, Distributions: []string{item.distribution},
		})
		snapshot.Distributions = append(snapshot.Distributions, domain.DirectoryDistribution{
			SchemaVersion: 1, ID: item.distribution, ProductID: item.product,
			Kind: item.kind, Status: domain.DistributionActive, Packager: "777genius",
			Releases: []domain.DirectoryRelease{release},
			ReleasePolicies: []domain.DirectoryReleasePolicy{{
				ReleaseSequence: item.sequence, Status: domain.ReleaseActive,
				MinimumInstallerVersion: "0.1.104",
				Targets: []domain.DirectoryTarget{{
					Client: domain.ClientPi, Scopes: []domain.InstallScope{domain.ScopeUser},
					Delivery: "managed", Authentication: item.auth,
				}},
				CurrentEvidence: []string{evidenceID},
			}},
		})
	}
	directory.bundle = signPiModernTestOnlyCohort(t, snapshot)
	fixture.app.DirectoryClient = lockedFeedTestClient{
		fixedDirectoryClient: directory, feedID: piModernFixtureFeed,
	}
	return fixture
}

// This proves pinned public source bytes and saved dependency behavior, but
// intentionally cannot qualify a production signer or authenticated Pi runtime.
func TestPiRealMarketplaceSignedTestCohortSavedClosure(t *testing.T) {
	if !strings.EqualFold("1", os.Getenv("UAP_PI_REAL_SAVED_E2E")) {
		t.Skip("real pinned public-source E2E is only enabled in isolated three-OS workflow")
	}
	for _, scope := range []domain.InstallScope{domain.ScopeUser, domain.ScopeProject} {
		t.Run(string(scope), func(t *testing.T) {
			fixture := piRealMarketplaceFixture(t, scope)
			requirements := environment.Requirements{
				SchemaVersion: 1, Dependencies: map[string]environment.Requirement{
					"marketing-kit": {
						Alias: "marketing-kit",
						Registry: &environment.RegistrySource{
							Distribution: "777genius/marketing-kit",
							Selector: environment.Selector{Kind: environment.SelectorVersion, Value: "0.1.0"},
						},
					},
				},
				Targets: map[string][]string{"pi": {"marketing-kit"}},
			}
			lock, err := resolveProjectSourceGraph(context.Background(), fixture.app, requirements)
			if err != nil || len(lock.Packages) != len(piRealMarketplaceClosure) {
				t.Fatalf("real pinned Marketing Kit Pi saved closure (%s) refused or omitted dependency: %+v %v", scope, lock, err)
			}
			wanted := make(map[string]piRealRelease, len(piRealMarketplaceClosure))
			for _, source := range piRealMarketplaceClosure {
				wanted[source.distribution] = source
			}
			for _, pin := range lock.Packages {
				if pin.Source.Registry == nil {
					t.Fatalf("real pinned closure used an unsigned dependency: %+v", pin)
				}
				source, known := wanted[pin.Source.Registry.DistributionID]
				if !known || pin.Source.Registry.FeedID != piModernFixtureFeed ||
					pin.Source.Registry.ReleaseSequence != source.sequence ||
					pin.Source.Registry.Commit != source.revision || pin.TreeDigest != source.tree ||
					pin.ManifestDigest != source.manifestHash || len(pin.Targets) != 1 || pin.Targets[0] != "pi" {
					t.Fatalf("real pinned package mismatched immutable Registry tuple or signed test feed: %+v", pin)
				}
				loaded, err := materializeLockedPackage(context.Background(), fixture.app, pin, false, nil)
				if err != nil {
					t.Fatalf("real pinned public-source Git acquisition for %s: %v", source.distribution, err)
				}
				if loaded.envelope.TreeDigest != source.tree || loaded.envelope.ManifestDigest != source.manifestHash {
					t.Fatalf("real pinned source bytes changed: %s", source.distribution)
				}
				if err := loaded.cleanup(); err != nil {
					t.Fatal(err)
				}
				delete(wanted, source.distribution)
			}
			if len(wanted) != 0 {
				t.Fatalf("required real public package sources omitted: %+v", wanted)
			}
			if scope == domain.ScopeUser {
				dependencyRun(t, fixture.cliFixture, "install", "marketing-kit", "--target", "pi", "--yes", "--format", "json")
				view, err := fixture.app.Installer.Inspect(context.Background())
				if err != nil || view.Recovery.Required || len(view.EnvironmentReceipts) != 4 || len(view.Installations) != 4 {
					t.Fatalf("real pinned Marketing Kit signed-test saved install lacks four receipts: %+v %v", view, err)
				}
				dependencyRun(t, fixture.cliFixture, "remove", "marketing-kit", "--target", "pi", "--yes", "--format", "json")
				removed, err := fixture.app.Installer.Inspect(context.Background())
				if err != nil || len(removed.EnvironmentReceipts) != 0 {
					t.Fatalf("real pinned saved root removal left owned graph: %+v %v", removed, err)
				}
			}
		})
	}
}

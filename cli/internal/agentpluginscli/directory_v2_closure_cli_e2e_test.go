package agentpluginscli

import (
	"errors"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/directoryv1"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// The CLI must refuse a signed ineligible project dependency closure before
// source acquisition or any installer state change, even when user is allowed.
func TestDirectoryV2ProjectClosureRefusesBeforeAcquisition(t *testing.T) {
	for _, tc := range []struct {
		name    string
		closure domain.DirectoryCanonicalClosure
		want    string
	}{
		{name: "ineligible required child", closure: domain.DirectoryCanonicalClosure{Eligible: false},
			want: "signed required dependency closure is ineligible"},
		{name: "required child installer floor", closure: domain.DirectoryCanonicalClosure{
			Eligible: true, ClosureDigest: "sha256:" + strings.Repeat("c", 64), MinimumInstallerVersion: "1.0.0",
		}, want: "minimum installer version for signed required closure"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := fixtureClient(t, domain.ClientCodex)
			fixture := newCLIFixture(t, []domain.DetectedClient{client})
			fixture.app.Scope = domain.ScopeProject
			fixture.app.ProjectRoot = t.TempDir()
			acquirer := &localBackedSourceAcquirer{delegate: fixture.app.SourceAcquirer, root: writeCLIPlugin(t)}
			fixture.app.SourceAcquirer = acquirer

			release := domain.DirectoryRelease{
				Sequence: 1, PackageVersion: "1.0.0", ManifestName: "demo",
				AgentPluginsSchema:  domain.PluginSchemaV1,
				PackageSource:       domain.DirectorySource{Repository: "owner/demo", Revision: strings.Repeat("a", 40), Path: "plugin"},
				TreeDigestAlgorithm: domain.TreeDigestAlgorithm,
				TreeDigest:          "sha256:" + strings.Repeat("b", 64),
				ManifestDigest:      "sha256:" + strings.Repeat("d", 64),
				Components:          []string{"skills"}, PublishedAt: "2026-10-01T00:00:00Z",
			}
			policy := domain.DirectoryReleasePolicy{
				ReleaseSequence: 1, Status: domain.ReleaseActive, MinimumInstallerVersion: "0.1.0",
				Targets: []domain.DirectoryTarget{
					{Client: domain.ClientCodex, Scope: domain.ScopeUser, Delivery: "managed",
						CanonicalClosure: &domain.DirectoryCanonicalClosure{Eligible: true, MinimumInstallerVersion: "0.1.0"}},
					{Client: domain.ClientCodex, Scope: domain.ScopeProject, Delivery: "managed",
						CanonicalClosure: &tc.closure},
				},
				CurrentEvidence: []string{},
			}
			snapshot := domain.DirectorySnapshot{
				SnapshotSchemaVersion: 2, Sequence: 1,
				Products: []domain.DirectoryProduct{{
					SchemaVersion: 1, ID: "demo", ManifestName: "demo", DefaultDistribution: "owner/demo",
					Distributions:       []string{"owner/demo"},
					MinimumCapabilities: domain.DirectoryMinimumCapabilities{Skills: "optional", MCP: "optional"},
				}},
				Distributions: []domain.DirectoryDistribution{{
					SchemaVersion: 2, ID: "owner/demo", ProductID: "demo", Kind: domain.DistributionCommunity,
					Status: domain.DistributionActive, Releases: []domain.DirectoryRelease{release},
					ReleasePolicies: []domain.DirectoryReleasePolicy{policy},
				}},
				Evidence: []domain.DirectoryEvidence{}, Revocations: []domain.DirectoryRevocation{},
			}
			reader := &fixedDirectoryClient{bundle: directoryv1.VerifiedBundle{
				Snapshot: snapshot, Digest: "sha256:" + strings.Repeat("e", 64),
			}}
			fixture.app.DirectoryClient = reader
			before, err := fixture.store.Load()
			if err != nil {
				t.Fatal(err)
			}
			_, err = fixture.app.acquireDirectory(t.Context(), "demo", packageResolutionRequest{
				Targets: []domain.ClientID{domain.ClientCodex},
				Clients: map[domain.ClientID]domain.DetectedClient{domain.ClientCodex: client},
			})
			if !errors.Is(err, domain.ErrDirectoryIneligible) || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("CLI project accepted signed ineligible required closure: %v", err)
			}
			if acquirer.verifiedCalls != 0 || acquirer.directGitCalls != 0 || acquirer.localCalls != 0 || reader.calls != 1 {
				t.Fatalf("refusal reached source acquisition or skipped signed policy: %+v calls=%d", acquirer, reader.calls)
			}
			after, err := fixture.store.Load()
			if err != nil || len(after.Installations) != len(before.Installations) {
				t.Fatalf("refused project closure mutated installation state: %+v %v", after, err)
			}
			// A different signed grant for the same client must stay independent.
			if _, err := domain.ResolveDirectory(snapshot, domain.DirectoryResolveRequest{
				Selector: "demo", Targets: []domain.ClientID{domain.ClientCodex}, Scope: domain.ScopeUser,
				InstallerVersion: fixture.app.Version, SchemaVersion: "1.0.0", Operation: domain.DirectoryInstall,
			}); err != nil {
				t.Fatalf("project refusal leaked into independent signed user grant: %v", err)
			}
		})
	}
}

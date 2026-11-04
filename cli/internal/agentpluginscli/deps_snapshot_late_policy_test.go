package agentpluginscli

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

// This policy matches only after the acquired manifest establishes its npx
// runtime identity. Candidate enumeration must still admit the publication.
func lateDependencyPolicyFixture(t *testing.T) rolloutDirectoryFixture {
	t.Helper()
	fixture := newRolloutDirectoryFixture(t, []domain.ClientID{domain.ClientCodex}, nil)
	dependencyWrite(t, filepath.Join(fixture.v1.root, "mcp.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"demo":{"type":"stdio","command":"npx"}}}`)
	local, err := fixture.cli.app.acquireLocal(t.Context(), fixture.v1.root)
	if err != nil {
		t.Fatal(err)
	}
	distribution := &fixture.directory.bundle.Snapshot.Distributions[0]
	release := &distribution.Releases[0]
	release.TreeDigest, release.ManifestDigest = local.envelope.TreeDigest, local.envelope.ManifestDigest
	if err := local.cleanup(); err != nil {
		t.Fatal(err)
	}
	for i := range fixture.directory.bundle.Snapshot.Evidence {
		evidence := &fixture.directory.bundle.Snapshot.Evidence[i]
		if evidence.ReleaseSequence == release.Sequence {
			evidence.PackageTreeDigest = release.TreeDigest
		}
	}
	failure := intendedTrustedDirectoryEvidence(domain.DirectoryEvidence{
		ID: "failed/late-runtime/npx", DistributionID: distribution.ID, ReleaseSequence: release.Sequence,
		PackageTreeDigest: release.TreeDigest, Level: "runtime", Outcome: "failed", Client: domain.ClientCodex,
		InstallerVersion: fixture.cli.app.Version, OS: runtime.GOOS, Architecture: runtime.GOARCH, DependencyIdentity: "npx",
	})
	failure.ClientVersion = ""
	fixture.directory.bundle.Snapshot.Evidence = append(fixture.directory.bundle.Snapshot.Evidence, failure)
	distribution.ReleasePolicies[0].CurrentEvidence = append(distribution.ReleasePolicies[0].CurrentEvidence, failure.ID)
	return fixture
}

func TestDependencyCandidateLatePolicyCleanup(t *testing.T) {
	for _, mode := range []string{"pure_policy", "cleanup_refusal"} {
		t.Run(mode, func(t *testing.T) {
			fixture := lateDependencyPolicyFixture(t)
			snapshot, err := (packagedigest.Builder{}).Snapshot(t.Context(), fixture.v1.root, domain.SourceIdentity{})
			if err != nil {
				t.Fatal(err)
			}
			if mode == "cleanup_refusal" {
				snapshot = refuseDependencySnapshotCleanup(t, snapshot)
			} else {
				t.Cleanup(func() { _ = packagedigest.Remove(snapshot) })
			}
			acquirer := &dependencyCleanupAcquirer{snapshot: snapshot}
			fixture.cli.app.SourceAcquirer = acquirer
			requirement := environment.Requirement{Alias: "demo", Registry: &environment.RegistrySource{Distribution: "owner/rollout", Selector: environment.Selector{Kind: environment.SelectorVersion, Value: "1.0.0"}}}
			resolver := dependencyResolver{app: fixture.cli.app}
			before := observeDependencyCheckFiles(t, fixture.cli.root)
			candidates, err := resolver.Candidates(t.Context(), requirement, []string{"codex"})
			if err != nil || len(candidates) != 1 || acquirer.calls != 0 {
				t.Fatalf("publication was not admitted before acquisition: candidates=%+v err=%v source=%d", candidates, err, acquirer.calls)
			}
			resolved, err := resolver.AcquireCandidate(t.Context(), requirement, candidates[0], []string{"codex"})
			if acquirer.calls != 1 || !errors.Is(err, domain.ErrDirectoryIneligible) || !strings.Contains(err.Error(), "failed compatibility recheck") {
				t.Fatalf("did not reach acquired publication's late policy refusal: source=%d err=%v", acquirer.calls, err)
			}
			if !reflect.DeepEqual(resolved, environment.ResolvedPackage{}) {
				t.Fatalf("late policy refusal returned a usable package: %+v", resolved)
			}
			if mode == "cleanup_refusal" {
				if !errors.Is(err, errDependencySnapshotCleanup) || errors.Is(err, environment.ErrCandidateIneligible) {
					t.Fatalf("late policy refusal hid terminal cleanup failure: %v", err)
				}
			} else {
				if !errors.Is(err, environment.ErrCandidateIneligible) || errors.Is(err, errDependencySnapshotCleanup) {
					t.Fatalf("pure policy refusal lost candidate fallback: %v", err)
				}
				if _, err := os.Stat(snapshot.Root); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("pure policy refusal retained its snapshot: %v", err)
				}
			}
			assertDependencyCheckFilesUnchanged(t, fixture.cli.root, before)
		})
	}
}

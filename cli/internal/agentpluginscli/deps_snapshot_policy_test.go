package agentpluginscli

import (
	"context"
	"errors"
	"os"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

type dependencyCleanupSecurityEvaluator struct{ err error }

func (evaluator dependencyCleanupSecurityEvaluator) Evaluate(context.Context, domain.SecurityEvaluationInput) (domain.SecurityAssessment, error) {
	return domain.SecurityAssessment{}, evaluator.err
}

func TestDependencyEligibilitySuccessfulMaterializationCleanup(t *testing.T) {
	for _, mode := range []string{"success", "cleanup_refusal"} {
		t.Run(mode, func(t *testing.T) {
			f, pin, acquirer := coldDependencyCleanupFixture(t, mode == "cleanup_refusal")
			directory := newRolloutDirectoryFixture(t, []domain.ClientID{domain.ClientCodex}, nil).directory
			distribution := &directory.bundle.Snapshot.Distributions[0]
			release := &distribution.Releases[0]
			release.TreeDigest, release.ManifestDigest = pin.TreeDigest, pin.ManifestDigest
			release.PackageSource = domain.DirectorySource{Repository: pin.Source.Git.Repository, Revision: pin.Source.Git.Commit, Path: pin.Source.Git.Path}
			for i := range directory.bundle.Snapshot.Evidence {
				if directory.bundle.Snapshot.Evidence[i].ReleaseSequence == release.Sequence {
					directory.bundle.Snapshot.Evidence[i].PackageTreeDigest = pin.TreeDigest
				}
			}
			pin.SourceKey = "directory:" + distribution.ID
			pin.Source = environment.LockedSource{Registry: &environment.LockedRegistrySource{
				Distribution: distribution.ID, DistributionID: distribution.ID, ProductID: distribution.ProductID, ReleaseSequence: release.Sequence,
				Requested: environment.Selector{Kind: environment.SelectorRelease, Value: "1"}, Repository: release.PackageSource.Repository, Path: release.PackageSource.Path, Commit: release.PackageSource.Revision,
			}}
			f.app.DirectoryClient = directory
			before := observeDependencyCheckFiles(t, f.root)
			eligible, err := (dependencyResolver{app: f.app}).Eligible(t.Context(), pin, []string{"codex"})
			if acquirer.calls != 1 || directory.calls != 1 {
				t.Fatalf("cold eligibility did not verify the acquired exact publication: source=%d directory=%d", acquirer.calls, directory.calls)
			}
			if mode == "cleanup_refusal" {
				if eligible || !errors.Is(err, errDependencySnapshotCleanup) {
					t.Fatalf("successful materialization hid cleanup failure: eligible=%t err=%v", eligible, err)
				}
			} else {
				if !eligible || err != nil {
					t.Fatalf("successful eligibility changed: eligible=%t err=%v", eligible, err)
				}
				if _, err := os.Stat(acquirer.snapshot.Root); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("successful eligibility retained its snapshot: %v", err)
				}
			}
			assertDependencyCheckFilesUnchanged(t, f.root, before)
		})
	}
}

func TestDependencyCandidatePreservesSharedLoaderOperationalFailure(t *testing.T) {
	for _, mode := range []string{"pure_policy", "cleanup", "canceled", "deadline"} {
		t.Run(mode, func(t *testing.T) {
			fixture := newRolloutDirectoryFixture(t, []domain.ClientID{domain.ClientCodex}, nil)
			snapshot, err := (packagedigest.Builder{}).Snapshot(t.Context(), fixture.v1.root, domain.SourceIdentity{})
			if err != nil {
				t.Fatal(err)
			}
			if mode == "cleanup" {
				snapshot = refuseDependencySnapshotCleanup(t, snapshot)
			} else {
				t.Cleanup(func() { _ = packagedigest.Remove(snapshot) })
			}
			acquirer := &dependencyCleanupAcquirer{snapshot: snapshot}
			fixture.cli.app.SourceAcquirer = acquirer
			original := domain.ErrDirectoryIneligible
			var terminal error
			switch mode {
			case "cleanup":
				terminal = errDependencySnapshotCleanup
			case "canceled":
				terminal = context.Canceled
			case "deadline":
				terminal = context.DeadlineExceeded
			}
			if terminal != nil && mode != "cleanup" {
				original = errors.Join(original, terminal)
			}
			// Exercise the shared loader's existing failure owner with real bytes.
			fixture.cli.app.SecurityEvaluator = dependencyCleanupSecurityEvaluator{err: original}
			requirement := environment.Requirement{Alias: "demo", Registry: &environment.RegistrySource{Distribution: "owner/rollout", Selector: environment.Selector{Kind: environment.SelectorVersion, Value: "1.0.0"}}}
			before := observeDependencyCheckFiles(t, fixture.cli.root)
			resolved, err := (dependencyResolver{app: fixture.cli.app}).AcquireCandidate(t.Context(), requirement, environment.Candidate{Version: "1.0.0", ReleaseSequence: 1}, []string{"codex"})
			if acquirer.calls != 1 || !reflect.DeepEqual(resolved, environment.ResolvedPackage{}) || !errors.Is(err, domain.ErrDirectoryIneligible) {
				t.Fatalf("shared loader failure: acquired=%d resolved=%+v err=%v", acquirer.calls, resolved, err)
			}
			if terminal == nil {
				if !errors.Is(err, environment.ErrCandidateIneligible) {
					t.Fatalf("pure policy refusal became terminal: %v", err)
				}
			} else if !errors.Is(err, terminal) || errors.Is(err, environment.ErrCandidateIneligible) {
				t.Fatalf("operational failure became candidate ineligibility: %v", err)
			}
			assertDependencyCheckFilesUnchanged(t, fixture.cli.root, before)
		})
	}
}

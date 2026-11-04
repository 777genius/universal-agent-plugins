package domain

import (
	"errors"
	"strings"
	"testing"
)

// Fresh signed v2 selections must honor required-child eligibility and floors;
// restoring a valid historical exact lock uses its own pinned-release authority.
func TestResolveDirectoryV2CanonicalClosureFreshAndLockedRestore(t *testing.T) {
	snapshot := testDirectory()
	snapshot.SnapshotSchemaVersion = 2
	grant := &snapshot.Distributions[0].ReleasePolicies[0].Targets[0]
	grant.Scope, grant.Scopes = ScopeUser, nil
	grant.CanonicalClosure = &DirectoryCanonicalClosure{Eligible: false}
	fresh := request("other/tool", ClientCodex)
	if _, err := ResolveDirectory(snapshot, fresh); !errors.Is(err, ErrDirectoryIneligible) ||
		!strings.Contains(err.Error(), "required dependency closure is ineligible") {
		t.Fatalf("fresh install ignored ineligible closure: %v", err)
	}
	grant.CanonicalClosure = &DirectoryCanonicalClosure{
		Eligible:                true,
		ClosureDigest:           "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		MinimumInstallerVersion: "2.0.0",
	}
	if _, err := ResolveDirectory(snapshot, fresh); !errors.Is(err, ErrDirectoryIneligible) ||
		!strings.Contains(err.Error(), "minimum installer version for signed required closure") {
		t.Fatalf("fresh install ignored transitive floor: %v", err)
	}
	fresh.InstallerVersion = "2.1.0"
	if got, err := ResolveDirectory(snapshot, fresh); err != nil || got.DistributionID != "other/tool" || got.ReleaseSequence != 1 {
		t.Fatalf("qualified fresh closure rejected: %+v %v", got, err)
	}
	// A new target on a recorded immutable release is not an existing-target
	// repair. It must qualify against the current signed dependency closure.
	newTarget := fresh
	newTarget.Operation = DirectoryNewTarget
	newTarget.Recorded = &RecordedDirectoryRelease{
		ProductID: "tool", DistributionID: "other/tool", ReleaseSequence: 1,
		ResolvedRevision: snapshot.Distributions[0].Releases[0].PackageSource.Revision,
	}
	grant.CanonicalClosure.Eligible = false
	if _, err := ResolveDirectory(snapshot, newTarget); !errors.Is(err, ErrDirectoryIneligible) ||
		!strings.Contains(err.Error(), "required dependency closure is ineligible") {
		t.Fatalf("new target on recorded release ignored ineligible closure: %v", err)
	}
	grant.CanonicalClosure.Eligible = true
	newTarget.InstallerVersion = "1.2.3"
	if _, err := ResolveDirectory(snapshot, newTarget); !errors.Is(err, ErrDirectoryIneligible) ||
		!strings.Contains(err.Error(), "minimum installer version for signed required closure") {
		t.Fatalf("new target on recorded release ignored closure floor: %v", err)
	}
	newTarget.InstallerVersion = "2.1.0"
	if got, err := ResolveDirectory(snapshot, newTarget); err != nil || got.ReleaseSequence != 1 {
		t.Fatalf("new target with current eligible closure rejected: %+v %v", got, err)
	}
	grant.CanonicalClosure.Eligible = false
	locked := fresh
	locked.InstallerVersion = "1.2.3"
	locked.Purpose = DirectoryResolveLockedRestore
	locked.DesiredReleaseSequence = 1
	if got, err := ResolveDirectory(snapshot, locked); err != nil || got.DistributionID != "other/tool" || got.ReleaseSequence != 1 {
		t.Fatalf("historical exact lock inherited current closure: %+v %v", got, err)
	}
}

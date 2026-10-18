package domain

import (
	"errors"
	"strings"
	"testing"
)

func lockedRestoreFixture() (DirectorySnapshot, DirectoryResolveRequest) {
	snapshot := testDirectory()
	snapshot.Distributions[2].ReleasePolicies[0].Status = ReleaseSuperseded
	q := request("owner/tool", ClientCodex)
	q.Purpose = DirectoryResolveLockedRestore
	q.DesiredReleaseSequence = 2
	return snapshot, q
}

// A portable lock can restore its historical release without installed-state
// proof, but a fresh selector must still select the current release.
func TestResolveDirectoryLockedRestoreKeepsExactSupersededRelease(t *testing.T) {
	snapshot, q := lockedRestoreFixture()
	got, err := ResolveDirectory(snapshot, q)
	release := snapshot.Distributions[2].Releases[0]
	if err != nil || got.DistributionID != "owner/tool" || got.ReleaseSequence != 2 || got.Fallback ||
		got.Source != release.PackageSource || got.TreeDigest != release.TreeDigest || got.ManifestDigest != release.ManifestDigest {
		t.Fatalf("exact superseded lock was not restored: %+v %v", got, err)
	}
	q.Purpose = ""
	if _, err := ResolveDirectory(snapshot, q); !errors.Is(err, ErrDirectoryIneligible) || !strings.Contains(err.Error(), "historical only") {
		t.Fatalf("ordinary exact install accepted superseded release: %v", err)
	}
	q.DesiredReleaseSequence = 0
	if latest, err := ResolveDirectory(snapshot, q); err != nil || latest.ReleaseSequence != 3 {
		t.Fatalf("ordinary fresh selection changed: %+v %v", latest, err)
	}
}

// Misusing the purpose must fail before the historical-release exception can
// run, especially if a caller tries a broader recorded-state operation.
func TestResolveDirectoryLockedRestoreRejectsInvalidIntent(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*DirectoryResolveRequest)
	}{
		{"unknown purpose", func(q *DirectoryResolveRequest) { q.Purpose = "unrecognized_restore" }},
		{"unselected release", func(q *DirectoryResolveRequest) { q.DesiredReleaseSequence = 0 }},
		{"implicit scope", func(q *DirectoryResolveRequest) { q.Scope = "" }},
		{"project scope", func(q *DirectoryResolveRequest) { q.Scope = ScopeProject }},
		{"empty targets", func(q *DirectoryResolveRequest) { q.Targets = nil }},
		{"recorded proof", func(q *DirectoryResolveRequest) {
			q.Recorded = &RecordedDirectoryRelease{ProductID: "tool", DistributionID: "owner/tool", ReleaseSequence: 2, ResolvedRevision: testRelease(2, "").PackageSource.Revision}
		}},
	}
	for _, operation := range []DirectoryOperation{DirectoryNewTarget, DirectoryUpdate, DirectoryRepair, DirectoryRematerialize, DirectoryReproduce, DirectoryRemove, "unknown"} {
		tests = append(tests, struct {
			name   string
			mutate func(*DirectoryResolveRequest)
		}{"operation " + string(operation), func(q *DirectoryResolveRequest) { q.Operation = operation }})
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			snapshot, q := lockedRestoreFixture()
			test.mutate(&q)
			if _, err := ResolveDirectory(snapshot, q); !errors.Is(err, ErrDirectoryIneligible) {
				t.Fatalf("invalid restore intent was accepted: %v", err)
			}
		})
	}
}

// The exception changes only historical-release eligibility. Revocation,
// current policy and target compatibility still block the exact locked release
// even while a newer eligible release exists in the same distribution.
func TestResolveDirectoryLockedRestoreRetainsCurrentPolicyGates(t *testing.T) {
	tests := []struct {
		name, reason string
		mutate       func(*DirectorySnapshot, *DirectoryResolveRequest)
	}{
		{"revoked policy", "revoked", func(s *DirectorySnapshot, _ *DirectoryResolveRequest) {
			s.Distributions[2].ReleasePolicies[0].Status = ReleaseRevoked
		}},
		{"snapshot revocation", "revoked", func(s *DirectorySnapshot, _ *DirectoryResolveRequest) {
			s.Revocations = []DirectoryRevocation{{DistributionID: "owner/tool", ReleaseSequence: 2}}
		}},
		// Verified schema-1 snapshots already reject sequence zero. The domain
		// restore exception must also fail closed for this unvalidated input.
		{"distribution-wide revocation", "revoked", func(s *DirectorySnapshot, _ *DirectoryResolveRequest) {
			s.Revocations = []DirectoryRevocation{{DistributionID: "owner/tool"}}
		}},
		{"suspended distribution", "suspended", func(s *DirectorySnapshot, _ *DirectoryResolveRequest) {
			s.Distributions[2].Status = DistributionSuspended
		}},
		{"revoked distribution status", "distribution", func(s *DirectorySnapshot, _ *DirectoryResolveRequest) {
			s.Distributions[2].Status = "revoked"
		}},
		{"unknown distribution status", "distribution", func(s *DirectorySnapshot, _ *DirectoryResolveRequest) {
			s.Distributions[2].Status = "unknown"
		}},
		{"installer minimum", "minimum installer version", func(s *DirectorySnapshot, _ *DirectoryResolveRequest) {
			s.Distributions[2].ReleasePolicies[0].MinimumInstallerVersion = "9.0.0"
		}},
		{"unsupported target", "missing cursor", func(_ *DirectorySnapshot, q *DirectoryResolveRequest) {
			q.Targets = append(q.Targets, ClientCursor)
		}},
		{"unsupported scope", "missing codex", func(s *DirectorySnapshot, _ *DirectoryResolveRequest) {
			s.Distributions[2].ReleasePolicies[0].Targets[0].Scopes = []InstallScope{ScopeProject}
		}},
		{"incompatible delivery", "incompatible", func(s *DirectorySnapshot, _ *DirectoryResolveRequest) {
			s.Distributions[2].ReleasePolicies[0].Targets[0].Delivery = "manual_activation"
		}},
		{"missing current promotion", "materialization", func(s *DirectorySnapshot, _ *DirectoryResolveRequest) {
			s.Distributions[2].ReleasePolicies[0].CurrentEvidence = nil
		}},
		{"missing required component", "required component", func(s *DirectorySnapshot, _ *DirectoryResolveRequest) {
			s.Distributions[2].Releases[0].Components = []string{"skills"}
		}},
		{"missing exact release", "no eligible", func(_ *DirectorySnapshot, q *DirectoryResolveRequest) {
			q.DesiredReleaseSequence = 99
		}},
		{"ambiguous publisher", "ambiguous", func(_ *DirectorySnapshot, q *DirectoryResolveRequest) {
			q.Selector = "tool"
		}},
	}
	for _, level := range []string{"schema", "materialization", "runtime"} {
		tests = append(tests, struct {
			name, reason string
			mutate       func(*DirectorySnapshot, *DirectoryResolveRequest)
		}{"current failed " + level, "current trusted " + level + " evidence failed", func(s *DirectorySnapshot, q *DirectoryResolveRequest) {
			evidence := testTrustedEvidence(DirectoryEvidence{ID: "failed/" + level, DistributionID: "owner/tool", ReleaseSequence: 2,
				PackageTreeDigest: s.Distributions[2].Releases[0].TreeDigest, Level: level, Outcome: "failed"})
			if level != "schema" {
				evidence.Client, evidence.ClientVersion, evidence.InstallerVersion = ClientCodex, q.ClientVersions[ClientCodex], q.InstallerVersion
				evidence.OS, evidence.Architecture = q.OS, q.Architecture
			}
			s.Evidence = append(s.Evidence, evidence)
			s.Distributions[2].ReleasePolicies[0].CurrentEvidence = append(s.Distributions[2].ReleasePolicies[0].CurrentEvidence, evidence.ID)
		}})
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			snapshot, q := lockedRestoreFixture()
			test.mutate(&snapshot, &q)
			got, err := ResolveDirectory(snapshot, q)
			if err == nil || got.ReleaseSequence != 0 || !strings.Contains(err.Error(), test.reason) {
				t.Fatalf("exact restore escaped %s gate: %+v %v", test.name, got, err)
			}
		})
	}
}

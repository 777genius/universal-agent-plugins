package domain

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func projectSourceFixture() (DirectorySnapshot, DirectoryResolveRequest) {
	snapshot := testDirectory()
	snapshot.Distributions = snapshot.Distributions[:1]
	snapshot.Distributions[0].ReleasePolicies[0] = testPolicy(1, ClientCopilot)
	query := request("other/tool", ClientCopilot)
	query.Scope = ScopeProject
	return snapshot, query
}

func TestDirectoryV1ProjectSourceKeepsPublicationAndPlacementScopesSeparate(t *testing.T) {
	snapshot, query := projectSourceFixture()
	before := testDirectory()
	before.Distributions = before.Distributions[:1]
	before.Distributions[0].ReleasePolicies[0] = testPolicy(1, ClientCopilot)
	selected, err := ResolveDirectoryV1ProjectSource(snapshot, query)
	if err != nil || selected.DistributionID != "other/tool" || selected.ReleaseSequence != 1 {
		t.Fatalf("qualified Project source: %+v %v", selected, err)
	}
	if !reflect.DeepEqual(snapshot, before) || query.Scope != ScopeProject {
		t.Fatal("source selection mutated signed publication or caller scope")
	}
	user := query
	user.Scope = ScopeUser
	if _, err := ResolveDirectory(snapshot, user); !errors.Is(err, ErrDirectoryIneligible) || !strings.Contains(err.Error(), "missing vscode") {
		t.Fatalf("User shared-backend admission changed: %v", err)
	}
	if _, err := ResolveDirectory(snapshot, query); !errors.Is(err, ErrDirectoryIneligible) {
		t.Fatalf("strict signed Project resolution fabricated a grant: %v", err)
	}
}

func TestDirectoryV1ProjectSourcePreservesSignedRefusals(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(*DirectorySnapshot, *DirectoryResolveRequest)
	}{
		{"missing target", func(s *DirectorySnapshot, _ *DirectoryResolveRequest) {
			s.Distributions[0].ReleasePolicies[0].Targets = nil
		}},
		{"revoked policy", func(s *DirectorySnapshot, _ *DirectoryResolveRequest) {
			s.Distributions[0].ReleasePolicies[0].Status = ReleaseRevoked
		}},
		{"revoked source", func(s *DirectorySnapshot, _ *DirectoryResolveRequest) {
			s.Revocations = []DirectoryRevocation{{DistributionID: "other/tool", ReleaseSequence: 1}}
		}},
		{"publication floor", func(s *DirectorySnapshot, _ *DirectoryResolveRequest) {
			s.Distributions[0].ReleasePolicies[0].MinimumInstallerVersion = "99.0.0"
		}},
		{"target floor", func(s *DirectorySnapshot, _ *DirectoryResolveRequest) {
			s.Distributions[0].ReleasePolicies[0].Targets[0].MinimumInstallerVersion = "99.0.0"
		}},
		{"wrong publication delivery", func(s *DirectorySnapshot, _ *DirectoryResolveRequest) {
			s.Distributions[0].ReleasePolicies[0].Targets[0].Delivery = "prepared"
		}},
		{"failed User publication evidence", func(s *DirectorySnapshot, _ *DirectoryResolveRequest) {
			evidence := testTrustedEvidence(DirectoryEvidence{ID: "failed/user/runtime", DistributionID: "other/tool", ReleaseSequence: 1,
				PackageTreeDigest: s.Distributions[0].Releases[0].TreeDigest, Client: ClientCopilot, Scope: ScopeUser, Level: "runtime", Outcome: "failed"})
			s.Evidence = append(s.Evidence, evidence)
			s.Distributions[0].ReleasePolicies[0].CurrentEvidence = []string{evidence.ID}
		}},
		{"schema two", func(s *DirectorySnapshot, _ *DirectoryResolveRequest) { s.SnapshotSchemaVersion = 2 }},
		{"User request", func(_ *DirectorySnapshot, q *DirectoryResolveRequest) { q.Scope = ScopeUser }},
		{"implicit scope", func(_ *DirectorySnapshot, q *DirectoryResolveRequest) { q.Scope = "" }},
		{"unqualified Project client", func(_ *DirectorySnapshot, q *DirectoryResolveRequest) { q.Targets = []ClientID{ClientChatGPT} }},
		{"unknown Project client", func(_ *DirectorySnapshot, q *DirectoryResolveRequest) { q.Targets = []ClientID{"unknown"} }},
	} {
		t.Run(test.name, func(t *testing.T) {
			snapshot, query := projectSourceFixture()
			test.change(&snapshot, &query)
			if _, err := ResolveDirectoryV1ProjectSource(snapshot, query); !errors.Is(err, ErrDirectoryIneligible) {
				t.Fatalf("Project source bypassed signed/placement refusal: %v", err)
			}
		})
	}
}

func TestDirectoryV1ProjectSourceExactLockPreservesImmutableIdentity(t *testing.T) {
	snapshot, query := projectSourceFixture()
	snapshot.Distributions[0].ReleasePolicies[0].Status = ReleaseSuperseded
	if _, err := ResolveDirectoryV1ProjectSource(snapshot, query); !errors.Is(err, ErrDirectoryIneligible) {
		t.Fatalf("fresh selection admitted superseded release: %v", err)
	}
	query.Purpose, query.DesiredReleaseSequence = DirectoryResolveLockedRestore, 1
	selected, err := ResolveDirectoryV1ProjectSource(snapshot, query)
	want := snapshot.Distributions[0].Releases[0]
	if err != nil || selected.Source != want.PackageSource || selected.TreeDigest != want.TreeDigest || selected.ManifestDigest != want.ManifestDigest {
		t.Fatalf("exact Project source restore changed pins: %+v %v", selected, err)
	}
	query.DesiredReleaseSequence = 2
	if _, err := ResolveDirectoryV1ProjectSource(snapshot, query); !errors.Is(err, ErrDirectoryIneligible) {
		t.Fatalf("missing exact lock silently moved source: %v", err)
	}
	query.DesiredReleaseSequence = 1
	snapshot.Revocations = []DirectoryRevocation{{DistributionID: "other/tool", ReleaseSequence: 0}}
	if _, err := ResolveDirectoryV1ProjectSource(snapshot, query); !errors.Is(err, ErrDirectoryIneligible) {
		t.Fatalf("exact Project lock bypassed distribution revocation: %v", err)
	}
}

package directoryv1

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

const futureTestClient domain.ClientID = "future-test-agent"

func futureClientSnapshot(sequence uint64) domain.DirectorySnapshot {
	// Independent modern sequence one uses current evidence, never the
	// historical lineage's pre-15 source-only evidence representation.
	snapshot := fixtureSnapshot(max(sequence, domain.DirectoryEvidenceTrustCutoverSequence))
	snapshot.Sequence, snapshot.PublicationID = sequence, fmt.Sprintf("publication-%d", sequence)
	policy := &snapshot.Distributions[0].ReleasePolicies[0]
	policy.Targets = append(policy.Targets, domain.DirectoryTarget{
		Client: futureTestClient, Scopes: []domain.InstallScope{domain.ScopeUser},
		Delivery: "managed", Authentication: domain.AuthenticationRequirementUnknown,
	})
	evidence := snapshot.Evidence[0]
	evidence.ID, evidence.Level, evidence.Client = "future-materialization", "materialization", futureTestClient
	evidence.ClientVersion, evidence.InstallerVersion = "fixture-agent-1", "1.0.0"
	evidence.OS, evidence.Architecture, evidence.ObservedAt = "linux", "amd64", "2026-08-20T10:30:00Z"
	if evidence.Trust != nil {
		trust := *evidence.Trust
		evidence.Trust = &trust
	}
	snapshot.Evidence = append(snapshot.Evidence, evidence)
	policy.CurrentEvidence = append(policy.CurrentEvidence, evidence.ID)
	return snapshot
}

func verifyModernSnapshot(t *testing.T, snapshot domain.DirectorySnapshot) (VerifiedBundle, TrustStore) {
	t.Helper()
	key, private := fixtureKey("modern-reader-test-only", KeyCurrent, 34)
	trust := NewModernTrustStore([]TrustedKey{key})
	body, envelope, _ := signedSnapshotFixture(t, snapshot, key, private)
	verified, err := VerifyBundle(body, envelope, trust)
	if err != nil {
		t.Fatal(err)
	}
	return verified, trust
}

func TestModernReaderRetainsAuthenticatedFutureTargetsAndEvidence(t *testing.T) {
	for _, sequence := range []uint64{1, 13, 41} {
		snapshot := futureClientSnapshot(sequence)
		verified, trust := verifyModernSnapshot(t, snapshot)
		if !reflect.DeepEqual(verified.Snapshot, snapshot) {
			t.Fatal("modern reader discarded or rewrote authenticated future records")
		}
		if !bytes.Equal(verified.SnapshotBytes, append(encodeSnapshot(t, snapshot), '\n')) {
			t.Fatal("modern reader changed exact signed bytes")
		}
		if _, err := ParseSnapshot(verified.SnapshotBytes); !errors.Is(err, ErrStrictJSON) {
			t.Fatalf("historical parser accepted future client: %v", err)
		}
		if _, err := VerifyBundle(verified.SnapshotBytes, verified.EnvelopeBytes, TrustStore{Keys: trust.Keys}); !errors.Is(err, ErrStrictJSON) {
			t.Fatalf("historical trust accepted future client: %v", err)
		}
	}
}

func TestModernReaderRequiresCurrentEvidenceTrustFromSequenceOne(t *testing.T) {
	key, private := fixtureKey("modern-reader-test-only", KeyCurrent, 34)
	trust := NewModernTrustStore([]TrustedKey{key})
	for _, sequence := range []uint64{1, 14} {
		legacy := fixtureSnapshot(sequence)
		body, envelope, _ := signedSnapshotFixture(t, legacy, key, private)
		if _, err := VerifyBundle(body, envelope, trust); !errors.Is(err, ErrStrictJSON) {
			t.Fatalf("modern lineage accepted historical evidence at sequence %d: %v", sequence, err)
		}
		if _, err := ParseSnapshot(body); err != nil {
			t.Fatalf("historical evidence contract changed at sequence %d: %v", sequence, err)
		}
		current := futureClientSnapshot(sequence)
		current.Evidence[1].Trust = nil
		body, envelope, _ = signedSnapshotFixture(t, current, key, private)
		if _, err := VerifyBundle(body, envelope, trust); !errors.Is(err, ErrStrictJSON) {
			t.Fatalf("modern lineage accepted missing trust at sequence %d: %v", sequence, err)
		}
	}
}

func TestModernReaderRequiresBothFutureTargetsAndEvidenceToBeWellFormed(t *testing.T) {
	key, private := fixtureKey("modern-reader-test-only", KeyCurrent, 34)
	trust := NewModernTrustStore([]TrustedKey{key})
	cases := []struct {
		name   string
		mutate func(*domain.DirectorySnapshot)
	}{
		{"unsafe target ID", func(s *domain.DirectorySnapshot) {
			s.Distributions[0].ReleasePolicies[0].Targets[2].Client = "../agent"
		}},
		{"empty target ID", func(s *domain.DirectorySnapshot) { s.Distributions[0].ReleasePolicies[0].Targets[2].Client = "" }},
		{"noncanonical target ID", func(s *domain.DirectorySnapshot) {
			s.Distributions[0].ReleasePolicies[0].Targets[2].Client = "Future-Agent"
		}},
		{"numeric target ID", func(s *domain.DirectorySnapshot) {
			s.Distributions[0].ReleasePolicies[0].Targets[2].Client = "123-agent"
		}},
		{"empty target ID segment", func(s *domain.DirectorySnapshot) {
			s.Distributions[0].ReleasePolicies[0].Targets[2].Client = "future--agent"
		}},
		{"duplicate target", func(s *domain.DirectorySnapshot) {
			p := &s.Distributions[0].ReleasePolicies[0]
			p.Targets = append(p.Targets, p.Targets[2])
		}},
		{"future delivery", func(s *domain.DirectorySnapshot) {
			s.Distributions[0].ReleasePolicies[0].Targets[2].Delivery = "unreviewed"
		}},
		{"known delivery", func(s *domain.DirectorySnapshot) {
			s.Distributions[0].ReleasePolicies[0].Targets[0].Delivery = "prepared"
		}},
		{"authentication", func(s *domain.DirectorySnapshot) {
			s.Distributions[0].ReleasePolicies[0].Targets[2].Authentication = "automatic"
		}},
		{"project grant", func(s *domain.DirectorySnapshot) {
			s.Distributions[0].ReleasePolicies[0].Targets[2].Scopes = []domain.InstallScope{domain.ScopeProject}
		}},
		{"multiple scopes", func(s *domain.DirectorySnapshot) {
			s.Distributions[0].ReleasePolicies[0].Targets[2].Scopes = []domain.InstallScope{domain.ScopeUser, domain.ScopeProject}
		}},
		{"future app binding", func(s *domain.DirectorySnapshot) {
			s.Distributions[0].ReleasePolicies[0].Targets[2].AppBinding = &domain.DirectoryAppBinding{AppKey: "tool", ID: "app_tool", MCPServer: "tool"}
		}},
		{"unsafe evidence ID", func(s *domain.DirectorySnapshot) { s.Evidence[1].Client = "bad/client" }},
		{"empty evidence ID", func(s *domain.DirectorySnapshot) { s.Evidence[1].Client = "" }},
		{"missing applicability", func(s *domain.DirectorySnapshot) { s.Evidence[1].ClientVersion = "" }},
		{"invalid outcome", func(s *domain.DirectorySnapshot) { s.Evidence[1].Outcome = "approved" }},
		{"wrong release", func(s *domain.DirectorySnapshot) { s.Evidence[1].ReleaseSequence++ }},
		{"wrong tree", func(s *domain.DirectorySnapshot) {
			s.Evidence[1].PackageTreeDigest = "sha256:" + strings.Repeat("f", 64)
		}},
		{"missing trust", func(s *domain.DirectorySnapshot) { s.Evidence[1].Trust = nil }},
		{"wrong workflow repository", func(s *domain.DirectorySnapshot) {
			s.Evidence[1].Trust.Workflow = "other/evidence/.github/workflows/directory.yml"
		}},
		{"duplicate evidence", func(s *domain.DirectorySnapshot) { s.Evidence = append(s.Evidence, s.Evidence[1]) }},
		{"unselected evidence", func(s *domain.DirectorySnapshot) {
			s.Distributions[0].ReleasePolicies[0].CurrentEvidence = []string{"schema-pass"}
		}},
		{"duplicate applicability tuple", func(s *domain.DirectorySnapshot) {
			e := s.Evidence[1]
			e.ID = "second-future-materialization"
			s.Evidence = append(s.Evidence, e)
			p := &s.Distributions[0].ReleasePolicies[0]
			p.CurrentEvidence = append(p.CurrentEvidence, e.ID)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			snapshot := futureClientSnapshot(41)
			tc.mutate(&snapshot)
			body, envelope, _ := signedSnapshotFixture(t, snapshot, key, private)
			if _, err := VerifyBundle(body, envelope, trust); !errors.Is(err, ErrStrictJSON) {
				t.Fatalf("signed malformed future/known record accepted: %v", err)
			}
		})
	}
}

func modernResolveRequest(client domain.ClientID) domain.DirectoryResolveRequest {
	return domain.DirectoryResolveRequest{
		Selector: "tool", Targets: []domain.ClientID{client}, Scope: domain.ScopeUser,
		InstallerVersion: "1.0.0", ClientVersions: map[domain.ClientID]string{client: "fixture-agent-1"},
		OS: "linux", Architecture: "amd64", SchemaVersion: "1.0.0", Operation: domain.DirectoryInstall,
	}
}

func TestModernReaderUnknownClientsCannotAuthorizeInstallation(t *testing.T) {
	for _, sequence := range []uint64{1, 41} {
		t.Run(fmt.Sprint(sequence), func(t *testing.T) {
			snapshot := futureClientSnapshot(sequence)
			snapshot.Distributions[0].Kind = domain.DistributionCommunity
			verified, _ := verifyModernSnapshot(t, snapshot)
			known := modernResolveRequest(domain.ClientCursor)
			if _, err := domain.ResolveDirectory(verified.Snapshot, known); err != nil {
				t.Fatalf("unrelated future ID disabled known client: %v", err)
			}
			unknown := modernResolveRequest(futureTestClient)
			if _, err := domain.ResolveDirectory(verified.Snapshot, unknown); !errors.Is(err, domain.ErrDirectoryIneligible) || !strings.Contains(err.Error(), "incompatible") {
				t.Fatalf("signed future target became selectable: %v", err)
			}
			unknown.Scope = domain.ScopeProject
			if _, err := domain.ResolveDirectoryV1ProjectSource(verified.Snapshot, unknown); !errors.Is(err, domain.ErrDirectoryIneligible) {
				t.Fatalf("future target authorized Project source selection: %v", err)
			}
			snapshot.Distributions[0].Kind = domain.DistributionUpstream
			verified, _ = verifyModernSnapshot(t, snapshot)
			if _, err := domain.ResolveDirectory(verified.Snapshot, known); !errors.Is(err, domain.ErrDirectoryIneligible) || !strings.Contains(err.Error(), "materialization") {
				t.Fatalf("future evidence promoted an unproven known client: %v", err)
			}
		})
	}
}

func TestModernReaderRetainsBlockingKnownClientEvidence(t *testing.T) {
	for _, sequence := range []uint64{1, 41} {
		for _, level := range []string{"materialization", "discovery", "runtime", "oauth"} {
			t.Run(fmt.Sprintf("%d/%s", sequence, level), func(t *testing.T) {
				snapshot := futureClientSnapshot(sequence)
				snapshot.Distributions[0].Kind = domain.DistributionCommunity
				failed := snapshot.Evidence[1]
				failed.ID, failed.Client, failed.Level, failed.Outcome = "known-failure", domain.ClientCursor, level, "failed"
				snapshot.Evidence = append(snapshot.Evidence, failed)
				policy := &snapshot.Distributions[0].ReleasePolicies[0]
				policy.CurrentEvidence = append(policy.CurrentEvidence, failed.ID)
				verified, _ := verifyModernSnapshot(t, snapshot)
				query := modernResolveRequest(domain.ClientCursor)
				if _, err := domain.ResolveDirectory(verified.Snapshot, query); !errors.Is(err, domain.ErrDirectoryIneligible) || !strings.Contains(err.Error(), "current trusted "+level+" evidence failed") {
					t.Fatalf("future records bypassed current known-client failure: %v", err)
				}
				query.Scope = domain.ScopeProject
				if _, err := domain.ResolveDirectoryV1ProjectSource(verified.Snapshot, query); !errors.Is(err, domain.ErrDirectoryIneligible) || !strings.Contains(err.Error(), "current trusted "+level+" evidence failed") {
					t.Fatalf("Project source selection bypassed known User failure: %v", err)
				}
			})
		}
	}
}

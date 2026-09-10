package domain

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func context7PreparationFixture() (DirectorySnapshot, DirectoryResolveRequest) {
	s := testDirectory()
	d := s.Distributions[2]
	d.ID, d.ProductID = "upstash/context7", "context7"
	d.Releases, d.ReleasePolicies = d.Releases[1:], d.ReleasePolicies[1:]
	r := &d.Releases[0]
	r.ManifestName = "context7"
	r.PackageSource.Repository, r.PackageSource.Path = "upstash/context7", "plugins/agent-plugins/context7"
	p := &s.Products[0]
	p.ID, p.ManifestName, p.Aliases = "context7", "context7", nil
	p.DefaultDistribution, p.Distributions = d.ID, []string{d.ID}
	s.Distributions = []DirectoryDistribution{d}
	s.Evidence = s.Evidence[1:]
	s.Evidence[0].DistributionID = d.ID
	q := request("context7", ClientChatGPT)
	q.Purpose = DirectoryResolveContext7ChatGPTPreparation
	return s, q
}

func TestResolveDirectoryContext7PreparationSourceOnly(t *testing.T) {
	s, q := context7PreparationFixture()
	before, _ := context7PreparationFixture()
	normal := q
	normal.Purpose = ""
	if _, err := ResolveDirectory(s, normal); !errors.Is(err, ErrDirectoryIneligible) || !strings.Contains(err.Error(), "complete target set") {
		t.Fatalf("normal missing ChatGPT target: %v", err)
	}
	for _, selector := range []string{"context7", "upstash/context7"} {
		q.Selector = selector
		got, err := ResolveDirectory(s, q)
		r := s.Distributions[0].Releases[0]
		if err != nil || got.DistributionID != "upstash/context7" || got.Source != r.PackageSource || got.TreeDigest != r.TreeDigest || got.ManifestDigest != r.ManifestDigest || got.Fallback {
			t.Fatalf("source selection: %+v %v", got, err)
		}
	}
	if !reflect.DeepEqual(s, before) {
		t.Fatal("resolution mutated signed metadata")
	}
	// A later revision is governed by signed policy/evidence, never a pinned SHA.
	s.Distributions[0].Releases[0].PackageSource.Revision = strings.Repeat("f", 40)
	if _, err := ResolveDirectory(s, q); err != nil {
		t.Fatal(err)
	}
}

func TestResolveDirectoryContext7PreparationBoundaries(t *testing.T) {
	cases := []struct {
		name   string
		change func(*DirectorySnapshot, *DirectoryResolveRequest)
	}{
		{"unknown purpose", func(s *DirectorySnapshot, q *DirectoryResolveRequest) { q.Purpose = "bypass" }},
		{"project", func(s *DirectorySnapshot, q *DirectoryResolveRequest) { q.Scope = ScopeProject }},
		{"implicit scope", func(s *DirectorySnapshot, q *DirectoryResolveRequest) { q.Scope = "" }},
		{"wrong target", func(s *DirectorySnapshot, q *DirectoryResolveRequest) { q.Targets = []ClientID{ClientCodex} }},
		{"empty targets", func(s *DirectorySnapshot, q *DirectoryResolveRequest) { q.Targets = nil }},
		{"mixed targets", func(s *DirectorySnapshot, q *DirectoryResolveRequest) { q.Targets = append(q.Targets, ClientCodex) }},
		{"product", func(s *DirectorySnapshot, q *DirectoryResolveRequest) { s.Products[0].ID = "other" }},
		{"product manifest", func(s *DirectorySnapshot, q *DirectoryResolveRequest) { s.Products[0].ManifestName = "other" }},
		{"distribution", func(s *DirectorySnapshot, q *DirectoryResolveRequest) {
			s.Distributions[0].ID = "other/context7"
			s.Products[0].Distributions = []string{"other/context7"}
			q.Selector = "other/context7"
		}},
		{"kind", func(s *DirectorySnapshot, q *DirectoryResolveRequest) {
			s.Distributions[0].Kind = DistributionCommunity
		}},
		{"repository", func(s *DirectorySnapshot, q *DirectoryResolveRequest) {
			s.Distributions[0].Releases[0].PackageSource.Repository = "other/context7"
		}},
		{"path", func(s *DirectorySnapshot, q *DirectoryResolveRequest) {
			s.Distributions[0].Releases[0].PackageSource.Path = "other"
		}},
		{"manifest", func(s *DirectorySnapshot, q *DirectoryResolveRequest) {
			s.Distributions[0].Releases[0].ManifestName = "other"
		}},
		{"missing promotion", func(s *DirectorySnapshot, q *DirectoryResolveRequest) { s.Evidence = nil }},
		{"untrusted promotion", func(s *DirectorySnapshot, q *DirectoryResolveRequest) { s.Evidence[0].Trust = nil }},
		{"stale promotion", func(s *DirectorySnapshot, q *DirectoryResolveRequest) {
			s.Distributions[0].ReleasePolicies[0].CurrentEvidence = nil
		}},
		{"wrong digest", func(s *DirectorySnapshot, q *DirectoryResolveRequest) {
			s.Evidence[0].PackageTreeDigest = "sha256:wrong"
		}},
		{"wrong sequence", func(s *DirectorySnapshot, q *DirectoryResolveRequest) { s.Evidence[0].ReleaseSequence++ }},
		{"wrong evidence distribution", func(s *DirectorySnapshot, q *DirectoryResolveRequest) {
			s.Evidence[0].DistributionID = "other/context7"
		}},
		{"undeclared promotion", func(s *DirectorySnapshot, q *DirectoryResolveRequest) {
			s.Distributions[0].ReleasePolicies[0].Targets = nil
		}},
		{"unsupported promotion", func(s *DirectorySnapshot, q *DirectoryResolveRequest) {
			s.Distributions[0].ReleasePolicies[0].Targets[0].Client = "unknown"
			s.Evidence[0].Client = "unknown"
		}},
		{"promotion delivery", func(s *DirectorySnapshot, q *DirectoryResolveRequest) {
			s.Distributions[0].ReleasePolicies[0].Targets[0].Delivery = "wrong"
		}},
		{"promotion scope", func(s *DirectorySnapshot, q *DirectoryResolveRequest) {
			s.Distributions[0].ReleasePolicies[0].Targets[0].Scopes = nil
		}},
		{"schema", func(s *DirectorySnapshot, q *DirectoryResolveRequest) {
			s.Distributions[0].Releases[0].AgentPluginsSchema = "unsupported"
		}},
		{"requested schema", func(s *DirectorySnapshot, q *DirectoryResolveRequest) { q.SchemaVersion = "2.0.0" }},
		{"minimum version", func(s *DirectorySnapshot, q *DirectoryResolveRequest) {
			s.Distributions[0].ReleasePolicies[0].MinimumInstallerVersion = "99.0.0"
		}},
		{"required component", func(s *DirectorySnapshot, q *DirectoryResolveRequest) { q.RequiredComponents = []string{"absent"} }},
		{"product component", func(s *DirectorySnapshot, q *DirectoryResolveRequest) {
			s.Distributions[0].Releases[0].Components = nil
		}},
		{"declared ChatGPT scope", func(s *DirectorySnapshot, q *DirectoryResolveRequest) {
			p := &s.Distributions[0].ReleasePolicies[0]
			p.Targets = append(p.Targets, DirectoryTarget{Client: ClientChatGPT, Scopes: []InstallScope{ScopeProject}, Delivery: "manual_activation"})
		}},
		{"declared ChatGPT missing promotion", func(s *DirectorySnapshot, q *DirectoryResolveRequest) {
			p := &s.Distributions[0].ReleasePolicies[0]
			p.Targets = append(p.Targets, testPolicy(3, ClientChatGPT).Targets...)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, q := context7PreparationFixture()
			tc.change(&s, &q)
			if got, err := ResolveDirectory(s, q); err == nil {
				t.Fatalf("accepted: %+v", got)
			}
		})
	}
}

func TestResolveDirectoryContext7PreparationNegativeEvidence(t *testing.T) {
	for _, level := range []string{"schema", "materialization", "discovery", "runtime", "oauth"} {
		t.Run(level, func(t *testing.T) {
			s, q := context7PreparationFixture()
			e := testTrustedEvidence(DirectoryEvidence{ID: "failure", DistributionID: "upstash/context7", ReleaseSequence: 3, PackageTreeDigest: s.Distributions[0].Releases[0].TreeDigest, Client: ClientChatGPT, Level: level, Outcome: "failed"})
			if level == "schema" {
				e.Client = ""
			}
			s.Evidence = append(s.Evidence, e)
			s.Distributions[0].ReleasePolicies[0].CurrentEvidence = append(s.Distributions[0].ReleasePolicies[0].CurrentEvidence, e.ID)
			if _, err := ResolveDirectory(s, q); err == nil || !strings.Contains(err.Error(), "evidence failed") {
				t.Fatalf("negative evidence: %v", err)
			}
			if level != "schema" {
				s.Evidence[1].OS = "different-os"
				if _, err := ResolveDirectory(s, q); err != nil {
					t.Fatalf("inapplicable evidence: %v", err)
				}
				s.Evidence[1].OS = ""
			}
			s.Evidence[1].Trust = nil
			if _, err := ResolveDirectory(s, q); err != nil {
				t.Fatalf("untrusted failure: %v", err)
			}
		})
	}
}

func TestResolveDirectoryContext7PreparationRecordedPolicy(t *testing.T) {
	for _, op := range []DirectoryOperation{DirectoryInstall, DirectoryNewTarget, DirectoryUpdate, DirectoryRepair, DirectoryRematerialize, DirectoryReproduce, DirectoryRemove} {
		for _, status := range []string{"active", "suspended", "superseded", "revoked", "top-level revoked"} {
			for _, recorded := range []bool{false, true} {
				t.Run(string(op)+"/"+status+"/recorded="+map[bool]string{true: "yes", false: "no"}[recorded], func(t *testing.T) {
					s, q := context7PreparationFixture()
					q.Operation = op
					d := &s.Distributions[0]
					if recorded {
						q.Recorded = &RecordedDirectoryRelease{ProductID: "context7", DistributionID: d.ID, ReleaseSequence: 3, ResolvedRevision: d.Releases[0].PackageSource.Revision, TreeDigest: d.Releases[0].TreeDigest}
					}
					switch status {
					case "suspended":
						d.Status = DistributionSuspended
					case "superseded":
						d.ReleasePolicies[0].Status = ReleaseSuperseded
					case "revoked":
						d.ReleasePolicies[0].Status = ReleaseRevoked
					case "top-level revoked":
						s.Revocations = []DirectoryRevocation{{DistributionID: d.ID, ReleaseSequence: 3}}
					}
					// Compare to the existing real resolver on the declared promoted target.
					normal := q
					normal.Purpose, normal.Targets = "", []ClientID{ClientCodex}
					want, wantErr := ResolveDirectory(s, normal)
					got, err := ResolveDirectory(s, q)
					if (err == nil) != (wantErr == nil) || (err == nil && !reflect.DeepEqual(got, want)) {
						t.Fatalf("prep %+v %v; normal %+v %v", got, err, want, wantErr)
					}
					if errors.Is(err, ErrDirectoryNoSafeUpdate) != errors.Is(wantErr, ErrDirectoryNoSafeUpdate) {
						t.Fatalf("changed update error: %v vs %v", err, wantErr)
					}
					if recorded {
						q.Recorded.TreeDigest = "sha256:wrong"
						if _, err := ResolveDirectory(s, q); err == nil {
							t.Fatal("recorded digest mismatch accepted")
						}
					}
				})
			}
		}
	}
}

func TestResolveDirectoryContext7PreparationUpdateAndQualifiedIdentity(t *testing.T) {
	s, q := context7PreparationFixture()
	d := &s.Distributions[0]
	q.Recorded = &RecordedDirectoryRelease{ProductID: "context7", DistributionID: d.ID, ReleaseSequence: 3, ResolvedRevision: d.Releases[0].PackageSource.Revision}
	newer := d.Releases[0]
	newer.Sequence, newer.PackageVersion = 4, "0.0.1"
	newer.PackageSource.Revision = strings.Repeat("e", 40)
	d.Releases = append(d.Releases, newer)
	policy := testPolicy(4, ClientCodex)
	policy.CurrentEvidence = []string{"new-promotion"}
	d.ReleasePolicies = append(d.ReleasePolicies, policy)
	evidence := s.Evidence[0]
	evidence.ID, evidence.ReleaseSequence = "new-promotion", 4
	s.Evidence = append(s.Evidence, evidence)
	for _, op := range []DirectoryOperation{DirectoryInstall, DirectoryRepair, DirectoryUpdate} {
		q.Operation = op
		want := uint64(3)
		if op == DirectoryUpdate {
			want = 4
		}
		got, err := ResolveDirectory(s, q)
		if err != nil || got.ReleaseSequence != want {
			t.Fatalf("%s: %+v %v", op, got, err)
		}
	}
	s.Revocations = []DirectoryRevocation{{DistributionID: d.ID, ReleaseSequence: 3}}
	if got, err := ResolveDirectory(s, q); err != nil || got.ReleaseSequence != 4 {
		t.Fatalf("update from revoked: %+v %v", got, err)
	}
	// Neither selector qualification nor recorded identity can widen the purpose.
	other := *d
	other.ID = "other/context7"
	s.Distributions = append(s.Distributions, other)
	s.Products[0].Distributions = append(s.Products[0].Distributions, other.ID)
	q.Selector = other.ID
	if _, err := ResolveDirectory(s, q); !errors.Is(err, ErrDirectoryIneligible) {
		t.Fatalf("wrong qualified selector with upstream record: %v", err)
	}
	q.Selector, q.Recorded.DistributionID = "context7", other.ID
	if _, err := ResolveDirectory(s, q); !errors.Is(err, ErrDirectoryIneligible) {
		t.Fatalf("wrong recorded distribution: %v", err)
	}
	q.Recorded.DistributionID = "upstash/context7"
	q.Recorded.ResolvedRevision = strings.Repeat("f", 40)
	if _, err := ResolveDirectory(s, q); err == nil {
		t.Fatal("changed recorded revision accepted")
	}
}

package directoryv1

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestV2PortableIdentityParityWithV1(t *testing.T) {
	signed, _, _ := v2SignedFixture(t)
	tests := []struct {
		name   string
		mutate func(*domain.DirectorySnapshot)
	}{
		{"unknown component", func(s *domain.DirectorySnapshot) {
			s.Distributions[0].Releases[0].Components = []string{"mcp", "unrecognized"}
		}},
		{"duplicate component", func(s *domain.DirectorySnapshot) {
			s.Distributions[0].Releases[0].Components = []string{"mcp", "mcp"}
		}},
		{"invalid packager", func(s *domain.DirectorySnapshot) {
			s.Distributions[0].Packager = "not/@valid"
		}},
		{"duplicate revocation", func(s *domain.DirectorySnapshot) {
			s.Revocations = []domain.DirectoryRevocation{
				{DistributionID: "owner/tool", ReleaseSequence: 9},
				{DistributionID: "owner/tool", ReleaseSequence: 9},
			}
		}},
		{"untrusted schema evidence", func(s *domain.DirectorySnapshot) {
			s.Evidence[0].Trust = &domain.DirectoryEvidenceTrust{Kind: "reviewed_external"}
		}},
		{"duplicate evidence applicability", func(s *domain.DirectorySnapshot) {
			duplicate := s.Evidence[0]
			duplicate.ID = "duplicated-schema-pass"
			s.Evidence = append(s.Evidence, duplicate)
			s.Distributions[0].ReleasePolicies[0].CurrentEvidence = append(
				s.Distributions[0].ReleasePolicies[0].CurrentEvidence, duplicate.ID)
		}},
		{"invalid provenance", func(s *domain.DirectorySnapshot) {
			s.Distributions[0].Releases[0].BuildProvenance = &domain.DirectoryBuildProvenance{
				UpstreamRepository: "invalid/repo@tag", UpstreamRevision: "bad",
			}
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var candidate domain.DirectorySnapshot
			if err := json.Unmarshal(signed, &candidate); err != nil {
				t.Fatal(err)
			}
			tc.mutate(&candidate)
			body, err := json.Marshal(candidate)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := ParseV2Snapshot(body); !errors.Is(err, ErrStrictJSON) {
				t.Fatalf("v2 accepted invalid %s: %v", tc.name, err)
			}
		})
	}
}

func TestV2RejectsCrossProductSelectorTakeover(t *testing.T) {
	var snapshot domain.DirectorySnapshot
	signed, _, _ := v2SignedFixture(t)
	if err := json.Unmarshal(signed, &snapshot); err != nil {
		t.Fatal(err)
	}
	other := snapshot.Products[0]
	other.ID, other.ManifestName = "other", "other"
	other.Aliases = []string{"tool"}
	other.ReservedAliases = []string{"other"}
	other.Distributions = []string{"owner/other"}
	other.DefaultDistribution = "owner/other"
	if err := validateV2ProductSelectors(append(snapshot.Products, other)); !errors.Is(err, ErrStrictJSON) {
		t.Fatalf("v2 failed to reject cross-product alias takeover: %v", err)
	}
}

func TestV2RejectsCrossProductDistributionTakeover(t *testing.T) {
	var snapshot domain.DirectorySnapshot
	signed, _, _ := v2SignedFixture(t)
	if err := json.Unmarshal(signed, &snapshot); err != nil {
		t.Fatal(err)
	}
	other := snapshot.Products[0]
	other.ID, other.ManifestName = "other", "other"
	other.Aliases, other.ReservedAliases = []string{"other"}, []string{"other"}
	snapshot.Products = append(snapshot.Products, other)
	if err := validateV2ProductDistributionOwnership(snapshot); !errors.Is(err, ErrStrictJSON) {
		t.Fatalf("v2 allowed a second product to claim a foreign distribution: %v", err)
	}
}

func TestV2RequiresExactProjectRuntimeEvidence(t *testing.T) {
	signed, _, _ := v2SignedFixture(t)
	var original domain.DirectorySnapshot
	if err := json.Unmarshal(signed, &original); err != nil {
		t.Fatal(err)
	}
	// A schema record cannot stand in for a real client/project MCP call.
	policy := &original.Distributions[0].ReleasePolicies[0]
	project := policy.Targets[0]
	project.Scope = domain.ScopeProject
	project.CurrentEvidence = []string{"schema-pass"}
	policy.Targets = append(policy.Targets, project)
	body, err := json.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseV2Snapshot(body); !errors.Is(err, ErrStrictJSON) {
		t.Fatalf("project grant borrowed common schema evidence: %v", err)
	}
	// A genuine, exact scoped runtime receipt is required; another client's
	// result never qualifies this client even when both use the same package.
	runtime := original.Evidence[0]
	runtime.ID, runtime.Client, runtime.Scope = "project-runtime", domain.ClientCursor, domain.ScopeProject
	runtime.Level, runtime.Outcome = "runtime", "passed"
	runtime.ClientVersion, runtime.InstallerVersion = "1.0.0", "1.2.3"
	runtime.OS, runtime.Architecture, runtime.ObservedAt = "linux", "amd64", "2026-08-20T10:30:00Z"
	original.Evidence = append(original.Evidence, runtime)
	policy.Targets[len(policy.Targets)-1].CurrentEvidence = []string{runtime.ID}
	body, _ = json.Marshal(original)
	if _, err := ParseV2Snapshot(body); !errors.Is(err, ErrStrictJSON) {
		t.Fatalf("project grant borrowed other client's runtime: %v", err)
	}
	original.Evidence[len(original.Evidence)-1].Client = domain.ClientCodex
	body, _ = json.Marshal(original)
	if _, err := ParseV2Snapshot(body); err != nil {
		t.Fatalf("exact client/project runtime evidence should qualify: %v", err)
	}
}

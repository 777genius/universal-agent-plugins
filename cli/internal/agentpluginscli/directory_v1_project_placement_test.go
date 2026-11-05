package agentpluginscli

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func v1ProjectSelectionRequest() domain.DirectoryResolveRequest {
	return domain.DirectoryResolveRequest{
		Selector: "demo", Targets: []domain.ClientID{domain.ClientCursor},
		Scope: domain.ScopeProject, SchemaVersion: "1.0.0", InstallerVersion: "1.0.0",
	}
}

// A verified schema-1 source can be selected for explicit Project placement.
// The unchanged domain resolver must still reject Project as a publisher grant.
func TestDirectoryV1ProjectPlacementUsesSameSignedRelease(t *testing.T) {
	bundle := readModelBundle()
	request := v1ProjectSelectionRequest()
	if _, err := domain.ResolveDirectory(bundle.Snapshot, request); !errors.Is(err, domain.ErrDirectoryIneligible) {
		t.Fatalf("v1 published a Project grant: %v", err)
	}
	project, err := selectDirectorySourceForPlacement(bundle.Snapshot, request)
	if err != nil {
		t.Fatal(err)
	}
	request.Scope = domain.ScopeUser
	user, err := domain.ResolveDirectory(bundle.Snapshot, request)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(project, user) || project.ReleaseSequence != 2 || project.Source.Repository != "owner/demo" {
		t.Fatalf("Project source is not the exact signed User publication: project=%+v user=%+v", project, user)
	}
}

func TestDirectoryV1ProjectPlacementKeepsExistingLockAuthority(t *testing.T) {
	bundle := readModelBundle()
	request := v1ProjectSelectionRequest()
	request.Selector = "owner/demo"
	request.DesiredReleaseSequence = 2
	request.Purpose = domain.DirectoryResolveLockedRestore
	project, err := selectDirectorySourceForPlacement(bundle.Snapshot, request)
	if err != nil || project.ReleaseSequence != 2 {
		t.Fatalf("exact signed Project lock restore: %+v %v", project, err)
	}
	// No Project grant is introduced into the v1 signed records.
	for _, grant := range bundle.Snapshot.Distributions[0].ReleasePolicies[1].Targets {
		if len(grant.Scopes) != 1 || grant.Scopes[0] != domain.ScopeUser {
			t.Fatalf("mutated legacy signed grant: %+v", grant)
		}
	}
}

func TestDirectoryV1ProjectPlacementRejectsMissingPolicyAndRevokedRelease(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(*domain.DirectorySnapshot, *domain.DirectoryResolveRequest)
	}{
		{"unsupported native client", func(_ *domain.DirectorySnapshot, req *domain.DirectoryResolveRequest) {
			req.Targets = []domain.ClientID{domain.ClientChatGPT}
		}},
		{"revoked release", func(s *domain.DirectorySnapshot, _ *domain.DirectoryResolveRequest) {
			s.Revocations = append(s.Revocations, domain.DirectoryRevocation{DistributionID: "owner/demo", ReleaseSequence: 2})
		}},
		{"revoked publication policy", func(s *domain.DirectorySnapshot, _ *domain.DirectoryResolveRequest) {
			s.Distributions[0].ReleasePolicies[1].Status = domain.ReleaseRevoked
		}},
		{"installer below signed floor", func(s *domain.DirectorySnapshot, _ *domain.DirectoryResolveRequest) {
			s.Distributions[0].ReleasePolicies[1].MinimumInstallerVersion = "99.0.0"
		}},
		{"missing target authorization", func(s *domain.DirectorySnapshot, _ *domain.DirectoryResolveRequest) {
			s.Distributions[0].ReleasePolicies[1].Targets = nil
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			bundle := readModelBundle()
			request := v1ProjectSelectionRequest()
			request.Selector = "owner/demo" // Negative proof cannot fall back to another signed distributor.
			test.change(&bundle.Snapshot, &request)
			if _, err := selectDirectorySourceForPlacement(bundle.Snapshot, request); !errors.Is(err, domain.ErrDirectoryIneligible) {
				t.Fatalf("Project placement bypassed a signed eligibility denial: %v", err)
			}
		})
	}
}

func TestDirectoryV1ProjectPlacementNeverClaimsUserRuntimeAsProjectRuntime(t *testing.T) {
	bundle := readModelBundle()
	request := v1ProjectSelectionRequest()
	project, err := selectDirectorySourceForPlacement(bundle.Snapshot, request)
	if err != nil {
		t.Fatal(err)
	}
	policy := bundle.Snapshot.Distributions[0].ReleasePolicies[1]
	environment := directoryEvidenceEnvironment{
		InstallerVersion: request.InstallerVersion, ClientVersions: map[domain.ClientID]string{},
		DependencyIdentity: map[domain.ClientID]string{},
	}
	loaded := loadedPackage{}
	if err := applyDirectoryCompatibility(&loaded, bundle, project, policy, environment, domain.ScopeProject); err != nil {
		t.Fatal(err)
	}
	catalog := loaded.envelope.CatalogEvidence
	if catalog == nil || catalog.CatalogVersion != "directory-snapshot-1" {
		t.Fatalf("lost v1 source authority: %+v", catalog)
	}
	proof, ok := catalog.Compatibility[string(domain.ClientCursor)]
	if !ok || proof.Verification == "tested" || len(proof.Evidence) != 0 {
		t.Fatalf("reused User runtime claims for Project: %+v", proof)
	}
	for _, proof := range catalog.CurrentEvidence {
		if proof.Level != "schema" {
			t.Fatalf("exposed User-scoped proof as Project evidence: %+v", proof)
		}
	}
	if strings.Contains(catalog.CatalogVersion, "v2") {
		t.Fatalf("project install invented a Directory v2 publication: %+v", catalog)
	}
}

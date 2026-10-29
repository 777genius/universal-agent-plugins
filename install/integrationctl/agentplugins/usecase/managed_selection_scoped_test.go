package usecase

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

type legacySelectionStager struct {
	ports.PackageStager
	calls int
}

func (s *legacySelectionStager) ManagedMCPNames(context.Context, domain.ClientID, string, string) ([]string, error) {
	s.calls++
	return []string{"legacy"}, nil
}

type selectionRead struct {
	client       domain.ClientID
	scope        domain.InstallScope
	root, digest string
}

type scopedSelectionStager struct {
	*legacySelectionStager
	read  selectionRead
	calls int
	err   error
}

func (s *scopedSelectionStager) ManagedMCPNamesForScope(_ context.Context, client domain.ClientID, scope domain.InstallScope, root, digest string) ([]string, error) {
	s.calls++
	s.read = selectionRead{client, scope, root, digest}
	return []string{"scoped"}, s.err
}

func TestScopedManagedSelectionProfileGuardAndErrorNeverFallBack(t *testing.T) {
	sentinel := errors.New("scoped refusal")
	inner := &scopedSelectionStager{legacySelectionStager: &legacySelectionStager{}, err: sentinel}
	guardErr := errors.New("original profile changed")
	stager := profileStager{PackageStager: inner, check: func() error { return guardErr }}
	if _, err := stager.ManagedMCPNamesForScope(t.Context(), domain.ClientKimi, domain.ScopeProject, "/held", "held-digest"); !errors.Is(err, guardErr) || inner.calls != 0 {
		t.Fatal("profile failure did not precede the scoped read")
	}
	guardErr = nil
	if _, err := stager.ManagedMCPNamesForScope(t.Context(), domain.ClientKimi, domain.ScopeProject, "/held", "held-digest"); !errors.Is(err, sentinel) || inner.calls != 1 || inner.legacySelectionStager.calls != 0 {
		t.Fatal("scoped refusal retried the legacy reader")
	}
	if inner.read != (selectionRead{domain.ClientKimi, domain.ScopeProject, "/held", "held-digest"}) {
		t.Fatal("profile wrapper lost explicit selection authority")
	}
	if _, err := stager.ManagedMCPNamesForScope(t.Context(), domain.ClientKimi, "", "/held", "held-digest"); err == nil || inner.calls != 1 {
		t.Fatal("unknown scope reached an inner reader")
	}
}

func TestScopedManagedSelectionLegacyAndAbsentPortsRemainCompatible(t *testing.T) {
	legacy := &legacySelectionStager{}
	stager := profileStager{PackageStager: legacy, check: func() error { return nil }}
	for _, read := range []func() ([]string, error){
		func() ([]string, error) {
			return stager.ManagedMCPNames(t.Context(), domain.ClientKimi, "/held", "digest")
		},
		func() ([]string, error) {
			return stager.ManagedMCPNamesForScope(t.Context(), domain.ClientKimi, domain.ScopeUser, "/held", "digest")
		},
	} {
		names, err := read()
		if err != nil || !reflect.DeepEqual(names, []string{"legacy"}) {
			t.Fatal("legacy selection API changed", names, err)
		}
	}
	if legacy.calls != 2 {
		t.Fatal("legacy reader was not forwarded exactly once per call")
	}
	stager.PackageStager = nil
	if names, err := stager.ManagedMCPNamesForScope(t.Context(), domain.ClientKimi, domain.ScopeProject, "/held", "digest"); names != nil || err != nil {
		t.Fatal("absent optional reader gained a selection")
	}
}

type selectionTarget struct {
	path  string
	scope domain.InstallScope
	calls int
}

func (r *selectionTarget) ResolveTarget(_ context.Context, _ domain.DetectedClient, scope domain.InstallScope, _ string) (domain.DeliveryTarget, error) {
	r.calls++
	r.scope = scope
	return domain.DeliveryTarget{ActivePath: r.path}, nil
}

func TestScopedManagedSelectionRequiresHeldScopeTargetAndDigest(t *testing.T) {
	root := t.TempDir()
	reader := &scopedSelectionStager{legacySelectionStager: &legacySelectionStager{}}
	target := &selectionTarget{path: root}
	service := Service{Stager: reader, Targets: target, Paths: pathpolicy.Policy{}}
	prior := &domain.ClientBinding{ClientID: "kimi", Scope: "project", TargetLocator: root, PhysicalArtifact: "held-physical", NativeObjects: []domain.NativeObjectOwnership{{Kind: "managed_package_directory", ManagedDigest: "held-digest"}}}
	input := AddInput{Scope: domain.ScopeProject, Client: domain.DetectedClient{ClientID: domain.ClientKimi}}
	names, unverified, err := service.recordedManagedMCPNames(t.Context(), input, domain.ScopeProject, prior, true, false)
	if err != nil || unverified || !reflect.DeepEqual(names, []string{"scoped"}) || reader.read != (selectionRead{domain.ClientKimi, domain.ScopeProject, root, "held-digest"}) || target.scope != domain.ScopeProject {
		t.Fatal("selected scope/locator/digest did not reach the scoped reader", names, err)
	}
	for _, scope := range []domain.InstallScope{"", "unknown", domain.ScopeUser} {
		if _, _, err := service.recordedManagedMCPNames(t.Context(), input, scope, prior, true, false); err == nil || reader.calls != 1 || target.calls != 1 {
			t.Fatal("wrong held scope reached target resolution or artifact read")
		}
	}
	target.path = filepath.Join(root, "different-target")
	if _, _, err := service.recordedManagedMCPNames(t.Context(), input, domain.ScopeProject, prior, true, false); err == nil || reader.calls != 1 {
		t.Fatal("different target locator was trusted")
	}
	input.Scope = domain.ScopeUser
	if _, _, err := service.recordedManagedMCPNames(t.Context(), input, domain.ScopeProject, prior, true, false); err == nil || reader.calls != 1 {
		t.Fatal("contradictory input scope was trusted")
	}
}

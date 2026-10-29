package providers_test

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/planner"
)

func TestProjectKimiActualStagedSelectionKeepsProjectAndUserLayouts(t *testing.T) {
	f := newKimiProjectFixture(t, false, false)
	delivery := f.request.Delivery
	names, err := f.stager.ManagedMCPNamesForScope(t.Context(), domain.ClientKimi, domain.ScopeProject, delivery.ActivePath, delivery.ArtifactDigest)
	if err != nil || !reflect.DeepEqual(names, []string{"docs"}) {
		t.Fatal("actual project delivery lost recorded MCP selection", names, err)
	}
	p := planner.Planner{Registry: f.registry, Paths: pathpolicy.Policy{}, ManagedRoot: filepath.Join(f.base, "user-managed")}
	plan, err := p.Plan(t.Context(), domain.PlanRequest{Envelope: f.envelope, Client: f.request.Client, Scope: domain.ScopeUser, PhysicalArtifactID: "user-held-0123456789ab"})
	if err != nil || plan.Status != domain.PlanReady {
		t.Fatal("actual user plan failed", err)
	}
	delivery, err = f.stager.Stage(t.Context(), f.envelope, plan, "user-selection", domain.CompatibilityHints{})
	if err != nil {
		t.Fatal(err)
	}
	for _, read := range []func() ([]string, error){
		func() ([]string, error) {
			return f.stager.ManagedMCPNames(t.Context(), domain.ClientKimi, delivery.StagingPath, delivery.ArtifactDigest)
		},
		func() ([]string, error) {
			return f.stager.ManagedMCPNamesForScope(t.Context(), domain.ClientKimi, domain.ScopeUser, delivery.StagingPath, delivery.ArtifactDigest)
		},
	} {
		names, err := read()
		if err != nil || !reflect.DeepEqual(names, []string{"docs"}) {
			t.Fatal("actual user delivery or legacy API changed", names, err)
		}
	}
}

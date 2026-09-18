package agentpluginscli

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// TestPlanningFailsClosedWithoutAClientRegistry pins what only CI caught the
// first time: App.ClientRegistry is a runtime invariant, so an App assembled
// without one has to refuse to plan and say why, rather than panic, dereference
// nil, or quietly plan against no clients at all.
//
// It covers every place the CLI builds a planner from the App, because each one
// is a separate opportunity to forget the field.
func TestPlanningFailsClosedWithoutAClientRegistry(t *testing.T) {
	t.Parallel()
	fixture := newCLIFixture(t, []domain.DetectedClient{fixtureClient(t, domain.ClientCursor)})
	fixture.app.ClientRegistry = nil
	plugin := writeCLIPlugin(t)

	// add reaches the planner through both lifecycleService and
	// compatibleLoadedTargets, and is the command the Authoring regression hit.
	_, _, err := fixture.execute(false, "add", plugin, "--target", "cursor", "--dry-run", "--format", "json")
	if err == nil {
		t.Fatal("add planned without a client registry")
	}
	if !strings.Contains(err.Error(), "client registry is required") {
		t.Fatalf("add error does not name the missing registry: %v", err)
	}

	// doctor resolves a recorded managed target through its own planner. A
	// caller gets a degraded finding rather than a crash.
	binding := domain.ClientBinding{
		ClientID: string(domain.ClientCursor), Scope: string(domain.ScopeUser),
		Materialization:  domain.MaterializationMaterialized,
		TargetLocator:    filepath.Join(fixture.app.ManagedRoot, "clients", "cursor", "demo-0123456789ab"),
		PhysicalArtifact: "demo-0123456789ab",
	}
	findings := checkManagedIntegrity(context.Background(), fixture.app, fixtureClient(t, domain.ClientCursor), domain.Installation{}, binding)
	if len(findings) != 1 || findings[0].Code != "managed_target_mismatch" {
		t.Fatalf("doctor findings without a client registry = %+v", findings)
	}
}

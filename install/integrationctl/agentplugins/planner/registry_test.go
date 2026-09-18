package planner

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// TestPlanningRequiresAClientRegistry pins the injection rule: a missing
// registry is an error at every planner entry point, never a silent fallback to
// "every client this binary happens to link".
func TestPlanningRequiresAClientRegistry(t *testing.T) {
	t.Parallel()
	bare := Planner{ManagedRoot: t.TempDir(), Paths: pathpolicy.Policy{}}
	request := domain.PlanRequest{
		Envelope:           testEnvelope(),
		Client:             detectedClient(domain.ClientCursor, filepath.Join(t.TempDir(), ".cursor")),
		Scope:              domain.ScopeUser,
		PhysicalArtifactID: "demo-0123456789ab",
	}
	if _, err := bare.Plan(context.Background(), request); !errors.Is(err, clients.ErrRegistryRequired) {
		t.Fatalf("Plan without a registry = %v, want ErrRegistryRequired", err)
	}
	if _, err := bare.ResolveTarget(context.Background(), request.Client, request.Scope, request.PhysicalArtifactID); !errors.Is(err, clients.ErrRegistryRequired) {
		t.Fatalf("ResolveTarget without a registry = %v, want ErrRegistryRequired", err)
	}
	if _, err := Compatibility(nil, request.Envelope, domain.SupportedClientIDs()); !errors.Is(err, clients.ErrRegistryRequired) {
		t.Fatalf("Compatibility without a registry = %v, want ErrRegistryRequired", err)
	}
	plan := domain.DeliveryPlan{ClientID: domain.ClientKiro, Scope: domain.ScopeUser}
	if err := ApplyInstallIntent(nil, &plan, domain.InstallIntentPrepare); !errors.Is(err, clients.ErrRegistryRequired) {
		t.Fatalf("ApplyInstallIntent without a registry = %v, want ErrRegistryRequired", err)
	}
}

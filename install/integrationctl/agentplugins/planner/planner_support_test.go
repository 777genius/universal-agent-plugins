package planner

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// completedPlanner keeps the positional call shape the in-package tests were
// written against. The tests outside this package use plannertest.NewPlanner;
// an import of that package from here would be a cycle.
type completedPlanner struct{ Planner }

func (planner completedPlanner) Plan(
	ctx context.Context,
	envelope domain.PackageEnvelope,
	client domain.DetectedClient,
	scope domain.InstallScope,
	physicalArtifactID string,
) (domain.DeliveryPlan, error) {
	return planner.Planner.Plan(ctx, domain.PlanRequest{
		Envelope: envelope, Client: client, Scope: scope, PhysicalArtifactID: physicalArtifactID,
	})
}

// testPlanner completes a planner literal with the production path policy.
func testPlanner(base Planner) completedPlanner {
	if base.Paths == nil {
		base.Paths = pathpolicy.Policy{}
	}
	return completedPlanner{base}
}

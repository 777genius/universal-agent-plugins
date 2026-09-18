package clients

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// TargetLayout overrides where a client's managed package lives. Clients that
// install under the managed root do not implement it; the planner falls back to
// shared.ManagedTargetRoot.
type TargetLayout interface {
	TargetRoot(client domain.DetectedClient, mode domain.PackageMode, managedRoot string) (anchor, root string, err error)
}

// PlanRefiner applies client-specific decisions on top of a generic plan:
// readiness promotions, user actions, warnings and install-intent handling.
//
// A refiner may not change the plan identity (ClientID, Scope,
// PhysicalArtifactID, TargetRoot, ActivePath) and may not promote a plan the
// generic pipeline already marked unsupported.
type PlanRefiner interface {
	RefinePlan(ctx context.Context, in PlanInput, plan *domain.DeliveryPlan) error
}

// PlanInput is everything a refiner is allowed to see.
type PlanInput struct {
	Envelope domain.PackageEnvelope
	Client   domain.DetectedClient
	Detected map[domain.ClientID]domain.DetectedClient
	Intent   domain.InstallIntent
}

// BackendSibling returns the entry of the other client sharing this client's
// backend family, for example Copilot behind VS Code. The entry comes back
// exactly as the detection map holds it, without a DetectionStatus filter: the
// decisions built on it read ExecutablePath, not status.
func (in PlanInput) BackendSibling() (domain.DetectedClient, bool) {
	definition, ok := domain.ClientDefinitionFor(in.Client.ClientID)
	if !ok {
		return domain.DetectedClient{}, false
	}
	for _, candidate := range domain.ClientDefinitions() {
		if candidate.ID == in.Client.ClientID || candidate.BackendFamily != definition.BackendFamily {
			continue
		}
		if sibling, present := in.Detected[candidate.ID]; present {
			return sibling, true
		}
	}
	return domain.DetectedClient{}, false
}

// CompatibilityLimiter reports the client-specific limitations shown by the
// read-only compatibility view.
type CompatibilityLimiter interface {
	ClientLimitations(envelope domain.PackageEnvelope) []string
	ComponentLimitations(envelope domain.PackageEnvelope, item domain.ComponentDecision) (reject []string, note []string)
}

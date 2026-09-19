package clients

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// The planner is a fixed generic pipeline with a few named places where a
// client may speak. Each hook below names the stage it runs in, because the
// order of the warnings and user actions a plan ends up carrying is part of the
// observable contract:
//
//  1. NativeRegistryLayout      - which native registry the plan writes into
//  2. PlanPrecondition          - reject before a target is resolved
//  3. TargetLayout              - where the package lands
//  4. LocalPreparationAuthorizer - a personal receipt instead of catalog evidence
//  5. PlanQualifier             - reject on the client's own component rules
//  6. PlanRefiner               - readiness promotions, user actions, warnings
//  7. PreparationRefiner        - the prepare install intent
//
// A client implements only the stages it has something to say in; the planner
// asks for each one with As[T] and falls back to the generic behavior.

// TargetLayout overrides where a client's managed package lives. Clients that
// install under the managed root do not implement it; the planner falls back to
// shared.ManagedTargetRoot.
type TargetLayout interface {
	TargetRoot(client domain.DetectedClient, mode domain.PackageMode, managedRoot string) (anchor, root string, err error)
}

// NativeRegistryLayout overrides which native client registry a delivery is
// recorded against. VS Code is installed through the Copilot CLI it shares a
// backend with, so its plan points at the sibling's locators rather than its
// own. The default is the client's own configuration root and executable.
type NativeRegistryLayout interface {
	NativeRegistry(in PlanInput) (root, executable string)
}

// PlanPrecondition rejects a package the client cannot take at all, before a
// target is resolved for it. An implementation marks the plan unsupported and
// records why; the planner returns the plan as it stands, so the rejection
// never carries resolved paths. Returning an error aborts planning instead.
type PlanPrecondition interface {
	CheckPlanPrecondition(in PlanInput, plan *domain.DeliveryPlan) error
}

// LocalPreparationAuthorizer lets a client accept a personal, user-supplied
// registration receipt in place of the pinned catalog compatibility evidence
// the generic pipeline otherwise requires. Reporting true means the client took
// responsibility for the decision and the catalog step is skipped.
type LocalPreparationAuthorizer interface {
	AuthorizeLocalPreparation(in PlanInput, plan *domain.DeliveryPlan) (authorized bool, err error)
}

// PlanQualifier applies the client's own admission rules once the components,
// the catalog verdict and the package diagnostics are on the plan, and before
// the generic pipeline decides that nothing usable is left. A client that can
// reject a package on its own terms does it here, so its warning keeps its
// place ahead of the generic ones.
type PlanQualifier interface {
	QualifyPlan(in PlanInput, plan *domain.DeliveryPlan) error
}

// PlanRefiner applies client-specific decisions on top of a viable generic
// plan: readiness promotions, user actions and warnings. It runs last, on a
// plan the generic pipeline has already accepted.
//
// A refiner may not change the plan identity (ClientID, Scope,
// PhysicalArtifactID, TargetRoot, ActivePath) and may not promote a plan the
// generic pipeline already marked unsupported.
type PlanRefiner interface {
	RefinePlan(ctx context.Context, in PlanInput, plan *domain.DeliveryPlan) error
}

// PreparationRefiner turns a viable plan into a prepared one for the clients
// that support the prepare install intent. It is a separate stage from
// RefinePlan because the intent is also applied on its own, to a plan that was
// already built, when a caller changes its mind about a target.
type PreparationRefiner interface {
	RefinePreparation(plan *domain.DeliveryPlan) error
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
	for _, id := range domain.BackendSiblings(in.Client.ClientID) {
		if sibling, present := in.Detected[id]; present {
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

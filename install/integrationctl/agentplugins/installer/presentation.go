package installer

import (
	"slices"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func deliveryPlan(plan domain.DeliveryPlan) DeliveryPlan {
	out := DeliveryPlan{
		ActivePath: plan.ActivePath,
		Status:     string(plan.Status), PackageMode: string(plan.PackageMode), InstallIntent: string(plan.InstallIntent),
		PhysicalArtifactID: plan.PhysicalArtifactID, Activation: string(plan.Activation),
		Authentication: string(plan.Authentication), Policy: string(plan.Policy), Verification: string(plan.Verification),
		UserActions: slices.Clone(plan.UserActions), LocalActions: slices.Clone(plan.LocalActions), Warnings: slices.Clone(plan.Warnings),
	}
	for _, component := range plan.Components {
		out.Components = append(out.Components, ComponentDecision{Kind: string(component.Kind), Name: component.Name, Support: string(component.Support), Reason: component.Reason})
	}
	for _, diagnostic := range plan.Diagnostics {
		out.Diagnostics = append(out.Diagnostics, PlanDiagnostic{Severity: string(diagnostic.Severity), Boundary: string(diagnostic.Boundary), Code: diagnostic.Code, Path: diagnostic.Path, Item: diagnostic.Item, Message: diagnostic.Message})
	}
	return out
}

func cloneDeliveryPlan(plan DeliveryPlan) DeliveryPlan {
	plan.Components = slices.Clone(plan.Components)
	plan.UserActions = slices.Clone(plan.UserActions)
	plan.LocalActions = slices.Clone(plan.LocalActions)
	plan.Warnings = slices.Clone(plan.Warnings)
	plan.Diagnostics = slices.Clone(plan.Diagnostics)
	return plan
}

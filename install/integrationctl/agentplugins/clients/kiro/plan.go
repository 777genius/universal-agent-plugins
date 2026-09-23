package kiro

import (
	"context"
	"fmt"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// PrepareAction is what a user still has to do after a prepared Kiro delivery.
// Runtime connections are explicitly not claimed as verified.
const PrepareAction = "After preparation, open or restart Kiro, review the MCP servers and complete authentication in Kiro on first connection when prompted, then verify their tools in Kiro. Runtime connections have not been verified."

var (
	_ clients.PlanRefiner        = (*Adapter)(nil)
	_ clients.PreparationRefiner = (*Adapter)(nil)
)

func (*Adapter) RefinePlan(_ context.Context, in clients.PlanInput, plan *domain.DeliveryPlan) error {
	eligible := shared.OnlyNativeComponents(plan.Components)
	needsCLI := shared.HasSupportedMCP(plan.Components)
	hasConfigRoot := strings.TrimSpace(in.Client.ConfigRoot) != ""
	canActivate := hasConfigRoot && eligible && (!needsCLI || isKiroCLI(in.Client.ExecutablePath))
	shared.PromoteNativeReady(plan, in.Client.ConfigRoot, canActivate)
	if !hasConfigRoot {
		plan.UserActions = shared.AppendUnique(plan.UserActions, "rerun add with a detected writable Kiro configuration root")
	} else if needsCLI && !isKiroCLI(in.Client.ExecutablePath) {
		plan.UserActions = shared.AppendUnique(plan.UserActions, "install a current Kiro CLI and rerun add to register and verify its MCP servers")
	} else if canActivate {
		plan.UserActions = shared.AppendUnique(plan.UserActions, "agentplugins will install and verify the package's global Kiro skills and MCP servers automatically")
	}
	return nil
}

// RefinePreparation hands the user a configuration Kiro can load instead of
// mutating it, which needs a native registry root and a selection Kiro delivers
// whole.
func (*Adapter) RefinePreparation(plan *domain.DeliveryPlan) error {
	if strings.TrimSpace(plan.NativeRegistryRoot) == "" || !shared.OnlyNativeComponents(plan.Components) {
		return fmt.Errorf("a Kiro preparation requires a native config root and supported skills or MCP servers")
	}
	plan.Status = domain.PlanReady
	plan.Activation = domain.ActivationPrepared
	plan.Verification = domain.VerificationPackageValid
	plan.UserActions = []string{PrepareAction}
	return nil
}

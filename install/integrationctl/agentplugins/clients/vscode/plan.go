package vscode

import (
	"context"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var (
	_ clients.NativeRegistryLayout = (*Adapter)(nil)
	_ clients.PlanRefiner          = (*Adapter)(nil)
)

// NativeRegistry points at the Copilot installation VS Code discovers: the
// package is registered through the CLI of the sibling this client shares the
// github-copilot backend with, so the plan records that registry, not its own.
func (*Adapter) NativeRegistry(in clients.PlanInput) (string, string) {
	sibling, _ := in.BackendSibling()
	return sibling.ConfigRoot, sibling.ExecutablePath
}

// RefinePlan promotes the plan when the sibling's Copilot CLI is present, which
// is what turns a prepared VS Code package into an installed one.
func (*Adapter) RefinePlan(_ context.Context, in clients.PlanInput, plan *domain.DeliveryPlan) error {
	sibling, _ := in.BackendSibling()
	shared.PromoteBackendReady(plan, sibling.ExecutablePath)
	action := "register the prepared local plugin in VS Code after installation"
	if strings.TrimSpace(sibling.ExecutablePath) != "" {
		action = "agentplugins will install through GitHub Copilot CLI; VS Code discovers it automatically"
	}
	plan.UserActions = shared.AppendUnique(plan.UserActions, action)
	return nil
}

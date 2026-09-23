package grok

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var (
	_ clients.TargetLayout = (*Adapter)(nil)
	_ clients.PlanRefiner  = (*Adapter)(nil)
)

func (*Adapter) TargetRoot(client domain.DetectedClient, mode domain.PackageMode, managedRoot string) (string, string, error) {
	if mode != domain.PackageNative {
		return shared.ManagedTargetRoot(client, mode, managedRoot)
	}
	if strings.TrimSpace(client.ConfigRoot) == "" {
		return "", "", fmt.Errorf("grok config root is unavailable")
	}
	return client.ConfigRoot, filepath.Join(client.ConfigRoot, "plugins"), nil
}

func (*Adapter) RefinePlan(_ context.Context, in clients.PlanInput, plan *domain.DeliveryPlan) error {
	shared.PromoteNativeReady(plan, in.Client.ConfigRoot, shared.OnlyNativeComponents(plan.Components))
	if plan.Status == domain.PlanUnsupported {
		return nil
	}
	if in.Client.ExecutablePath == "" {
		plan.UserActions = shared.AppendUnique(plan.UserActions, "install Grok Build CLI to verify and manage this native plugin, then rerun agentplugins update")
	} else {
		plan.UserActions = shared.AppendUnique(plan.UserActions, "agentplugins will register and verify the plugin through Grok Build CLI; reload Grok Build to use it")
	}
	return nil
}

package claude

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
	_ clients.TargetLayout     = (*Adapter)(nil)
	_ clients.PlanPrecondition = (*Adapter)(nil)
	_ clients.PlanRefiner      = (*Adapter)(nil)
)

// TargetRoot delivers the portable projection into Claude Code's official
// in-place @skills-dir plugin slot (full plugin.json + skills + MCP), not
// under the managed marketplace root.
func (*Adapter) TargetRoot(client domain.DetectedClient, mode domain.PackageMode, managedRoot string) (string, string, error) {
	if mode != domain.PackageProjection {
		return shared.ManagedTargetRoot(client, mode, managedRoot)
	}
	if strings.TrimSpace(client.ConfigRoot) == "" {
		return "", "", fmt.Errorf("the Claude Code config root is unavailable")
	}
	return client.ConfigRoot, filepath.Join(client.ConfigRoot, "skills"), nil
}

// CheckPlanPrecondition refuses to plan without the CLI: the exact @skills-dir
// identity of a delivered package can only be confirmed through it.
func (*Adapter) CheckPlanPrecondition(in clients.PlanInput, plan *domain.DeliveryPlan) error {
	if strings.TrimSpace(in.Client.ExecutablePath) != "" {
		return nil
	}
	plan.Status = domain.PlanUnsupported
	plan.Activation = domain.ActivationFailed
	plan.Warnings = shared.AppendUnique(plan.Warnings, "trusted_claude_cli_required")
	plan.UserActions = shared.AppendUnique(plan.UserActions, "install Claude Code CLI so agentplugins can verify the exact @skills-dir identity")
	return nil
}

func (*Adapter) RefinePlan(_ context.Context, _ clients.PlanInput, plan *domain.DeliveryPlan) error {
	plan.UserActions = shared.AppendUnique(plan.UserActions, "start a new Claude Code session or run /reload-plugins")
	return nil
}

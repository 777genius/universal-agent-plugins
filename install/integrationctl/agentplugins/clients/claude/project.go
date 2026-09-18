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
	_ clients.Projector       = (*Adapter)(nil)
	_ clients.StagingLayout   = (*Adapter)(nil)
	_ clients.SelectionReader = (*Adapter)(nil)
)

// StagingBase keeps the transaction staging directory beside Claude Code's
// watched skills root so a pre-commit `plugin list` cannot mistake it for an
// installed plugin. Discard may only have TargetRoot, which is the skills
// directory, so the parent of that root is the same location.
func (*Adapter) StagingBase(plan domain.DeliveryPlan) string {
	if strings.TrimSpace(plan.TargetAnchor) != "" {
		return plan.TargetAnchor
	}
	return filepath.Dir(plan.TargetRoot)
}

// ValidateTargetLayout requires the planned target root to be the exact
// configured skills directory under the Claude config root.
func (*Adapter) ValidateTargetLayout(plan domain.DeliveryPlan) error {
	if filepath.Clean(plan.TargetRoot) != filepath.Join(filepath.Clean(plan.TargetAnchor), "skills") {
		return fmt.Errorf("the Claude Code delivery target root must be the exact configured skills directory")
	}
	return nil
}

// ManagedMCPSelection reports Claude Code's official MCP document: a
// `.mcp.json` whose members are the servers themselves, not nested under
// mcpServers.
func (*Adapter) ManagedMCPSelection() clients.SelectionLayout {
	return clients.SelectionLayout{File: ".mcp.json"}
}

// Project renders the portable envelope into Claude Code's skills-directory
// layout. Native-mode packages keep the sanitized snapshot as-is.
func (*Adapter) Project(_ context.Context, in clients.ProjectionInput) ([]domain.NativeObjectOwnership, error) {
	if in.Plan.PackageMode != domain.PackageProjection {
		return nil, nil
	}
	if err := shared.DeliverManagedStdio(in.DeliverLauncher(), in.StagingPath, in.Envelope, in.Plan); err != nil {
		return nil, err
	}
	if err := ProjectClaude(in.StagingPath, in.Envelope, in.Plan, in.PluginDataPath); err != nil {
		return nil, err
	}
	return nil, nil
}

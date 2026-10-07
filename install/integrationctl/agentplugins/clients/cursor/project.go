package cursor

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var _ clients.Projector = (*Adapter)(nil)
var _ clients.ActiveNativeProjector = (*Adapter)(nil)

// ProjectActiveNative validates the verified package's requested projection.
// Cursor's package layout owns no profile entries. The lifecycle retains any
// separately acknowledged Stop receipt; a planned receipt is not ownership.
func (*Adapter) ProjectActiveNative(ctx context.Context, root string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan, _ string) ([]domain.NativeObjectOwnership, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if plan.ClientID != domain.ClientCursor || root == "" || root != plan.ActivePath {
		return nil, fmt.Errorf("cursor active projection client or root differs")
	}
	if err := plan.SelectedDelivery.ValidatePlan(plan, envelope.TreeDigest); err != nil {
		return nil, err
	}
	return nil, nil
}

// Project writes Cursor's compatibility manifest and MCP document.
func (*Adapter) Project(ctx context.Context, in clients.ProjectionInput) ([]domain.NativeObjectOwnership, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := projectCursor(in.StagingPath, in.Envelope, in.Plan, in.PluginDataPath); err != nil {
		return nil, err
	}
	return nil, nil
}

func projectCursor(root string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan, dataPath string) error {
	manifest := shared.ManifestFromEnvelope(envelope, shared.WithAuthorNameEmail())
	if shared.ComponentKindPresent(plan.Components, domain.ComponentSkill) {
		manifest["skills"] = "./skills/"
	}
	serverNames := shared.SupportedMCPNames(plan)
	if len(serverNames) > 0 {
		manifest["mcpServers"] = "./mcp.json"
	}
	if err := shared.WriteJSON(filepath.Join(root, ".cursor-plugin", "plugin.json"), manifest); err != nil {
		return fmt.Errorf("write Cursor plugin manifest: %w", err)
	}
	return shared.ProjectMCPServers(shared.MCPProjection{
		Root:       root,
		Envelope:   envelope,
		Names:      serverNames,
		Dialect:    shared.MCPDialectCursor,
		PluginRoot: plan.ActivePath,
		DataPath:   dataPath,
	})
}

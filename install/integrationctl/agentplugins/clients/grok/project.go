package grok

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var _ clients.Projector = (*Adapter)(nil)

func (*Adapter) Project(_ context.Context, in clients.ProjectionInput) ([]domain.NativeObjectOwnership, error) {
	manifest := shared.ManifestFromEnvelope(in.Envelope, shared.WithAuthorNameEmail())
	if shared.ComponentKindPresent(in.Plan.Components, domain.ComponentSkill) {
		manifest["skills"] = "./skills/"
	}
	names := shared.SupportedMCPNames(in.Plan)
	if len(names) > 0 {
		manifest["mcpServers"] = "./mcp.json"
	}
	if err := shared.WriteJSON(filepath.Join(in.StagingPath, ".grok-plugin", "plugin.json"), manifest); err != nil {
		return nil, fmt.Errorf("write Grok plugin manifest: %w", err)
	}
	if err := shared.ProjectMCPServers(shared.MCPProjection{
		Root: in.StagingPath, Envelope: in.Envelope, Names: names,
		Dialect: shared.MCPDialectCursor, PluginRoot: in.Plan.ActivePath, DataPath: in.PluginDataPath,
	}); err != nil {
		return nil, err
	}
	return nil, nil
}

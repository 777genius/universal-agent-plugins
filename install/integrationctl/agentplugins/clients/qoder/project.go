package qoder

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var (
	_ clients.Projector       = (*Adapter)(nil)
	_ clients.SelectionReader = (*Adapter)(nil)
)

// ManagedMCPSelection reports Qoder's OpenAI-shaped `.mcp.json`.
func (*Adapter) ManagedMCPSelection() clients.SelectionLayout {
	return clients.SelectionLayout{File: ".mcp.json", Nested: true}
}

func (*Adapter) Project(_ context.Context, in clients.ProjectionInput) ([]domain.NativeObjectOwnership, error) {
	manifest := shared.ManifestFromEnvelope(in.Envelope, shared.WithAuthorNameEmail())
	if shared.ComponentKindPresent(in.Plan.Components, domain.ComponentSkill) {
		manifest["skills"] = "./skills/"
	}
	names := shared.SupportedMCPNames(in.Plan)
	if len(names) > 0 {
		manifest["mcpServers"] = "./.mcp.json"
	}
	if err := shared.WriteJSON(filepath.Join(in.StagingPath, ".qoder-plugin", "plugin.json"), manifest); err != nil {
		return nil, fmt.Errorf("write Qoder plugin manifest: %w", err)
	}
	// Qoder's OpenAI-shaped .mcp.json carries the catalog OpenAI auth hints,
	// but a declared generic compatibility entry for qoder makes those hints
	// non-authoritative (the usecase openAIOAuthApplies gate): they are
	// suppressed instead of projected.
	hints := in.Hints
	if _, declared := hints.Compatibility[string(domain.ClientQoder)]; declared {
		hints.OpenAIMCPAuth = nil
	}
	if err := shared.ProjectMCPServers(shared.MCPProjection{
		Root: in.StagingPath, Envelope: in.Envelope, Names: names,
		Dialect: shared.MCPDialectOpenAI, PluginRoot: in.Plan.ActivePath, DataPath: in.PluginDataPath,
		Hints: hints,
	}); err != nil {
		return nil, err
	}
	return nil, nil
}

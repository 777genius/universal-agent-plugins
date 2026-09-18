package cursor

import (
	"fmt"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func Project(root string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan, dataPath string) error {
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

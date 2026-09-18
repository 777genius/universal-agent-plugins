package codex

import (
	"fmt"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func Project(root string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan, hints domain.CompatibilityHints, dataPath string) error {
	manifest, err := shared.ProjectedOpenAIManifest(envelope)
	if err != nil {
		return err
	}
	// A preserved upstream manifest may already declare these members against a
	// layout this projection does not produce, so they are dropped and then
	// re-declared from what the plan actually selected.
	delete(manifest, "apps")
	delete(manifest, "skills")
	delete(manifest, "mcpServers")
	shared.ApplyManifestMetadata(manifest, envelope, shared.WithAuthorObject())
	if shared.ComponentKindPresent(plan.Components, domain.ComponentSkill) {
		manifest["skills"] = "./skills/"
	}
	serverNames := shared.SupportedMCPNames(plan)
	if len(serverNames) > 0 {
		manifest["mcpServers"] = "./.mcp.json"
	}
	manifestPath := filepath.Join(root, ".codex-plugin", "plugin.json")
	if err := shared.WriteJSON(manifestPath, manifest); err != nil {
		return fmt.Errorf("write OpenAI compatibility manifest: %w", err)
	}
	return ProjectMCP(root, envelope, serverNames, hints, plan.ActivePath, dataPath)
}

func ProjectMCP(root string, envelope domain.PackageEnvelope, serverNames []string, hints domain.CompatibilityHints, pluginRoot, dataPath string) error {
	return shared.ProjectMCPServers(shared.MCPProjection{
		Root:       root,
		Envelope:   envelope,
		Names:      serverNames,
		Dialect:    shared.MCPDialectOpenAI,
		PluginRoot: pluginRoot,
		DataPath:   dataPath,
		Hints:      hints,
	})
}

func ProjectMarketplace(root string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan) error {
	document := map[string]any{
		"name": shared.ManagedMarketplaceName(plan.PhysicalArtifactID),
		"plugins": []map[string]any{{
			"name": envelope.Manifest.Name,
			"source": map[string]any{
				"source": "local",
				"path":   "./",
			},
			"policy": map[string]any{
				"installation":   "AVAILABLE",
				"authentication": "ON_INSTALL",
			},
			"category": "Productivity",
		}},
	}
	if err := shared.WriteJSON(filepath.Join(root, ".agents", "plugins", "marketplace.json"), document); err != nil {
		return fmt.Errorf("write Codex managed marketplace: %w", err)
	}
	return nil
}

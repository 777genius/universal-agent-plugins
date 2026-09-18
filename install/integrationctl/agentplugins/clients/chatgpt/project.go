package chatgpt

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func Project(root string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan, hints domain.CompatibilityHints, dataPath string) error {
	manifest, err := shared.ProjectedOpenAIManifest(envelope)
	if err != nil {
		return err
	}
	serverNames := shared.SupportedMCPNames(plan)
	if len(serverNames) > 0 {
		manifest["mcpServers"] = "./.mcp.json"
	} else {
		delete(manifest, "mcpServers")
	}
	if shared.ComponentKindPresent(plan.Components, domain.ComponentSkill) {
		manifest["skills"] = "./skills/"
	} else {
		delete(manifest, "skills")
	}
	if envelope.App.Enabled && shared.ComponentKindPresent(plan.Components, domain.ComponentApp) {
		manifest["apps"] = "./.app.json"
	} else {
		delete(manifest, "apps")
	}
	if err := shared.WriteJSON(filepath.Join(root, ".codex-plugin", "plugin.json"), manifest); err != nil {
		return fmt.Errorf("write ChatGPT plugin manifest: %w", err)
	}
	if err := shared.ProjectMCPServers(shared.MCPProjection{
		Root:       root,
		Envelope:   envelope,
		Names:      serverNames,
		Dialect:    shared.MCPDialectOpenAI,
		PluginRoot: plan.ActivePath,
		DataPath:   dataPath,
		Hints:      hints,
	}); err != nil {
		return err
	}
	for _, portableManifest := range []string{"plugin.json", "mcp.json"} {
		if err := os.Remove(filepath.Join(root, portableManifest)); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove portable %s from official ChatGPT projection: %w", portableManifest, err)
		}
	}
	return nil
}

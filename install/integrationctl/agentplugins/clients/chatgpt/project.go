package chatgpt

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/atomicfile"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var (
	_ clients.Projector       = (*Adapter)(nil)
	_ clients.SelectionReader = (*Adapter)(nil)
)

// ManagedMCPSelection reports ChatGPT's OpenAI-shaped `.mcp.json`.
func (*Adapter) ManagedMCPSelection() clients.SelectionLayout {
	return clients.SelectionLayout{File: ".mcp.json", Nested: true}
}

// Project writes `.app.json` itself after the generic sanitizer has removed
// it, then renders the official ChatGPT projection and managed marketplace.
func (*Adapter) Project(_ context.Context, in clients.ProjectionInput) ([]domain.NativeObjectOwnership, error) {
	if err := writeAppJSON(in.StagingPath, in.Envelope); err != nil {
		return nil, err
	}
	if in.Plan.PackageMode != domain.PackageProjection {
		return nil, nil
	}
	if err := projectChatGPT(in.StagingPath, in.Envelope, in.Plan, in.Hints, in.PluginDataPath); err != nil {
		return nil, err
	}
	if err := shared.ProjectCodexMarketplace(in.StagingPath, in.Envelope, in.Plan); err != nil {
		return nil, err
	}
	return nil, nil
}

func writeAppJSON(root string, envelope domain.PackageEnvelope) error {
	path := filepath.Join(root, ".app.json")
	if !envelope.App.Enabled || len(envelope.App.Raw) == 0 {
		return nil
	}
	mode := os.FileMode(0o644)
	if envelope.LocalChatGPTMapping != nil {
		mode = 0o600
	}
	return atomicfile.Write(path, append([]byte(nil), envelope.App.Raw...), mode)
}

func projectChatGPT(root string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan, hints domain.CompatibilityHints, dataPath string) error {
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
	if err := shared.ProjectOpenAIMCP(root, envelope, serverNames, hints, plan.ActivePath, dataPath); err != nil {
		return err
	}
	for _, portableManifest := range []string{"plugin.json", "mcp.json"} {
		if err := os.Remove(filepath.Join(root, portableManifest)); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove portable %s from official ChatGPT projection: %w", portableManifest, err)
		}
	}
	return nil
}

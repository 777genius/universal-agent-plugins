package codex

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

// ManagedMCPSelection reports Codex's OpenAI-shaped `.mcp.json`.
func (*Adapter) ManagedMCPSelection() clients.SelectionLayout {
	return clients.SelectionLayout{File: ".mcp.json", Nested: true}
}

// Project renders the OpenAI compatibility layout and the managed marketplace.
func (*Adapter) Project(_ context.Context, in clients.ProjectionInput) ([]domain.NativeObjectOwnership, error) {
	if in.Plan.PackageMode != domain.PackageProjection {
		return nil, nil
	}
	if err := projectOpenAI(in.StagingPath, in.Envelope, in.Plan, in.Hints, in.PluginDataPath); err != nil {
		return nil, err
	}
	if err := shared.ProjectCodexMarketplace(in.StagingPath, in.Envelope, in.Plan); err != nil {
		return nil, err
	}
	return nil, nil
}

func projectOpenAI(root string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan, hints domain.CompatibilityHints, dataPath string) error {
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
	return shared.ProjectOpenAIMCP(root, envelope, serverNames, hints, plan.ActivePath, dataPath)
}

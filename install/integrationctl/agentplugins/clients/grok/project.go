package grok

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var _ clients.Projector = (*Adapter)(nil)

func (*Adapter) Project(_ context.Context, in clients.ProjectionInput) ([]domain.NativeObjectOwnership, error) {
	project := in.Plan.Scope == domain.ScopeProject || in.Plan.TargetContext.Scope == domain.ScopeProject || in.Plan.TargetContext.ProjectRoot != ""
	if project {
		if _, err := grokPlanRoot(domain.DetectedClient{ClientID: domain.ClientGrok, ConfigRoot: in.Plan.TargetContext.UserConfigRoot}, in.Plan); err != nil {
			return nil, err
		}
		if !filepath.IsAbs(in.StagingPath) || filepath.Clean(in.StagingPath) != in.StagingPath || filepath.Dir(in.StagingPath) != in.Plan.TargetAnchor || !strings.HasPrefix(filepath.Base(in.StagingPath), ".agentplugins-staging-") {
			return nil, fmt.Errorf("grok project staging differs from exact selected anchor")
		}
		if err := pathpolicy.RequireContainedChild(in.Plan.TargetAnchor, in.StagingPath); err != nil {
			return nil, err
		}
		for _, component := range in.Plan.Components {
			if component.Support != domain.SupportUnsupported && component.Kind != domain.ComponentSkill && component.Kind != domain.ComponentMCPServer {
				return nil, fmt.Errorf("grok project component is unqualified")
			}
		}
	}
	manifest := shared.ManifestFromEnvelope(in.Envelope, shared.WithAuthorNameEmail())
	if shared.ComponentKindPresent(in.Plan.Components, domain.ComponentSkill) {
		manifest["skills"] = "./skills/"
	}
	names := shared.SupportedMCPNames(in.Plan)
	if len(names) > 0 {
		if project {
			manifest["mcpServers"] = "./.mcp.json"
		} else {
			manifest["mcpServers"] = "./mcp.json"
		}
	}
	if err := shared.WriteJSON(filepath.Join(in.StagingPath, ".grok-plugin", "plugin.json"), manifest); err != nil {
		return nil, fmt.Errorf("write Grok plugin manifest: %w", err)
	}
	dialect := shared.MCPDialectCursor
	if project {
		// Grok discovers plugin-scoped .mcp.json; preserve the user bundle layout.
		dialect = shared.MCPDialectOpenAI
	}
	if err := shared.ProjectMCPServers(shared.MCPProjection{
		Root: in.StagingPath, Envelope: in.Envelope, Names: names,
		Dialect: dialect, PluginRoot: in.Plan.ActivePath, DataPath: in.PluginDataPath,
	}); err != nil {
		return nil, err
	}
	return nil, nil
}

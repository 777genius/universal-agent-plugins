package publicationexec

import (
	"encoding/json"
	"slices"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
	"github.com/777genius/plugin-kit-ai/cli/internal/publishschema"
)

const CodexMarketplaceArtifactPath = ".agents/plugins/marketplace.json"
const ClaudeMarketplaceArtifactPath = ".claude-plugin/marketplace.json"

func Generate(graph pluginmodel.PackageGraph, publication publishschema.State, selected []string) ([]pluginmodel.Artifact, error) {
	var artifacts []pluginmodel.Artifact
	if shouldRenderCodexMarketplace(publication, selected) {
		body, err := renderCodexMarketplace(graph, publication.Codex)
		if err != nil {
			return nil, err
		}
		artifacts = append(artifacts, pluginmodel.Artifact{
			RelPath: CodexMarketplaceArtifactPath,
			Content: body,
		})
	}
	if shouldRenderClaudeMarketplace(publication, selected) {
		body, err := renderClaudeMarketplace(graph, publication.Claude)
		if err != nil {
			return nil, err
		}
		artifacts = append(artifacts, pluginmodel.Artifact{
			RelPath: ClaudeMarketplaceArtifactPath,
			Content: body,
		})
	}
	return artifacts, nil
}

func ManagedPaths(publication publishschema.State, selected []string) []string {
	var out []string
	if shouldManageCodexMarketplace(selected) {
		out = append(out, CodexMarketplaceArtifactPath)
	}
	if shouldManageClaudeMarketplace(selected) {
		out = append(out, ClaudeMarketplaceArtifactPath)
	}
	slices.Sort(out)
	return out
}

func shouldRenderCodexMarketplace(publication publishschema.State, selected []string) bool {
	return publication.Codex != nil && shouldManageCodexMarketplace(selected)
}

func shouldManageCodexMarketplace(selected []string) bool {
	return slices.Contains(selected, "codex-package")
}

func shouldRenderClaudeMarketplace(publication publishschema.State, selected []string) bool {
	return publication.Claude != nil && shouldManageClaudeMarketplace(selected)
}

func shouldManageClaudeMarketplace(selected []string) bool {
	return slices.Contains(selected, "claude")
}

func renderCodexMarketplace(graph pluginmodel.PackageGraph, doc *publishschema.CodexMarketplace) ([]byte, error) {
	payload := map[string]any{
		"name": doc.MarketplaceName,
		"plugins": []map[string]any{
			{
				"name": graph.Manifest.Name,
				"source": map[string]any{
					"source": "local",
					"path":   doc.SourceRoot,
				},
				"policy": map[string]any{
					"installation":   doc.InstallationPolicy,
					"authentication": doc.AuthenticationPolicy,
				},
				"category": doc.Category,
			},
		},
	}
	if strings.TrimSpace(doc.DisplayName) != "" {
		payload["interface"] = map[string]any{
			"displayName": doc.DisplayName,
		}
	}
	return json.MarshalIndent(payload, "", "  ")
}

func renderClaudeMarketplace(graph pluginmodel.PackageGraph, doc *publishschema.ClaudeMarketplace) ([]byte, error) {
	payload := map[string]any{
		"name": doc.MarketplaceName,
		"owner": map[string]any{
			"name": doc.OwnerName,
		},
		"plugins": []map[string]any{
			{
				"name":        graph.Manifest.Name,
				"source":      doc.SourceRoot,
				"description": graph.Manifest.Description,
				"version":     graph.Manifest.Version,
			},
		},
	}
	return json.MarshalIndent(payload, "", "  ")
}

package publicationexec

import (
	"fmt"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
	"github.com/777genius/plugin-kit-ai/cli/internal/publishschema"
)

func RenderLocalCatalogArtifact(graph pluginmodel.PackageGraph, publication publishschema.State, target, packageRoot string) (pluginmodel.Artifact, error) {
	switch strings.TrimSpace(target) {
	case "codex-package":
		if publication.Codex == nil {
			return pluginmodel.Artifact{}, fmt.Errorf("%s/publish/codex/marketplace.yaml is required for target %q", pluginmodel.SourceDirName, target)
		}
		body, err := renderCodexMarketplaceWithSourceRoot(graph, publication.Codex, packageRoot)
		if err != nil {
			return pluginmodel.Artifact{}, err
		}
		return pluginmodel.Artifact{RelPath: CodexMarketplaceArtifactPath, Content: body}, nil
	case "claude":
		if publication.Claude == nil {
			return pluginmodel.Artifact{}, fmt.Errorf("%s/publish/claude/marketplace.yaml is required for target %q", pluginmodel.SourceDirName, target)
		}
		body, err := renderClaudeMarketplaceWithSourceRoot(graph, publication.Claude, packageRoot)
		if err != nil {
			return pluginmodel.Artifact{}, err
		}
		return pluginmodel.Artifact{RelPath: ClaudeMarketplaceArtifactPath, Content: body}, nil
	default:
		return pluginmodel.Artifact{}, fmt.Errorf("local publication materialization supports only %q or %q", "codex-package", "claude")
	}
}

func CatalogArtifactPath(target string) (string, error) {
	switch strings.TrimSpace(target) {
	case "codex-package":
		return CodexMarketplaceArtifactPath, nil
	case "claude":
		return ClaudeMarketplaceArtifactPath, nil
	default:
		return "", fmt.Errorf("local publication materialization supports only %q or %q", "codex-package", "claude")
	}
}

func renderCodexMarketplaceWithSourceRoot(graph pluginmodel.PackageGraph, doc *publishschema.CodexMarketplace, packageRoot string) ([]byte, error) {
	clone := *doc
	clone.SourceRoot = strings.TrimSpace(packageRoot)
	return renderCodexMarketplace(graph, &clone)
}

func renderClaudeMarketplaceWithSourceRoot(graph pluginmodel.PackageGraph, doc *publishschema.ClaudeMarketplace, packageRoot string) ([]byte, error) {
	clone := *doc
	clone.SourceRoot = strings.TrimSpace(packageRoot)
	return renderClaudeMarketplace(graph, &clone)
}

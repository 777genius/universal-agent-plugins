package pluginmanifest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
	"github.com/777genius/plugin-kit-ai/sdk/platformmeta"
)

func isSupportedImportSource(from string) bool {
	return slices.Contains(platformmeta.IDs(), from)
}

func inferRuntime(root string) string {
	switch {
	case fileExists(filepath.Join(root, "go.mod")):
		return "go"
	case fileExists(filepath.Join(root, pluginmodel.SourceDirName, "main.py")):
		return "python"
	case fileExists(filepath.Join(root, pluginmodel.SourceDirName, "main.mjs")):
		return "node"
	case fileExists(filepath.Join(root, "scripts", "main.sh")):
		return "shell"
	default:
		return "go"
	}
}

func importedPortableMCPArtifacts(root string) ([]Artifact, error) {
	body, err := os.ReadFile(filepath.Join(root, ".mcp.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	doc := map[string]any{}
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("parse portable MCP .mcp.json: %w", err)
	}
	normalized, err := pluginmodel.ImportedPortableMCPYAML("", doc)
	if err != nil {
		return nil, err
	}
	return []Artifact{{RelPath: filepath.Join("mcp", "servers.yaml"), Content: normalized}}, nil
}

func prefixAuthoredArtifacts(artifacts []Artifact, layout authoredLayout) []Artifact {
	if strings.TrimSpace(layout.RootRel) == "" {
		return artifacts
	}
	out := make([]Artifact, 0, len(artifacts))
	for _, artifact := range artifacts {
		rel := filepath.ToSlash(filepath.Clean(artifact.RelPath))
		prefix := filepath.ToSlash(layout.RootRel)
		if rel != prefix && !strings.HasPrefix(rel, prefix+"/") {
			artifact.RelPath = layout.Path(rel)
		} else {
			artifact.RelPath = rel
		}
		out = append(out, artifact)
	}
	return out
}

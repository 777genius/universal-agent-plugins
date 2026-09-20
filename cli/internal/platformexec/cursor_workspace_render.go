package platformexec

import (
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func (cursorWorkspaceAdapter) Generate(root string, graph pluginmodel.PackageGraph, state pluginmodel.TargetState) ([]pluginmodel.Artifact, error) {
	var artifacts []pluginmodel.Artifact
	if graph.Portable.MCP != nil {
		projected, err := renderPortableMCPForTarget(graph.Portable.MCP, "cursor-workspace")
		if err != nil {
			return nil, err
		}
		body, err := marshalJSON(map[string]any{
			"mcpServers": projected,
		})
		if err != nil {
			return nil, err
		}
		artifacts = append(artifacts, pluginmodel.Artifact{
			RelPath: filepath.Join(".cursor", "mcp.json"),
			Content: body,
		})
	}
	rules, err := copyArtifacts(root, authoredComponentDir(state, "rules", filepath.Join("targets", "cursor-workspace", "rules")), filepath.Join(".cursor", "rules"))
	if err != nil {
		return nil, err
	}
	artifacts = append(artifacts, rules...)
	return compactArtifacts(artifacts), nil
}

func extractCursorManagedAgentsSection(body string) string {
	body = strings.ReplaceAll(body, "\r\n", "\n")
	body = strings.ReplaceAll(body, "\r", "\n")
	start := strings.Index(body, cursorAgentsSectionStart)
	end := strings.Index(body, cursorAgentsSectionEnd)
	if start < 0 || end < 0 || end <= start {
		return ""
	}
	start += len(cursorAgentsSectionStart)
	return strings.TrimSpace(body[start:end])
}

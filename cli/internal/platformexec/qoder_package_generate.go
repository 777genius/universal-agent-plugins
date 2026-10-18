package platformexec

import (
	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func (qoderAdapter) Generate(root string, graph pluginmodel.PackageGraph, state pluginmodel.TargetState) ([]pluginmodel.Artifact, error) {
	doc := map[string]any{
		"name":        graph.Manifest.Name,
		"version":     graph.Manifest.Version,
		"description": graph.Manifest.Description,
	}
	if author := qoderPluginAuthor(graph.Manifest.Author); len(author) > 0 {
		doc["author"] = author
	}
	if len(graph.Portable.Paths("skills")) > 0 {
		doc["skills"] = qoderPluginSkillsRef
	}
	var artifacts []pluginmodel.Artifact
	if graph.Portable.MCP != nil {
		doc["mcpServers"] = qoderPluginMCPRef
		projected, err := renderPortableMCPForTarget(graph.Portable.MCP, "qoder")
		if err != nil {
			return nil, err
		}
		body, err := marshalJSON(map[string]any{"mcpServers": projected})
		if err != nil {
			return nil, err
		}
		artifacts = append(artifacts, pluginmodel.Artifact{
			RelPath: ".mcp.json",
			Content: body,
		})
	}
	body, err := marshalJSON(doc)
	if err != nil {
		return nil, err
	}
	artifacts = append(artifacts, pluginmodel.Artifact{
		RelPath: qoderPluginManifestPath,
		Content: body,
	})
	skillArtifacts, err := renderPortableSkills(root, graph.Portable.Paths("skills"), "skills")
	if err != nil {
		return nil, err
	}
	artifacts = append(artifacts, skillArtifacts...)
	return compactArtifacts(artifacts), nil
}

package platformexec

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func (claudeAdapter) Generate(root string, graph pluginmodel.PackageGraph, state pluginmodel.TargetState) ([]pluginmodel.Artifact, error) {
	authoredRoot := authoredRootHint(state, graph.Portable)
	entrypoint := ""
	if graph.Launcher != nil {
		entrypoint = graph.Launcher.Entrypoint
	}
	if claudeHooksRequireLauncher(graph, state) && strings.TrimSpace(entrypoint) == "" {
		return nil, fmt.Errorf("required launcher missing: %s", pluginmodel.LauncherFileName)
	}
	if graph.Launcher == nil && !claudePackageOnlyMode(graph, state) {
		return nil, fmt.Errorf("invalid %s: target claude without %s/launcher.yaml must author at least one package-only surface such as %s/mcp/servers.yaml, %s/skills/, %s/targets/claude/settings.json, %s/targets/claude/lsp.json, %s/targets/claude/user-config.json, %s/targets/claude/manifest.extra.json, %s/targets/claude/commands/**, or %s/targets/claude/agents/**", pluginmodel.FileName, authoredRoot, authoredRoot, authoredRoot, authoredRoot, authoredRoot, authoredRoot, authoredRoot, authoredRoot, authoredRoot)
	}
	_, settingsBody, settingsPresent, err := loadClaudeJSONDoc(root, state.DocPath("settings"), "Claude settings")
	if err != nil {
		return nil, err
	}
	_, lspBody, lspPresent, err := loadClaudeJSONDoc(root, state.DocPath("lsp"), "Claude LSP")
	if err != nil {
		return nil, err
	}
	userConfig, _, userConfigPresent, err := loadClaudeJSONDoc(root, state.DocPath("user_config"), "Claude userConfig")
	if err != nil {
		return nil, err
	}
	extra, err := loadNativeExtraDoc(root, state, "manifest_extra", pluginmodel.NativeDocFormatJSON)
	if err != nil {
		return nil, err
	}
	doc := map[string]any{
		"name":        graph.Manifest.Name,
		"version":     graph.Manifest.Version,
		"description": graph.Manifest.Description,
	}
	if len(graph.Portable.Paths("skills")) > 0 {
		doc["skills"] = "./skills/"
	}
	if len(state.ComponentPaths("agents")) > 0 {
		doc["agents"] = "./agents/"
	}
	if graph.Portable.MCP != nil {
		doc["mcpServers"] = "./.mcp.json"
	}
	if userConfigPresent {
		doc["userConfig"] = userConfig
	}
	if err := pluginmodel.MergeNativeExtraObject(doc, extra, "claude manifest.extra.json", claudeManifestManagedPaths()); err != nil {
		return nil, err
	}
	pluginJSON, err := marshalJSON(doc)
	if err != nil {
		return nil, err
	}
	artifacts := []pluginmodel.Artifact{{
		RelPath: filepath.Join(".claude-plugin", "plugin.json"),
		Content: pluginJSON,
	}}
	if graph.Portable.MCP != nil {
		projected, err := renderPortableMCPForTarget(graph.Portable.MCP, "claude")
		if err != nil {
			return nil, err
		}
		mcpJSON, err := marshalJSON(projected)
		if err != nil {
			return nil, err
		}
		artifacts = append(artifacts, pluginmodel.Artifact{RelPath: ".mcp.json", Content: mcpJSON})
	}
	skillArtifacts, err := renderPortableSkills(root, graph.Portable.Paths("skills"), "skills")
	if err != nil {
		return nil, err
	}
	artifacts = append(artifacts, skillArtifacts...)
	if settingsPresent {
		artifacts = append(artifacts, pluginmodel.Artifact{RelPath: "settings.json", Content: settingsBody})
	}
	if lspPresent {
		artifacts = append(artifacts, pluginmodel.Artifact{RelPath: ".lsp.json", Content: lspBody})
	}
	if hookPaths := state.ComponentPaths("hooks"); len(hookPaths) > 0 {
		copied, err := copyArtifacts(root, authoredComponentDir(state, "hooks", filepath.Join("targets", "claude", "hooks")), "hooks")
		if err != nil {
			return nil, err
		}
		artifacts = append(artifacts, copied...)
	} else if claudeUsesGeneratedHooks(graph, state) {
		artifacts = append(artifacts, pluginmodel.Artifact{
			RelPath: filepath.Join("hooks", "hooks.json"),
			Content: defaultClaudeHooks(entrypoint),
		})
	}
	copiedKinds := []artifactDir{
		{src: authoredComponentDir(state, "agents", filepath.Join("targets", "claude", "agents")), dst: "agents"},
		{src: authoredComponentDir(state, "commands", filepath.Join("targets", "claude", "commands")), dst: "commands"},
	}
	copied, err := copyArtifactDirs(root, copiedKinds...)
	if err != nil {
		return nil, err
	}
	return append(artifacts, copied...), nil
}

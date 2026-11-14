package platformexec

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/codexmanifest"
	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func generateCodexPackageArtifacts(root string, graph pluginmodel.PackageGraph, state pluginmodel.TargetState) ([]pluginmodel.Artifact, error) {
	extra, err := loadNativeExtraDoc(root, state, "manifest_extra", pluginmodel.NativeDocFormatJSON)
	if err != nil {
		return nil, err
	}
	managedPaths := managedKeysForNativeDoc("codex-package", "manifest_extra")
	if err := pluginmodel.ValidateNativeExtraDocConflicts(extra, "codex-package manifest.extra.json", managedPaths); err != nil {
		return nil, err
	}
	doc, err := newCodexPackageManifestDoc(root, graph, state)
	if err != nil {
		return nil, err
	}
	artifacts, err := codexPackageNativeArtifacts(root, graph, state, doc)
	if err != nil {
		return nil, err
	}
	if err := pluginmodel.MergeNativeExtraObject(doc, extra, "codex-package manifest.extra.json", managedPaths); err != nil {
		return nil, err
	}
	pluginJSON, err := marshalJSON(doc)
	if err != nil {
		return nil, err
	}
	artifacts = append(artifacts, pluginmodel.Artifact{
		RelPath: filepath.Join(".codex-plugin", "plugin.json"),
		Content: pluginJSON,
	})
	return appendCodexPackagePortableArtifacts(root, graph, artifacts)
}

func newCodexPackageManifestDoc(root string, graph pluginmodel.PackageGraph, state pluginmodel.TargetState) (map[string]any, error) {
	meta, err := loadCodexPackageMeta(root, graph, state)
	if err != nil {
		return nil, err
	}
	doc := map[string]any{
		"name":        graph.Manifest.Name,
		"version":     graph.Manifest.Version,
		"description": graph.Manifest.Description,
	}
	meta.Apply(doc)
	if len(graph.Portable.Paths("skills")) > 0 {
		doc["skills"] = codexmanifest.SkillsRef
	}
	if graph.Portable.MCP != nil {
		doc["mcpServers"] = codexmanifest.MCPServersRef
	}
	return doc, nil
}

func loadCodexPackageMeta(root string, graph pluginmodel.PackageGraph, state pluginmodel.TargetState) (codexPackageMeta, error) {
	meta := codexPackageMeta{
		Author:     manifestAuthorToCodex(graph.Manifest.Author),
		Homepage:   strings.TrimSpace(graph.Manifest.Homepage),
		Repository: strings.TrimSpace(graph.Manifest.Repository),
		License:    strings.TrimSpace(graph.Manifest.License),
		Keywords:   append([]string(nil), graph.Manifest.Keywords...),
	}
	override, _, err := readYAMLDoc[codexPackageMeta](root, state.DocPath("package_metadata"))
	if err != nil {
		return codexPackageMeta{}, fmt.Errorf("parse %s: %w", state.DocPath("package_metadata"), err)
	}
	mergeCodexPackageMeta(&meta, override)
	meta.Normalize()
	return meta, nil
}

func codexPackageNativeArtifacts(root string, graph pluginmodel.PackageGraph, state pluginmodel.TargetState, doc map[string]any) ([]pluginmodel.Artifact, error) {
	var artifacts []pluginmodel.Artifact
	if err := mergeCodexPackageInterfaceDoc(root, state, doc); err != nil {
		return nil, err
	}
	appArtifact, err := codexPackageAppArtifact(root, state, doc)
	if err != nil {
		return nil, err
	}
	if appArtifact != nil {
		artifacts = append(artifacts, *appArtifact)
	}
	hookArtifacts, err := copyArtifactDirs(root, artifactDir{
		src: authoredComponentDir(state, "hooks", filepath.Join("targets", "codex-package", "hooks")),
		dst: "hooks",
	})
	if err != nil {
		return nil, err
	}
	return append(artifacts, hookArtifacts...), nil
}

func mergeCodexPackageInterfaceDoc(root string, state pluginmodel.TargetState, doc map[string]any) error {
	rel := strings.TrimSpace(state.DocPath("interface"))
	if rel == "" {
		return nil
	}
	body, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		return err
	}
	interfaceDoc, err := codexmanifest.ParseInterfaceDoc(body)
	if err != nil {
		return fmt.Errorf("parse %s: %w", rel, err)
	}
	doc["interface"] = interfaceDoc
	return nil
}

func codexPackageAppArtifact(root string, state pluginmodel.TargetState, doc map[string]any) (*pluginmodel.Artifact, error) {
	rel := strings.TrimSpace(state.DocPath("app_manifest"))
	if rel == "" {
		return nil, nil
	}
	body, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		return nil, err
	}
	appDoc, err := codexmanifest.ParseAppManifestDoc(body)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", rel, err)
	}
	if !codexmanifest.AppManifestEnabled(appDoc) {
		return nil, nil
	}
	doc["apps"] = codexmanifest.AppsRef
	return &pluginmodel.Artifact{RelPath: ".app.json", Content: body}, nil
}

func appendCodexPackagePortableArtifacts(root string, graph pluginmodel.PackageGraph, artifacts []pluginmodel.Artifact) ([]pluginmodel.Artifact, error) {
	if graph.Portable.MCP != nil {
		projected, err := renderPortableMCPForTarget(graph.Portable.MCP, "codex-package")
		if err != nil {
			return nil, err
		}
		mcpJSON, err := marshalJSON(projected)
		if err != nil {
			return nil, err
		}
		artifacts = append(artifacts, pluginmodel.Artifact{
			RelPath: ".mcp.json",
			Content: mcpJSON,
		})
	}
	skillArtifacts, err := renderPortableSkills(root, graph.Portable.Paths("skills"), "skills")
	if err != nil {
		return nil, err
	}
	artifacts = append(artifacts, skillArtifacts...)
	return compactArtifacts(artifacts), nil
}

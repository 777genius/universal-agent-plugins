package platformexec

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

type qoderAdapter struct{}

const (
	qoderPluginManifestPath = ".qoder-plugin/plugin.json"
	qoderPluginMCPRef       = "./.mcp.json"
	qoderPluginSkillsRef    = "./skills/"
)

func (qoderAdapter) ID() string { return "qoder" }

func (qoderAdapter) DetectNative(root string) bool {
	return fileExists(filepath.Join(root, ".qoder-plugin", "plugin.json"))
}

func (qoderAdapter) RefineDiscovery(root string, state *pluginmodel.TargetState) error {
	authoredRoot := authoredRootHint(*state, pluginmodel.NewPortableComponents())
	targetDir := filepath.Join(root, authoredRoot, "targets", "qoder")
	entries, err := os.ReadDir(targetDir)
	switch {
	case os.IsNotExist(err):
		return nil
	case err != nil:
		return err
	case len(entries) == 0:
		return nil
	default:
		return fmt.Errorf("target qoder does not support %s/targets/qoder/... in phase 1: use %s/skills/** and %s/mcp/servers.yaml", authoredRoot, authoredRoot, authoredRoot)
	}
}

func (qoderAdapter) ManagedPaths(root string, graph pluginmodel.PackageGraph, state pluginmodel.TargetState) ([]string, error) {
	// .mcp.json is adapter-generated whenever portable MCP exists and becomes
	// stale (and must be removed) when portable MCP is dropped, otherwise a
	// later import resurrects servers from the orphaned sidecar.
	return []string{".mcp.json"}, nil
}

package platformexec

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

type cursorAdapter struct{}

const (
	cursorPluginManifestPath = ".cursor-plugin/plugin.json"
	cursorPluginMCPRef       = "./.mcp.json"
	cursorPluginSkillsRef    = "./skills/"
)

func (cursorAdapter) ID() string { return "cursor" }

func (cursorAdapter) DetectNative(root string) bool {
	return fileExists(filepath.Join(root, ".cursor-plugin", "plugin.json"))
}

func (cursorAdapter) RefineDiscovery(root string, state *pluginmodel.TargetState) error {
	authoredRoot := authoredRootHint(*state, pluginmodel.NewPortableComponents())
	targetDir := filepath.Join(root, authoredRoot, "targets", "cursor")
	entries, err := os.ReadDir(targetDir)
	switch {
	case os.IsNotExist(err):
		return nil
	case err != nil:
		return err
	case len(entries) == 0:
		return nil
	default:
		return fmt.Errorf("target cursor does not support %s/targets/cursor/... in phase 1: use %s/skills/** and %s/mcp/servers.yaml, or move repo-local Cursor config to %s/targets/cursor-workspace/...", authoredRoot, authoredRoot, authoredRoot, authoredRoot)
	}
}

func (cursorAdapter) ManagedPaths(root string, graph pluginmodel.PackageGraph, state pluginmodel.TargetState) ([]string, error) {
	return nil, nil
}

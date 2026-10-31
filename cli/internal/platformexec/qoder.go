package platformexec

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

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
	}
	for _, entry := range entries {
		if entry.Name() == "hooks" || entry.Name() == "commands" || entry.Name() == "bin" {
			continue
		}
		return fmt.Errorf("target qoder does not support %s/targets/qoder/%s: supported target-native kinds are hooks, commands, and bin", authoredRoot, entry.Name())
	}
	return nil
}

func (qoderAdapter) ManagedPaths(root string, graph pluginmodel.PackageGraph, state pluginmodel.TargetState) ([]string, error) {
	// .mcp.json is adapter-generated whenever portable MCP exists and becomes
	// stale (and must be removed) when portable MCP is dropped, otherwise a
	// later import resurrects servers from the orphaned sidecar. The same
	// symmetry applies to the mirrored hooks/, commands/, and bin/ trees.
	paths := []string{".mcp.json"}
	for _, dir := range []string{"hooks", "commands", "bin"} {
		found, err := existingFilesUnder(root, dir)
		if err != nil {
			return nil, err
		}
		paths = append(paths, found...)
	}
	return paths, nil
}

func existingFilesUnder(root, dir string) ([]string, error) {
	full := filepath.Join(root, dir)
	if _, err := os.Stat(full); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var paths []string
	err := filepath.WalkDir(full, func(path string, d os.DirEntry, err error) error {
		if err != nil || d == nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		paths = append(paths, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	return paths, nil
}

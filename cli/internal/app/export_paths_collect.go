package app

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmanifest"
	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
	"github.com/777genius/plugin-kit-ai/cli/internal/runtimecheck"
)

func exportFileList(root string, graph pluginmanifest.PackageGraph, project runtimecheck.Project, generated []pluginmanifest.Artifact) ([]string, error) {
	set := map[string]struct{}{}
	for _, rel := range graph.SourceFiles {
		addExportPath(set, rel)
	}
	for _, artifact := range generated {
		addExportPath(set, artifact.RelPath)
	}
	for _, rel := range launcherBundlePaths(root, project.Entrypoint) {
		addExportPath(set, rel)
	}
	addRuntimeExportPaths(root, set, project)
	excludes := exportExcludedPaths(root, project)
	out := make([]string, 0, len(set))
	for rel := range set {
		if shouldExcludeExportPath(rel, excludes) {
			continue
		}
		info, err := os.Lstat(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			return nil, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("export refuses symlinked path %s", rel)
		}
		if info.IsDir() {
			continue
		}
		out = append(out, rel)
	}
	slices.Sort(out)
	return out, nil
}

func addRuntimeExportPaths(root string, set map[string]struct{}, project runtimecheck.Project) {
	authoredRoot := exportAuthoredRoot(root)
	switch project.Runtime {
	case "python":
		addDirIfExists(root, set, authoredRoot)
		addIfExists(root, set, "requirements.txt")
		addIfExists(root, set, "pyproject.toml")
		addIfExists(root, set, "uv.lock")
		addIfExists(root, set, "poetry.lock")
		addIfExists(root, set, "Pipfile")
		addIfExists(root, set, "Pipfile.lock")
	case "node":
		addDirIfExists(root, set, authoredRoot)
		addIfExists(root, set, "package.json")
		addIfExists(root, set, "tsconfig.json")
		addIfExists(root, set, ".yarnrc.yml")
		addIfExists(root, set, "package-lock.json")
		addIfExists(root, set, "npm-shrinkwrap.json")
		addIfExists(root, set, "pnpm-lock.yaml")
		addIfExists(root, set, "yarn.lock")
		addIfExists(root, set, "bun.lock")
		addIfExists(root, set, "bun.lockb")
		if project.Node.IsTypeScript && project.Node.UsesBuiltOutput {
			addDirIfExists(root, set, project.Node.OutputDir)
		} else if strings.TrimSpace(project.Node.RuntimeTarget) != "" {
			addIfExists(root, set, project.Node.RuntimeTarget)
		}
	case "shell":
		addDirIfExists(root, set, "scripts")
	}
}

func exportAuthoredRoot(root string) string {
	if fileExists(filepath.Join(root, pluginmodel.SourceDirName, pluginmanifest.FileName)) {
		return pluginmodel.SourceDirName
	}
	return pluginmodel.SourceDirName
}

package app

import (
	"os"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/cli/internal/runtimecheck"
)

func addIfExists(root string, set map[string]struct{}, rel string) {
	rel = normalizeExportPath(rel)
	if rel == "" {
		return
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); err == nil {
		set[rel] = struct{}{}
	}
}

func addDirIfExists(root string, set map[string]struct{}, rel string) {
	rel = normalizeExportPath(rel)
	if rel == "" {
		return
	}
	dir := filepath.Join(root, filepath.FromSlash(rel))
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return
	}
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		sub, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		set[filepath.ToSlash(sub)] = struct{}{}
		return nil
	})
}

func exportExcludedPaths(root string, project runtimecheck.Project) []string {
	excludes := []string{".venv", "node_modules", ".pnp.cjs", ".pnp.loader.mjs"}
	if project.Python.ReadySource == runtimecheck.PythonEnvSourceManagerOwned && project.Python.ProbedEnvPath != "" {
		if rel, ok := relWithinRoot(root, project.Python.ProbedEnvPath); ok {
			excludes = append(excludes, rel)
		}
	}
	return excludes
}

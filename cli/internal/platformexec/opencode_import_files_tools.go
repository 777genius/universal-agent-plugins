package platformexec

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func importDirectoryArtifactsRejectingSymlinks(sources []opencodeImportSource, dstRoot string, keep func(string) bool) ([]pluginmodel.Artifact, []pluginmodel.Warning, error) {
	artifacts := map[string]pluginmodel.Artifact{}
	for _, source := range sources {
		if err := appendImportedArtifactsRejectingSymlinks(artifacts, source, dstRoot, keep); err != nil {
			return nil, nil, err
		}
	}
	return sortedImportedArtifacts(artifacts), nil, nil
}

func appendImportedArtifactsRejectingSymlinks(artifacts map[string]pluginmodel.Artifact, source opencodeImportSource, dstRoot string, keep func(string) bool) error {
	full := source.dir
	if _, err := os.Stat(full); err != nil {
		return nil
	}
	return filepath.WalkDir(full, func(path string, d os.DirEntry, err error) error {
		if err != nil || d == nil {
			return err
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("OpenCode native import does not support symlinks under %s", source.display)
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(full, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if keep != nil && !keep(rel) {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		dst := filepath.ToSlash(filepath.Join(dstRoot, rel))
		artifacts[dst] = pluginmodel.Artifact{RelPath: dst, Content: body}
		return nil
	})
}

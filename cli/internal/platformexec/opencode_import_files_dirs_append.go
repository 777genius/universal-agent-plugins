package platformexec

import (
	"os"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func appendImportedDirectorySources(artifacts map[string]pluginmodel.Artifact, sources []opencodeImportSource, dstRoot string, keep func(string) bool) ([]pluginmodel.Warning, error) {
	var warnings []pluginmodel.Warning
	for _, source := range sources {
		used, err := appendImportedDirectoryArtifacts(artifacts, source, dstRoot, keep)
		if err != nil {
			return nil, err
		}
		if warning, ok := importedDirectoryUseWarning(source, used); ok {
			warnings = append(warnings, warning)
		}
	}
	return warnings, nil
}

func appendImportedDirectoryArtifacts(artifacts map[string]pluginmodel.Artifact, source opencodeImportSource, dstRoot string, keep func(string) bool) (bool, error) {
	full := source.dir
	if _, err := os.Stat(full); err != nil {
		return false, nil
	}
	var used bool
	err := filepath.WalkDir(full, func(path string, d os.DirEntry, err error) error {
		if err != nil || d == nil || d.IsDir() {
			return err
		}
		rel, artifact, ok, err := importedDirectoryArtifact(path, full, dstRoot, keep)
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
		artifacts[rel] = artifact
		used = true
		return nil
	})
	return used, err
}

func importedDirectoryArtifact(path, full, dstRoot string, keep func(string) bool) (string, pluginmodel.Artifact, bool, error) {
	rel, err := filepath.Rel(full, path)
	if err != nil {
		return "", pluginmodel.Artifact{}, false, err
	}
	rel = filepath.ToSlash(rel)
	if keep != nil && !keep(rel) {
		return "", pluginmodel.Artifact{}, false, nil
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return "", pluginmodel.Artifact{}, false, err
	}
	dst := filepath.ToSlash(filepath.Join(dstRoot, rel))
	return dst, pluginmodel.Artifact{RelPath: dst, Content: body}, true, nil
}

func importedDirectoryUseWarning(source opencodeImportSource, used bool) (pluginmodel.Warning, bool) {
	if !source.warnOnUse || !used {
		return pluginmodel.Warning{}, false
	}
	return pluginmodel.Warning{
		Kind:    pluginmodel.WarningFidelity,
		Path:    source.warnPath,
		Message: source.warnMsg,
	}, true
}

package validate

import (
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmanifest"
	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func authoredProjectRoot(root string) string {
	if _, err := pluginmanifest.Load(root); err == nil {
		if fileExists(filepath.Join(root, pluginmodel.SourceDirName, pluginmanifest.FileName)) {
			return pluginmodel.SourceDirName
		}
	}
	return pluginmodel.SourceDirName
}

func authoredProjectPath(root, rel string) string {
	return filepath.ToSlash(filepath.Join(authoredProjectRoot(root), rel))
}

func authoredRuntimeTargetPath(root string, candidates ...string) string {
	authoredRoot := authoredProjectRoot(root)
	for _, candidate := range candidates {
		rel := filepath.ToSlash(filepath.Join(authoredRoot, candidate))
		if fileExists(filepath.Join(root, rel)) {
			return rel
		}
	}
	if len(candidates) == 0 {
		return authoredRoot
	}
	return filepath.ToSlash(filepath.Join(authoredRoot, candidates[0]))
}

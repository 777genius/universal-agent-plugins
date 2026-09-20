package platformexec

import (
	"os"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func resolveOpenCodePackageJSONArtifact(cfg opencodeScopeConfig) (pluginmodel.Artifact, bool, error) {
	packageJSON := filepath.Join(cfg.workspaceRoot, "package.json")
	if body, err := os.ReadFile(packageJSON); err == nil {
		return pluginmodel.Artifact{
			RelPath: filepath.ToSlash(filepath.Join(pluginmodel.SourceDirName, "targets", "opencode", "package.json")),
			Content: body,
		}, true, nil
	} else if !os.IsNotExist(err) {
		return pluginmodel.Artifact{}, false, err
	}
	return pluginmodel.Artifact{}, false, nil
}

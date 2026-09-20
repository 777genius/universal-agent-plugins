package platformexec

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func readImportedOpenCodeConfigFromDir(root string, displayBase string) (importedOpenCodeConfig, string, []pluginmodel.Warning, bool, error) {
	return readImportedOpenCodeConfig(root, displayBase)
}

func importOpenCodeToolArtifacts(workspaceRoot, workspaceDisplay string) ([]pluginmodel.Artifact, []pluginmodel.Warning, error) {
	legacyDir := filepath.Join(workspaceRoot, "tool")
	if _, err := os.Stat(legacyDir); err == nil {
		return nil, nil, fmt.Errorf("unsupported OpenCode native path %s: use %s", filepath.ToSlash(filepath.Join(workspaceDisplay, "tool")), filepath.ToSlash(filepath.Join(workspaceDisplay, "tools")))
	} else if err != nil && !os.IsNotExist(err) {
		return nil, nil, err
	}
	sources := []opencodeImportSource{{
		dir:     filepath.Join(workspaceRoot, "tools"),
		display: filepath.ToSlash(filepath.Join(workspaceDisplay, "tools")),
	}}
	return importDirectoryArtifactsRejectingSymlinks(sources, filepath.Join(pluginmodel.SourceDirName, "targets", "opencode", "tools"), nil)
}

package platformexec

import (
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

type importedOpenCodeConfigSource struct {
	path        string
	displayPath string
	warnings    []pluginmodel.Warning
}

func resolveImportedOpenCodeConfigSource(root, displayBase string) (importedOpenCodeConfigSource, bool, error) {
	path, warnings, ok, err := resolveOpenCodeConfigPathInDir(root, displayBase)
	if err != nil || !ok {
		return importedOpenCodeConfigSource{warnings: warnings}, ok, err
	}
	return importedOpenCodeConfigSource{
		path:        path,
		displayPath: importedOpenCodeConfigDisplayPath(path, displayBase),
		warnings:    warnings,
	}, true, nil
}

func importedOpenCodeConfigDisplayPath(path, displayBase string) string {
	displayPath := filepath.Base(path)
	if strings.TrimSpace(displayBase) != "" {
		displayPath = filepath.ToSlash(filepath.Join(displayBase, filepath.Base(path)))
	}
	return displayPath
}

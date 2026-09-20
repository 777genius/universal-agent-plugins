package platformexec

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func readImportedOpenCodeConfigFromFile(path string, displayPath string) (importedOpenCodeConfig, string, []pluginmodel.Warning, bool, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return importedOpenCodeConfig{}, "", nil, false, err
	}
	data, err := decodeImportedOpenCodeConfig(body)
	if err != nil {
		return importedOpenCodeConfig{}, displayPath, nil, false, err
	}
	if strings.TrimSpace(displayPath) == "" {
		displayPath = filepath.ToSlash(path)
	}
	return data, displayPath, nil, true, nil
}

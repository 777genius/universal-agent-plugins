package platformexec

import "github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"

func readImportedOpenCodeConfig(root string, displayBase string) (importedOpenCodeConfig, string, []pluginmodel.Warning, bool, error) {
	source, ok, err := resolveImportedOpenCodeConfigSource(root, displayBase)
	if err != nil || !ok {
		return importedOpenCodeConfig{}, "", source.warnings, ok, err
	}
	data, err := readImportedOpenCodeConfigSource(source)
	if err != nil {
		return importedOpenCodeConfig{}, "", source.warnings, false, err
	}
	return data, source.displayPath, source.warnings, true, nil
}

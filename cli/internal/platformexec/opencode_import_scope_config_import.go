package platformexec

import "github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"

type openCodeScopeConfigImport struct {
	importedConfig    importedOpenCodeConfig
	configDisplayPath string
	warnings          []pluginmodel.Warning
	ok                bool
}

func resolveOpenCodeScopeConfigImport(cfg opencodeScopeConfig) (openCodeScopeConfigImport, error) {
	importedConfig, configDisplayPath, warnings, ok, err := readImportedOpenCodeConfigFromDir(cfg.root, cfg.displayConfigRoot)
	if err != nil {
		return openCodeScopeConfigImport{}, err
	}
	return openCodeScopeConfigImport{
		importedConfig:    importedConfig,
		configDisplayPath: configDisplayPath,
		warnings:          warnings,
		ok:                ok,
	}, nil
}

func applyOpenCodeScopeConfigImport(state *opencodeImportedState, configImport openCodeScopeConfigImport) error {
	state.warnings = append(state.warnings, configImport.warnings...)
	if !configImport.ok {
		return nil
	}
	if err := importOpenCodeConfigArtifacts(state, configImport.importedConfig, configImport.configDisplayPath); err != nil {
		return err
	}
	state.hasInput = true
	return nil
}

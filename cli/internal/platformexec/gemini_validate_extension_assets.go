package platformexec

import "github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"

func validateGeminiExtensionAssetContracts(root string, graph pluginmodel.PackageGraph, state pluginmodel.TargetState, extension importedGeminiExtension) ([]Diagnostic, error) {
	var diagnostics []Diagnostic
	settingsDiagnostics, err := validateGeminiExtensionSettingsContracts(root, state, extension)
	if err != nil {
		return nil, err
	}
	diagnostics = append(diagnostics, settingsDiagnostics...)

	themeDiagnostics, err := validateGeminiExtensionThemeContracts(root, state, extension)
	if err != nil {
		return nil, err
	}
	diagnostics = append(diagnostics, themeDiagnostics...)

	mcpDiagnostics, err := validateGeminiExtensionMCPContracts(graph, extension)
	if err != nil {
		return nil, err
	}
	diagnostics = append(diagnostics, mcpDiagnostics...)
	return diagnostics, nil
}

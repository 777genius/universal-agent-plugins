package platformexec

import "github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"

func validateGeminiAuthoredAssetContracts(root string, _ pluginmodel.PackageGraph, state pluginmodel.TargetState, _ geminiPackageMeta) []Diagnostic {
	var diagnostics []Diagnostic
	diagnostics = append(diagnostics, validateGeminiSettings(root, state.ComponentPaths("settings"))...)
	diagnostics = append(diagnostics, validateGeminiThemes(root, state.ComponentPaths("themes"))...)
	diagnostics = append(diagnostics, validateGeminiPolicies(root, state.ComponentPaths("policies"))...)
	diagnostics = append(diagnostics, validateGeminiCommands(root, state.ComponentPaths("commands"))...)
	return diagnostics
}

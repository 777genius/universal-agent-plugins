package platformexec

import "github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"

func validateGeminiSurfaces(root string, graph pluginmodel.PackageGraph, state pluginmodel.TargetState, meta geminiPackageMeta) ([]Diagnostic, error) {
	hookPaths := state.ComponentPaths("hooks")
	diagnostics, err := validateGeminiAuthoredSurfaces(root, graph, state, meta, hookPaths)
	if err != nil {
		return nil, err
	}
	generatedDiagnostics, err := validateGeminiGeneratedExtension(root, graph, state, meta)
	if err != nil {
		return nil, err
	}
	return append(diagnostics, generatedDiagnostics...), nil
}

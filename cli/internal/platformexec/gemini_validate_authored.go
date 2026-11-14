package platformexec

import "github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"

func validateGeminiAuthoredSurfaces(
	root string,
	graph pluginmodel.PackageGraph,
	state pluginmodel.TargetState,
	meta geminiPackageMeta,
	hookPaths []string,
) ([]Diagnostic, error) {
	var diagnostics []Diagnostic
	coreDiagnostics, err := validateGeminiAuthoredCore(root, graph, state, meta)
	if err != nil {
		return nil, err
	}
	diagnostics = append(diagnostics, coreDiagnostics...)

	assetDiagnostics := validateGeminiAuthoredAssetContracts(root, graph, state, meta)
	diagnostics = append(diagnostics, assetDiagnostics...)

	hookDiagnostics := validateGeminiAuthoredHookContracts(root, graph, hookPaths)
	diagnostics = append(diagnostics, hookDiagnostics...)
	return diagnostics, nil
}

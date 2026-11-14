package platformexec

import "github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"

func appendGeminiGeneratedExtensionContract(
	root string,
	graph pluginmodel.PackageGraph,
	state pluginmodel.TargetState,
	meta geminiPackageMeta,
	extension importedGeminiExtension,
	diagnostics []Diagnostic,
) ([]Diagnostic, error) {
	extensionDiagnostics, err := validateGeminiExtensionContract(root, graph, state, meta, extension)
	if err != nil {
		return nil, err
	}
	return append(diagnostics, extensionDiagnostics...), nil
}

package platformexec

import "github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"

func validateGeminiGeneratedExtension(
	root string,
	graph pluginmodel.PackageGraph,
	state pluginmodel.TargetState,
	meta geminiPackageMeta,
) ([]Diagnostic, error) {
	extension, ok, diagnostics := readGeminiGeneratedExtension(root)
	if !ok {
		return diagnostics, nil
	}
	return appendGeminiGeneratedExtensionContract(root, graph, state, meta, extension, diagnostics)
}

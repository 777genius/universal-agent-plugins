package platformexec

import (
	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func validateGeminiExtensionContract(root string, graph pluginmodel.PackageGraph, state pluginmodel.TargetState, meta geminiPackageMeta, extension importedGeminiExtension) ([]Diagnostic, error) {
	var diagnostics []Diagnostic
	diagnostics = append(diagnostics, validateGeminiExtensionIdentityContract(graph, extension)...)
	diagnostics = append(diagnostics, validateGeminiExtensionMetaContract(meta, extension)...)

	assetDiagnostics, err := validateGeminiExtensionAssetContracts(root, graph, state, extension)
	if err != nil {
		return nil, err
	}
	diagnostics = append(diagnostics, assetDiagnostics...)

	contextDiagnostics, err := validateGeminiExtensionContextContract(root, graph, state, meta, extension)
	if err != nil {
		return nil, err
	}
	diagnostics = append(diagnostics, contextDiagnostics...)
	return diagnostics, nil
}

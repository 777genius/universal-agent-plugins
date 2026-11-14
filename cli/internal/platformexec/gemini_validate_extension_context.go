package platformexec

import (
	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

type geminiExtensionContextSelection struct {
	expected geminiContextSelection
	ok       bool
}

func validateGeminiExtensionContextContract(root string, graph pluginmodel.PackageGraph, state pluginmodel.TargetState, meta geminiPackageMeta, extension importedGeminiExtension) ([]Diagnostic, error) {
	selection, err := validateGeminiExtensionContextSelection(graph, state, meta)
	if err != nil {
		return nil, err
	}
	return selection.contractDiagnostics(root, extension), nil
}

func validateGeminiExtensionContextSelection(graph pluginmodel.PackageGraph, state pluginmodel.TargetState, meta geminiPackageMeta) (geminiExtensionContextSelection, error) {
	expected, ok, err := resolveGeminiExpectedContext(graph, state, meta)
	if err != nil {
		return geminiExtensionContextSelection{}, err
	}
	return geminiExtensionContextSelection{expected: expected, ok: ok}, nil
}

func validateGeminiExtensionContextDiagnostics(root string, selection geminiExtensionContextSelection, extension importedGeminiExtension) []Diagnostic {
	return selection.contractDiagnostics(root, extension)
}

func (selection geminiExtensionContextSelection) contractDiagnostics(root string, extension importedGeminiExtension) []Diagnostic {
	if selection.ok {
		return validateGeminiExpectedContextContract(root, selection.expected, extension)
	}
	return validateGeminiUnexpectedContextContract(extension)
}

func validateGeminiExpectedContextContract(root string, expected geminiContextSelection, extension importedGeminiExtension) []Diagnostic {
	return validateGeminiExpectedContext(root, expected, extension)
}

func validateGeminiUnexpectedContextContract(extension importedGeminiExtension) []Diagnostic {
	return validateGeminiUnexpectedContext(extension)
}

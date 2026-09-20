package platformexec

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func resolveGeminiExpectedContext(graph pluginmodel.PackageGraph, state pluginmodel.TargetState, meta geminiPackageMeta) (geminiContextSelection, bool, error) {
	return selectGeminiPrimaryContext(graph, state, meta)
}

func validateGeminiExpectedContext(root string, expected geminiContextSelection, extension importedGeminiExtension) []Diagnostic {
	return appendGeminiContextDiagnostics(
		validateGeminiExpectedContextProjection(expected, extension),
		validateGeminiExpectedContextArtifact(root, expected),
	)
}

func validateGeminiExpectedContextProjection(expected geminiContextSelection, extension importedGeminiExtension) []Diagnostic {
	return validateGeminiContextFileNameProjection(expected, extension)
}

func validateGeminiExpectedContextArtifact(root string, expected geminiContextSelection) []Diagnostic {
	return validateGeminiContextFileReadable(root, expected)
}

func appendGeminiContextDiagnostics(parts ...[]Diagnostic) []Diagnostic {
	var diagnostics []Diagnostic
	for _, part := range parts {
		diagnostics = append(diagnostics, part...)
	}
	return diagnostics
}

func validateGeminiContextFileNameProjection(expected geminiContextSelection, extension importedGeminiExtension) []Diagnostic {
	if geminiContextFileNameMatchesExpected(expected, extension) {
		return nil
	}
	return []Diagnostic{{
		Severity: SeverityFailure,
		Code:     CodeGeneratedContractInvalid,
		Path:     "gemini-extension.json",
		Target:   "gemini",
		Message:  fmt.Sprintf("Gemini extension manifest gemini-extension.json sets contextFileName %q; expected %q from authored context selection", strings.TrimSpace(extension.Meta.ContextFileName), expected.ArtifactName),
	}}
}

func geminiContextFileNameMatchesExpected(expected geminiContextSelection, extension importedGeminiExtension) bool {
	return strings.TrimSpace(extension.Meta.ContextFileName) == expected.ArtifactName
}

func validateGeminiContextFileReadable(root string, expected geminiContextSelection) []Diagnostic {
	if geminiContextFileExists(root, expected) {
		return nil
	}
	return []Diagnostic{{
		Severity: SeverityFailure,
		Code:     CodeGeneratedContractInvalid,
		Path:     expected.ArtifactName,
		Target:   "gemini",
		Message:  fmt.Sprintf("Gemini primary context file %s is not readable", expected.ArtifactName),
	}}
}

func geminiContextFileExists(root string, expected geminiContextSelection) bool {
	return fileExists(filepath.Join(root, expected.ArtifactName))
}

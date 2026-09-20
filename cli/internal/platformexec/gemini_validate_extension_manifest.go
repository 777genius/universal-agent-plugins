package platformexec

import (
	"fmt"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func validateGeminiExtensionIdentityContract(graph pluginmodel.PackageGraph, extension importedGeminiExtension) []Diagnostic {
	var diagnostics []Diagnostic
	if strings.TrimSpace(extension.Name) != strings.TrimSpace(graph.Manifest.Name) {
		diagnostics = append(diagnostics, Diagnostic{
			Severity: SeverityFailure,
			Code:     CodeGeneratedContractInvalid,
			Path:     "gemini-extension.json",
			Target:   "gemini",
			Message:  fmt.Sprintf("Gemini extension manifest gemini-extension.json sets name %q; expected %q from plugin.yaml", strings.TrimSpace(extension.Name), strings.TrimSpace(graph.Manifest.Name)),
		})
	}
	if strings.TrimSpace(extension.Version) != strings.TrimSpace(graph.Manifest.Version) {
		diagnostics = append(diagnostics, Diagnostic{
			Severity: SeverityFailure,
			Code:     CodeGeneratedContractInvalid,
			Path:     "gemini-extension.json",
			Target:   "gemini",
			Message:  fmt.Sprintf("Gemini extension manifest gemini-extension.json sets version %q; expected %q from plugin.yaml", strings.TrimSpace(extension.Version), strings.TrimSpace(graph.Manifest.Version)),
		})
	}
	if strings.TrimSpace(extension.Description) != strings.TrimSpace(graph.Manifest.Description) {
		diagnostics = append(diagnostics, Diagnostic{
			Severity: SeverityFailure,
			Code:     CodeGeneratedContractInvalid,
			Path:     "gemini-extension.json",
			Target:   "gemini",
			Message:  fmt.Sprintf("Gemini extension manifest gemini-extension.json sets description %q; expected %q from plugin.yaml", strings.TrimSpace(extension.Description), strings.TrimSpace(graph.Manifest.Description)),
		})
	}
	return diagnostics
}

func validateGeminiExtensionMetaContract(meta geminiPackageMeta, extension importedGeminiExtension) []Diagnostic {
	if geminiPackageMetaEqual(meta, extension.Meta) {
		return nil
	}
	return []Diagnostic{{
		Severity: SeverityFailure,
		Code:     CodeGeneratedContractInvalid,
		Path:     "gemini-extension.json",
		Target:   "gemini",
		Message:  "Gemini extension manifest gemini-extension.json package metadata does not match targets/gemini/package.yaml",
	}}
}

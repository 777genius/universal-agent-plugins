package platformexec

import (
	"fmt"
	"strings"
)

func validateGeminiUnexpectedContext(extension importedGeminiExtension) []Diagnostic {
	name := unexpectedGeminiContextFileName(extension)
	if name == "" {
		return nil
	}
	return []Diagnostic{unexpectedGeminiContextDiagnostic(name)}
}

func unexpectedGeminiContextFileName(extension importedGeminiExtension) string {
	return strings.TrimSpace(extension.Meta.ContextFileName)
}

func unexpectedGeminiContextDiagnostic(name string) Diagnostic {
	return Diagnostic{
		Severity: SeverityFailure,
		Code:     CodeGeneratedContractInvalid,
		Path:     "gemini-extension.json",
		Target:   "gemini",
		Message:  fmt.Sprintf("Gemini extension manifest gemini-extension.json sets contextFileName %q without an authored primary context", name),
	}
}

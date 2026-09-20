package platformexec

import "fmt"

func invalidGeminiGeneratedExtensionDiagnostics(err error) []Diagnostic {
	return []Diagnostic{{
		Severity: SeverityFailure,
		Code:     CodeManifestInvalid,
		Path:     "gemini-extension.json",
		Target:   "gemini",
		Message:  fmt.Sprintf("Gemini extension manifest gemini-extension.json is invalid: %v", err),
	}}
}

func missingGeminiGeneratedExtensionDiagnostics() []Diagnostic {
	return []Diagnostic{{
		Severity: SeverityFailure,
		Code:     CodeGeneratedContractInvalid,
		Path:     "gemini-extension.json",
		Target:   "gemini",
		Message:  "Gemini extension manifest gemini-extension.json is not readable",
	}}
}

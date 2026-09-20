package platformexec

import (
	"fmt"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func validateNamedGeminiContextSelection(state pluginmodel.TargetState, selected string, matches []string) []Diagnostic {
	switch len(matches) {
	case 0:
		return []Diagnostic{geminiContextManifestDiagnostic(state.DocPath("package_metadata"), fmt.Sprintf("Gemini context_file_name %q does not resolve to a Gemini-native context source", selected))}
	case 1:
		return nil
	default:
		return []Diagnostic{geminiContextManifestDiagnostic(state.DocPath("package_metadata"), fmt.Sprintf("Gemini context_file_name %q is ambiguous across multiple context sources", selected))}
	}
}

func validateDefaultGeminiContextSelection(candidates, geminiMD []string) []Diagnostic {
	if len(geminiMD) > 1 {
		return []Diagnostic{geminiContextManifestDiagnostic("contexts", "Gemini primary context selection is ambiguous for GEMINI.md; keep one root context or set context_file_name explicitly")}
	}
	if len(geminiMD) == 1 || len(candidates) <= 1 {
		return nil
	}
	return []Diagnostic{geminiContextManifestDiagnostic("contexts", "Gemini primary context selection is ambiguous; set targets/gemini/package.yaml context_file_name explicitly")}
}

func geminiContextManifestDiagnostic(path, message string) Diagnostic {
	return Diagnostic{
		Severity: SeverityFailure,
		Code:     CodeManifestInvalid,
		Path:     path,
		Target:   "gemini",
		Message:  message,
	}
}

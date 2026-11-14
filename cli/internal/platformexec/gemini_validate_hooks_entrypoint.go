package platformexec

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func validateGeminiHookEntrypointConsistency(root string, rels []string, entrypoint string) []Diagnostic {
	if strings.TrimSpace(entrypoint) == "" {
		return nil
	}
	var diagnostics []Diagnostic
	for _, rel := range rels {
		body, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			continue
		}
		mismatches, err := validateGeminiHookEntrypoints(body, entrypoint)
		if err != nil {
			diagnostics = append(diagnostics, geminiHookDiagnostic(
				CodeManifestInvalid,
				rel,
				fmt.Sprintf("Gemini hooks file %s is invalid JSON: %v", rel, err),
			))
			continue
		}
		for _, msg := range mismatches {
			diagnostics = append(diagnostics, Diagnostic{
				Severity: SeverityFailure,
				Code:     CodeEntrypointMismatch,
				Path:     rel,
				Target:   "gemini",
				Message:  msg,
			})
		}
	}
	return diagnostics
}

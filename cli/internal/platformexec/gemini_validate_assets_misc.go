package platformexec

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func validateGeminiPolicies(root string, rels []string) []Diagnostic {
	var diagnostics []Diagnostic
	for _, rel := range rels {
		text, ok, fileDiagnostics := readGeminiPolicyText(root, rel)
		diagnostics = append(diagnostics, fileDiagnostics...)
		if !ok {
			continue
		}
		diagnostics = append(diagnostics, validateGeminiPolicyKeys(rel, text)...)
	}
	return diagnostics
}

func readGeminiPolicyText(root, rel string) (string, bool, []Diagnostic) {
	body, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		return "", false, []Diagnostic{{
			Severity: SeverityFailure,
			Code:     CodeManifestInvalid,
			Path:     rel,
			Target:   "gemini",
			Message:  fmt.Sprintf("Gemini policy file %s is not readable: %v", rel, err),
		}}
	}
	return string(body), true, nil
}

func validateGeminiPolicyKeys(rel, text string) []Diagnostic {
	var diagnostics []Diagnostic
	for _, key := range []string{"allow", "yolo"} {
		if strings.Contains(text, key+" =") {
			diagnostics = append(diagnostics, Diagnostic{
				Severity: SeverityWarning,
				Code:     CodeGeminiPolicyIgnored,
				Path:     rel,
				Target:   "gemini",
				Message:  fmt.Sprintf("Gemini extension policies ignore %q at extension tier", key),
			})
		}
	}
	return diagnostics
}

func validateGeminiCommandFile(root, rel string) []Diagnostic {
	if filepath.Ext(rel) != ".toml" {
		return []Diagnostic{{
			Severity: SeverityFailure,
			Code:     CodeManifestInvalid,
			Path:     rel,
			Target:   "gemini",
			Message:  fmt.Sprintf("Gemini command file %s must use the .toml extension", rel),
		}}
	}
	body, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		return []Diagnostic{{
			Severity: SeverityFailure,
			Code:     CodeManifestInvalid,
			Path:     rel,
			Target:   "gemini",
			Message:  fmt.Sprintf("Gemini command file %s is not readable: %v", rel, err),
		}}
	}
	return invalidGeminiCommandTOMLDiagnostics(rel, body)
}

func validateGeminiCommands(root string, rels []string) []Diagnostic {
	var diagnostics []Diagnostic
	for _, rel := range rels {
		diagnostics = append(diagnostics, validateGeminiCommandFile(root, rel)...)
	}
	return diagnostics
}

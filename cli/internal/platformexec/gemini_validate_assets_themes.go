package platformexec

import "strings"

func validateGeminiThemes(root string, rels []string) []Diagnostic {
	var diagnostics []Diagnostic
	seenNames := geminiValidationTracker{}
	for _, rel := range rels {
		name, ok, fileDiagnostics := readGeminiThemeNameForValidation(root, rel)
		diagnostics = append(diagnostics, fileDiagnostics...)
		if !ok {
			continue
		}
		diagnostics = append(diagnostics, seenNames.duplicateDiagnostic(rel, name, "theme", "theme name")...)
	}
	return diagnostics
}

func readGeminiThemeNameForValidation(root, rel string) (string, bool, []Diagnostic) {
	_, raw, err := readGeminiYAMLMap(root, rel)
	if err != nil {
		return "", false, []Diagnostic{invalidGeminiAssetYAMLDiagnostic("theme", rel, err)}
	}
	if message := validateGeminiThemeMap(rel, raw); message != "" {
		return "", false, []Diagnostic{invalidGeminiAssetContractDiagnostic("theme", rel, message)}
	}
	name, _ := raw["name"].(string)
	return strings.TrimSpace(name), true, nil
}

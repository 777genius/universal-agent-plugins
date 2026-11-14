package platformexec

func validateGeminiSettings(root string, rels []string) []Diagnostic {
	var diagnostics []Diagnostic
	seenNames := geminiValidationTracker{}
	seenEnvVars := geminiValidationTracker{}
	for _, rel := range rels {
		setting, ok, fileDiagnostics := readGeminiSettingForValidation(root, rel)
		diagnostics = append(diagnostics, fileDiagnostics...)
		if !ok {
			continue
		}
		diagnostics = append(diagnostics, seenNames.duplicateDiagnostic(rel, setting.Name, "setting", "name")...)
		diagnostics = append(diagnostics, seenEnvVars.duplicateDiagnostic(rel, setting.EnvVar, "setting", "env_var")...)
	}
	return diagnostics
}

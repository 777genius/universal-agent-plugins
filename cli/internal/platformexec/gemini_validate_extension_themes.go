package platformexec

import "github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"

func validateGeminiExtensionThemeContracts(root string, state pluginmodel.TargetState, extension importedGeminiExtension) ([]Diagnostic, error) {
	themes, err := loadGeminiThemes(root, state.ComponentPaths("themes"))
	if err != nil {
		return nil, err
	}
	if len(themes) == 0 {
		return validateGeminiExtensionUnexpectedThemes(extension), nil
	}
	if jsonDocumentsEqual(themes, extension.Themes) {
		return nil, nil
	}
	return []Diagnostic{geminiExtensionThemesMismatchDiagnostic()}, nil
}

func validateGeminiExtensionUnexpectedThemes(extension importedGeminiExtension) []Diagnostic {
	if len(extension.Themes) == 0 {
		return nil
	}
	return []Diagnostic{{
		Severity: SeverityFailure,
		Code:     CodeGeneratedContractInvalid,
		Path:     "gemini-extension.json",
		Target:   "gemini",
		Message:  "Gemini extension manifest gemini-extension.json may not define themes when targets/gemini/themes/** is absent",
	}}
}

func geminiExtensionThemesMismatchDiagnostic() Diagnostic {
	return Diagnostic{
		Severity: SeverityFailure,
		Code:     CodeGeneratedContractInvalid,
		Path:     "gemini-extension.json",
		Target:   "gemini",
		Message:  "Gemini extension manifest gemini-extension.json themes do not match authored targets/gemini/themes/**",
	}
}

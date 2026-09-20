package platformexec

import "github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"

func validateGeminiExtensionSettingsContracts(root string, state pluginmodel.TargetState, extension importedGeminiExtension) ([]Diagnostic, error) {
	settings, err := loadGeminiSettings(root, state.ComponentPaths("settings"))
	if err != nil {
		return nil, err
	}
	if len(settings) == 0 {
		return validateGeminiExtensionUnexpectedSettings(extension), nil
	}
	if jsonDocumentsEqual(settings, extension.Settings) {
		return nil, nil
	}
	return []Diagnostic{geminiExtensionSettingsMismatchDiagnostic()}, nil
}

func validateGeminiExtensionUnexpectedSettings(extension importedGeminiExtension) []Diagnostic {
	if len(extension.Settings) == 0 {
		return nil
	}
	return []Diagnostic{{
		Severity: SeverityFailure,
		Code:     CodeGeneratedContractInvalid,
		Path:     "gemini-extension.json",
		Target:   "gemini",
		Message:  "Gemini extension manifest gemini-extension.json may not define settings when targets/gemini/settings/** is absent",
	}}
}

func geminiExtensionSettingsMismatchDiagnostic() Diagnostic {
	return Diagnostic{
		Severity: SeverityFailure,
		Code:     CodeGeneratedContractInvalid,
		Path:     "gemini-extension.json",
		Target:   "gemini",
		Message:  "Gemini extension manifest gemini-extension.json settings do not match authored targets/gemini/settings/**",
	}
}

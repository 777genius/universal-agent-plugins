package platformexec

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

func readGeminiSettingForValidation(root, rel string) (geminiSetting, bool, []Diagnostic) {
	body, raw, err := readGeminiYAMLMap(root, rel)
	if err != nil {
		return geminiSetting{}, false, []Diagnostic{invalidGeminiAssetYAMLDiagnostic("setting", rel, err)}
	}
	var setting geminiSetting
	if err := yaml.Unmarshal(body, &setting); err != nil {
		return geminiSetting{}, false, []Diagnostic{invalidGeminiAssetYAMLDiagnostic("setting", rel, err)}
	}
	if message := validateGeminiSettingMap(raw, setting); message != "" {
		return geminiSetting{}, false, []Diagnostic{invalidGeminiAssetContractDiagnostic("setting", rel, message)}
	}
	return setting, true, nil
}

func invalidGeminiAssetYAMLDiagnostic(kind, rel string, err error) Diagnostic {
	return Diagnostic{
		Severity: SeverityFailure,
		Code:     CodeManifestInvalid,
		Path:     rel,
		Target:   "gemini",
		Message:  fmt.Sprintf("Gemini %s file %s is invalid YAML: %v", kind, rel, err),
	}
}

func invalidGeminiAssetContractDiagnostic(kind, rel, message string) Diagnostic {
	return Diagnostic{
		Severity: SeverityFailure,
		Code:     CodeManifestInvalid,
		Path:     rel,
		Target:   "gemini",
		Message:  fmt.Sprintf("Gemini %s file %s: %s", kind, rel, message),
	}
}

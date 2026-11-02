package platformexec

import "fmt"

func validateQoderManifestRead(root string) (map[string]any, []Diagnostic) {
	manifest, ok, err := readImportedQoderPluginManifest(root)
	if err != nil {
		return nil, []Diagnostic{qoderPluginManifestDiagnostic(CodeManifestInvalid, fmt.Sprintf("Qoder plugin manifest %s is invalid: %v", qoderPluginManifestPath, err))}
	}
	if !ok {
		return nil, []Diagnostic{qoderPluginManifestDiagnostic(CodeGeneratedContractInvalid, fmt.Sprintf("Qoder plugin manifest %s is not readable", qoderPluginManifestPath))}
	}
	return manifest, nil
}

func validateQoderManifestRequiredKeys(manifest map[string]any) []Diagnostic {
	if stringMapField(manifest, "name") == "" {
		return []Diagnostic{qoderPluginManifestDiagnostic(
			CodeGeneratedContractInvalid,
			fmt.Sprintf("Qoder plugin manifest %s must define required field %q", qoderPluginManifestPath, "name"),
		)}
	}
	return nil
}

func qoderPluginManifestDiagnostic(code string, message string) Diagnostic {
	return Diagnostic{
		Severity: SeverityFailure,
		Code:     code,
		Path:     qoderPluginManifestPath,
		Target:   "qoder",
		Message:  message,
	}
}

package platformexec

import "fmt"

func validateCursorManifestRead(root string) (map[string]any, []Diagnostic) {
	manifest, ok, err := readImportedCursorPluginManifest(root)
	if err != nil {
		return nil, []Diagnostic{cursorPluginManifestDiagnostic(CodeManifestInvalid, fmt.Sprintf("Cursor plugin manifest %s is invalid: %v", cursorPluginManifestPath, err))}
	}
	if !ok {
		return nil, []Diagnostic{cursorPluginManifestDiagnostic(CodeGeneratedContractInvalid, fmt.Sprintf("Cursor plugin manifest %s is not readable", cursorPluginManifestPath))}
	}
	return manifest, nil
}

func cursorPluginManifestDiagnostic(code string, message string) Diagnostic {
	return Diagnostic{
		Severity: SeverityFailure,
		Code:     code,
		Path:     cursorPluginManifestPath,
		Target:   "cursor",
		Message:  message,
	}
}

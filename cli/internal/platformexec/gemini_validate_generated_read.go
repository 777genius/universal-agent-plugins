package platformexec

func readGeminiGeneratedExtension(root string) (importedGeminiExtension, bool, []Diagnostic) {
	extension, ok, err := readImportedGeminiExtension(root)
	if err != nil {
		return importedGeminiExtension{}, false, invalidGeminiGeneratedExtensionDiagnostics(err)
	}
	if !ok {
		return importedGeminiExtension{}, false, missingGeminiGeneratedExtensionDiagnostics()
	}
	return extension, true, nil
}

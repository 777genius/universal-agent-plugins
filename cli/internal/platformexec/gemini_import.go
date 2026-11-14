package platformexec

func (geminiAdapter) Import(root string, seed ImportSeed) (ImportResult, error) {
	result := ImportResult{
		Manifest: seed.Manifest,
		Launcher: seed.Launcher,
	}
	if err := appendImportedGeminiHooks(root, &result); err != nil {
		return ImportResult{}, err
	}
	if err := appendImportedGeminiDirs(root, &result); err != nil {
		return ImportResult{}, err
	}
	data, ok, err := readImportedGeminiExtension(root)
	if err != nil {
		return ImportResult{}, err
	}
	if ok {
		if err := appendImportedGeminiExtensionArtifacts(root, data, &result); err != nil {
			return ImportResult{}, err
		}
	}
	result.Artifacts = compactArtifacts(result.Artifacts)
	return result, nil
}

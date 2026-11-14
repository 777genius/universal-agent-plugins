package app

import "os"

func createBundlePublishExportPath() (string, func(), error) {
	tmpFile, err := os.CreateTemp("", ".plugin-kit-ai-publish-*.tar.gz")
	if err != nil {
		return "", nil, err
	}
	exportPath := tmpFile.Name()
	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(exportPath)
		return "", nil, err
	}
	return exportPath, func() { _ = os.Remove(exportPath) }, nil
}

func exportBundlePublishArchive(root, platform, exportPath string, deps bundlePublishDeps) (exportMetadata, []byte, error) {
	if _, err := deps.Export(PluginExportOptions{
		Root:     root,
		Platform: platform,
		Output:   exportPath,
	}); err != nil {
		return exportMetadata{}, nil, err
	}
	metadata, err := inspectBundleArchive(exportPath)
	if err != nil {
		return exportMetadata{}, nil, err
	}
	if err := validateBundleMetadata(metadata); err != nil {
		return exportMetadata{}, nil, err
	}
	body, err := os.ReadFile(exportPath)
	if err != nil {
		return exportMetadata{}, nil, err
	}
	return metadata, body, nil
}

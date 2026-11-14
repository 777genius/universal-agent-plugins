package app

func loadBundleInstallMetadata(archivePath string) (exportMetadata, error) {
	metadata, err := inspectBundleArchive(archivePath)
	if err != nil {
		return exportMetadata{}, err
	}
	if err := validateBundleMetadata(metadata); err != nil {
		return exportMetadata{}, err
	}
	return metadata, nil
}

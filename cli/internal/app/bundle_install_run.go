package app

func runBundleInstall(input bundleInstallInput) (exportMetadata, string, error) {
	metadata, err := loadBundleInstallMetadata(input.archivePath)
	if err != nil {
		return exportMetadata{}, "", err
	}
	installedPath, err := installBundleArchive(input.archivePath, input.dest, input.force)
	if err != nil {
		return exportMetadata{}, "", err
	}
	return metadata, installedPath, nil
}

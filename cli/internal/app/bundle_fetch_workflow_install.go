package app

func installFetchedBundleSource(source bundleRemoteSource, opts PluginBundleFetchOptions) (exportMetadata, string, error) {
	archivePath, cleanup, err := prepareFetchedBundleArchive(source)
	if err != nil {
		return exportMetadata{}, "", err
	}
	defer cleanup()

	metadata, err := loadFetchedBundleMetadata(archivePath, opts)
	if err != nil {
		return exportMetadata{}, "", err
	}
	installedPath, err := applyFetchedBundleInstall(archivePath, opts)
	if err != nil {
		return exportMetadata{}, "", err
	}
	return metadata, installedPath, nil
}

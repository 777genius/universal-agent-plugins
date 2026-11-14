package app

func loadFetchedBundleMetadata(archivePath string, opts PluginBundleFetchOptions) (exportMetadata, error) {
	metadata, err := inspectBundleArchive(archivePath)
	if err != nil {
		return exportMetadata{}, err
	}
	if err := validateBundleMetadata(metadata); err != nil {
		return exportMetadata{}, err
	}
	if err := validateFetchedBundleMatchesRequest(metadata, opts); err != nil {
		return exportMetadata{}, err
	}
	return metadata, nil
}

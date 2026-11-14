package app

func prepareFetchedBundleArchive(source bundleRemoteSource) (string, func(), error) {
	return writeTempBundleArchive(source.ArchiveBytes)
}

func applyFetchedBundleInstall(archivePath string, opts PluginBundleFetchOptions) (string, error) {
	return installBundleArchive(archivePath, opts.Dest, opts.Force)
}

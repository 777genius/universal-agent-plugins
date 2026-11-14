package app

type bundlePublishArtifact struct {
	Metadata    exportMetadata
	Body        []byte
	BundleName  string
	SidecarName string
	SidecarBody []byte
}

func prepareBundlePublishArtifact(root, platform string, deps bundlePublishDeps) (bundlePublishArtifact, error) {
	exportPath, cleanup, err := createBundlePublishExportPath()
	if err != nil {
		return bundlePublishArtifact{}, err
	}
	defer cleanup()

	metadata, body, err := exportBundlePublishArchive(root, platform, exportPath, deps)
	if err != nil {
		return bundlePublishArtifact{}, err
	}
	return buildBundlePublishArtifact(metadata, body), nil
}

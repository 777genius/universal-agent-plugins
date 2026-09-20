package app

type PluginBundleInstallOptions struct {
	Archive string
	Dest    string
	Force   bool
}

type PluginBundleInstallResult struct {
	Lines []string
}

func (PluginService) BundleInstall(opts PluginBundleInstallOptions) (PluginBundleInstallResult, error) {
	input, err := resolveBundleInstallInput(opts)
	if err != nil {
		return PluginBundleInstallResult{}, err
	}
	metadata, installedPath, err := runBundleInstall(input)
	if err != nil {
		return PluginBundleInstallResult{}, err
	}
	return buildBundleInstallResult(metadata, input.archivePath, installedPath), nil
}

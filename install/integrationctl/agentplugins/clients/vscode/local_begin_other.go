//go:build !(darwin && arm64)

package vscode

import "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"

func beginLocalProfile(kernel nativeconfig.Kernel, path string) (*nativeconfig.ExactFile, error) {
	return kernel.BeginExactFile(path)
}

func checkLocalProfileMetadata(_ *nativeconfig.ExactFile, path string) error {
	return localWritableMetadata(path)
}

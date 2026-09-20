package platformexec

import (
	"fmt"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/codexmanifest"
	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func validateImportedCodexPackageBundle(root string, pluginManifest importedCodexPluginManifest) error {
	if unexpected := codexmanifest.UnexpectedBundleSidecars(root, pluginManifest); len(unexpected) > 0 {
		return fmt.Errorf("Codex package bundle contains unexpected sidecar artifacts without matching plugin.json refs: %s", strings.Join(unexpected, ", "))
	}
	return nil
}

func mergeImportedCodexPackageManifest(seed pluginmodel.Manifest, pluginManifest importedCodexPluginManifest) pluginmodel.Manifest {
	if strings.TrimSpace(pluginManifest.Name) != "" {
		seed.Name = pluginManifest.Name
	}
	if strings.TrimSpace(pluginManifest.Version) != "" {
		seed.Version = pluginManifest.Version
	}
	if strings.TrimSpace(pluginManifest.Description) != "" {
		seed.Description = pluginManifest.Description
	}
	return seed
}

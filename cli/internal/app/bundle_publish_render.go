package app

import (
	"fmt"

	"github.com/777genius/plugin-kit-ai/plugininstall/domain"
)

func buildBundlePublishResult(input bundlePublishInput, artifact bundlePublishArtifact, _ *domain.Release, state string) PluginBundlePublishResult {
	releaseLabel := input.owner + "/" + input.repo + "@" + input.tag
	lines := []string{
		fmt.Sprintf("Bundle: plugin=%s platform=%s runtime=%s manager=%s", artifact.Metadata.PluginName, artifact.Metadata.Platform, artifact.Metadata.Runtime, displayBundleManager(artifact.Metadata.Manager)),
		"Release: " + releaseLabel,
		"Release state: " + state,
		"Uploaded assets:",
		"  " + artifact.BundleName,
		"  " + artifact.SidecarName,
		"Next:",
		fmt.Sprintf("  plugin-kit-ai bundle fetch %s --tag %s --platform %s --runtime %s --dest <path>", input.ref, input.tag, artifact.Metadata.Platform, artifact.Metadata.Runtime),
		fmt.Sprintf("  plugin-kit-ai bundle install ./%s --dest <path>", artifact.BundleName),
	}
	return PluginBundlePublishResult{Lines: lines}
}

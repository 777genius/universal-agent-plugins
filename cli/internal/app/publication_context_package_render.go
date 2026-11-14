package app

import "github.com/777genius/plugin-kit-ai/cli/internal/pluginmanifest"

func buildExpectedMaterializedPackageArtifacts(ctx publicationContext, managedPaths []string, generated pluginmanifest.RenderResult) ([]pluginmanifest.Artifact, error) {
	return materializedPackageArtifacts(ctx.root, ctx.packageRoot, managedPaths, generated)
}

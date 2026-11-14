package app

import "github.com/777genius/plugin-kit-ai/cli/internal/publishschema"

func discoverPublicationContextState(ctx publicationContext) (publishschema.State, error) {
	return publishschema.DiscoverInLayout(ctx.root, ctx.inspection.Layout.AuthoredRoot)
}

func resolvePublicationContextPackageRoot(ctx publicationContext, packageRootInput string) (string, error) {
	return normalizePackageRoot(packageRootInput, ctx.graph.Manifest.Name)
}

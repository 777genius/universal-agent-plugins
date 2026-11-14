package app

import (
	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmanifest"
	"github.com/777genius/plugin-kit-ai/cli/internal/publicationexec"
)

type artifactResult = pluginmanifest.Artifact

func renderPublicationContextCatalogArtifact(ctx publicationContext) (pluginmanifest.Artifact, error) {
	return publicationexec.RenderLocalCatalogArtifact(ctx.graph, ctx.publicationState, ctx.target, "./"+ctx.packageRoot)
}

func publicationContextCatalogPath(ctx publicationContext) (string, error) {
	return publicationexec.CatalogArtifactPath(ctx.target)
}

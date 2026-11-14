package app

import "github.com/777genius/plugin-kit-ai/cli/internal/pluginmanifest"

func preparePublicationMaterializePackageArtifacts(ctx publicationContext) ([]pluginmanifest.Artifact, pluginmanifest.RenderResult, error) {
	return ctx.expectedMaterializedPackageArtifacts()
}

func preparePublicationMaterializeCatalog(ctx publicationContext) (pluginmanifest.Artifact, []byte, error) {
	catalogArtifact, err := ctx.renderLocalCatalogArtifact()
	if err != nil {
		return pluginmanifest.Artifact{}, nil, err
	}
	mergedCatalog, err := mergeCatalogAtDestination(ctx.dest, ctx.target, catalogArtifact)
	if err != nil {
		return pluginmanifest.Artifact{}, nil, err
	}
	return catalogArtifact, mergedCatalog, nil
}

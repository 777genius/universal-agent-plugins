package app

func (ctx publicationContext) renderLocalCatalogArtifact() (artifactResult, error) {
	return renderPublicationContextCatalogArtifact(ctx)
}

func (ctx publicationContext) catalogArtifactPath() (string, error) {
	return publicationContextCatalogPath(ctx)
}

func (ctx publicationContext) destPackageRoot() string {
	return publicationContextDestPackageRoot(ctx)
}

package app

import "github.com/777genius/plugin-kit-ai/cli/internal/pluginmanifest"

func detectPublicationMaterializeActions(ctx publicationContext, catalogArtifact pluginmanifest.Artifact) (string, string, error) {
	packageRootAction, err := detectMaterializePackageRootAction(ctx)
	if err != nil {
		return "", "", err
	}
	catalogAction, err := detectMaterializeCatalogAction(ctx, catalogArtifact)
	if err != nil {
		return "", "", err
	}
	return packageRootAction, catalogAction, nil
}

func buildPublicationMaterializePlan(packageFiles []pluginmanifest.Artifact, generated pluginmanifest.RenderResult, catalogArtifact pluginmanifest.Artifact, mergedCatalog []byte, packageRootAction, catalogAction string) publicationMaterializePlan {
	return publicationMaterializePlan{
		packageFiles:       packageFiles,
		generated:          generated,
		catalogArtifact:    catalogArtifact,
		mergedCatalog:      mergedCatalog,
		packageRootAction:  packageRootAction,
		catalogArtifactAct: catalogAction,
	}
}

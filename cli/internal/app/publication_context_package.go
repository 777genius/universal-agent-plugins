package app

import "github.com/777genius/plugin-kit-ai/cli/internal/pluginmanifest"

func (ctx publicationContext) expectedMaterializedPackageArtifacts() ([]pluginmanifest.Artifact, pluginmanifest.RenderResult, error) {
	generated, err := pluginmanifest.Generate(ctx.root, ctx.target)
	if err != nil {
		return nil, pluginmanifest.RenderResult{}, err
	}
	managedPaths, err := resolvePublicationManagedPaths(ctx)
	if err != nil {
		return nil, pluginmanifest.RenderResult{}, err
	}
	packageFiles, err := buildExpectedMaterializedPackageArtifacts(ctx, managedPaths, generated)
	if err != nil {
		return nil, pluginmanifest.RenderResult{}, err
	}
	return packageFiles, generated, nil
}

package app

import "github.com/777genius/plugin-kit-ai/cli/internal/pluginmanifest"

type publicationVerifyInputs struct {
	expectedPackageFiles []pluginmanifest.Artifact
	catalogRel           string
	generatedCatalog     []byte
}

func resolvePublicationVerifyInputs(ctx publicationContext) (publicationVerifyInputs, error) {
	expectedPackageFiles, _, err := ctx.expectedMaterializedPackageArtifacts()
	if err != nil {
		return publicationVerifyInputs{}, err
	}
	catalogArtifact, err := ctx.renderLocalCatalogArtifact()
	if err != nil {
		return publicationVerifyInputs{}, err
	}
	catalogRel, err := ctx.catalogArtifactPath()
	if err != nil {
		return publicationVerifyInputs{}, err
	}
	return publicationVerifyInputs{
		expectedPackageFiles: expectedPackageFiles,
		catalogRel:           catalogRel,
		generatedCatalog:     catalogArtifact.Content,
	}, nil
}

func buildPublicationVerifyPlan(ctx publicationContext, inputs publicationVerifyInputs) (publicationVerifyPlan, error) {
	issues, err := collectPublicationVerifyIssues(ctx, inputs.expectedPackageFiles, inputs.catalogRel, inputs.generatedCatalog)
	if err != nil {
		return publicationVerifyPlan{}, err
	}
	return publicationVerifyPlan{
		catalogRel: inputs.catalogRel,
		issues:     issues,
	}, nil
}

func collectPublicationVerifyIssues(ctx publicationContext, expectedPackageFiles []pluginmanifest.Artifact, catalogRel string, generatedCatalog []byte) ([]PluginPublicationRootIssue, error) {
	issues, err := collectPublicationVerifyPackageIssues(ctx, expectedPackageFiles)
	if err != nil {
		return nil, err
	}
	catalogIssues, err := collectPublicationVerifyCatalogIssues(ctx, catalogRel, generatedCatalog)
	if err != nil {
		return nil, err
	}
	return append(issues, catalogIssues...), nil
}

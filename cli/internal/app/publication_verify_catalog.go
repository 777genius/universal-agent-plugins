package app

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/cli/internal/publicationexec"
)

func collectPublicationVerifyCatalogIssues(ctx publicationContext, catalogRel string, generatedCatalog []byte) ([]PluginPublicationRootIssue, error) {
	catalogFull := filepath.Join(ctx.dest, filepath.FromSlash(catalogRel))
	existing, err := os.ReadFile(catalogFull)
	if err == nil {
		return diagnosePublicationVerifyCatalogIssues(ctx, existing, generatedCatalog)
	}
	if os.IsNotExist(err) {
		return missingPublicationVerifyCatalogArtifactIssue(catalogRel), nil
	}
	return nil, err
}

func diagnosePublicationVerifyCatalogIssues(ctx publicationContext, existing, generatedCatalog []byte) ([]PluginPublicationRootIssue, error) {
	catalogIssues, err := publicationexec.DiagnoseCatalogArtifact(ctx.target, existing, generatedCatalog, ctx.graph.Manifest.Name)
	if err != nil {
		return nil, err
	}
	issues := make([]PluginPublicationRootIssue, 0, len(catalogIssues))
	for _, issue := range catalogIssues {
		issues = append(issues, PluginPublicationRootIssue{
			Code:    issue.Code,
			Path:    issue.Path,
			Message: issue.Message,
		})
	}
	return issues, nil
}

func missingPublicationVerifyCatalogArtifactIssue(catalogRel string) []PluginPublicationRootIssue {
	return []PluginPublicationRootIssue{{
		Code:    "missing_materialized_catalog_artifact",
		Path:    catalogRel,
		Message: fmt.Sprintf("materialized catalog artifact %s is missing", catalogRel),
	}}
}

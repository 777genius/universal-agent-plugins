package app

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmanifest"
)

func collectPublicationVerifyPackageIssues(ctx publicationContext, expectedPackageFiles []pluginmanifest.Artifact) ([]PluginPublicationRootIssue, error) {
	issues, err := collectPublicationVerifyPackageRootIssues(ctx)
	if err != nil {
		return nil, err
	}
	artifactIssues, err := collectPublicationVerifyPackageArtifactIssues(ctx, expectedPackageFiles)
	if err != nil {
		return nil, err
	}
	return append(issues, artifactIssues...), nil
}

func collectPublicationVerifyPackageRootIssues(ctx publicationContext) ([]PluginPublicationRootIssue, error) {
	if info, err := os.Stat(ctx.destPackageRoot()); err == nil && info.IsDir() {
		return nil, nil
	}
	return []PluginPublicationRootIssue{{
		Code:    "missing_materialized_package_root",
		Path:    ctx.packageRoot,
		Message: fmt.Sprintf("materialized package root %s is missing", ctx.packageRoot),
	}}, nil
}

func collectPublicationVerifyPackageArtifactIssues(ctx publicationContext, expectedPackageFiles []pluginmanifest.Artifact) ([]PluginPublicationRootIssue, error) {
	var issues []PluginPublicationRootIssue
	for _, artifact := range expectedPackageFiles {
		if _, err := os.Stat(filepath.Join(ctx.dest, filepath.FromSlash(artifact.RelPath))); err != nil {
			if os.IsNotExist(err) {
				issues = append(issues, PluginPublicationRootIssue{
					Code:    "missing_materialized_package_artifact",
					Path:    artifact.RelPath,
					Message: fmt.Sprintf("materialized package artifact %s is missing", artifact.RelPath),
				})
				continue
			}
			return nil, err
		}
	}
	return issues, nil
}

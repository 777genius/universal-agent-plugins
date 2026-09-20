package app

import (
	"os"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmanifest"
)

func applyPublicationMaterialize(ctx publicationContext, plan publicationMaterializePlan, dryRun bool) error {
	if dryRun {
		return nil
	}
	if err := os.RemoveAll(ctx.destPackageRoot()); err != nil {
		return err
	}
	if err := pluginmanifest.WriteArtifacts(ctx.dest, plan.packageFiles); err != nil {
		return err
	}
	return pluginmanifest.WriteArtifacts(ctx.dest, []pluginmanifest.Artifact{{
		RelPath: plan.catalogArtifact.RelPath,
		Content: plan.mergedCatalog,
	}})
}

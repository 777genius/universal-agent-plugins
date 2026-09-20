package app

import (
	"os"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmanifest"
)

func applyPublicationRemove(ctx publicationContext, plan publicationRemovePlan, dryRun bool) error {
	if dryRun {
		return nil
	}
	if plan.removedPackage {
		if err := os.RemoveAll(ctx.destPackageRoot()); err != nil {
			return err
		}
	}
	if plan.removedCatalogEntry {
		return pluginmanifest.WriteArtifacts(ctx.dest, []pluginmanifest.Artifact{{
			RelPath: plan.catalogRel,
			Content: plan.updatedCatalog,
		}})
	}
	return nil
}

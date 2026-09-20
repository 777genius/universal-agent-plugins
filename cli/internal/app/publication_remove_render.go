package app

import (
	"fmt"
	"path/filepath"
)

func buildPublicationRemoveResult(ctx publicationContext, plan publicationRemovePlan, dryRun bool) PluginPublicationRemoveResult {
	lines := []string{
		fmt.Sprintf("Removed publication target: %s", ctx.target),
		fmt.Sprintf("Marketplace family: %s", ctx.channel.Family),
		fmt.Sprintf("Marketplace root: %s", filepath.Clean(ctx.dest)),
		fmt.Sprintf("Package root: %s", ctx.packageRoot),
		fmt.Sprintf("Mode: %s", publicationModeLabel(dryRun)),
	}
	if plan.removedPackage {
		lines = append(lines, "Package root action: remove")
	} else {
		lines = append(lines, "Package root action: no existing package root")
	}
	if plan.removedCatalogEntry {
		lines = append(lines, fmt.Sprintf("Catalog artifact action: prune %s", plan.catalogRel))
	} else {
		lines = append(lines, fmt.Sprintf("Catalog artifact action: no matching %q entry was present", ctx.graph.Manifest.Name))
	}
	lines = append(lines,
		"Next:",
		fmt.Sprintf("  plugin-kit-ai publication doctor %s --target %s --dest %s", ctx.root, ctx.target, ctx.dest),
		fmt.Sprintf("  review %s from the marketplace root if you keep additional plugins there", plan.catalogRel),
	)
	return PluginPublicationRemoveResult{Lines: lines}
}

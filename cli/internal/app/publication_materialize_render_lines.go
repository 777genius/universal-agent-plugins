package app

import (
	"fmt"
	"path/filepath"
)

func buildPublicationMaterializeLines(ctx publicationContext, plan publicationMaterializePlan, dryRun bool, nextSteps []string) []string {
	lines := []string{
		fmt.Sprintf("Materialized publication target: %s", ctx.target),
		fmt.Sprintf("Marketplace family: %s", ctx.channel.Family),
		fmt.Sprintf("Marketplace root: %s", filepath.Clean(ctx.dest)),
		fmt.Sprintf("Package root: %s", ctx.packageRoot),
		fmt.Sprintf("Mode: %s", publicationModeLabel(dryRun)),
		fmt.Sprintf("Package root action: %s", plan.packageRootAction),
		fmt.Sprintf("Package files: %d", len(plan.packageFiles)),
		fmt.Sprintf("Catalog artifact action: %s %s", plan.catalogArtifactAct, plan.catalogArtifact.RelPath),
	}
	if len(plan.generated.StalePaths) > 0 {
		lines = append(lines, fmt.Sprintf("Source generate drift observed: %d stale managed path(s) were bypassed by materializing fresh generated outputs", len(plan.generated.StalePaths)))
	}
	lines = append(lines, "Next:")
	for _, step := range nextSteps {
		lines = append(lines, "  "+step)
	}
	return lines
}

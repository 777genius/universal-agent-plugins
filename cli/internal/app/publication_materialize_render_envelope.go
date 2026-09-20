package app

import "path/filepath"

func buildPublicationMaterializeEnvelope(ctx publicationContext, plan publicationMaterializePlan, dryRun bool, nextSteps, lines []string) PluginPublicationMaterializeResult {
	return PluginPublicationMaterializeResult{
		Target:            ctx.target,
		Mode:              publicationModeLabel(dryRun),
		MarketplaceFamily: ctx.channel.Family,
		Dest:              filepath.Clean(ctx.dest),
		PackageRoot:       ctx.packageRoot,
		Details:           buildPublicationMaterializeDetails(plan),
		NextSteps:         nextSteps,
		Lines:             lines,
	}
}

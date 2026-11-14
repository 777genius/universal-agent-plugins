package app

func buildPublicationMaterializeResult(ctx publicationContext, plan publicationMaterializePlan, dryRun bool) PluginPublicationMaterializeResult {
	nextSteps := buildPublicationMaterializeNextSteps(ctx)
	lines := buildPublicationMaterializeLines(ctx, plan, dryRun, nextSteps)
	return buildPublicationMaterializeEnvelope(ctx, plan, dryRun, nextSteps, lines)
}

package app

import "path/filepath"

func buildPublicationVerifyRootResult(ctx publicationContext, plan publicationVerifyPlan) PluginPublicationVerifyRootResult {
	status := buildPublicationVerifyRootStatus(ctx, plan)
	lines := buildPublicationVerifyRootLines(ctx, plan, status)
	return buildPublicationVerifyRootResultEnvelope(ctx, plan, status, lines)
}

func buildPublicationVerifyRootResultEnvelope(ctx publicationContext, plan publicationVerifyPlan, status publicationVerifyStatus, lines []string) PluginPublicationVerifyRootResult {
	return PluginPublicationVerifyRootResult{
		Ready:       status.ready,
		Status:      status.label,
		Dest:        filepath.Clean(ctx.dest),
		PackageRoot: ctx.packageRoot,
		CatalogPath: plan.catalogRel,
		IssueCount:  len(plan.issues),
		Issues:      plan.issues,
		NextSteps:   status.nextSteps,
		Lines:       lines,
	}
}

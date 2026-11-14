package app

func buildPublicationVerifyRootLines(ctx publicationContext, plan publicationVerifyPlan, status publicationVerifyStatus) []string {
	lines := basePublicationVerifyRootLines(ctx, plan)
	if status.ready {
		return appendPublicationVerifyReadyLines(lines)
	}
	lines = appendPublicationVerifyIssueLines(lines, plan.issues)
	return appendPublicationVerifyNeedsSyncLines(lines, status.nextSteps)
}

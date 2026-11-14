package app

import "fmt"

type publicationVerifyStatus struct {
	ready     bool
	label     string
	nextSteps []string
}

func buildPublicationVerifyRootStatus(ctx publicationContext, plan publicationVerifyPlan) publicationVerifyStatus {
	if len(plan.issues) == 0 {
		return buildReadyPublicationVerifyStatus()
	}
	return buildNeedsSyncPublicationVerifyStatus(ctx)
}

func buildReadyPublicationVerifyStatus() publicationVerifyStatus {
	return publicationVerifyStatus{
		ready: true,
		label: "ready",
	}
}

func buildNeedsSyncPublicationVerifyStatus(ctx publicationContext) publicationVerifyStatus {
	return publicationVerifyStatus{
		ready:     false,
		label:     "needs_sync",
		nextSteps: buildPublicationVerifyNextSteps(ctx),
	}
}

func buildPublicationVerifyNextSteps(ctx publicationContext) []string {
	return []string{
		fmt.Sprintf("run plugin-kit-ai publication materialize %s --target %s --dest %s", ctx.root, ctx.target, ctx.dest),
	}
}

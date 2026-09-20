package app

import "fmt"

func buildPublicationMaterializeDetails(plan publicationMaterializePlan) map[string]string {
	return map[string]string{
		"package_root_action":     plan.packageRootAction,
		"package_file_count":      fmt.Sprintf("%d", len(plan.packageFiles)),
		"catalog_artifact":        plan.catalogArtifact.RelPath,
		"catalog_artifact_action": plan.catalogArtifactAct,
	}
}

func buildPublicationMaterializeNextSteps(ctx publicationContext) []string {
	return []string{
		fmt.Sprintf("plugin-kit-ai publication doctor %s", ctx.root),
		fmt.Sprintf("plugin-kit-ai publication doctor %s --target %s --dest %s", ctx.root, ctx.target, ctx.dest),
		fmt.Sprintf("inspect %s with the vendor CLI from the marketplace root", ctx.channel.Family),
	}
}

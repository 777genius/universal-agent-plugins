package app

func executePublicationMaterialize(ctx publicationContext, plan publicationMaterializePlan, dryRun bool) (PluginPublicationMaterializeResult, error) {
	if err := applyPublicationMaterialize(ctx, plan, dryRun); err != nil {
		return PluginPublicationMaterializeResult{}, err
	}
	return buildPublicationMaterializeResult(ctx, plan, dryRun), nil
}

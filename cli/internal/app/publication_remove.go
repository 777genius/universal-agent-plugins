package app

func (PluginService) publicationRemove(opts PluginPublicationRemoveOptions) (PluginPublicationRemoveResult, error) {
	ctx, err := loadPublicationContextForRemove(opts)
	if err != nil {
		return PluginPublicationRemoveResult{}, err
	}
	plan, err := preparePublicationRemove(ctx)
	if err != nil {
		return PluginPublicationRemoveResult{}, err
	}
	if err := applyPublicationRemove(ctx, plan, opts.DryRun); err != nil {
		return PluginPublicationRemoveResult{}, err
	}
	return buildPublicationRemoveResult(ctx, plan, opts.DryRun), nil
}

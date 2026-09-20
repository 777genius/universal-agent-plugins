package app

func (PluginService) publicationMaterialize(opts PluginPublicationMaterializeOptions) (PluginPublicationMaterializeResult, error) {
	ctx, plan, err := loadPreparedPublicationMaterialize(opts)
	if err != nil {
		return PluginPublicationMaterializeResult{}, err
	}
	return executePublicationMaterialize(ctx, plan, opts.DryRun)
}

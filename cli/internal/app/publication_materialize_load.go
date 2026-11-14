package app

func loadPreparedPublicationMaterialize(opts PluginPublicationMaterializeOptions) (publicationContext, publicationMaterializePlan, error) {
	ctx, err := loadPublicationContextForMaterialize(opts)
	if err != nil {
		return publicationContext{}, publicationMaterializePlan{}, err
	}
	plan, err := preparePublicationMaterialize(ctx)
	if err != nil {
		return publicationContext{}, publicationMaterializePlan{}, err
	}
	return ctx, plan, nil
}

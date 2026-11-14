package app

func (PluginService) publicationVerifyRoot(opts PluginPublicationVerifyRootOptions) (PluginPublicationVerifyRootResult, error) {
	ctx, err := loadPublicationContextForVerify(opts)
	if err != nil {
		return PluginPublicationVerifyRootResult{}, err
	}
	plan, err := preparePublicationVerifyRoot(ctx)
	if err != nil {
		return PluginPublicationVerifyRootResult{}, err
	}
	return buildPublicationVerifyRootResult(ctx, plan), nil
}

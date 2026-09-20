package app

func loadPublicationContextForMaterialize(opts PluginPublicationMaterializeOptions) (publicationContext, error) {
	ctx, err := resolvePublicationBaseContext(
		opts.Root,
		opts.Target,
		opts.Dest,
		"publication materialize supports only %q or %q",
		"publication materialize requires --dest",
	)
	if err != nil {
		return publicationContext{}, err
	}

	input, err := resolveMaterializePublicationContextInput(ctx, opts.PackageRoot)
	if err != nil {
		return publicationContext{}, err
	}
	return withPublicationContextMaterialize(ctx, input.packageRoot, input.publication, input.publicationState, input.channel), nil
}

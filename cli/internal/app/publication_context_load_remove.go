package app

func loadPublicationContextForRemove(opts PluginPublicationRemoveOptions) (publicationContext, error) {
	ctx, err := resolvePublicationBaseContext(
		opts.Root,
		opts.Target,
		opts.Dest,
		"publication remove supports only %q or %q",
		"publication remove requires --dest",
	)
	if err != nil {
		return publicationContext{}, err
	}
	input, err := resolveRemovePublicationContextInput(ctx, opts.PackageRoot)
	if err != nil {
		return publicationContext{}, err
	}
	return withPublicationContextRemove(ctx, input.packageRoot, input.publication, input.channel), nil
}

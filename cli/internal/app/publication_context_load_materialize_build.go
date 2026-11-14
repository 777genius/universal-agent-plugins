package app

func resolveMaterializePublicationContextInput(ctx publicationContext, packageRootInput string) (materializePublicationContextInput, error) {
	policyInput, err := resolveMaterializePublicationPolicyInput(ctx)
	if err != nil {
		return materializePublicationContextInput{}, err
	}
	packageRoot, err := resolveMaterializePublicationPackageRoot(ctx, packageRootInput)
	if err != nil {
		return materializePublicationContextInput{}, err
	}
	return buildMaterializePublicationContextInput(policyInput, packageRoot), nil
}

func resolveMaterializePublicationPackageRoot(ctx publicationContext, packageRootInput string) (string, error) {
	return resolvePublicationContextPackageRoot(ctx, packageRootInput)
}

func buildMaterializePublicationContextInput(policy materializePublicationPolicyInput, packageRoot string) materializePublicationContextInput {
	return materializePublicationContextInput{
		publication:      policy.publication,
		publicationState: policy.publicationState,
		channel:          policy.channel,
		packageRoot:      packageRoot,
	}
}

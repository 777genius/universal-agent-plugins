package app

type publicationVerifyPlan struct {
	catalogRel string
	issues     []PluginPublicationRootIssue
}

func preparePublicationVerifyRoot(ctx publicationContext) (publicationVerifyPlan, error) {
	inputs, err := resolvePublicationVerifyInputs(ctx)
	if err != nil {
		return publicationVerifyPlan{}, err
	}
	return buildPublicationVerifyPlan(ctx, inputs)
}

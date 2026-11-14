package app

import "github.com/777genius/plugin-kit-ai/cli/internal/publicationmodel"

type removePublicationContextInput struct {
	publication publicationmodel.Model
	channel     publicationmodel.Channel
	packageRoot string
}

func resolveRemovePublicationContextInput(ctx publicationContext, packageRootInput string) (removePublicationContextInput, error) {
	publication, err := requirePublicationCapableTarget(ctx)
	if err != nil {
		return removePublicationContextInput{}, err
	}
	channel, err := requireRemovePublicationChannel(ctx, publication)
	if err != nil {
		return removePublicationContextInput{}, err
	}
	packageRoot, err := resolvePublicationContextPackageRoot(ctx, packageRootInput)
	if err != nil {
		return removePublicationContextInput{}, err
	}
	return removePublicationContextInput{
		publication: publication,
		channel:     channel,
		packageRoot: packageRoot,
	}, nil
}

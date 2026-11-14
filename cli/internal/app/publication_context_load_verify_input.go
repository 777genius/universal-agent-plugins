package app

import "github.com/777genius/plugin-kit-ai/cli/internal/publishschema"

type verifyPublicationContextInput struct {
	publicationState publishschema.State
	packageRoot      string
}

func resolveVerifyPublicationContextInput(ctx publicationContext, packageRootInput string) (verifyPublicationContextInput, error) {
	publicationState, err := discoverPublicationContextState(ctx)
	if err != nil {
		return verifyPublicationContextInput{}, err
	}
	packageRoot, err := resolvePublicationContextPackageRoot(ctx, packageRootInput)
	if err != nil {
		return verifyPublicationContextInput{}, err
	}
	return buildVerifyPublicationContextInput(packageRoot, publicationState), nil
}

func buildVerifyPublicationContextInput(packageRoot string, publicationState publishschema.State) verifyPublicationContextInput {
	return verifyPublicationContextInput{
		publicationState: publicationState,
		packageRoot:      packageRoot,
	}
}

package app

import (
	"github.com/777genius/plugin-kit-ai/cli/internal/publicationmodel"
	"github.com/777genius/plugin-kit-ai/cli/internal/publishschema"
)

type materializePublicationPolicyInput struct {
	publication      publicationmodel.Model
	publicationState publishschema.State
	channel          publicationmodel.Channel
}

func resolveMaterializePublicationPolicyInput(ctx publicationContext) (materializePublicationPolicyInput, error) {
	publicationState, err := discoverPublicationContextState(ctx)
	if err != nil {
		return materializePublicationPolicyInput{}, err
	}
	publication, err := requirePublicationCapableTarget(ctx)
	if err != nil {
		return materializePublicationPolicyInput{}, err
	}
	channel, err := requireMaterializePublicationChannel(ctx, publication)
	if err != nil {
		return materializePublicationPolicyInput{}, err
	}
	return materializePublicationPolicyInput{
		publication:      publication,
		publicationState: publicationState,
		channel:          channel,
	}, nil
}

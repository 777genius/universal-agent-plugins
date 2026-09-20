package app

import "github.com/777genius/plugin-kit-ai/cli/internal/publicationmodel"

func requirePublicationCapableTarget(ctx publicationContext) (publicationmodel.Model, error) {
	return requirePublicationTargetModel(ctx)
}

func requireMaterializePublicationChannel(ctx publicationContext, publication publicationmodel.Model) (publicationmodel.Channel, error) {
	return requireMaterializeTargetChannel(ctx, publication)
}

func requireRemovePublicationChannel(ctx publicationContext, publication publicationmodel.Model) (publicationmodel.Channel, error) {
	return requireRemoveTargetChannel(ctx, publication)
}

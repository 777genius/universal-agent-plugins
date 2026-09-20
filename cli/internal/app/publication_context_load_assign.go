package app

import (
	"github.com/777genius/plugin-kit-ai/cli/internal/publicationmodel"
	"github.com/777genius/plugin-kit-ai/cli/internal/publishschema"
)

func withPublicationContextMaterialize(ctx publicationContext, packageRoot string, publication publicationmodel.Model, publicationState publishschema.State, channel publicationmodel.Channel) publicationContext {
	ctx.packageRoot = packageRoot
	ctx.publication = publication
	ctx.publicationState = publicationState
	ctx.channel = channel
	return ctx
}

func withPublicationContextRemove(ctx publicationContext, packageRoot string, publication publicationmodel.Model, channel publicationmodel.Channel) publicationContext {
	ctx.packageRoot = packageRoot
	ctx.publication = publication
	ctx.channel = channel
	return ctx
}

func withPublicationContextVerify(ctx publicationContext, packageRoot string, publicationState publishschema.State) publicationContext {
	ctx.packageRoot = packageRoot
	ctx.publication = ctx.inspection.Publication
	ctx.publicationState = publicationState
	return ctx
}

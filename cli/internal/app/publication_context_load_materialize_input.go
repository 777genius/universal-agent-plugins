package app

import (
	"github.com/777genius/plugin-kit-ai/cli/internal/publicationmodel"
	"github.com/777genius/plugin-kit-ai/cli/internal/publishschema"
)

type materializePublicationContextInput struct {
	publication      publicationmodel.Model
	publicationState publishschema.State
	channel          publicationmodel.Channel
	packageRoot      string
}

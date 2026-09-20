package app

import (
	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmanifest"
	"github.com/777genius/plugin-kit-ai/cli/internal/publicationmodel"
	"github.com/777genius/plugin-kit-ai/cli/internal/publishschema"
)

type publicationContext struct {
	root             string
	target           string
	dest             string
	packageRoot      string
	graph            pluginmanifest.PackageGraph
	inspection       pluginmanifest.Inspection
	publication      publicationmodel.Model
	publicationState publishschema.State
	channel          publicationmodel.Channel
}

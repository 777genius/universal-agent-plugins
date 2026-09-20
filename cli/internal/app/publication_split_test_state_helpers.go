package app

import (
	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmanifest"
	"github.com/777genius/plugin-kit-ai/cli/internal/publicationmodel"
	"github.com/777genius/plugin-kit-ai/cli/internal/publishschema"
)

func publicationInspectionStub(name string) pluginmanifest.Inspection {
	return pluginmanifest.Inspection{
		Publication: publicationmodel.Model{
			Core: publicationmodel.Core{
				Name: name,
			},
			Packages: []publicationmodel.Package{{
				Target: "claude",
			}},
		},
	}
}

func publicationStateStub() publishschema.State {
	return publishschema.State{}
}

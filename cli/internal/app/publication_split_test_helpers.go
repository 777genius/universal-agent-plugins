package app

import (
	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmanifest"
	"github.com/777genius/plugin-kit-ai/cli/internal/publicationmodel"
)

func publicationChannelStub(family string) publicationmodel.Channel {
	return publicationmodel.Channel{Family: family}
}

func publicationGraphStub(name string) pluginmanifest.PackageGraph {
	return pluginmanifest.PackageGraph{Manifest: pluginmanifest.Manifest{Name: name}}
}

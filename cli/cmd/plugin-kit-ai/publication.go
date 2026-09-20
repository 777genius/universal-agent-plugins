package main

import "github.com/777genius/plugin-kit-ai/cli/internal/app"

var publicationCmd = newPublicationCmd(pluginService)

type publicationMaterializeRunner interface {
	PublicationMaterialize(app.PluginPublicationMaterializeOptions) (app.PluginPublicationMaterializeResult, error)
	PublicationRemove(app.PluginPublicationRemoveOptions) (app.PluginPublicationRemoveResult, error)
	PublicationVerifyRoot(app.PluginPublicationVerifyRootOptions) (app.PluginPublicationVerifyRootResult, error)
}

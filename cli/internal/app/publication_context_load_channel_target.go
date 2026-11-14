package app

import (
	"fmt"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
	"github.com/777genius/plugin-kit-ai/cli/internal/publicationmodel"
)

func requireMaterializeTargetChannel(ctx publicationContext, publication publicationmodel.Model) (publicationmodel.Channel, error) {
	channel, ok := publicationChannelForTarget(publication, ctx.target)
	if ok {
		return channel, nil
	}
	authoredRoot := ctx.inspection.Layout.AuthoredRoot
	if authoredRoot == "" {
		authoredRoot = pluginmodel.SourceDirName
	}
	return publicationmodel.Channel{}, fmt.Errorf("target %s requires authored publication channel metadata under %s/publish/...", ctx.target, authoredRoot)
}

func requireRemoveTargetChannel(ctx publicationContext, publication publicationmodel.Model) (publicationmodel.Channel, error) {
	channel, ok := publicationChannelForTarget(publication, ctx.target)
	if ok {
		return channel, nil
	}
	return publicationmodel.Channel{}, fmt.Errorf("target %s requires authored publication channel metadata under publish/...", ctx.target)
}

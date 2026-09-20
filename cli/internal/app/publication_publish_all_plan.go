package app

import (
	"fmt"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmanifest"
	"github.com/777genius/plugin-kit-ai/cli/internal/publicationmodel"
)

func plannedPublishAllChannels(inspection pluginmanifest.Inspection) []publicationmodel.Channel {
	return orderedPublicationChannels(inspection.Publication)
}

func validatePublishAllDest(opts PluginPublishOptions, channels []publicationmodel.Channel) error {
	if channelsNeedLocalDest(channels) && strings.TrimSpace(opts.Dest) == "" {
		return fmt.Errorf("publish --all --dry-run requires --dest because authored publication channels include local marketplace roots")
	}
	return nil
}

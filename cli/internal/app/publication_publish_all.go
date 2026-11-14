package app

import (
	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmanifest"
)

func (service PluginService) planPublishAll(opts PluginPublishOptions) (PluginPublishResult, error) {
	root, err := validatePublishAllOptions(opts)
	if err != nil {
		return PluginPublishResult{}, err
	}
	inspection, _, err := pluginmanifest.Inspect(root, "all")
	if err != nil {
		return PluginPublishResult{}, err
	}
	channels := plannedPublishAllChannels(inspection)
	if len(channels) == 0 {
		return emptyPublishPlan(root), nil
	}
	if err := validatePublishAllDest(opts, channels); err != nil {
		return PluginPublishResult{}, err
	}
	plan, err := service.runPublishPlan(root, opts, channels)
	if err != nil {
		return PluginPublishResult{}, err
	}
	return buildPublishPlanResult(opts, channels, plan), nil
}

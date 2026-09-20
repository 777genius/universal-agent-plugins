package app

import "github.com/777genius/plugin-kit-ai/cli/internal/publicationmodel"

func (service PluginService) runPublishPlan(root string, opts PluginPublishOptions, channels []publicationmodel.Channel) (publishAllPlan, error) {
	plan := publishAllPlan{ready: true}
	if !channelsNeedLocalDest(channels) {
		plan.warnings = append(plan.warnings, ignoredPublishPlanWarnings(opts)...)
	}
	for _, channel := range channels {
		result, err := service.Publish(PluginPublishOptions{
			Root:        root,
			Channel:     channel.Family,
			Dest:        opts.Dest,
			PackageRoot: opts.PackageRoot,
			DryRun:      true,
		})
		if err != nil {
			return publishAllPlan{}, err
		}
		plan.results = append(plan.results, result)
		if !result.Ready {
			plan.ready = false
		}
		plan.next = appendUniquePublishSteps(append(plan.next, result.NextSteps...))
	}
	return plan, nil
}

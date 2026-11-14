package app

func (service PluginService) publish(opts PluginPublishOptions) (PluginPublishResult, error) {
	if opts.All {
		return service.publishAll(opts)
	}
	return service.publishSelectedChannel(opts)
}

func (service PluginService) publishAll(opts PluginPublishOptions) (PluginPublishResult, error) {
	return service.planPublishAll(opts)
}

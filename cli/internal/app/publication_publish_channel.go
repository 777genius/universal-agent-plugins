package app

import (
	"strings"
)

func (service PluginService) publishSelectedChannel(opts PluginPublishOptions) (PluginPublishResult, error) {
	channel := strings.TrimSpace(opts.Channel)
	if channel == "gemini-gallery" {
		return service.publishGeminiGallery(opts)
	}
	input, err := newLocalPublishInput(opts, channel)
	if err != nil {
		return PluginPublishResult{}, err
	}
	result, err := service.PublicationMaterialize(input)
	if err != nil {
		return PluginPublishResult{}, err
	}
	return buildLocalPublishResult(channel, result), nil
}

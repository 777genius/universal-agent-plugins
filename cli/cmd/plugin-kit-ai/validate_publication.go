package main

import (
	"fmt"
	"slices"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmanifest"
	"github.com/777genius/plugin-kit-ai/cli/internal/publicationmodel"
)

func discoverValidatePublication(root, platform string) *publicationmodel.Model {
	inspection, _, err := pluginmanifest.Inspect(root, validatePublicationTarget(platform))
	if err != nil {
		return nil
	}
	if len(inspection.Publication.Packages) == 0 && len(inspection.Publication.Channels) == 0 {
		return nil
	}
	publication := inspection.Publication
	return &publication
}

func validatePublicationTarget(platform string) string {
	platform = strings.TrimSpace(platform)
	if platform == "" {
		return "all"
	}
	return platform
}

func validatePublicationText(publication *publicationmodel.Model) []string {
	if publication == nil {
		return nil
	}
	lines := []string{
		fmt.Sprintf("Publication: api_version=%s packages=%d channels=%d", publication.Core.APIVersion, len(publication.Packages), len(publication.Channels)),
	}
	for _, channel := range publication.Channels {
		line := fmt.Sprintf("Publication channel: %s path=%s targets=%s",
			channel.Family,
			channel.Path,
			strings.Join(channel.PackageTargets, ","),
		)
		if details := formatValidatePublicationDetails(channel.Details); details != "" {
			line += " details=" + details
		}
		lines = append(lines, line)
	}
	return lines
}

func formatValidatePublicationDetails(details map[string]string) string {
	if len(details) == 0 {
		return ""
	}
	keys := make([]string, 0, len(details))
	for key, value := range details {
		if strings.TrimSpace(value) == "" {
			continue
		}
		keys = append(keys, key)
	}
	slices.Sort(keys)
	if len(keys) == 0 {
		return ""
	}
	items := make([]string, 0, len(keys))
	for _, key := range keys {
		items = append(items, key+"="+details[key])
	}
	return strings.Join(items, ",")
}

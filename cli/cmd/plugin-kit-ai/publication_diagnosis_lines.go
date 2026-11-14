package main

import (
	"fmt"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/publicationmodel"
)

func buildPublicationDiagnosisLines(model publicationmodel.Model) ([]string, map[string]struct{}) {
	lines := []string{
		fmt.Sprintf("Publication: %s %s api_version=%s", model.Core.Name, model.Core.Version, model.Core.APIVersion),
		fmt.Sprintf("Packages: %d", len(model.Packages)),
		fmt.Sprintf("Channels: %d", len(model.Channels)),
	}
	channelTargets := map[string]struct{}{}
	for _, channel := range model.Channels {
		for _, target := range channel.PackageTargets {
			channelTargets[target] = struct{}{}
		}
		line := fmt.Sprintf("Channel[%s]: path=%s targets=%s", channel.Family, channel.Path, strings.Join(channel.PackageTargets, ","))
		if details := inspectChannelDetails(channel.Details); details != "" {
			line += " details=" + details
		}
		lines = append(lines, line)
	}
	for _, pkg := range model.Packages {
		lines = append(lines, fmt.Sprintf("Package[%s]: family=%s channels=%s managed=%d",
			pkg.Target,
			pkg.PackageFamily,
			strings.Join(pkg.ChannelFamilies, ","),
			len(pkg.ManagedArtifacts),
		))
	}
	return lines, channelTargets
}

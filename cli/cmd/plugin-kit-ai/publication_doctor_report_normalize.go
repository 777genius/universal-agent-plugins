package main

import (
	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmanifest"
	"github.com/777genius/plugin-kit-ai/cli/internal/publicationmodel"
)

func warningMessages(warnings []pluginmanifest.Warning) []string {
	out := make([]string, 0, len(warnings))
	for _, warning := range warnings {
		out = append(out, warningMessage(warning))
	}
	return out
}

func warningMessage(warning pluginmanifest.Warning) string {
	return warning.Message
}

func normalizePublicationModel(model publicationmodel.Model) publicationmodel.Model {
	model.Packages = normalizePublicationPackages(model.Packages)
	model.Channels = normalizePublicationChannels(model.Channels)
	return model
}

func normalizePublicationPackages(packages []publicationmodel.Package) []publicationmodel.Package {
	if packages == nil {
		return []publicationmodel.Package{}
	}
	for i := range packages {
		packages[i] = normalizePublicationPackage(packages[i])
	}
	return packages
}

func normalizePublicationPackage(pkg publicationmodel.Package) publicationmodel.Package {
	if pkg.ChannelFamilies == nil {
		pkg.ChannelFamilies = []string{}
	}
	if pkg.AuthoredInputs == nil {
		pkg.AuthoredInputs = []string{}
	}
	if pkg.ManagedArtifacts == nil {
		pkg.ManagedArtifacts = []string{}
	}
	return pkg
}

func normalizePublicationChannels(channels []publicationmodel.Channel) []publicationmodel.Channel {
	if channels == nil {
		return []publicationmodel.Channel{}
	}
	for i := range channels {
		channels[i] = normalizePublicationChannel(channels[i])
	}
	return channels
}

func normalizePublicationChannel(channel publicationmodel.Channel) publicationmodel.Channel {
	if channel.PackageTargets == nil {
		channel.PackageTargets = []string{}
	}
	return channel
}

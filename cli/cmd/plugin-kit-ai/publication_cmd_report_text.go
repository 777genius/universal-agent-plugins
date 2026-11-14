package main

import (
	"fmt"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmanifest"
	"github.com/777genius/plugin-kit-ai/cli/internal/publicationmodel"
	"github.com/spf13/cobra"
)

func writePublicationWarnings(cmd *cobra.Command, warnings []pluginmanifest.Warning) {
	for _, warning := range warnings {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Warning: %s\n", warning.Message)
	}
}

func writePublicationSummary(cmd *cobra.Command, report pluginmanifest.Inspection) {
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "publication %s %s api_version=%s\n",
		report.Publication.Core.Name,
		report.Publication.Core.Version,
		report.Publication.Core.APIVersion,
	)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "packages: %d channels: %d\n",
		len(report.Publication.Packages),
		len(report.Publication.Channels),
	)
}

func writePublicationPackageLine(cmd *cobra.Command, pkg publicationmodel.Package) {
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  package[%s]: family=%s channels=%s inputs=%d managed=%d\n",
		pkg.Target,
		pkg.PackageFamily,
		strings.Join(pkg.ChannelFamilies, ","),
		len(pkg.AuthoredInputs),
		len(pkg.ManagedArtifacts),
	)
}

func writePublicationChannelLine(cmd *cobra.Command, channel publicationmodel.Channel) {
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  channel[%s]: path=%s targets=%s",
		channel.Family,
		channel.Path,
		strings.Join(channel.PackageTargets, ","),
	)
	if details := inspectChannelDetails(channel.Details); details != "" {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), " details=%s", details)
	}
	_, _ = fmt.Fprintln(cmd.OutOrStdout())
}

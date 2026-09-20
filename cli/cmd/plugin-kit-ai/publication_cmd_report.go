package main

import (
	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmanifest"
	"github.com/spf13/cobra"
)

func runPublication(cmd *cobra.Command, runner inspectRunner, target, format string, args []string) error {
	report, warnings, err := inspectPublicationReport(runner, publicationRoot(args), target)
	if err != nil {
		return err
	}
	return renderPublicationReport(cmd, report, warnings, target, format)
}

func writePublicationTextReport(cmd *cobra.Command, report pluginmanifest.Inspection, warnings []pluginmanifest.Warning) error {
	writePublicationWarnings(cmd, warnings)
	writePublicationSummary(cmd, report)
	for _, pkg := range report.Publication.Packages {
		writePublicationPackageLine(cmd, pkg)
	}
	for _, channel := range report.Publication.Channels {
		writePublicationChannelLine(cmd, channel)
	}
	return nil
}

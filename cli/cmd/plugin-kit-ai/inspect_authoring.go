package main

import (
	"fmt"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmanifest"
)

func renderInspectAuthoring(report pluginmanifest.Inspection) string {
	lines := []string{
		fmt.Sprintf("Plugin repo: %s %s", report.Manifest.Name, report.Manifest.Version),
		fmt.Sprintf("This repo is set up to %s.", inspectAuthoringPath(report)),
	}

	lines = append(lines, inspectAuthoringEditableSourceLines(report)...)
	lines = append(lines, inspectAuthoringGeneratedOutputLines(report)...)
	lines = append(lines, inspectAuthoringSupportedOutputLines(report)...)
	lines = append(lines, inspectAuthoringNextCommandLines(report)...)

	return strings.Join(lines, "\n") + "\n"
}

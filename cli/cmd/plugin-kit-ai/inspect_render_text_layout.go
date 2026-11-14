package main

import (
	"fmt"
	"slices"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmanifest"
)

func renderInspectLayoutSection(out *strings.Builder, report pluginmanifest.Inspection) {
	if len(report.Layout.AuthoredInputs) > 0 {
		_, _ = fmt.Fprintln(out, "authored_inputs:")
		for _, path := range report.Layout.AuthoredInputs {
			_, _ = fmt.Fprintf(out, "  - %s\n", path)
		}
	}
	if len(report.Layout.GeneratedOutputs) > 0 {
		_, _ = fmt.Fprintln(out, "generated_outputs:")
		for _, path := range report.Layout.GeneratedOutputs {
			_, _ = fmt.Fprintf(out, "  - %s\n", path)
		}
	}
	if len(report.Layout.GeneratedByTarget) == 0 {
		return
	}
	_, _ = fmt.Fprintln(out, "generated_by_target:")
	var targetNames []string
	for name := range report.Layout.GeneratedByTarget {
		targetNames = append(targetNames, name)
	}
	slices.Sort(targetNames)
	for _, name := range targetNames {
		_, _ = fmt.Fprintf(out, "  %s:\n", name)
		for _, path := range report.Layout.GeneratedByTarget[name] {
			_, _ = fmt.Fprintf(out, "    - %s\n", path)
		}
	}
}

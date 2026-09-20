package main

import (
	"fmt"
	"slices"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmanifest"
)

func renderInspectPublicationSection(out *strings.Builder, report pluginmanifest.Inspection) {
	for _, channel := range report.Publication.Channels {
		_, _ = fmt.Fprintf(out, "  channel[%s]: path=%s targets=%s",
			channel.Family,
			channel.Path,
			strings.Join(channel.PackageTargets, ","),
		)
		if details := inspectChannelDetails(channel.Details); details != "" {
			_, _ = fmt.Fprintf(out, " details=%s", details)
		}
		_, _ = fmt.Fprintln(out)
	}
	for _, pkg := range report.Publication.Packages {
		_, _ = fmt.Fprintf(out, "  publish[%s]: family=%s channels=%s inputs=%d managed=%d\n",
			pkg.Target,
			pkg.PackageFamily,
			strings.Join(pkg.ChannelFamilies, ","),
			len(pkg.AuthoredInputs),
			len(pkg.ManagedArtifacts),
		)
	}
}

func inspectChannelDetails(details map[string]string) string {
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

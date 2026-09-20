package main

import (
	"fmt"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmanifest"
)

func renderInspectText(report pluginmanifest.Inspection) string {
	var out strings.Builder
	_, _ = fmt.Fprintf(&out, "package %s %s\n", report.Manifest.Name, report.Manifest.Version)
	_, _ = fmt.Fprintf(&out, "targets: %s\n", strings.Join(report.Manifest.Targets, ", "))
	if authoredRoot := strings.TrimSpace(report.Layout.AuthoredRoot); authoredRoot != "" {
		_, _ = fmt.Fprintf(&out, "layout: authored_root=%s", authoredRoot)
		if len(report.Layout.BoundaryDocs) > 0 {
			_, _ = fmt.Fprintf(&out, " boundary_docs=%s", strings.Join(report.Layout.BoundaryDocs, ","))
		}
		if generatedGuide := strings.TrimSpace(report.Layout.GeneratedGuide); generatedGuide != "" {
			_, _ = fmt.Fprintf(&out, " generated_guide=%s", generatedGuide)
		}
		_, _ = fmt.Fprintln(&out)
	}
	_, _ = fmt.Fprintf(&out, "portable: skills=%d mcp=%t\n", len(report.Portable.Paths("skills")), report.Portable.MCP != nil)
	_, _ = fmt.Fprintf(&out, "publication: api_version=%s packages=%d channels=%d\n", report.Publication.Core.APIVersion, len(report.Publication.Packages), len(report.Publication.Channels))
	if report.Launcher != nil {
		_, _ = fmt.Fprintf(&out, "launcher: runtime=%s entrypoint=%s\n", report.Launcher.Runtime, report.Launcher.Entrypoint)
	}
	renderInspectLayoutSection(&out, report)
	renderInspectPublicationSection(&out, report)
	renderInspectTargetsSection(&out, report)
	return out.String()
}

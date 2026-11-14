package main

import (
	"fmt"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/app"
	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmanifest"
	"github.com/spf13/cobra"
)

type publicationDoctorRenderInput struct {
	format    string
	target    string
	report    pluginmanifest.Inspection
	warnings  []pluginmanifest.Warning
	diagnosis publicationDiagnosis
	localRoot *app.PluginPublicationVerifyRootResult
}

type publicationDoctorRenderer func(*cobra.Command, publicationDoctorRenderInput) error

func bindPublicationDoctorFlags(cmd *cobra.Command, flags *publicationDoctorFlags) {
	cmd.Flags().StringVar(&flags.target, "target", "all", `publication target ("all", "claude", "codex-package", or "gemini")`)
	cmd.Flags().StringVar(&flags.format, "format", "text", "output format: text or json")
	cmd.Flags().StringVar(&flags.dest, "dest", "", "optional materialized marketplace root to verify for local codex-package or claude publication flows")
	cmd.Flags().StringVar(&flags.packageRoot, "package-root", "", "relative package root inside the destination marketplace root (default: plugins/<name>)")
}

func renderPublicationDoctor(cmd *cobra.Command, in publicationDoctorRenderInput) error {
	renderer, err := publicationDoctorRendererForFormat(in.format)
	if err != nil {
		return err
	}
	return renderer(cmd, in)
}

func normalizedPublicationDoctorFormat(format string) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "", "text":
		return "text"
	case "json":
		return "json"
	default:
		return "invalid"
	}
}

func publicationDoctorRendererForFormat(format string) (publicationDoctorRenderer, error) {
	switch normalizedPublicationDoctorFormat(format) {
	case "text":
		return renderPublicationDoctorTextInput, nil
	case "json":
		return renderPublicationDoctorJSONInput, nil
	default:
		return nil, fmt.Errorf("unsupported format %q (use text or json)", format)
	}
}

func renderPublicationDoctorTextInput(cmd *cobra.Command, in publicationDoctorRenderInput) error {
	return renderPublicationDoctorText(cmd, in.warnings, in.diagnosis, in.localRoot)
}

func renderPublicationDoctorJSONInput(cmd *cobra.Command, in publicationDoctorRenderInput) error {
	return renderPublicationDoctorJSON(cmd, in.report, in.warnings, in.target, in.diagnosis, in.localRoot)
}

func renderPublicationDoctorText(cmd *cobra.Command, warnings []pluginmanifest.Warning, diagnosis publicationDiagnosis, localRoot *app.PluginPublicationVerifyRootResult) error {
	writePublicationDoctorWarnings(cmd, warnings)
	writePublicationDoctorLines(cmd, publicationDoctorTextLines(diagnosis, localRoot))
	return publicationDoctorIssueErr(diagnosis.Ready)
}

func publicationDoctorTextLines(diagnosis publicationDiagnosis, localRoot *app.PluginPublicationVerifyRootResult) []string {
	lines := append([]string(nil), diagnosis.Lines...)
	return appendPublicationDoctorLocalRootLines(lines, localRoot)
}

func appendPublicationDoctorLocalRootLines(lines []string, localRoot *app.PluginPublicationVerifyRootResult) []string {
	if localRoot == nil {
		return lines
	}
	return append(lines, localRoot.Lines...)
}

func writePublicationDoctorWarnings(cmd *cobra.Command, warnings []pluginmanifest.Warning) {
	for _, warning := range warnings {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Warning: %s\n", warning.Message)
	}
}

func writePublicationDoctorLines(cmd *cobra.Command, lines []string) {
	for _, line := range lines {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), line)
	}
}

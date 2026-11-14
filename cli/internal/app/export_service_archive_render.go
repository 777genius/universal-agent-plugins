package app

import (
	"fmt"
	"path/filepath"
	"strings"
)

func buildExportResultLines(ctx exportServiceContext, plan exportArchivePlan) []string {
	lines := buildExportResultBaseLines(ctx, plan)
	lines = appendExportRuntimeLines(lines, plan.metadata)
	return appendExportResultNextLines(lines, ctx.platform, plan.outputPath)
}

func buildExportResultBaseLines(ctx exportServiceContext, plan exportArchivePlan) []string {
	return []string{
		ctx.project.ProjectLine(),
		"Exported bundle: " + plan.outputPath,
		fmt.Sprintf("Included files: %d", len(plan.files)+1),
	}
}

func appendExportResultNextLines(lines []string, platform, outputPath string) []string {
	lines = append(lines,
		"Next:",
		"  tar -xzf "+filepath.Base(outputPath),
		"  plugin-kit-ai doctor .",
		"  plugin-kit-ai bootstrap .",
		fmt.Sprintf("  plugin-kit-ai validate . --platform %s --strict", platform),
	)
	return lines
}

func appendExportRuntimeLines(lines []string, metadata exportMetadata) []string {
	if strings.TrimSpace(metadata.RuntimeRequirement) != "" {
		lines = append(lines, "Runtime requirement: "+metadata.RuntimeRequirement)
	}
	if strings.TrimSpace(metadata.RuntimeInstallHint) != "" {
		lines = append(lines, "Runtime install hint: "+metadata.RuntimeInstallHint)
	}
	return lines
}

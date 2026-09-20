package main

import (
	"fmt"
	"slices"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmanifest"
)

func inspectAuthoringEditableSourceLines(report pluginmanifest.Inspection) []string {
	var lines []string
	if authoredRoot := strings.TrimSpace(report.Layout.AuthoredRoot); authoredRoot != "" {
		lines = append(lines, fmt.Sprintf("Editable source lives under %s.", authoredRoot))
	}
	if len(report.Layout.AuthoredInputs) > 0 {
		lines = append(lines, "", "Edit these files:")
		for _, path := range report.Layout.AuthoredInputs {
			lines = append(lines, "  - "+path)
		}
	}
	return lines
}

func inspectAuthoringGeneratedOutputLines(report pluginmanifest.Inspection) []string {
	guides, outputs := authoredGeneratedOutputs(report)
	if len(guides) == 0 && len(outputs) == 0 {
		return nil
	}

	var lines []string
	if len(guides) > 0 {
		lines = append(lines, "", "Managed guidance files:")
		for _, path := range guides {
			lines = append(lines, "  - "+path)
		}
	}
	if len(outputs) > 0 {
		lines = append(lines, "", "Generated target outputs:")
		for _, path := range outputs {
			lines = append(lines, "  - "+path)
		}
	}
	return lines
}

func inspectAuthoringSupportedOutputLines(report pluginmanifest.Inspection) []string {
	if len(report.Manifest.Targets) == 0 {
		return nil
	}
	lines := []string{"", "Supported outputs:"}
	for _, target := range report.Manifest.Targets {
		lines = append(lines, "  - "+inspectAuthoringTargetLabel(target))
	}
	return lines
}

func inspectAuthoringNextCommandLines(report pluginmanifest.Inspection) []string {
	lines := []string{"", "Next commands:"}
	for _, command := range inspectAuthoringNextCommands(report) {
		lines = append(lines, "  - "+command)
	}
	return lines
}

func authoredGeneratedOutputs(report pluginmanifest.Inspection) ([]string, []string) {
	seen := map[string]struct{}{}
	guides := make([]string, 0, len(report.Layout.GeneratedOutputs)+len(report.Layout.BoundaryDocs)+1)
	for _, path := range report.Layout.GeneratedOutputs {
		if !isAuthoringGuideFile(path) {
			continue
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		guides = append(guides, path)
	}
	for _, path := range report.Layout.BoundaryDocs {
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		guides = append(guides, path)
	}
	if generatedGuide := strings.TrimSpace(report.Layout.GeneratedGuide); generatedGuide != "" {
		if _, ok := seen[generatedGuide]; !ok {
			seen[generatedGuide] = struct{}{}
			guides = append(guides, generatedGuide)
		}
	}
	outputs := make([]string, 0, len(report.Layout.GeneratedOutputs))
	for _, path := range report.Layout.GeneratedOutputs {
		if _, ok := seen[path]; ok {
			continue
		}
		outputs = append(outputs, path)
	}
	if len(guides) == 0 && len(outputs) == 0 {
		return nil, nil
	}
	return orderAuthoringGuideFiles(slices.Compact(guides)), slices.Compact(outputs)
}

func isAuthoringGuideFile(path string) bool {
	switch path {
	case "README.md", "CLAUDE.md", "AGENTS.md", "GENERATED.md":
		return true
	default:
		return false
	}
}

func orderAuthoringGuideFiles(paths []string) []string {
	preferred := []string{"README.md", "CLAUDE.md", "AGENTS.md", "GENERATED.md"}
	seen := make(map[string]struct{}, len(paths))
	ordered := make([]string, 0, len(paths))
	for _, want := range preferred {
		for _, path := range paths {
			if path != want {
				continue
			}
			if _, ok := seen[path]; ok {
				continue
			}
			seen[path] = struct{}{}
			ordered = append(ordered, path)
		}
	}
	for _, path := range paths {
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		ordered = append(ordered, path)
	}
	return ordered
}

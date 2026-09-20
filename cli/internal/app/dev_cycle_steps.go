package app

import (
	"context"
	"fmt"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmanifest"
	"github.com/777genius/plugin-kit-ai/cli/internal/runtimecheck"
	"github.com/777genius/plugin-kit-ai/cli/internal/validate"
)

func (svc PluginService) runDevGenerate(root, platform string, lines *[]string) bool {
	generated, err := svc.Generate(PluginGenerateOptions{Root: root, Target: platform})
	if err != nil {
		*lines = append(*lines, "Generate: "+err.Error())
		return false
	}
	*lines = append(*lines, fmt.Sprintf("Generate: wrote %d artifact(s)", len(generated)))
	return true
}

func (svc PluginService) runDevAutoBuild(ctx context.Context, root string, graph pluginmanifest.PackageGraph, project runtimecheck.Project, lines *[]string) bool {
	buildLines, err := devAutoBuild(ctx, root, graph, project)
	*lines = append(*lines, buildLines...)
	if err != nil {
		*lines = append(*lines, "Build: "+err.Error())
		return false
	}
	return true
}

func runDevValidate(root, platform string, lines *[]string) bool {
	report, err := validate.Validate(root, platform)
	if err != nil {
		if re, ok := err.(*validate.ReportError); ok {
			report = re.Report
		} else {
			*lines = append(*lines, "Validate: "+err.Error())
			return false
		}
	}
	if len(report.Failures) > 0 {
		*lines = append(*lines, fmt.Sprintf("Validate: %d failure(s)", len(report.Failures)))
		for _, failure := range report.Failures {
			*lines = append(*lines, "  - "+failure.Message)
		}
		return false
	}
	if len(report.Warnings) > 0 {
		*lines = append(*lines, fmt.Sprintf("Validate: %d warning(s) (strict)", len(report.Warnings)))
		for _, warning := range report.Warnings {
			*lines = append(*lines, "  - "+warning.Message)
		}
		return false
	}
	*lines = append(*lines, "Validate: ok")
	return true
}

func runDevTests(ctx context.Context, root, platform string, project runtimecheck.Project, opts PluginDevOptions, lines *[]string) bool {
	selected, ok := selectDevRuntimeTests(platform, opts, lines)
	if !ok {
		return false
	}

	passed := true
	for _, item := range selected {
		tc := runRuntimeTestCase(ctx, root, project, PluginTestOptions{
			Root:      root,
			Platform:  platform,
			Event:     opts.Event,
			Fixture:   opts.Fixture,
			GoldenDir: opts.GoldenDir,
			All:       opts.All,
		}, item)
		appendDevRuntimeTestLines(tc, lines)
		if tc.Failure != "" || !tc.Passed {
			passed = false
		}
	}
	return passed
}

func selectDevRuntimeTests(platform string, opts PluginDevOptions, lines *[]string) ([]runtimeTestSupport, bool) {
	supported := stableRuntimeSupport(platform)
	selected, err := selectRuntimeTestCases(supported, opts.Event, opts.All)
	if err != nil {
		*lines = append(*lines, err.Error())
		return nil, false
	}
	if opts.All && strings.TrimSpace(opts.Fixture) != "" {
		*lines = append(*lines, "--fixture cannot be used with --all")
		return nil, false
	}
	return selected, true
}

func appendDevRuntimeTestLines(tc PluginTestCase, lines *[]string) {
	*lines = append(*lines, formatRuntimeTestCaseLine(tc))
	if strings.TrimSpace(tc.Stdout) != "" {
		*lines = append(*lines, "  stdout: "+singleLinePreview(tc.Stdout))
	}
	if strings.TrimSpace(tc.Stderr) != "" {
		*lines = append(*lines, "  stderr: "+singleLinePreview(tc.Stderr))
	}
}

func runtimeStatusLine(diagnosis runtimecheck.Diagnosis) string {
	return fmt.Sprintf("Runtime: %s (%s)", diagnosis.Status, diagnosis.Reason)
}

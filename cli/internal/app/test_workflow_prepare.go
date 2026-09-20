package app

import (
	"fmt"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmanifest"
	"github.com/777genius/plugin-kit-ai/cli/internal/runtimecheck"
	"github.com/777genius/plugin-kit-ai/cli/internal/validate"
)

type pluginTestRun struct {
	root      string
	platform  string
	project   runtimecheck.Project
	selected  []runtimeTestSupport
	baseLines []string
}

func preparePluginTestRun(opts PluginTestOptions) (pluginTestRun, error) {
	root := normalizePluginTestRoot(opts.Root)
	graph, err := discoverPluginTestGraph(root)
	if err != nil {
		return pluginTestRun{}, err
	}
	platform, err := resolvePluginTestPlatform(graph, opts.Platform)
	if err != nil {
		return pluginTestRun{}, err
	}
	project, err := inspectPluginTestRuntime(root, platform, graph)
	if err != nil {
		return pluginTestRun{}, err
	}
	selected, err := selectPluginTestCases(platform, opts)
	if err != nil {
		return pluginTestRun{}, err
	}
	return pluginTestRun{
		root:      root,
		platform:  platform,
		project:   project,
		selected:  selected,
		baseLines: []string{project.ProjectLine(), "Validate: ok"},
	}, nil
}

func normalizePluginTestRoot(root string) string {
	root = strings.TrimSpace(root)
	if root == "" {
		return "."
	}
	return root
}

func discoverPluginTestGraph(root string) (pluginmanifest.PackageGraph, error) {
	graph, _, err := pluginmanifest.Discover(root)
	if err != nil {
		return pluginmanifest.PackageGraph{}, err
	}
	return graph, nil
}

func resolvePluginTestPlatform(graph pluginmanifest.PackageGraph, requested string) (string, error) {
	platform, err := resolveRuntimeTestPlatform(graph.Manifest.EnabledTargets(), requested)
	if err != nil {
		return "", err
	}
	if graph.Launcher == nil {
		return "", fmt.Errorf("test requires launcher-based target %q with launcher.yaml", platform)
	}
	return platform, nil
}

func inspectPluginTestRuntime(root, platform string, graph pluginmanifest.PackageGraph) (runtimecheck.Project, error) {
	if err := validatePluginTestStrict(root, platform); err != nil {
		return runtimecheck.Project{}, err
	}
	project, err := runtimecheck.Inspect(runtimecheck.Inputs{
		Root:     root,
		Targets:  []string{platform},
		Launcher: graph.Launcher,
	})
	if err != nil {
		return runtimecheck.Project{}, err
	}
	diagnosis := runtimecheck.Diagnose(project)
	if diagnosis.Status != runtimecheck.StatusReady {
		return runtimecheck.Project{}, fmt.Errorf("test requires runtime readiness: %s", diagnosis.Reason)
	}
	return project, nil
}

func validatePluginTestStrict(root, platform string) error {
	report, err := validate.Validate(root, platform)
	if err != nil {
		if re, ok := err.(*validate.ReportError); ok {
			report = re.Report
		} else {
			return err
		}
	}
	if len(report.Failures) > 0 {
		return fmt.Errorf("test requires validate --strict to pass for %s", platform)
	}
	if len(report.Warnings) > 0 {
		return fmt.Errorf("test requires validate --strict to pass for %s: %d warning(s) present", platform, len(report.Warnings))
	}
	return nil
}

func selectPluginTestCases(platform string, opts PluginTestOptions) ([]runtimeTestSupport, error) {
	supported := stableRuntimeSupport(platform)
	if len(supported) == 0 {
		return nil, fmt.Errorf("test supports only stable runtime targets with built-in event metadata: claude or codex-runtime")
	}
	selected, err := selectRuntimeTestCases(supported, opts.Event, opts.All)
	if err != nil {
		return nil, err
	}
	if opts.All && strings.TrimSpace(opts.Fixture) != "" {
		return nil, fmt.Errorf("--fixture cannot be used with --all")
	}
	return selected, nil
}

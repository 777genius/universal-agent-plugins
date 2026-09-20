package app

import (
	"context"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmanifest"
	"github.com/777genius/plugin-kit-ai/cli/internal/runtimecheck"
)

func (svc PluginService) runDevCycle(ctx context.Context, root, platform string, opts PluginDevOptions, cycle int, trigger string, changed []string) PluginDevUpdate {
	lines := []string{devCycleHeader(cycle, trigger, changed)}
	update := PluginDevUpdate{Cycle: cycle, Passed: false}

	graph, ok := discoverDevGraph(root, platform, &lines)
	if !ok {
		update.Lines = lines
		return update
	}
	if !svc.runDevGenerate(root, platform, &lines) {
		update.Lines = lines
		return update
	}

	project, ok := inspectDevRuntime(root, platform, graph, &lines)
	if !ok {
		update.Lines = lines
		return update
	}
	if ok := svc.runDevAutoBuild(ctx, root, graph, project, &lines); !ok {
		update.Lines = lines
		return update
	}
	if ok := runDevValidate(root, platform, &lines); !ok {
		update.Lines = lines
		return update
	}

	project, ok = inspectDevRuntime(root, platform, graph, &lines)
	if !ok {
		update.Lines = lines
		return update
	}
	if !appendDevRuntimeStatus(project, &lines) {
		update.Lines = lines
		return update
	}

	update.Passed = runDevTests(ctx, root, platform, project, opts, &lines)
	update.Lines = lines
	return update
}

func discoverDevGraph(root, platform string, lines *[]string) (pluginmanifest.PackageGraph, bool) {
	graph, _, err := pluginmanifest.Discover(root)
	if err != nil {
		*lines = append(*lines, "Discover: "+err.Error())
		return pluginmanifest.PackageGraph{}, false
	}
	if _, err := graph.Manifest.SelectedTargets(platform); err != nil {
		*lines = append(*lines, err.Error())
		return pluginmanifest.PackageGraph{}, false
	}
	if graph.Launcher == nil {
		*lines = append(*lines, "launcher invalid: missing launcher.yaml")
		return pluginmanifest.PackageGraph{}, false
	}
	return graph, true
}

func inspectDevRuntime(root, platform string, graph pluginmanifest.PackageGraph, lines *[]string) (runtimecheck.Project, bool) {
	project, err := runtimecheck.Inspect(runtimecheck.Inputs{
		Root:     root,
		Targets:  []string{platform},
		Launcher: graph.Launcher,
	})
	if err != nil {
		*lines = append(*lines, "Runtime inspect: "+err.Error())
		return runtimecheck.Project{}, false
	}
	return project, true
}

func appendDevRuntimeStatus(project runtimecheck.Project, lines *[]string) bool {
	diagnosis := runtimecheck.Diagnose(project)
	*lines = append(*lines, project.ProjectLine())
	*lines = append(*lines, runtimeStatusLine(diagnosis))
	return diagnosis.Status == runtimecheck.StatusReady
}

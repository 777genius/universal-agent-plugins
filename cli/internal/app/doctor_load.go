package app

import (
	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmanifest"
	"github.com/777genius/plugin-kit-ai/cli/internal/runtimecheck"
)

func loadDoctorProject(root string) (runtimecheck.Project, error) {
	graph, err := loadDoctorGraph(root)
	if err != nil {
		return runtimecheck.Project{}, err
	}
	return inspectDoctorProject(root, graph)
}

func loadDoctorGraph(root string) (pluginmanifest.PackageGraph, error) {
	graph, _, err := pluginmanifest.Discover(root)
	if err != nil {
		return pluginmanifest.PackageGraph{}, err
	}
	return graph, nil
}

func inspectDoctorProject(root string, graph pluginmanifest.PackageGraph) (runtimecheck.Project, error) {
	return runtimecheck.Inspect(runtimecheck.Inputs{
		Root:     root,
		Targets:  graph.Manifest.EnabledTargets(),
		Launcher: graph.Launcher,
	})
}

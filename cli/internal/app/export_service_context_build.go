package app

import (
	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmanifest"
	"github.com/777genius/plugin-kit-ai/cli/internal/runtimecheck"
)

func loadExportServiceContextDeps(input exportServiceInput) (pluginmanifest.PackageGraph, runtimecheck.Project, error) {
	graph, err := loadExportServiceGraph(input.root, input.platform)
	if err != nil {
		return pluginmanifest.PackageGraph{}, runtimecheck.Project{}, err
	}
	project, err := loadReadyExportProject(input.root, input.platform, graph.Launcher)
	if err != nil {
		return pluginmanifest.PackageGraph{}, runtimecheck.Project{}, err
	}
	return graph, project, nil
}

func buildExportServiceContext(input exportServiceInput, graph pluginmanifest.PackageGraph, project runtimecheck.Project) exportServiceContext {
	return exportServiceContext{
		root:     input.root,
		platform: input.platform,
		graph:    graph,
		project:  project,
	}
}

package app

import (
	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmanifest"
	"github.com/777genius/plugin-kit-ai/cli/internal/runtimecheck"
)

type exportServiceContext struct {
	root     string
	platform string
	graph    pluginmanifest.PackageGraph
	project  runtimecheck.Project
}

func loadExportServiceContext(opts PluginExportOptions) (exportServiceContext, error) {
	input, err := resolveExportServiceInput(opts)
	if err != nil {
		return exportServiceContext{}, err
	}
	graph, project, err := loadExportServiceContextDeps(input)
	if err != nil {
		return exportServiceContext{}, err
	}
	return buildExportServiceContext(input, graph, project), nil
}

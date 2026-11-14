package app

import "github.com/777genius/plugin-kit-ai/cli/internal/pluginmanifest"

func loadGeneratedExportArtifacts(ctx exportServiceContext) (pluginmanifest.RenderResult, error) {
	return pluginmanifest.Generate(ctx.root, ctx.platform)
}

func buildExportArchiveFileSet(ctx exportServiceContext, generated pluginmanifest.RenderResult) ([]string, error) {
	return exportFileList(ctx.root, ctx.graph, ctx.project, generated.Artifacts)
}

package app

import "slices"

func resolveExportArchiveOutputPath(ctx exportServiceContext, output string) string {
	return exportOutputPath(ctx.root, ctx.graph.Manifest.Name, ctx.platform, ctx.graph.Launcher.Runtime, output)
}

func dropExportArchiveOutput(files []string, root, outputPath string) []string {
	if rel, ok := relWithinRoot(root, outputPath); ok {
		return slices.DeleteFunc(files, func(path string) bool { return path == rel })
	}
	return files
}

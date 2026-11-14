package platformexec

import "github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"

func buildOpenCodeBaseConfig(graph pluginmodel.PackageGraph, meta opencodePackageMeta) (map[string]any, error) {
	doc := map[string]any{
		"$schema": "https://opencode.ai/config.json",
	}
	if len(meta.Plugins) > 0 {
		doc["plugin"] = jsonValuesForOpenCodePlugins(meta.Plugins)
	}
	if graph.Portable.MCP != nil {
		projected, err := renderPortableMCPForTarget(graph.Portable.MCP, "opencode")
		if err != nil {
			return nil, err
		}
		doc["mcp"] = projected
	}
	return doc, nil
}

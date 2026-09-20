package platformexec

import "github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"

func buildOpenCodeConfig(root string, graph pluginmodel.PackageGraph, state pluginmodel.TargetState) (map[string]any, error) {
	inputs, err := loadOpenCodeConfigInputs(root, state)
	if err != nil {
		return nil, err
	}
	doc, err := buildOpenCodeBaseConfig(graph, inputs.meta)
	if err != nil {
		return nil, err
	}
	if err := appendOpenCodeConfigDocs(root, state, doc); err != nil {
		return nil, err
	}
	if err := pluginmodel.MergeNativeExtraObject(doc, inputs.extra, "opencode config.extra.json", inputs.managedPaths); err != nil {
		return nil, err
	}
	return doc, nil
}

func managedOpenCodeConfigPaths() []string {
	return []string{"$schema", "plugin", "mcp", "default_agent", "instructions", "permission", "mode"}
}

package platformexec

import "github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"

func (geminiAdapter) Validate(root string, graph pluginmodel.PackageGraph, state pluginmodel.TargetState) ([]Diagnostic, error) {
	meta, err := loadGeminiValidateMeta(root, state)
	if err != nil {
		return nil, err
	}
	return validateGeminiSurfaces(root, graph, state, meta)
}

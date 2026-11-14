package platformexec

import (
	"fmt"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func loadGeminiRenderMeta(root string, state pluginmodel.TargetState) (geminiPackageMeta, error) {
	meta, _, err := readYAMLDoc[geminiPackageMeta](root, state.DocPath("package_metadata"))
	if err != nil {
		return geminiPackageMeta{}, fmt.Errorf("parse %s: %w", state.DocPath("package_metadata"), err)
	}
	return meta, nil
}

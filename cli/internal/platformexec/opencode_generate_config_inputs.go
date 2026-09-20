package platformexec

import (
	"fmt"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

type openCodeConfigInputs struct {
	meta         opencodePackageMeta
	extra        pluginmodel.NativeExtraDoc
	managedPaths []string
}

func loadOpenCodeConfigInputs(root string, state pluginmodel.TargetState) (openCodeConfigInputs, error) {
	meta, _, err := readYAMLDoc[opencodePackageMeta](root, state.DocPath("package_metadata"))
	if err != nil {
		return openCodeConfigInputs{}, fmt.Errorf("parse %s: %w", state.DocPath("package_metadata"), err)
	}
	if err := validateOpenCodePluginRefs(meta.Plugins); err != nil {
		return openCodeConfigInputs{}, fmt.Errorf("%s %w", state.DocPath("package_metadata"), err)
	}
	extra, err := loadNativeExtraDoc(root, state, "config_extra", pluginmodel.NativeDocFormatJSON)
	if err != nil {
		return openCodeConfigInputs{}, err
	}
	managedPaths := managedOpenCodeConfigPaths()
	if err := pluginmodel.ValidateNativeExtraDocConflicts(extra, "opencode config.extra.json", managedPaths); err != nil {
		return openCodeConfigInputs{}, err
	}
	return openCodeConfigInputs{
		meta:         meta,
		extra:        extra,
		managedPaths: managedPaths,
	}, nil
}

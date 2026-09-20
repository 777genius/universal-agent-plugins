package platformexec

import (
	"fmt"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

type opencodeAdapter struct{}

func (opencodeAdapter) ID() string { return "opencode" }

func (opencodeAdapter) DetectNative(root string) bool {
	_, _, ok, err := resolveOpenCodeConfigPath(root)
	return err == nil && ok
}

func (opencodeAdapter) RefineDiscovery(root string, state *pluginmodel.TargetState) error {
	if rel := strings.TrimSpace(state.DocPath("package_metadata")); rel != "" {
		meta, ok, err := readYAMLDoc[opencodePackageMeta](root, rel)
		if err != nil {
			return fmt.Errorf("parse %s: %w", rel, err)
		}
		if ok {
			if err := validateOpenCodePluginRefs(meta.Plugins); err != nil {
				return fmt.Errorf("%s %w", rel, err)
			}
		}
	}
	state.AddComponent("local_plugin_code", discoverFiles(root, authoredOpenCodePluginDir(root, *state), nil)...)
	return nil
}

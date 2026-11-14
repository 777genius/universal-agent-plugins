package platformexec

import "github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"

type artifactDir struct {
	src string
	dst string
}

func loadNativeExtraDoc(root string, state pluginmodel.TargetState, kind string, format pluginmodel.NativeDocFormat) (pluginmodel.NativeExtraDoc, error) {
	return pluginmodel.LoadNativeExtraDoc(root, state.DocPath(kind), format)
}

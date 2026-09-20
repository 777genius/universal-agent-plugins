package platformexec

import (
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func resolveOpenCodeConfigPathInDir(dir string, warningBase string) (string, []pluginmodel.Warning, bool, error) {
	jsonRel := "opencode.json"
	jsoncRel := "opencode.jsonc"
	jsonPath := filepath.Join(dir, jsonRel)
	jsoncPath := filepath.Join(dir, jsoncRel)
	hasJSON := fileExists(jsonPath)
	hasJSONC := fileExists(jsoncPath)
	warnPath := jsoncRel
	if strings.TrimSpace(warningBase) != "" {
		warnPath = filepath.ToSlash(filepath.Join(warningBase, jsoncRel))
	}
	switch {
	case hasJSON && hasJSONC:
		return jsonPath, []pluginmodel.Warning{{
			Kind:    pluginmodel.WarningFidelity,
			Path:    warnPath,
			Message: "ignored opencode.jsonc because opencode.json takes precedence during OpenCode import normalization",
		}}, true, nil
	case hasJSON:
		return jsonPath, nil, true, nil
	case hasJSONC:
		return jsoncPath, nil, true, nil
	default:
		return "", nil, false, nil
	}
}

func resolveOpenCodeConfigPath(root string) (string, []pluginmodel.Warning, bool, error) {
	path, warnings, ok, err := resolveOpenCodeConfigPathInDir(root, "")
	if err != nil || !ok {
		return "", warnings, ok, err
	}
	rel, rerr := filepath.Rel(root, path)
	if rerr != nil {
		return "", nil, false, rerr
	}
	return filepath.ToSlash(rel), warnings, true, nil
}

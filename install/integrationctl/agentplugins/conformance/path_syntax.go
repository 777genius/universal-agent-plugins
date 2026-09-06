package conformance

import "strings"

// ParseCommandPath preserves traversal segments for the observing filesystem adapter.
func ParseCommandPath(value string) (string, error) {
	if !strings.HasPrefix(value, "./") || strings.Contains(value, "\\") {
		return "", semanticError("stdio_command_path", "bundled command requires a plugin-relative path")
	}
	return value[2:], nil
}

// ParseCWDPath decodes only the published root grammar, without filesystem policy.
func ParseCWDPath(value string) (anchor, relative string, err error) {
	switch {
	case value == "" || value == "${PLUGIN_ROOT}":
		return "plugin", "", nil
	case value == "${PLUGIN_DATA}":
		return "data", "", nil
	case strings.HasPrefix(value, "./"):
		return "plugin", value[2:], nil
	case strings.HasPrefix(value, "${PLUGIN_ROOT}/"):
		return "plugin", strings.TrimPrefix(value, "${PLUGIN_ROOT}/"), nil
	case strings.HasPrefix(value, "${PLUGIN_DATA}/"):
		return "data", strings.TrimPrefix(value, "${PLUGIN_DATA}/"), nil
	default:
		return "", "", semanticError("stdio_cwd_root", "cwd requires a declared standard root")
	}
}

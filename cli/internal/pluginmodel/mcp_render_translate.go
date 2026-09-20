package pluginmodel

import (
	"os"
	"strings"
)

func translatePortableMCPProjectionValue(target string, value any) any {
	switch typed := value.(type) {
	case string:
		return translatePortableMCPProjectionString(target, typed)
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, entry := range typed {
			out[key] = translatePortableMCPProjectionValue(target, entry)
		}
		return out
	case []any:
		out := make([]any, 0, len(typed))
		for _, entry := range typed {
			out = append(out, translatePortableMCPProjectionValue(target, entry))
		}
		return out
	default:
		return value
	}
}

func translatePortableMCPProjectionString(target, value string) string {
	replacements := portableMCPProjectionVariableReplacements(target)
	for from, to := range replacements {
		value = strings.ReplaceAll(value, from, to)
	}
	return value
}

func portableMCPProjectionVariableReplacements(target string) map[string]string {
	target = NormalizeTarget(target)
	packageRoot := "."
	switch target {
	case "gemini":
		packageRoot = "${extensionPath}"
	case "cursor-workspace":
		packageRoot = "${workspaceFolder}"
	}
	return map[string]string{
		"${package.root}": packageRoot,
		"${path.sep}":     string(os.PathSeparator),
	}
}

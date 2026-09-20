package validate

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmanifest"
	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

var runtimeFoundPathPattern = regexp.MustCompile(`\bat (.+?) but\b`)

func extractFailurePath(message string) string {
	switch {
	case strings.HasPrefix(message, "parse "):
		rest := strings.TrimPrefix(message, "parse ")
		idx := strings.Index(rest, ":")
		if idx <= 0 {
			return ""
		}
		return canonicalAuthoredPath(rest[:idx])
	case strings.HasPrefix(message, "required launcher missing: "):
		return canonicalAuthoredPath(strings.TrimSpace(strings.TrimPrefix(message, "required launcher missing: ")))
	case strings.HasPrefix(message, "launcher invalid: missing "):
		return canonicalAuthoredPath(strings.TrimSpace(strings.TrimPrefix(message, "launcher invalid: missing ")))
	case strings.HasPrefix(message, "launcher invalid: not executable "):
		return canonicalAuthoredPath(strings.TrimSpace(strings.TrimPrefix(message, "launcher invalid: not executable ")))
	case strings.HasPrefix(message, "invalid "):
		rest := strings.TrimPrefix(message, "invalid ")
		idx := strings.Index(rest, ":")
		if idx <= 0 {
			return ""
		}
		return canonicalAuthoredPath(rest[:idx])
	case strings.HasPrefix(message, "unsupported portable MCP authored path "):
		rest := strings.TrimPrefix(message, "unsupported portable MCP authored path ")
		idx := strings.Index(rest, ":")
		if idx <= 0 {
			return ""
		}
		return canonicalAuthoredPath(rest[:idx])
	case strings.HasPrefix(message, "unsupported authored layout: legacy "+pluginmodel.LegacySourceDirName+"/"):
		return pluginmodel.LegacySourceDirName
	case strings.HasPrefix(message, "runtime not found: "):
		rest := strings.TrimPrefix(message, "runtime not found: ")
		if idx := strings.Index(rest, "parse "); idx >= 0 {
			if path := extractFailurePath(rest[idx:]); path != "" {
				return path
			}
		}
		switch {
		case strings.HasPrefix(rest, "node runtime required; checked PATH for node"):
			return "node"
		case strings.HasPrefix(rest, "bash"), strings.HasPrefix(rest, "shell runtime requires bash"):
			return "bash"
		}
		if match := runtimeFoundPathPattern.FindStringSubmatch(rest); len(match) == 2 {
			return strings.TrimSpace(match[1])
		}
		return ""
	case strings.Contains(message, " file "):
		idx := strings.LastIndex(message, " file ")
		if idx < 0 {
			return ""
		}
		rest := message[idx+len(" file "):]
		colon := strings.Index(rest, ":")
		if colon <= 0 {
			return ""
		}
		return canonicalAuthoredPath(rest[:colon])
	default:
		return ""
	}
}

func canonicalAuthoredPath(path string) string {
	path = filepath.ToSlash(filepath.Clean(strings.TrimSpace(path)))
	if path == "." || path == "" {
		return ""
	}
	if strings.HasPrefix(path, pluginmodel.SourceDirName+"/") || strings.HasPrefix(path, pluginmodel.LegacySourceDirName+"/") || pluginmodel.IsAuthoredRoot(path) {
		return path
	}
	switch {
	case path == pluginmanifest.FileName,
		path == pluginmanifest.LauncherFileName,
		strings.HasPrefix(path, "targets/"),
		strings.HasPrefix(path, "skills/"),
		strings.HasPrefix(path, "publish/"),
		strings.HasPrefix(path, "mcp/"):
		return filepath.ToSlash(filepath.Join(pluginmodel.SourceDirName, path))
	default:
		return path
	}
}

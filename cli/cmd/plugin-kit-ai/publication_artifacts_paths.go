package main

import (
	"os"
	"path/filepath"
	"strings"
)

func normalizePublicationRequestedTarget(target string) string {
	target = strings.TrimSpace(target)
	if target == "" {
		return "all"
	}
	return target
}

func expectedPackageArtifactPath(target string) string {
	switch target {
	case "codex-package":
		return filepath.ToSlash(filepath.Join(".codex-plugin", "plugin.json"))
	case "claude":
		return filepath.ToSlash(filepath.Join(".claude-plugin", "plugin.json"))
	case "gemini":
		return "gemini-extension.json"
	default:
		return ""
	}
}

func expectedChannelArtifactPath(family string) string {
	switch family {
	case "codex-marketplace":
		return filepath.ToSlash(filepath.Join(".agents", "plugins", "marketplace.json"))
	case "claude-marketplace":
		return filepath.ToSlash(filepath.Join(".claude-plugin", "marketplace.json"))
	default:
		return ""
	}
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func isPublicationRelevantPath(path string) bool {
	switch filepath.ToSlash(filepath.Clean(path)) {
	case filepath.ToSlash(filepath.Join(".agents", "plugins", "marketplace.json")),
		filepath.ToSlash(filepath.Join(".claude-plugin", "marketplace.json")),
		filepath.ToSlash(filepath.Join(".codex-plugin", "plugin.json")),
		filepath.ToSlash(filepath.Join(".claude-plugin", "plugin.json")),
		"gemini-extension.json":
		return true
	default:
		return false
	}
}

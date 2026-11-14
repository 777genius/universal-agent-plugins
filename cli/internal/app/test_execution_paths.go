package app

import (
	"path/filepath"
	"strings"
)

func resolveFixturePath(root, requested, platform, event string) string {
	if strings.TrimSpace(requested) == "" {
		return filepath.Join(root, "fixtures", platform, event+".json")
	}
	return resolvePath(root, requested)
}

func resolveGoldenDir(root, requested, platform string) string {
	if strings.TrimSpace(requested) == "" {
		return filepath.Join(root, "goldens", platform)
	}
	return resolvePath(root, requested)
}

func resolvePath(root, path string) string {
	path = strings.TrimSpace(path)
	if path == "" || filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(root, path)
}

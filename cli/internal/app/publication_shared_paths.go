package app

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
)

func normalizePackageRoot(value, pluginName string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		value = filepath.ToSlash(filepath.Join("plugins", pluginName))
	}
	value = filepath.ToSlash(filepath.Clean(value))
	if value == "." || value == "" {
		return "", fmt.Errorf("package root must stay below the marketplace root")
	}
	if strings.HasPrefix(value, "/") || value == ".." || strings.HasPrefix(value, "../") || strings.Contains(value, "/../") {
		return "", fmt.Errorf("package root must stay relative to the marketplace root")
	}
	return value, nil
}

func sortedSlashPaths(paths []string) []string {
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		path = filepath.ToSlash(strings.TrimSpace(path))
		if path == "" {
			continue
		}
		out = append(out, path)
	}
	slices.Sort(out)
	return out
}

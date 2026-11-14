package app

import (
	"fmt"
	"path/filepath"
	"strings"
)

func exportOutputPath(root, name, platform, runtime, output string) string {
	if strings.TrimSpace(output) != "" {
		return output
	}
	file := fmt.Sprintf("%s_%s_%s_bundle.tar.gz", name, platform, runtime)
	return filepath.Join(root, file)
}

func relWithinRoot(root, path string) (string, bool) {
	if strings.TrimSpace(path) == "" {
		return "", false
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", false
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", false
	}
	rel, err := filepath.Rel(absRoot, absPath)
	if err != nil {
		return "", false
	}
	rel = filepath.ToSlash(rel)
	if rel == "." || strings.HasPrefix(rel, "../") {
		return "", false
	}
	return rel, true
}

package app

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
)

func launcherBundlePaths(root, entrypoint string) []string {
	base := normalizeExportPath(entrypoint)
	if base == "" {
		return nil
	}
	candidates := []string{base}
	if !strings.HasSuffix(base, ".cmd") {
		candidates = append(candidates, base+".cmd")
	}
	out := make([]string, 0, len(candidates))
	for _, rel := range candidates {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); err == nil {
			out = append(out, rel)
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}

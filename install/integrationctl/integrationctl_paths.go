package integrationctl

import (
	"os"
	"path/filepath"
	"strings"
)

func discoverRepoRoot(start string) string {
	dir := start
	for {
		if fileExists(filepath.Join(dir, ".git")) && fileExists(filepath.Join(dir, "docs")) {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return start
		}
		dir = parent
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func NormalizeTargets(targets []string) []string {
	out := make([]string, 0, len(targets))
	for _, target := range targets {
		target = strings.ToLower(strings.TrimSpace(target))
		if target == "" {
			continue
		}
		out = append(out, target)
	}
	return out
}

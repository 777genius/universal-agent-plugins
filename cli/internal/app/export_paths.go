package app

import (
	"strings"
)

func addExportPath(set map[string]struct{}, rel string) {
	rel = normalizeExportPath(rel)
	if rel == "" {
		return
	}
	set[rel] = struct{}{}
}

func shouldExcludeExportPath(rel string, excludes []string) bool {
	for _, exclude := range excludes {
		exclude = normalizeExportPath(exclude)
		if exclude == "" {
			continue
		}
		if rel == exclude || strings.HasPrefix(rel, exclude+"/") {
			return true
		}
	}
	return false
}

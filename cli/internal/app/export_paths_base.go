package app

import (
	"path/filepath"
	"strings"
)

func normalizeExportPath(rel string) string {
	rel = strings.TrimPrefix(strings.TrimSpace(rel), "./")
	rel = filepath.ToSlash(filepath.Clean(rel))
	if rel == "." || rel == "" {
		return ""
	}
	return rel
}

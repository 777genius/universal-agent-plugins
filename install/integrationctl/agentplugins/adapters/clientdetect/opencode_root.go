package clientdetect

import (
	"path/filepath"
	"runtime"
	"strings"
)

// OpenCodeRootIdentity resolves existing ancestors even when the selected
// config root has not been created yet. Ordinary directory creation preserves it.
func OpenCodeRootIdentity(root string) string {
	if !filepath.IsAbs(root) {
		return ""
	}
	var tail []string
	for current := filepath.Clean(root); ; current = filepath.Dir(current) {
		if resolved, err := filepath.EvalSymlinks(current); err == nil {
			for i := len(tail) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, tail[i])
			}
			resolved = filepath.Clean(resolved)
			if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
				resolved = strings.ToLower(resolved)
			}
			return resolved
		}
		if filepath.Dir(current) == current {
			return ""
		}
		tail = append(tail, filepath.Base(current))
	}
}

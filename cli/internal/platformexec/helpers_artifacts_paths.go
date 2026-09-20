package platformexec

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"
)

func discoverFiles(root, dir string, allow func(string) bool) []string {
	full := filepath.Join(root, dir)
	var out []string
	filepath.WalkDir(full, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d == nil || d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if allow != nil && !allow(rel) {
			return nil
		}
		out = append(out, rel)
		return nil
	})
	slices.Sort(out)
	return out
}

func cleanRelativeRef(path string) string {
	path = filepath.Clean(strings.TrimSpace(path))
	path = strings.TrimPrefix(path, "./")
	if path == "." {
		return ""
	}
	return path
}

func resolveRelativeRef(root, ref string) (string, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", nil
	}
	if filepath.IsAbs(ref) {
		return "", fmt.Errorf("ref %q must stay within the plugin root", ref)
	}
	cleaned := filepath.Clean(ref)
	if cleaned == "." {
		return "", nil
	}
	if cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("ref %q must stay within the plugin root", ref)
	}
	cleaned = strings.TrimPrefix(cleaned, "."+string(filepath.Separator))
	cleaned = filepath.ToSlash(cleaned)
	if cleaned == "" || cleaned == "." {
		return "", nil
	}
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("ref %q must stay within the plugin root", ref)
	}
	return cleaned, nil
}

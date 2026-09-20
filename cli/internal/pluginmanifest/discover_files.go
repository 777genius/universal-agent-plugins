package pluginmanifest

import (
	"io/fs"
	"path/filepath"
	"slices"
)

func discoverFiles(root, dir string, keep func(rel string) bool) []string {
	full := filepath.Join(root, dir)
	var out []string
	_ = filepath.WalkDir(full, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d == nil || d.IsDir() {
			return nil
		}
		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if keep != nil && !keep(rel) {
			return nil
		}
		out = append(out, rel)
		return nil
	})
	slices.Sort(out)
	return out
}

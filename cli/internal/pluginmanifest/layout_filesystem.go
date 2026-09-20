package pluginmanifest

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

func authoredInputExists(root, rel string) bool {
	full := filepath.Join(root, filepath.FromSlash(rel))
	info, err := os.Stat(full)
	if err != nil {
		return false
	}
	if !info.IsDir() {
		return true
	}
	var hasFile bool
	_ = filepath.WalkDir(full, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d == nil || d.IsDir() {
			return nil
		}
		hasFile = true
		return io.EOF
	})
	return hasFile
}

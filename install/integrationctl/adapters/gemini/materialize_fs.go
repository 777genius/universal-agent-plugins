package gemini

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/filetree"
)

func discoverFiles(root string) ([]string, error) {
	if _, err := os.Stat(root); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d == nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		out = append(out, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, err
	}
	slices.Sort(out)
	return out, nil
}

func copyDirIfExists(src, dest string) error {
	info, err := os.Lstat(src)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if !info.IsDir() {
		return nil
	}
	return filetree.CopyDir(src, dest)
}

func copyFile(src, dest string) error {
	return filetree.CopyFile(src, dest)
}

func isYAMLPath(rel string) bool {
	switch strings.ToLower(filepath.Ext(rel)) {
	case ".yaml", ".yml":
		return true
	default:
		return false
	}
}

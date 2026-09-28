package clients

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// CanonicalDirectory resolves existing symlink components and retains a missing
// suffix without creating it. A dangling link, file or unreadable ancestor is
// an error, never permission to select a different profile. All observations
// use the injected probes; synthetic hosts never read the ambient filesystem.
func CanonicalDirectory(path string, lstat func(string) (fs.FileInfo, error), eval func(string) (string, error)) (string, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path || lstat == nil {
		return "", fmt.Errorf("profile directory must be an explicit clean absolute path")
	}
	root := filepath.VolumeName(path) + string(filepath.Separator)
	parts := strings.Split(strings.TrimPrefix(path, root), string(filepath.Separator))
	current := root
	for i, part := range parts {
		if part == "" {
			continue
		}
		current = filepath.Join(current, part)
		info, err := lstat(current)
		if os.IsNotExist(err) {
			return filepath.Join(append([]string{current}, parts[i+1:]...)...), nil
		}
		if err != nil {
			return "", fmt.Errorf("inspect profile directory: %w", err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			if eval == nil {
				return "", fmt.Errorf("profile symlink resolver is unavailable")
			}
			current, err = eval(current)
			if err != nil {
				return "", fmt.Errorf("resolve profile symlink: %w", err)
			}
			if !filepath.IsAbs(current) || filepath.Clean(current) != current {
				return "", fmt.Errorf("profile symlink resolved to an invalid path")
			}
			info, err = lstat(current)
			if err != nil {
				return "", fmt.Errorf("inspect resolved profile directory: %w", err)
			}
		}
		if !info.IsDir() {
			return "", fmt.Errorf("profile path is not a directory")
		}
	}
	return current, nil
}

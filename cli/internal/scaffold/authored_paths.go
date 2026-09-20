package scaffold

import "path/filepath"

const authoredRootDir = "plugin"

func authoredPath(rel string) string {
	return filepath.ToSlash(filepath.Join(authoredRootDir, rel))
}

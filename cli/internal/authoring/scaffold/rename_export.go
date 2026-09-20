package scaffold

import "os"

// RenameExclusive atomically moves one directory entry while requiring the
// destination to remain absent. Both names are rooted single components.
func RenameExclusive(from *os.File, old string, to *os.File, new string) error {
	return renameExclusive(from, old, to, new)
}

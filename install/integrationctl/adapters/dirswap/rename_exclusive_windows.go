//go:build windows

package dirswap

import "os"

// Windows rename rejects an existing directory destination.
func renameDirectoryExclusive(oldPath, newPath string) error { return os.Rename(oldPath, newPath) }

//go:build !darwin && !(linux && amd64) && !(linux && arm64)

package shared

import "os"

// Platforms without a native exclusive-rename primitive still get the
// immediate collision check in renameGeminiDirectoryNoReplace. Windows rename
// itself rejects an existing destination.
func RenameDirectoryExclusive(oldPath, newPath string) error {
	return os.Rename(oldPath, newPath)
}

//go:build windows

package dirswap

import (
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

// MoveFileEx without replace/copy flags refuses every existing destination and
// performs a same-volume rename, including publication and backup restoration.
func renameDirectoryExclusive(oldPath, newPath string) error {
	from, err := renamePathUTF16(oldPath)
	if err == nil {
		var to *uint16
		to, err = renamePathUTF16(newPath)
		if err == nil {
			err = windows.MoveFileEx(from, to, 0)
		}
	}
	if err != nil {
		return &os.LinkError{Op: "rename", Old: oldPath, New: newPath, Err: err}
	}
	return nil
}

// Preserve os.Rename's long-path support without depending on host policy.
// FullPath is lexical: it does not resolve aliases or follow filesystem links.
func renamePathUTF16(path string) (*uint16, error) {
	normalized := filepath.FromSlash(path)
	if strings.HasPrefix(normalized, `\\?\`) || strings.HasPrefix(normalized, `\??\`) || strings.HasPrefix(normalized, `\\.\`) {
		return windows.UTF16PtrFromString(path)
	}
	abs, err := windows.FullPath(path)
	if err != nil {
		return nil, err
	}
	// Go's directory-safe threshold is MAX_PATH minus 12, measured in bytes.
	if len(abs) >= 248 {
		if strings.HasPrefix(abs, `\\`) {
			path = `\\?\UNC\` + abs[2:]
		} else {
			path = `\\?\` + abs
		}
	}
	return windows.UTF16PtrFromString(path)
}

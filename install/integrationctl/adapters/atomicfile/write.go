package atomicfile

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Write replaces path with a fully written and synced file from the same
// directory, then syncs the parent directory where the platform supports it.
func Write(path string, data []byte, mode os.FileMode) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("atomic file path is required")
	}
	path = filepath.Clean(path)
	parent := filepath.Dir(path)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("create atomic file parent: %w", err)
	}
	if info, err := os.Lstat(path); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return fmt.Errorf("atomic file target must be a regular file")
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect atomic file target: %w", err)
	}
	temp, err := os.CreateTemp(parent, filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create atomic file temp: %w", err)
	}
	tempPath := temp.Name()
	cleanup := func() { _ = os.Remove(tempPath) }
	written, err := temp.Write(data)
	if err == nil && written != len(data) {
		err = fmt.Errorf("short write: wrote %d of %d bytes", written, len(data))
	}
	if err != nil {
		_ = temp.Close()
		cleanup()
		return fmt.Errorf("write atomic file temp: %w", err)
	}
	if err := temp.Chmod(mode.Perm()); err != nil {
		_ = temp.Close()
		cleanup()
		return fmt.Errorf("chmod atomic file temp: %w", err)
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		cleanup()
		return fmt.Errorf("sync atomic file temp: %w", err)
	}
	if err := temp.Close(); err != nil {
		cleanup()
		return fmt.Errorf("close atomic file temp: %w", err)
	}
	if err := Replace(tempPath, path); err != nil {
		cleanup()
		return fmt.Errorf("replace atomic file target: %w", err)
	}
	if err := SyncDirectory(parent); err != nil {
		return fmt.Errorf("sync atomic file parent: %w", err)
	}
	return nil
}

// SyncDirectory durably records directory-entry changes where supported.
func SyncDirectory(path string) error {
	return syncParent(path)
}

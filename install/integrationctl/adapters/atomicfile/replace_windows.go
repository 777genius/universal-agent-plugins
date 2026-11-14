//go:build windows

package atomicfile

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Replace publishes an already synced and closed temporary file. Root.Rename
// supports delete-sharing readers without removing the target first.
func Replace(source, target string) error {
	source, err := filepath.Abs(source)
	if err != nil {
		return err
	}
	target, err = filepath.Abs(target)
	if err != nil {
		return err
	}
	parent := filepath.Dir(source)
	if parent != filepath.Dir(target) {
		return fmt.Errorf("atomic file replacement requires the same parent directory")
	}
	root, err := os.OpenRoot(parent)
	if err != nil {
		return err
	}
	err = root.Rename(filepath.Base(source), filepath.Base(target))
	if closeErr := root.Close(); closeErr != nil {
		return errors.Join(err, closeErr)
	}
	return err
}

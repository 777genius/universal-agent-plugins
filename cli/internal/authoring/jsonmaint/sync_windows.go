//go:build windows

package jsonmaint

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

func syncParent(parent *os.File) error {
	err := parent.Sync()
	// FlushFileBuffers does not support directory handles on Windows. The new
	// file is flushed before ReplaceFileW; retain every other sync failure.
	if errors.Is(err, windows.ERROR_ACCESS_DENIED) {
		return nil
	}
	return err
}

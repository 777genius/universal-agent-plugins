//go:build !linux && !darwin && !windows

package scaffold

import (
	"fmt"
	"os"
	"runtime"
)

func renameExclusive(from *os.File, old string, to *os.File, new string) error {
	return fmt.Errorf("absence-preserving scaffold commit is unsupported on %s", runtime.GOOS)
}

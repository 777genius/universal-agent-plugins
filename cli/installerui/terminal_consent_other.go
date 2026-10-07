//go:build !linux && !darwin && !freebsd && !openbsd && !netbsd && !dragonfly

package installerui

import (
	"os"
)

func queuedInputBytes(*os.File) (int, error) {
	return 0, ErrUnavailable
}

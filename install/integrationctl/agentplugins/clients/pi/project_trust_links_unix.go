//go:build !windows

package pi

import (
	"fmt"
	"os"
	"syscall"
)

func piTrustSingleLink(_ *os.File, info os.FileInfo) (bool, error) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return false, fmt.Errorf("file link count is unavailable")
	}
	return stat.Nlink == 1, nil
}

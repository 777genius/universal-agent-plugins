//go:build darwin || linux

package dirswap

import (
	"fmt"
	"os"
	"syscall"
)

func directoryIdentity(path string, info os.FileInfo) (string, error) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return "", fmt.Errorf("directory identity unavailable")
	}
	return fmt.Sprintf("%d:%d", stat.Dev, stat.Ino), nil
}

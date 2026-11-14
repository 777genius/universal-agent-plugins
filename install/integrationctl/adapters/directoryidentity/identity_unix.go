//go:build darwin || linux

package directoryidentity

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// LegacyIdentity preserves the historical dev:inode encoding. It deliberately
// requires no UUID; legacy dirswap consumers have no durable-profile opt-in.
func LegacyIdentity(_ string, info os.FileInfo) (string, error) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return "", fmt.Errorf("directory identity unavailable")
	}
	return fmt.Sprintf("%d:%d", stat.Dev, stat.Ino), nil
}

// LegacyHandleIdentity preserves the legacy encoding from an already held
// ordinary directory. It does not opt this consumer into physical profiles.
func LegacyHandleIdentity(file *os.File) (string, error) {
	info, err := file.Stat()
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("held ordinary directory unavailable")
	}
	return LegacyIdentity("", info)
}

func canonicalize(p string) (string, error) {
	if err := validatePath(p, platformScheme()); err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(p)
}

//go:build darwin || linux

package dirswap

import (
	"fmt"
	"os"
	"syscall"
)

func publicationEntryFacts(_ *os.File, info os.FileInfo) (string, string, error) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Ino == 0 || stat.Nlink == 0 {
		return "", "", fmt.Errorf("publication object identity unavailable")
	}
	if info.Mode().IsRegular() && stat.Nlink != 1 {
		return "", "", fmt.Errorf("publication regular object has multiple links")
	}
	return fmt.Sprintf("%d:%d", stat.Dev, stat.Ino),
		fmt.Sprintf("%d:%d:%d", stat.Uid, stat.Gid, stat.Nlink), nil
}

func samePublicationMetadata(before, after os.FileInfo) bool {
	left, leftOK := before.Sys().(*syscall.Stat_t)
	right, rightOK := after.Sys().(*syscall.Stat_t)
	return leftOK && rightOK && left != nil && right != nil &&
		left.Uid == right.Uid && left.Gid == right.Gid && left.Nlink == right.Nlink
}

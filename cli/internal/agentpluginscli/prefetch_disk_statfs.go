//go:build darwin || linux

package agentpluginscli

import "golang.org/x/sys/unix"

// freeDiskBytes reports the bytes available to this user on the filesystem
// holding path.
func freeDiskBytes(path string) (uint64, bool) {
	var stat unix.Statfs_t
	if err := unix.Statfs(path, &stat); err != nil || stat.Bsize <= 0 {
		return 0, false
	}
	return stat.Bavail * uint64(stat.Bsize), true
}

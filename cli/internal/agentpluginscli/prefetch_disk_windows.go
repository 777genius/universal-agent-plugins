//go:build windows

package agentpluginscli

import "golang.org/x/sys/windows"

// freeDiskBytes reports the bytes available to this user on the volume holding
// path.
func freeDiskBytes(path string) (uint64, bool) {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, false
	}
	var available, total, free uint64
	if err := windows.GetDiskFreeSpaceEx(name, &available, &total, &free); err != nil {
		return 0, false
	}
	return available, true
}

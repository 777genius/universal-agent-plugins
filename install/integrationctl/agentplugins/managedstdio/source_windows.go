//go:build windows

package managedstdio

import "os"

// Windows does not expose POSIX execute bits. Trust is established by the
// caller-provided absolute path and byte digest; CreateProcess performs the
// platform executable validation when the helper is launched.
func executableFile(info os.FileInfo, _ string) bool {
	return info.Mode().IsRegular()
}

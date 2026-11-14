//go:build !windows

package managedstdio

import "os"

const ExecutableName = "agentplugins"

func executableFile(info os.FileInfo, _ string) bool {
	return info.Mode().IsRegular() && info.Mode().Perm()&0111 != 0
}

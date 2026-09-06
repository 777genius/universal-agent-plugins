//go:build darwin || linux

package managedstdio

import (
	"os"
	"syscall"
)

func Supported() bool { return true }
func replaceProcess(command string, args []string, cwd string) error {
	if err := os.Chdir(cwd); err != nil {
		return err
	}
	return syscall.Exec(command, args, os.Environ())
}

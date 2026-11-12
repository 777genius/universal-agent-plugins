//go:build darwin || linux

package managedstdio

import (
	"os"
	"syscall"
)

func Supported() bool { return true }
func replaceProcess(command string, args []string, cwd string) error {
	return replaceProcessEnvironment(command, args, cwd, os.Environ())
}

func replaceProcessEnvironment(command string, args []string, cwd string, environment []string) error {
	if err := os.Chdir(cwd); err != nil {
		return err
	}
	return syscall.Exec(command, args, environment)
}

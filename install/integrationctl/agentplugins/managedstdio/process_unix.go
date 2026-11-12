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
	// #nosec G702 -- StartSession verifies a consented absolute executable with exact argv[0];
	// replacing this process without shell evaluation is the explicit session boundary.
	return syscall.Exec(command, args, environment)
}

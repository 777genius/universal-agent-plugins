//go:build darwin || linux

package managedstdio

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

func Supported() bool { return true }
func replaceProcess(command string, args []string, cwd string) error {
	return replaceProcessEnvironment(command, args, cwd, os.Environ())
}

func replaceProcessEnvironment(command string, args []string, cwd string, environment []string) error {
	previous, err := os.Getwd()
	if err != nil {
		return err
	}
	if err := os.Chdir(cwd); err != nil {
		return err
	}
	// #nosec G702 -- StartSession validates explicit absolute argv and consent;
	// exec replaces the process directly without a shell or command parsing.
	execErr := syscall.Exec(command, args, environment)
	if restoreErr := os.Chdir(previous); restoreErr != nil {
		return errors.Join(execErr, fmt.Errorf("restore cwd after failed native exec: %w", restoreErr))
	}
	return execErr
}

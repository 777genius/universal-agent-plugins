//go:build !linux && !darwin && !windows

package hermes

import (
	"fmt"
	"os"
)

func validateSessionHandle(*os.File, os.FileInfo) error {
	return fmt.Errorf("hermes session file identity is unsupported on this platform")
}

func validateSessionDirectory(os.FileInfo) error {
	return fmt.Errorf("hermes session directory identity is unsupported on this platform")
}

func validateSessionDirectoryHandle(*os.File, os.FileInfo) error {
	return fmt.Errorf("hermes session directory identity is unsupported on this platform")
}

func validateSessionOwnedHandle(file *os.File, info os.FileInfo) error {
	return validateSessionHandle(file, info)
}

func validateSessionOwnedDirectory(file *os.File, info os.FileInfo) error {
	return validateSessionDirectoryHandle(file, info)
}

func openSessionFile(root *os.Root, name string) (*os.File, error) {
	return root.Open(name)
}

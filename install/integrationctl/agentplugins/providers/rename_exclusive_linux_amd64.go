//go:build linux && amd64

package providers

import (
	"os"
	"syscall"
	"unsafe"
)

func renameDirectoryExclusive(oldPath, newPath string) error {
	oldPointer, err := syscall.BytePtrFromString(oldPath)
	if err != nil {
		return err
	}
	newPointer, err := syscall.BytePtrFromString(newPath)
	if err != nil {
		return err
	}
	const (
		sysRenameat2    = 316
		renameNoReplace = 1
	)
	atFDCWD := ^uintptr(99) // -100
	_, _, errno := syscall.Syscall6(sysRenameat2, atFDCWD, uintptr(unsafe.Pointer(oldPointer)), atFDCWD, uintptr(unsafe.Pointer(newPointer)), renameNoReplace, 0)
	if errno != 0 {
		return &os.LinkError{Op: "rename", Old: oldPath, New: newPath, Err: errno}
	}
	return nil
}

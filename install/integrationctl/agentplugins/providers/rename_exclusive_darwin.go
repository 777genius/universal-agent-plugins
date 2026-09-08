//go:build darwin

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
		sysRenameatxNP = 488
		renameExcl     = 0x00000004
	)
	atFDCWD := ^uintptr(1) // -2
	_, _, errno := syscall.Syscall6(sysRenameatxNP, atFDCWD, uintptr(unsafe.Pointer(oldPointer)), atFDCWD, uintptr(unsafe.Pointer(newPointer)), renameExcl, 0)
	if errno != 0 {
		return &os.LinkError{Op: "rename", Old: oldPath, New: newPath, Err: errno}
	}
	return nil
}

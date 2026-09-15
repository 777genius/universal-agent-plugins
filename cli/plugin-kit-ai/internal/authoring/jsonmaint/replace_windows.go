//go:build windows

package jsonmaint

import (
	"os"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
)

var replaceFileW = windows.NewLazySystemDLL("kernel32.dll").NewProc("ReplaceFileW")

func replaceOwned(_ *os.File, rootPath, replacement, target string) (string, error) {
	backup := ".authoring-json-old-" + randomName()
	err := replaceFile(filepath.Join(rootPath, target), filepath.Join(rootPath, replacement), filepath.Join(rootPath, backup))
	return backup, err
}

func restoreOwned(_ *os.File, rootPath, old, target string) (string, error) {
	discard := ".authoring-json-discard-" + randomName()
	err := replaceFile(filepath.Join(rootPath, target), filepath.Join(rootPath, old), filepath.Join(rootPath, discard))
	return discard, err
}

func replaceFile(target, replacement, backup string) error {
	targetPtr, err := windows.UTF16PtrFromString(target)
	if err != nil {
		return err
	}
	replacementPtr, err := windows.UTF16PtrFromString(replacement)
	if err != nil {
		return err
	}
	backupPtr, err := windows.UTF16PtrFromString(backup)
	if err != nil {
		return err
	}
	ok, _, callErr := replaceFileW.Call(
		uintptr(unsafe.Pointer(targetPtr)),
		uintptr(unsafe.Pointer(replacementPtr)),
		uintptr(unsafe.Pointer(backupPtr)),
		0, 0, 0,
	)
	if ok == 0 {
		return callErr
	}
	return nil
}

//go:build windows

package grok

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

func openProjectHandle(path string, directory bool) (*os.File, error) {
	return openProjectWindows(path, directory, nil)
}

// Every child is opened relative to its held directory handle. Reparse refusal
// is applied by the object manager at each hop, before a leaf can be followed.
func openProjectWindows(path string, directory bool, beforeLeaf func()) (*os.File, error) {
	volume := filepath.VolumeName(path)
	root := volume + string(filepath.Separator)
	pointer, err := windows.UTF16PtrFromString(root)
	if err != nil {
		return nil, err
	}
	parent, err := windows.CreateFile(pointer, windows.GENERIC_READ, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_OPEN_REPARSE_POINT|windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return nil, err
	}
	if err := projectWindowsHandleType(parent, true); err != nil {
		_ = windows.CloseHandle(parent)
		return nil, err
	}
	parts := strings.Split(strings.TrimPrefix(path, root), string(filepath.Separator))
	for index, part := range parts {
		if part == "" {
			_ = windows.CloseHandle(parent)
			return nil, fmt.Errorf("grok discovery has an empty path component")
		}
		leaf := index == len(parts)-1
		if leaf && beforeLeaf != nil {
			beforeLeaf()
		}
		child, openErr := openProjectWindowsChild(parent, part, !leaf || directory)
		_ = windows.CloseHandle(parent)
		if openErr != nil {
			return nil, openErr
		}
		parent = child
	}
	if err := projectWindowsAncestors(path); err != nil {
		_ = windows.CloseHandle(parent)
		return nil, err
	}
	return os.NewFile(uintptr(parent), path), nil
}
func openProjectWindowsChild(parent windows.Handle, name string, directory bool) (windows.Handle, error) {
	nativeName, err := windows.NewNTUnicodeString(name)
	if err != nil {
		return 0, err
	}
	attributes := windows.OBJECT_ATTRIBUTES{Length: uint32(unsafe.Sizeof(windows.OBJECT_ATTRIBUTES{})), RootDirectory: parent, ObjectName: nativeName, Attributes: windows.OBJ_CASE_INSENSITIVE | windows.OBJ_DONT_REPARSE}
	options := uint32(windows.FILE_OPEN_REPARSE_POINT | windows.FILE_SYNCHRONOUS_IO_NONALERT | windows.FILE_NON_DIRECTORY_FILE)
	if directory {
		options = windows.FILE_OPEN_REPARSE_POINT | windows.FILE_SYNCHRONOUS_IO_NONALERT | windows.FILE_DIRECTORY_FILE
	}
	var handle windows.Handle
	var status windows.IO_STATUS_BLOCK
	err = windows.NtCreateFile(&handle, windows.GENERIC_READ|windows.SYNCHRONIZE, &attributes, &status, nil, 0, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, windows.FILE_OPEN, options, 0, 0)
	if err != nil {
		if native, ok := err.(windows.NTStatus); ok {
			return 0, native.Errno()
		}
		return 0, err
	}
	if err := projectWindowsHandleType(handle, directory); err != nil {
		_ = windows.CloseHandle(handle)
		return 0, err
	}
	return handle, nil
}
func projectWindowsHandleType(handle windows.Handle, directory bool) error {
	kind, err := windows.GetFileType(handle)
	if err != nil {
		return err
	}
	if kind != windows.FILE_TYPE_DISK {
		return fmt.Errorf("grok discovery requires a disk handle")
	}
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(handle, &info); err != nil {
		return err
	}
	if info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 || (info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY != 0) != directory {
		return fmt.Errorf("grok discovery handle type is unqualified")
	}
	return nil
}
func projectWindowsAncestors(path string) error {
	current := filepath.VolumeName(path) + string(filepath.Separator)
	for _, part := range append([]string{""}, strings.Split(strings.TrimPrefix(path, current), string(filepath.Separator))...) {
		if part != "" {
			current = filepath.Join(current, part)
		}
		pointer, err := windows.UTF16PtrFromString(current)
		if err != nil {
			return err
		}
		attributes, err := windows.GetFileAttributes(pointer)
		if err != nil {
			return err
		}
		if attributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
			return fmt.Errorf("grok discovery reparse point is unqualified")
		}
	}
	return nil
}

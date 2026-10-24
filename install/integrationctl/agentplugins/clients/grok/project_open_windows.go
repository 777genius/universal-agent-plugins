//go:build windows

package grok

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

func openProjectHandle(path string, directory bool) (*os.File, error) {
	// Refuse any existing reparse-point ancestor before obtaining a leaf handle.
	for current := path; ; current = filepath.Dir(current) {
		ptr, err := windows.UTF16PtrFromString(current)
		if err != nil {
			return nil, err
		}
		attributes, err := windows.GetFileAttributes(ptr)
		if err != nil {
			return nil, err
		}
		if attributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
			return nil, fmt.Errorf("grok discovery reparse point is unqualified")
		}
		if filepath.Dir(current) == current {
			break
		}
	}
	ptr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	handle, err := windows.CreateFile(ptr, windows.GENERIC_READ, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_OPEN_REPARSE_POINT|windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return nil, err
	}
	fail := func(err error) (*os.File, error) { _ = windows.CloseHandle(handle); return nil, err }
	kind, err := windows.GetFileType(handle)
	if err != nil {
		return fail(err)
	}
	if kind != windows.FILE_TYPE_DISK {
		return fail(fmt.Errorf("grok discovery requires a disk handle"))
	}
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(handle, &info); err != nil {
		return fail(err)
	}
	if info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 || (info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY != 0) != directory {
		return fail(fmt.Errorf("grok discovery handle type is unqualified"))
	}
	return os.NewFile(uintptr(handle), path), nil
}

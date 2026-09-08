//go:build windows

package nativeconfig

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

func openNoFollow(path string) (*os.File, error) {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	handle, err := windows.CreateFile(
		name,
		windows.GENERIC_READ,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_ATTRIBUTE_NORMAL|windows.FILE_FLAG_OPEN_REPARSE_POINT,
		0,
	)
	if err != nil {
		return nil, err
	}
	closeHandle := true
	defer func() {
		if closeHandle {
			_ = windows.CloseHandle(handle)
		}
	}()
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(handle, &info); err != nil {
		return nil, fmt.Errorf("inspect opened native config handle: %w", err)
	}
	if info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return nil, fmt.Errorf("native config reparse point is not allowed")
	}
	if info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY != 0 {
		return nil, fmt.Errorf("native config directory is not allowed")
	}
	fileType, err := windows.GetFileType(handle)
	if err != nil {
		return nil, fmt.Errorf("inspect opened native config type: %w", err)
	}
	if fileType != windows.FILE_TYPE_DISK {
		return nil, fmt.Errorf("native config must be a regular disk file")
	}
	closeHandle = false
	return os.NewFile(uintptr(handle), path), nil
}

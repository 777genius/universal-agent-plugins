//go:build windows

package qoder

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

func openProjectManifest(path string) (*os.File, error) {
	pointer, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	handle, err := windows.CreateFile(pointer, windows.GENERIC_READ, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return nil, err
	}
	var info windows.ByHandleFileInformation
	kind, kindErr := windows.GetFileType(handle)
	infoErr := windows.GetFileInformationByHandle(handle, &info)
	if kindErr != nil || infoErr != nil || kind != windows.FILE_TYPE_DISK || info.FileAttributes&(windows.FILE_ATTRIBUTE_REPARSE_POINT|windows.FILE_ATTRIBUTE_DIRECTORY) != 0 {
		_ = windows.CloseHandle(handle)
		return nil, fmt.Errorf("qoder native manifest requires an ordinary disk file")
	}
	return os.NewFile(uintptr(handle), path), nil
}

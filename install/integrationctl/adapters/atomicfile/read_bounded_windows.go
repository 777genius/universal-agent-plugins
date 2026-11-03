//go:build windows

package atomicfile

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

func openRegularNoFollow(path string) (*os.File, error) {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	handle, err := windows.CreateFile(name, windows.GENERIC_READ,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING,
		windows.FILE_FLAG_OPEN_REPARSE_POINT|windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_NO_RECALL, 0)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}
	valid := false
	defer func() {
		if !valid {
			_ = windows.CloseHandle(handle)
		}
	}()
	kind, err := windows.GetFileType(handle)
	if err != nil {
		return nil, &os.PathError{Op: "GetFileType", Path: path, Err: err}
	}
	if kind != windows.FILE_TYPE_DISK {
		return nil, fmt.Errorf("%w: %s is not a disk file", ErrUnsafeFile, path)
	}
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(handle, &info); err != nil {
		return nil, &os.PathError{Op: "GetFileInformationByHandle", Path: path, Err: err}
	}
	if info.FileAttributes&(windows.FILE_ATTRIBUTE_REPARSE_POINT|windows.FILE_ATTRIBUTE_DIRECTORY|windows.FILE_ATTRIBUTE_DEVICE|windows.FILE_ATTRIBUTE_OFFLINE|windows.FILE_ATTRIBUTE_RECALL_ON_OPEN|windows.FILE_ATTRIBUTE_RECALL_ON_DATA_ACCESS) != 0 {
		return nil, fmt.Errorf("%w: %s is a reparse, directory or unavailable file", ErrUnsafeFile, path)
	}
	file := os.NewFile(uintptr(handle), path)
	if file == nil {
		return nil, fmt.Errorf("adopt regular-file handle for %s", path)
	}
	valid = true
	return file, nil
}

func openRootReadFile(root *os.Root, name string) (*os.File, error) {
	// os.Root resolves contained links using held directory handles and opens
	// with delete sharing. Root escape and Windows device names are rejected.
	return root.Open(name)
}

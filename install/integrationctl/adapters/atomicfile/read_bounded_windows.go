//go:build windows

package atomicfile

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

func openRegularNoFollow(path string) (*os.File, error) {
	openPath, err := windowsRegularReadPath(path)
	if err != nil {
		return nil, err
	}
	name, err := windows.UTF16PtrFromString(openPath)
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
	var tag struct{ FileAttributes, ReparseTag uint32 }
	if info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		if err := windows.GetFileInformationByHandleEx(handle, windows.FileAttributeTagInfo, (*byte)(unsafe.Pointer(&tag)), uint32(unsafe.Sizeof(tag))); err != nil {
			return nil, &os.PathError{Op: "GetFileInformationByHandleEx", Path: path, Err: err}
		}
	}
	if err := admitWindowsRegularRead(path, info.FileAttributes, tag.ReparseTag); err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(handle), path)
	if file == nil {
		return nil, fmt.Errorf("adopt regular-file handle for %s", path)
	}
	valid = true
	return file, nil
}

func windowsRegularReadPath(path string) (string, error) {
	// Existing extended/device names have their own parsing semantics; do not
	// normalize them through GetFullPathName before opening the observed file.
	if strings.HasPrefix(path, `\\?\`) || strings.HasPrefix(path, `\??\`) || strings.HasPrefix(path, `\\.\`) {
		return path, nil
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	// Match Go's long-path handling before calling the raw Win32 API. Preserve
	// device/extended namespaces and only add a prefix when Win32 needs one.
	if len(absolute) < 248 {
		return absolute, nil
	}
	if strings.HasPrefix(absolute, `\\`) {
		return `\\?\UNC\` + absolute[2:], nil
	}
	return `\\?\` + absolute, nil
}

func admitWindowsRegularRead(path string, attributes, reparseTag uint32) error {
	// Refuse unavailable bytes without reading or hydrating them. Availability
	// failures are operational and must not authorize replacing stored evidence.
	if attributes&(windows.FILE_ATTRIBUTE_OFFLINE|windows.FILE_ATTRIBUTE_RECALL_ON_OPEN|windows.FILE_ATTRIBUTE_RECALL_ON_DATA_ACCESS) != 0 {
		return &os.PathError{Op: "open", Path: path, Err: windows.ERROR_FILE_OFFLINE}
	}
	// Go's os.FileInfo treats IO_REPARSE_TAG_DEDUP as regular: Windows Server
	// optimization can transparently deduplicate an ordinary persistent file.
	// All other reparse types remain refused, including name-surrogate links.
	const reparseTagDedup = 0x80000013
	if attributes&(windows.FILE_ATTRIBUTE_DIRECTORY|windows.FILE_ATTRIBUTE_DEVICE) != 0 || attributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 && reparseTag != reparseTagDedup {
		return fmt.Errorf("%w: %s is a reparse, directory or device file", ErrUnsafeFile, path)
	}
	return nil
}

func openRootReadFile(root *os.Root, name string) (*os.File, error) {
	// os.Root resolves contained links using held directory handles and opens
	// with delete sharing. Root escape and Windows device names are rejected.
	return root.Open(name)
}

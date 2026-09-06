//go:build windows

package scaffold

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

// FILE_RENAME_INFORMATION with ReplaceIfExists FALSE fails if any destination
// exists. Both names are single components relative to held directory handles.
// This avoids MoveFileEx's pathname races and never enables REPLACE_IF_EXISTS or
// POSIX_SEMANTICS. Unsupported filesystems fail without a weaker fallback.
func renameExclusive(from *os.File, old string, to *os.File, new string) error {
	err := renameWindows(from, old, to, new)
	if err != nil {
		err = windowsRenameError(err, to, new)
	}
	runtime.KeepAlive(from)
	runtime.KeepAlive(to)
	if err != nil {
		return &os.LinkError{Op: "rename-exclusive", Old: old, New: new, Err: err}
	}
	return nil
}

func windowsRenameError(err error, to *os.File, new string) error {
	// Both NT calls return NTStatus, which lacks Is/Unwrap in pinned x/sys.
	// Retain the native cause and expose its Win32 errno for Go classification.
	if status, ok := err.(windows.NTStatus); ok {
		err = errors.Join(status, status.Errno())
	}
	if !errors.Is(err, windows.STATUS_SHARING_VIOLATION) {
		return err
	}
	// A winner's delete-denying data handle can make sharing failure precede
	// name collision. Observe existence only after failure; never retry rename.
	// Metadata access avoids data opens, and the single rooted component is
	// opened as a reparse point so even a dangling/outside link counts as existing.
	name, e := windows.NewNTUnicodeString(new)
	if e != nil {
		return err
	}
	oa := windows.OBJECT_ATTRIBUTES{RootDirectory: windows.Handle(to.Fd()), ObjectName: name, Attributes: windows.OBJ_CASE_INSENSITIVE}
	oa.Length = uint32(unsafe.Sizeof(oa))
	var handle windows.Handle
	e = windows.NtCreateFile(&handle, windows.FILE_READ_ATTRIBUTES|windows.SYNCHRONIZE, &oa, &windows.IO_STATUS_BLOCK{}, nil, 0,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, windows.FILE_OPEN,
		windows.FILE_OPEN_REPARSE_POINT|windows.FILE_SYNCHRONOUS_IO_NONALERT, 0, 0)
	runtime.KeepAlive(to)
	if e != nil {
		return err // Missing or unobservable destination: no new existence claim.
	}
	if e = windows.CloseHandle(handle); e != nil {
		return errors.Join(err, e)
	}
	return errors.Join(err, fmt.Errorf("destination already exists: %w", fs.ErrExist))
}
func renameWindows(from *os.File, old string, to *os.File, new string) error {
	name, err := windows.NewNTUnicodeString(old)
	if err != nil {
		return err
	}
	oa := windows.OBJECT_ATTRIBUTES{RootDirectory: windows.Handle(from.Fd()), ObjectName: name, Attributes: windows.OBJ_CASE_INSENSITIVE}
	oa.Length = uint32(unsafe.Sizeof(oa))
	var status windows.IO_STATUS_BLOCK
	var handle windows.Handle
	err = windows.NtCreateFile(&handle, windows.DELETE|windows.SYNCHRONIZE, &oa, &status, nil, 0,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, windows.FILE_OPEN,
		windows.FILE_DIRECTORY_FILE|windows.FILE_OPEN_REPARSE_POINT|windows.FILE_SYNCHRONOUS_IO_NONALERT, 0, 0)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(handle)
	target, err := windows.UTF16FromString(new)
	if err != nil {
		return err
	}
	// Native structure alignment differs between 32/64 bit; use Offsetof.
	type renameInfo struct {
		ReplaceIfExists byte
		RootDirectory   windows.Handle
		FileNameLength  uint32
		FileName        [1]uint16
	}
	var header renameInfo
	size := max(int(unsafe.Sizeof(header)), int(unsafe.Offsetof(header.FileName))+(len(target)-1)*2)
	buffer := make([]byte, size)
	info := (*renameInfo)(unsafe.Pointer(&buffer[0]))
	info.RootDirectory = windows.Handle(to.Fd())
	info.FileNameLength = uint32((len(target) - 1) * 2)
	copy(unsafe.Slice(&info.FileName[0], len(target)-1), target[:len(target)-1])
	return windows.NtSetInformationFile(handle, &status, &buffer[0], uint32(size), windows.FileRenameInformation)
}

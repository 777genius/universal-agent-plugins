//go:build windows

package scaffold

import (
	"errors"
	"fmt"
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
	runtime.KeepAlive(from)
	runtime.KeepAlive(to)
	if err != nil {
		// Native NTSTATUS errors do not implement errors.Is. Convert to the
		// Win32 errno so callers can classify collisions with os.ErrExist.
		var status windows.NTStatus
		if errors.As(err, &status) {
			err = fmt.Errorf("%v: %w", err, status.Errno())
		}
		return &os.LinkError{Op: "rename-exclusive", Old: old, New: new, Err: err}
	}
	return nil
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
		return fmt.Errorf("open source directory for exclusive rename: %w", err)
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
	if err := windows.NtSetInformationFile(handle, &status, &buffer[0], uint32(size), windows.FileRenameInformation); err != nil {
		return fmt.Errorf("commit exclusive directory rename: %w", err)
	}
	return nil
}

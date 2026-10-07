//go:build windows && (amd64 || arm64)

package directoryidentity

import (
	"errors"
	"os"
	"runtime"
	"strconv"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

func platformScheme() string { return "windows-ntfs-volume-fileid-v1" }

// Only the explicit drive-root bootstrap uses Win32 naming. Every subsequent
// single component uses NtCreateFile relative to a live directory handle.
func openDirectory(parent *os.File, name string) (*os.File, error) {
	var f *os.File
	var err error
	if parent == nil {
		p, e := windows.UTF16PtrFromString(name)
		if e != nil {
			return nil, e
		}
		if windows.GetDriveType(p) != windows.DRIVE_FIXED {
			return nil, ErrUnsupported
		}
		h, e := windows.CreateFile(p, windows.FILE_READ_ATTRIBUTES|windows.FILE_LIST_DIRECTORY|windows.SYNCHRONIZE, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
		if e != nil {
			return nil, e
		}
		f = os.NewFile(uintptr(h), "directory-identity-pin")
	} else {
		f, err = relativeDirectory(parent, name)
	}
	if err != nil {
		return nil, err
	}
	if _, _, err := directoryFacts(f); err != nil {
		return nil, errors.Join(err, f.Close())
	}
	return f, nil
}

func relativeDirectory(parent *os.File, name string) (*os.File, error) {
	probe, err := ntDirectory(parent, name, windows.FILE_READ_ATTRIBUTES, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE)
	if err != nil {
		return nil, err
	}
	volume, object, err := directoryFacts(probe)
	if err != nil {
		return nil, errors.Join(err, probe.Close())
	}
	// The current packageview's empty-name HANDLE-relative directory upgrade
	// acquires list access only after proving disk/directory/no-reparse metadata.
	held, err := ntDirectory(probe, "", windows.FILE_READ_ATTRIBUTES|windows.FILE_LIST_DIRECTORY, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE)
	closeErr := probe.Close()
	if err != nil {
		return nil, errors.Join(err, closeErr)
	}
	afterVolume, afterObject, checkErr := directoryFacts(held)
	if err := errors.Join(checkErr, closeErr); err != nil {
		return nil, errors.Join(err, held.Close())
	}
	if volume != afterVolume || object != afterObject {
		return nil, errors.Join(ErrChanged, held.Close())
	}
	return held, nil
}

func ntDirectory(parent *os.File, name string, access, share uint32) (*os.File, error) {
	defer runtime.KeepAlive(parent)
	u, err := windows.NewNTUnicodeString(name)
	if err != nil {
		return nil, err
	}
	attrs := windows.OBJECT_ATTRIBUTES{RootDirectory: windows.Handle(parent.Fd()), ObjectName: u}
	if name != "" {
		attrs.Attributes = windows.OBJ_CASE_INSENSITIVE
	}
	attrs.Length = uint32(unsafe.Sizeof(attrs))
	const options = windows.FILE_OPEN_REPARSE_POINT | windows.FILE_OPEN_NO_RECALL | windows.FILE_SYNCHRONOUS_IO_NONALERT | windows.FILE_OPEN_FOR_BACKUP_INTENT
	var h windows.Handle
	err = windows.NtCreateFile(&h, access|windows.SYNCHRONIZE, &attrs, &windows.IO_STATUS_BLOCK{}, nil, 0, share, windows.FILE_OPEN, options, 0, 0)
	runtime.KeepAlive(u)
	if err != nil {
		var status windows.NTStatus
		if errors.As(err, &status) {
			return nil, errors.Join(err, status.Errno())
		}
		return nil, err
	}
	return os.NewFile(uintptr(h), "directory-identity-pin"), nil
}

func directoryFacts(f *os.File) (string, string, error) {
	defer runtime.KeepAlive(f)
	h := windows.Handle(f.Fd())
	kind, err := windows.GetFileType(h)
	if err != nil {
		return "", "", err
	}
	if kind != windows.FILE_TYPE_DISK {
		return "", "", ErrUnsupported
	}
	var stat windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(h, &stat); err != nil {
		return "", "", err
	}
	const unsafeAttrs = windows.FILE_ATTRIBUTE_REPARSE_POINT | windows.FILE_ATTRIBUTE_OFFLINE | windows.FILE_ATTRIBUTE_RECALL_ON_OPEN | windows.FILE_ATTRIBUTE_RECALL_ON_DATA_ACCESS | windows.FILE_ATTRIBUTE_DEVICE | windows.FILE_ATTRIBUTE_ENCRYPTED
	if stat.FileAttributes&unsafeAttrs != 0 {
		return "", "", errors.New("unsafe directory attributes")
	}
	if stat.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY == 0 {
		return "", "", windows.ERROR_DIRECTORY
	}
	var fs [32]uint16
	if err := windows.GetVolumeInformationByHandle(h, nil, 0, nil, nil, nil, &fs[0], uint32(len(fs))); err != nil {
		return "", "", err
	}
	if windows.UTF16ToString(fs[:]) != "NTFS" {
		return "", "", ErrUnsupported
	}
	if stat.VolumeSerialNumber == 0 || (stat.FileIndexHigh == 0 && stat.FileIndexLow == 0) {
		return "", "", errors.New("zero directory identity")
	}
	return strconv.FormatUint(uint64(stat.VolumeSerialNumber), 10), strconv.FormatUint(uint64(stat.FileIndexHigh), 10) + ":" + strconv.FormatUint(uint64(stat.FileIndexLow), 10), nil
}

// A held-handle normalized name is an alias guard, not the physical identity.
// Refuse unresolved case/Unicode/short-name aliases instead of inventing keys.
func verifyCanonicalName(f *os.File, expected string) error {
	defer runtime.KeepAlive(f)
	var b [4101]uint16
	n, err := windows.GetFinalPathNameByHandle(windows.Handle(f.Fd()), &b[0], uint32(len(b)), 0)
	if err != nil {
		return err
	}
	if n == 0 || n >= uint32(len(b)) {
		return errors.New("directory canonical name limit")
	}
	actual := strings.TrimPrefix(windows.UTF16ToString(b[:n]), `\\?\`)
	if actual != expected {
		return errors.New("unresolved directory path alias")
	}
	return nil
}

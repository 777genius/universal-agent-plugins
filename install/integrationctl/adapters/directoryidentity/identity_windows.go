//go:build windows

package directoryidentity

import (
	"fmt"
	"os"
	"syscall"
)

func LegacyIdentity(path string, info os.FileInfo) (string, error) {
	name, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return "", err
	}
	handle, err := syscall.CreateFile(name, 0, syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE, nil, syscall.OPEN_EXISTING, syscall.FILE_FLAG_BACKUP_SEMANTICS|syscall.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return "", err
	}
	defer func() { _ = syscall.CloseHandle(handle) }()
	var stat syscall.ByHandleFileInformation
	if err := syscall.GetFileInformationByHandle(handle, &stat); err != nil {
		return "", err
	}
	return fmt.Sprintf("%d:%d:%d", stat.VolumeSerialNumber, stat.FileIndexHigh, stat.FileIndexLow), nil
}

// LegacyHandleIdentity reads the existing volume:file-index encoding from the
// held handle itself; reopening its name would lose rooted authority.
func LegacyHandleIdentity(file *os.File) (string, error) {
	info, err := file.Stat()
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("held ordinary directory unavailable")
	}
	var stat syscall.ByHandleFileInformation
	if err := syscall.GetFileInformationByHandle(syscall.Handle(file.Fd()), &stat); err != nil {
		return "", err
	}
	if stat.FileAttributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0 || stat.FileAttributes&syscall.FILE_ATTRIBUTE_DIRECTORY == 0 {
		return "", fmt.Errorf("held directory is a reparse point or non-directory")
	}
	return fmt.Sprintf("%d:%d:%d", stat.VolumeSerialNumber, stat.FileIndexHigh, stat.FileIndexLow), nil
}

func canonicalize(p string) (string, error) {
	if err := validatePath(p, platformScheme()); err != nil {
		return "", err
	}
	return p, nil
}

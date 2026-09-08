//go:build windows

package dirswap

import (
	"fmt"
	"os"
	"syscall"
)

func directoryIdentity(path string, info os.FileInfo) (string, error) {
	name, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return "", err
	}
	handle, err := syscall.CreateFile(name, 0, syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE, nil, syscall.OPEN_EXISTING, syscall.FILE_FLAG_BACKUP_SEMANTICS|syscall.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return "", err
	}
	defer syscall.CloseHandle(handle)
	var stat syscall.ByHandleFileInformation
	if err := syscall.GetFileInformationByHandle(handle, &stat); err != nil {
		return "", err
	}
	return fmt.Sprintf("%d:%d:%d", stat.VolumeSerialNumber, stat.FileIndexHigh, stat.FileIndexLow), nil
}

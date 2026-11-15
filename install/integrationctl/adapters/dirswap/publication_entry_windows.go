//go:build windows

package dirswap

import (
	"fmt"
	"os"
	"runtime"
	"syscall"

	"golang.org/x/sys/windows"
)

func publicationEntryFacts(file *os.File, info os.FileInfo) (string, string, error) {
	defer runtime.KeepAlive(file)
	handle := windows.Handle(file.Fd())
	kind, err := windows.GetFileType(handle)
	if err != nil {
		return "", "", err
	}
	if kind != windows.FILE_TYPE_DISK {
		return "", "", fmt.Errorf("publication object is not a disk object")
	}
	var stat windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(handle, &stat); err != nil {
		return "", "", err
	}
	const unsafeAttributes = windows.FILE_ATTRIBUTE_REPARSE_POINT | windows.FILE_ATTRIBUTE_OFFLINE |
		windows.FILE_ATTRIBUTE_RECALL_ON_OPEN | windows.FILE_ATTRIBUTE_RECALL_ON_DATA_ACCESS |
		windows.FILE_ATTRIBUTE_DEVICE | windows.FILE_ATTRIBUTE_ENCRYPTED
	if stat.FileAttributes&unsafeAttributes != 0 || (stat.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY != 0) != info.IsDir() {
		return "", "", fmt.Errorf("publication object has unsafe attributes")
	}
	if stat.VolumeSerialNumber == 0 || stat.FileIndexHigh == 0 && stat.FileIndexLow == 0 || stat.NumberOfLinks == 0 {
		return "", "", fmt.Errorf("publication object identity unavailable")
	}
	if info.Mode().IsRegular() && stat.NumberOfLinks != 1 {
		return "", "", fmt.Errorf("publication regular object has multiple links")
	}
	owner, err := publicationOwnerSID(handle)
	if err != nil {
		return "", "", err
	}
	return fmt.Sprintf("%d:%d:%d", stat.VolumeSerialNumber, stat.FileIndexHigh, stat.FileIndexLow),
		fmt.Sprintf("%d:%d:%s", stat.FileAttributes, stat.NumberOfLinks, owner), nil
}

func publicationOwnerSID(handle windows.Handle) (string, error) {
	descriptor, err := windows.GetSecurityInfo(handle, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION)
	if err != nil {
		return "", err
	}
	if descriptor == nil || !descriptor.IsValid() {
		return "", fmt.Errorf("publication owner descriptor unavailable")
	}
	defer runtime.KeepAlive(descriptor)
	owner, _, err := descriptor.Owner()
	if err != nil {
		return "", err
	}
	if owner == nil || !owner.IsValid() {
		return "", fmt.Errorf("publication owner identity unavailable")
	}
	value := owner.String()
	if value == "" || len(value) > 256 {
		return "", fmt.Errorf("publication owner identity serialization refused")
	}
	return value, nil
}

func samePublicationMetadata(before, after os.FileInfo) bool {
	left, leftOK := before.Sys().(*syscall.Win32FileAttributeData)
	right, rightOK := after.Sys().(*syscall.Win32FileAttributeData)
	return leftOK && rightOK && left != nil && right != nil &&
		left.FileAttributes == right.FileAttributes
}

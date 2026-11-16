//go:build windows

package pi

import (
	"os"
	"runtime"

	"golang.org/x/sys/windows"
)

func piTrustSingleLink(file *os.File, _ os.FileInfo) (bool, error) {
	defer runtime.KeepAlive(file)
	var data windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(windows.Handle(file.Fd()), &data); err != nil {
		return false, err
	}
	return data.NumberOfLinks == 1, nil
}

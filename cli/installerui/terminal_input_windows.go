//go:build windows

package installerui

import (
	"fmt"
	"os"
	"runtime"

	"golang.org/x/sys/windows"
)

func validatePromptInput(f *os.File) error {
	defer runtime.KeepAlive(f)
	var count uint32
	if err := windows.GetNumberOfConsoleInputEvents(windows.Handle(f.Fd()), &count); err != nil {
		return fmt.Errorf("%w: validate console input: %w", ErrUnavailable, err)
	}
	return nil
}

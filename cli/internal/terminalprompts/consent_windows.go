//go:build windows

package terminalprompts

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

var readConsentConsoleInput = windows.NewLazySystemDLL("kernel32.dll").NewProc("ReadConsoleInputW")

// INPUT_RECORD's KEY_EVENT_RECORD union, including its DWORD alignment.
type consentConsoleRecord struct {
	eventType, padding             uint16
	keyDown                        int32
	repeat, virtualKey, scan, char uint16
	control                        uint32
}

// Snapshot console events without enabling raw mode or starting ReadConsole's
// cooked line editor. A byte count is not meaningful for Windows INPUT_RECORDs.
// Only this snapshot is consumed; the first submitted answer ends ownership.
func terminalQueuedInput(ctx context.Context, f *os.File) ([]byte, bool, error) {
	defer runtime.KeepAlive(f)
	h := windows.Handle(f.Fd())
	var count uint32
	if err := windows.GetNumberOfConsoleInputEvents(h, &count); err != nil {
		return nil, false, err
	}
	var queued []byte
	paste := false
	for i := uint32(0); i < count; i++ {
		if err := ctx.Err(); err != nil {
			return nil, false, err
		}
		var record consentConsoleRecord
		var read uint32
		ok, _, err := readConsentConsoleInput.Call(uintptr(h), uintptr(unsafe.Pointer(&record)), 1, uintptr(unsafe.Pointer(&read)))
		if ok == 0 {
			return nil, false, fmt.Errorf("read queued console event: %w", err)
		}
		if read != 1 {
			return nil, false, io.ErrNoProgress
		}
		if record.eventType != 1 || record.keyDown == 0 {
			continue
		}
		c := record.char
		if c == 0 && record.virtualKey == 0x0d {
			c = '\r'
		}
		if c == 0 {
			continue
		}
		// Repeats in one record all precede the boundary. They cannot grant consent.
		// Console Enter is a cooked line delimiter, not a raw Alt+Enter key.
		if c == '\r' {
			c = '\n'
		}
		queued = append(queued, []byte(string(rune(c)))...)
		if bytes.HasSuffix(queued, []byte("\x1b[200~")) {
			paste = true
		}
		if bytes.HasSuffix(queued, []byte("\x1b[201~")) {
			paste = false
		}
		if !paste && (c == '\r' || c == '\n') {
			return queued, true, nil
		}
	}
	return queued, false, nil
}

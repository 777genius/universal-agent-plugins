//go:build windows

package installerui

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
	if count > 4096 {
		return nil, false, fmt.Errorf("consent input budget exceeded")
	}

	queue := consentConsoleQueue{}
	for i := uint32(0); i < count; i++ {
		if err := ctx.Err(); err != nil {
			return nil, false, err
		}
		record, err := readConsentRecord(h)
		if err != nil {
			return nil, false, err
		}
		submitted, err := queue.append(record)
		if err != nil {
			return nil, false, err
		}
		if submitted {
			return queue.bytes, true, nil
		}
	}
	return queue.bytes, false, nil
}

func readConsentRecord(h windows.Handle) (consentConsoleRecord, error) {
	var record consentConsoleRecord
	var read uint32
	ok, _, err := readConsentConsoleInput.Call(uintptr(h), uintptr(unsafe.Pointer(&record)), 1, uintptr(unsafe.Pointer(&read)))
	if ok == 0 {
		return record, fmt.Errorf("read queued console event: %w", err)
	}
	if read != 1 {
		return record, io.ErrNoProgress
	}
	return record, nil
}

type consentConsoleQueue struct {
	bytes []byte
	paste bool
}

func (q *consentConsoleQueue) append(record consentConsoleRecord) (bool, error) {
	if record.eventType != 1 || record.keyDown == 0 {
		return false, nil
	}
	c := record.char
	if c == 0 && record.virtualKey == 0x0d {
		c = '\r'
	}
	if c == 0 {
		return false, nil
	}
	// Repeats cannot grant consent. Console Enter is a cooked line delimiter.
	if c == '\r' {
		c = '\n'
	}
	encoded := []byte(string(rune(c)))
	if len(q.bytes)+len(encoded) > 4096 {
		return false, fmt.Errorf("consent input budget exceeded")
	}
	q.bytes = append(q.bytes, encoded...)
	if bytes.HasSuffix(q.bytes, []byte("\x1b[200~")) {
		q.paste = true
	}
	if bytes.HasSuffix(q.bytes, []byte("\x1b[201~")) {
		q.paste = false
	}
	return !q.paste && c == '\n', nil
}

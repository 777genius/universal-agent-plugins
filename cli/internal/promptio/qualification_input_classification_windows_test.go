//go:build windows

package promptio

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Runs only inside the existing synthetic attached ConPTY qualification.
func qualificationConsoleInputClassification(t *testing.T, input windows.Handle, trace *qualificationHandleTrace) {
	t.Helper()
	output, err := windows.CreateFile(windows.StringToUTF16Ptr("CONOUT$"),
		windows.GENERIC_READ|windows.GENERIC_WRITE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE,
		nil, windows.OPEN_EXISTING, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	f := os.NewFile(uintptr(output), "qualification-console-output")
	defer func() {
		if err := f.Close(); err != nil {
			t.Errorf("close output: %v", err)
		}
	}()
	for _, h := range []windows.Handle{input, output} {
		var original uint32
		if err := windows.GetConsoleMode(h, &original); err != nil {
			t.Fatal(err)
		}
		defer func(h windows.Handle, original uint32) {
			if err := windows.SetConsoleMode(h, original); err != nil {
				t.Errorf("restore mode: %v", err)
			}
			var restored uint32
			if err := windows.GetConsoleMode(h, &restored); err != nil || restored != original {
				t.Errorf("restored mode=%#x want=%#x: %v", restored, original, err)
			}
		}(h, original)
		// 0x7 is valid for both input and output, with different meanings.
		if err := windows.SetConsoleMode(h, 0x7); err != nil {
			t.Fatal(err)
		}
		var mode uint32
		if err := windows.GetConsoleMode(h, &mode); err != nil || mode != 0x7 {
			t.Fatalf("matching mode=%#x: %v", mode, err)
		}
	}
	const want = "classification-queue-preserved"
	var records []qualificationKeyEvent
	for _, char := range want + "\r" {
		event := qualificationKeyEvent{eventType: 1, keyDown: 1, repeat: 1, char: uint16(char)}
		if char == '\r' {
			event.virtualKey = 0x0d
		}
		records = append(records, event)
		event.keyDown = 0
		records = append(records, event)
	}
	var written uint32
	ok, _, writeErr := qualificationWriteConsoleInput.Call(uintptr(input),
		uintptr(unsafe.Pointer(&records[0])), uintptr(len(records)), uintptr(unsafe.Pointer(&written)))
	if ok == 0 || written != uint32(len(records)) {
		t.Fatalf("queue classification input: %d/%d: %v", written, len(records), writeErr)
	}
	var before uint32
	if err := windows.GetNumberOfConsoleInputEvents(input, &before); err != nil || before < written {
		t.Fatalf("queued events=%d want at least %d: %v", before, written, err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	line, err := ReadLine(trace.context(ctx), f)
	cancel()
	if line != "" || !errors.Is(err, windows.ERROR_INVALID_HANDLE) {
		t.Fatalf("output handle accepted: line=%q error=%v", line, err)
	}
	var after uint32
	if err := windows.GetNumberOfConsoleInputEvents(input, &after); err != nil || after != before {
		t.Fatalf("output read changed queue: before=%d after=%d: %v", before, after, err)
	}
	// Read through the inherited input to prove the queued answer is intact.
	bound := time.AfterFunc(2*time.Second, func() { panic("classification queue read exceeded two seconds") })
	defer bound.Stop()
	line, err = readConsoleLine(context.Background(), input, 0x7)
	if err != nil || line != want {
		t.Fatalf("preserved queue: line=%q want=%q error=%v", line, want, err)
	}
	t.Log("matching-mode output rejected; console input queue preserved")
}

//go:build windows

package installerui

import (
	"bytes"
	"context"
	"os"
	"testing"
	"time"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/777genius/plugin-kit-ai/cli/internal/promptio"
)

// Opt in only inside a disposable console/ConPTY; never queue into a user's
// interactive console when running the ordinary unit suite.
func TestWindowsConsoleConsentBoundary(t *testing.T) {
	if os.Getenv("AGENTPLUGINS_WINDOWS_CONSENT_PROOF") != "1" {
		t.Skip("requires disposable Windows console")
	}
	h := windows.Handle(os.Stdin.Fd())
	var before uint32
	if err := windows.GetConsoleMode(h, &before); err != nil {
		t.Fatal(err)
	}
	write := windows.NewLazySystemDLL("kernel32.dll").NewProc("WriteConsoleInputW")
	if unsafe.Sizeof(consentConsoleRecord{}) != 20 {
		t.Fatal("incorrect INPUT_RECORD layout")
	}
	for _, answer := range []string{"\r", "y\r", "\x1b\r"} {
		var records []consentConsoleRecord
		for _, c := range utf16.Encode([]rune(answer + "next-owner\r")) {
			record := consentConsoleRecord{eventType: 1, keyDown: 1, repeat: 1, char: c}
			if c == '\r' {
				record.virtualKey = 0x0d
			}
			records = append(records, record)
			record.keyDown = 0
			records = append(records, record)
		}
		var n uint32
		ok, _, err := write.Call(uintptr(h), uintptr(unsafe.Pointer(&records[0])), uintptr(len(records)), uintptr(unsafe.Pointer(&n)))
		if ok == 0 || n != uint32(len(records)) {
			t.Fatalf("queue=%d %v", n, err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		var out bytes.Buffer
		p := testPlain{Input: os.Stdin, Output: &out}
		result, err := p.Confirm(ctx, ConfirmRequest{Title: "Apply?", Default: true})
		if result.Accepted {
			t.Error("queued input accepted")
		}
		if answer == "\x1b\r" {
			if err != nil || !result.Cancelled || result.Accepted { //nolint:misspell // Preserve the existing public cancellation API.
				t.Errorf("cancel result=%+v err=%v", result, err)
			}
		} else if err != nil {
			t.Error(err)
		}
		line, err := promptio.ReadLine(ctx, os.Stdin)
		cancel()
		if line != "next-owner" || err != nil {
			t.Fatalf("suffix=%q %v", line, err)
		}
		// ReadConsole may leave the key-up for the fixture's final Enter.
		// Consume only that owned release, never flush a future owner's input.
		var remaining uint32
		if err := windows.GetNumberOfConsoleInputEvents(h, &remaining); err != nil {
			t.Fatal(err)
		}
		if remaining == 1 {
			record, err := readConsentRecord(h)
			if err != nil || record.eventType != 1 || record.keyDown != 0 || record.virtualKey != 0x0d || record.char != '\r' {
				t.Fatalf("unexpected fixture suffix event: %+v %v", record, err)
			}
		} else if remaining != 0 {
			t.Fatalf("unexpected fixture suffix count: %d", remaining)
		}
		var after uint32
		if err := windows.GetConsoleMode(h, &after); err != nil || before != after {
			t.Fatalf("mode=%x want %x: %v", after, before, err)
		}
	}
}

// Native only: the two budgets have different units. Queue into the disposable
// opt-in console, retain untouched suffix events, and drain only fixture events.
func TestWindowsConsentSnapshotOverflow(t *testing.T) {
	if os.Getenv("AGENTPLUGINS_WINDOWS_CONSENT_PROOF") != "1" {
		t.Skip("requires disposable Windows console")
	}
	h := windows.Handle(os.Stdin.Fd())
	write := windows.NewLazySystemDLL("kernel32.dll").NewProc("WriteConsoleInputW")
	for _, tc := range []struct {
		name  string
		count int
		char  uint16
	}{{"events", 4097, 'a'}, {"utf8-bytes", 1400, '世'}} {
		t.Run(tc.name, func(t *testing.T) {
			var initial uint32
			if err := windows.GetNumberOfConsoleInputEvents(h, &initial); err != nil || initial != 0 {
				t.Fatalf("fixture queue not empty: %d %v", initial, err)
			}
			var before uint32
			if err := windows.GetConsoleMode(h, &before); err != nil {
				t.Fatal(err)
			}
			records := make([]consentConsoleRecord, tc.count+4)
			for i := 0; i < tc.count; i++ {
				records[i] = consentConsoleRecord{eventType: 1, keyDown: 1, repeat: 1, char: tc.char}
			}
			for i, c := range []uint16{'y', 'e', 's', '\r'} {
				records[tc.count+i] = consentConsoleRecord{eventType: 1, keyDown: 1, repeat: 1, char: c}
			}
			var n uint32
			ok, _, err := write.Call(uintptr(h), uintptr(unsafe.Pointer(&records[0])), uintptr(len(records)), uintptr(unsafe.Pointer(&n)))
			if ok == 0 || n != uint32(len(records)) {
				t.Fatalf("queue=%d %v", n, err)
			}
			var out bytes.Buffer
			got, err := (testPlain{Input: os.Stdin, Output: &out}).Confirm(context.Background(), ConfirmRequest{Title: "Apply?", Default: true})
			var remaining, after uint32
			_ = windows.GetNumberOfConsoleInputEvents(h, &remaining)
			_ = windows.GetConsoleMode(h, &after)
			if got.Accepted || err == nil || out.Len() != 0 || before != after || remaining < 4 {
				t.Fatalf("overflow result=%+v err=%v rendered=%d suffix=%d mode=%x/%x", got, err, out.Len(), remaining, before, after)
			}
			for remaining > 0 {
				var record consentConsoleRecord
				var read uint32
				ok, _, err := readConsentConsoleInput.Call(uintptr(h), uintptr(unsafe.Pointer(&record)), 1, uintptr(unsafe.Pointer(&read)))
				if ok == 0 || read != 1 {
					t.Fatalf("fixture drain %v", err)
				}
				remaining--
			}
		})
	}
}

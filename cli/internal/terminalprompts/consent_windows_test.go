//go:build windows

package terminalprompts

import (
	"bytes"
	"context"
	"errors"
	"os"
	"testing"
	"time"
	"unicode/utf16"
	"unsafe"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/cli/internal/promptio"
	"golang.org/x/sys/windows"
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
		p := PlainPrompter{Input: os.Stdin, Output: &out}
		result, err := p.Confirm(ctx, prompt.ConfirmationRequest{Title: "Apply?", Default: true})
		if result.Accepted {
			t.Error("queued input accepted")
		}
		if answer == "\x1b\r" {
			if !errors.Is(err, prompt.ErrPromptCanceled) {
				t.Errorf("cancel=%v", err)
			}
		} else if err != nil {
			t.Error(err)
		}
		line, err := promptio.ReadLine(ctx, os.Stdin)
		cancel()
		if line != "next-owner" || err != nil {
			t.Fatalf("suffix=%q %v", line, err)
		}
		var after uint32
		if err := windows.GetConsoleMode(h, &after); err != nil || before != after {
			t.Fatalf("mode=%x want %x: %v", after, before, err)
		}
	}
}

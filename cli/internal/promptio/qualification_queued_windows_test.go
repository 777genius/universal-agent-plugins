//go:build windows

package promptio

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"
	"unicode/utf16"
	"unsafe"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"golang.org/x/sys/windows"
)

var qualificationWriteConsoleInput = windows.NewLazySystemDLL("kernel32.dll").NewProc("WriteConsoleInputW")

// INPUT_RECORD containing KEY_EVENT_RECORD, including the union's DWORD padding.
type qualificationKeyEvent struct {
	eventType, padding             uint16
	keyDown                        int32
	repeat, virtualKey, scan, char uint16
	control                        uint32
}

// Called in the existing ConPTY qualification process after its original 30
// cancellation exchanges. Queue each triplet in one native write: no driver
// marker round trips can hide loss of an already submitted next-owner line.
func qualificationConsoleQueuedAnswers(t *testing.T, inherited windows.Handle, mode uint32, trace *qualificationHandleTrace) {
	t.Helper()
	if unsafe.Sizeof(qualificationKeyEvent{}) != 20 {
		t.Fatal("invalid Windows INPUT_RECORD layout")
	}
	for i := 0; i < 10; i++ {
		first := fmt.Sprintf("中文é😀-%d", i)
		next := fmt.Sprintf("queued-next-%d", i)
		var records []qualificationKeyEvent
		for _, unit := range utf16.Encode([]rune(first + "\r\x1a\r" + next + "\r")) {
			event := qualificationKeyEvent{eventType: 1, keyDown: 1, repeat: 1, char: unit}
			if unit == '\r' {
				event.virtualKey = 0x0d // VK_RETURN, one Enter per submitted answer.
			}
			records = append(records, event)
			event.keyDown = 0
			records = append(records, event)
		}
		var written uint32
		ok, _, err := qualificationWriteConsoleInput.Call(uintptr(inherited),
			uintptr(unsafe.Pointer(&records[0])), uintptr(len(records)), uintptr(unsafe.Pointer(&written)))
		if ok == 0 || written != uint32(len(records)) {
			t.Fatalf("queue triplet %d: records=%d/%d, result=%d, error=%v", i, written, len(records), ok, err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if line, err := ReadLine(trace.context(ctx), os.Stdin); line != "" || !errors.Is(err, context.Canceled) {
			t.Fatalf("queued pre-canceled read %d: %q %v", i, line, err)
		}
		for j, want := range []string{first, "", next} {
			ctx, stop := context.WithTimeout(context.Background(), 2*time.Second)
			// The final read uses the inherited native handle directly. Bound
			// that synchronous call too; a failed probe must terminate visibly.
			bound := time.AfterFunc(2*time.Second, func() {
				panic(fmt.Sprintf("queued triplet %d answer %d exceeded two seconds", i, j))
			})
			var line string
			var err error
			if j == 2 {
				// The next owner reads through the actual inherited handle, so a
				// private pending buffer in a closed prompt handle cannot help.
				line, err = readConsoleLine(ctx, inherited, mode)
			} else {
				line, err = ReadLine(trace.context(ctx), os.Stdin)
			}
			bound.Stop()
			stop()
			var wantErr error
			if j == 1 {
				wantErr = prompt.ErrPromptInputClosed
			}
			if line != want || !errors.Is(err, wantErr) {
				t.Fatalf("queued triplet %d answer %d: got %q %v; want %q %v", i, j, line, err, want, wantErr)
			}
		}
		var after uint32
		if err := windows.GetConsoleMode(inherited, &after); err != nil || after != mode {
			t.Fatalf("queued triplet %d changed inherited input: mode=%#x error=%v", i, after, err)
		}
	}
	t.Log("console queued Unicode, EOF, and inherited next-owner read: 10 triplets passed")
}

package terminalactivity

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestAnimationStopsBeforePromptAndClipsWidth(t *testing.T) {
	var out bytes.Buffer
	stop := Start(&out, "Resolving Skills source", true, func() int { return 25 })
	time.Sleep(220 * time.Millisecond)
	stop()
	stop()
	frames := strings.Split(out.String(), "\r\x1b[2K")
	if len(frames) < 4 || frames[1] == frames[2] {
		t.Fatalf("expected changing frames: %q", out.String())
	}
	for _, frame := range frames {
		if len([]rune(frame)) >= 25 || strings.Contains(frame, "\n") {
			t.Fatalf("frame wraps or scrolls: %q", frame)
		}
	}
	out.WriteString("Choose agents: ")
	before := out.String()
	time.Sleep(120 * time.Millisecond)
	if out.String() != before {
		t.Fatal("renderer overwrote the next prompt")
	}
}

func TestPlainStatus(t *testing.T) {
	var out bytes.Buffer
	stop := Start(&out, "Scanning Skills catalog", false, nil)
	stop()
	if out.String() != "Scanning Skills catalog…\n" {
		t.Fatalf("unexpected plain output: %q", out.String())
	}
}

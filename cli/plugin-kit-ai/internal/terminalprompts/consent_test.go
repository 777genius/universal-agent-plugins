package terminalprompts

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

// The count is captured before the new question. Extra input is already
// readable by the time the snapshot is consumed; it must remain untouched.
func TestConsentSnapshotDoesNotDrainLaterInput(t *testing.T) {
	for _, tc := range []struct {
		input        string
		n            int
		queued, rest string
		submitted    bool
	}{
		{" \r", 1, " ", "\r", false},
		{"\x1b[D \r", 3, "\x1b[D", " \r", false},
		{" \rnext\n", 8, " \r", "next\n", true},
		{"\x1b[200~ \r\x1b[201~ \rnext\n", 21, "\x1b[200~ \r\x1b[201~ \r", "next\n", true},
		{"\x1b\r \r", 4, "\x1b\r", " \r", true},
		{" \r", 0, "", " \r", false},
	} {
		t.Run(tc.queued, func(t *testing.T) {
			input := strings.NewReader(tc.input)
			queued, submitted, err := readQueuedInput(context.Background(), input, tc.n)
			if err != nil || string(queued) != tc.queued || submitted != tc.submitted {
				t.Fatalf("snapshot=%q submitted=%v %v", queued, submitted, err)
			}
			rest, err := io.ReadAll(input)
			if err != nil || string(rest) != tc.rest {
				t.Fatalf("later input=%q %v", rest, err)
			}
		})
	}
}

func TestConsentSnapshotCancellationAndEOF(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	input := strings.NewReader(" \r")
	if _, _, err := readQueuedInput(ctx, input, 2); !errors.Is(err, context.Canceled) || input.Len() != 2 {
		t.Fatalf("canceled snapshot consumed input: %v", err)
	}
	if _, _, err := readQueuedInput(context.Background(), strings.NewReader(" "), 2); !errors.Is(err, io.EOF) {
		t.Fatalf("snapshot EOF=%v", err)
	}
}

func TestQueuedConfirmationKeysCannotChooseYes(t *testing.T) {
	for _, tc := range []struct{ input, want string }{
		{" \r", "\r"}, {"y\r", "\r"}, {"\x1b[D\r", "\r"},
		{"\x1b[32u\x1b[13u", "\r"}, {"\x1b[1;1D\r", "\r"},
		{" ", ""}, {"\x1b[200~ \ry\x03\x1b[201~", ""},
		{"\x1b[200~ \r", "\x03"}, {"\x1b[32;", "\x03"}, {"\x1b[", "\x03"},
		{"\x1b", "\x1b"}, {"\x03", "\x03"}, {"\x04", "\x04"},
	} {
		if got := string(queuedConfirmationKeys([]byte(tc.input))); got != tc.want {
			t.Errorf("keys(%q)=%q; want %q", tc.input, got, tc.want)
		}
	}
}

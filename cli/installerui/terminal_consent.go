package installerui

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"

	uv "github.com/charmbracelet/ultraviolet"
	"golang.org/x/term"
)

// confirmationInput establishes a new consent boundary BEFORE displaying the
// question. Snapshot only the terminal bytes already queued, never flush the
// terminal or drain until empty: input arriving after the snapshot belongs to
// the new question. Scripted non-terminal readers keep their answer contract.
func confirmationInput(ctx context.Context, input io.Reader) (keys []byte, err error) {
	f, ok := input.(*os.File)
	if !ok || !term.IsTerminal(int(f.Fd())) {
		return nil, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	queued, submitted, err := terminalQueuedInput(ctx, f)
	if err != nil {
		return nil, fmt.Errorf("read consent boundary: %w", err)
	}
	keys = queuedConfirmationKeys(queued)
	// A raw submission gate ends this owner's stale answer even if the event
	// decoder ignores it (for example ESC CR is Alt+Enter). Resolve default No
	// before the live form can read the untouched suffix for the next owner.
	if submitted && len(keys) == 0 {
		keys = []byte{'\r'}
	}
	return keys, nil
}

func readQueuedInput(ctx context.Context, input io.Reader, n int) ([]byte, bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	if n < 0 || n > 4096 {
		return nil, false, fmt.Errorf("consent input budget exceeded")
	}
	// Reuse the handoff parser: even within the snapshot, stop at the first
	// non-pasted submission and leave the next owner's answer in the terminal.
	r := newSubmissionReader(ctx, input)
	defer r.finish()
	queued := make([]byte, 0, n)
	var b [1]byte
	for len(queued) < n {
		count, err := r.Read(b[:])
		if e := ctx.Err(); e != nil {
			return nil, false, e
		}
		if err != nil {
			return nil, false, err
		}
		if count == 0 {
			return nil, false, io.ErrNoProgress
		}
		queued = append(queued, b[0])
		// Canonical input has already mapped CR to LF. Both delimit a stale
		// answer, but rich live forms still use CR for submission.
		// Read is synchronous; there is no second reader accessing this gate.
		if r.gate != nil || b[0] == '\n' && !r.paste {
			return queued, true, nil
		}
	}
	return queued, false, nil
}

// Old keys may decline or cancel, but cannot choose Yes. Decode with the same
// event decoder as Bubble Tea, including enhanced keyboard encodings. Paste is
// ignored as a whole; an unfinished paste fails closed rather than interpreting
// its later suffix as fresh consent. The live form still parses all fresh input.
func queuedConfirmationKeys(queued []byte) []byte {
	var decoder uv.EventDecoder
	for len(queued) > 0 {
		// The decoder treats ESC+[ alone as Alt+[. At this boundary it may be
		// the beginning of a paste, whose later bytes must not become consent.
		if bytes.Equal(queued, []byte("\x1b[")) {
			return []byte{3}
		}
		// A canonical Escape followed by its line terminator is cancellation,
		// not the raw Alt+Enter encoding used by the rich form.
		if bytes.HasPrefix(queued, []byte{'\x1b', '\n'}) {
			return []byte{27}
		}
		n, event := decoder.Decode(queued)
		if n <= 0 {
			return []byte{3}
		}
		queued = queued[n:]
		switch event := event.(type) {
		case uv.PasteStartEvent:
			end := bytes.Index(queued, []byte("\x1b[201~"))
			if end < 0 {
				return []byte{3}
			}
			queued = queued[end+len("\x1b[201~"):]
		case uv.UnknownEvent, uv.UnknownCsiEvent, uv.UnknownSs3Event,
			uv.UnknownOscEvent, uv.UnknownDcsEvent, uv.UnknownSosEvent,
			uv.UnknownPmEvent, uv.UnknownApcEvent:
			return []byte{3}
		case uv.KeyPressEvent:
			switch event.String() {
			case "enter", "ctrl+j":
				return []byte{'\r'}
			case "esc":
				return []byte{27}
			case "ctrl+c":
				return []byte{3}
			case "ctrl+d":
				return []byte{4}
			}
		}
	}
	return nil
}

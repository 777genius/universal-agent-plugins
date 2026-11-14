package installerui

import (
	"bytes"
	"context"
	"io"
	"strings"
	"sync"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/muesli/cancelreader"
)

// submissionReader stops at an Enter boundary until the form either rejects
// that submission or exits. Ultraviolet otherwise reads 4096-byte batches and
// drops answers queued for the next form/plain prompt. It owns no read goroutine
// and no persistent buffer. The bounded tail recognizes enhanced Enter and bracketed paste,
// whose newlines belong to one paste event, not to a submitted answer.
type submissionReader struct {
	reader io.Reader
	ctx    context.Context
	mu     sync.Mutex
	gate   chan struct{}
	done   chan struct{}
	once   sync.Once
	tail   string
	paste  bool
}

func newSubmissionReader(ctx context.Context, r io.Reader) *submissionReader {
	return &submissionReader{reader: r, ctx: ctx, done: make(chan struct{})}
}
func (r *submissionReader) Read(p []byte) (int, error) {
	if err := r.wait(); err != nil {
		return 0, err
	}
	if len(p) == 0 {
		return 0, nil
	}
	n, err := r.reader.Read(p[:1])
	if n == 1 {
		r.consume(p[0])
	}
	return n, err
}
func (r *submissionReader) reject() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.gate != nil {
		close(r.gate)
		r.gate = nil
	}
}
func (r *submissionReader) finish() { r.once.Do(func() { close(r.done) }) }

func submissionEnter(tail string) bool {
	if strings.HasSuffix(tail, "\r") {
		return true
	}
	start := bytes.LastIndexByte([]byte(tail), 27)
	if start < 0 {
		return false
	}
	var decoder uv.EventDecoder
	n, event := decoder.Decode([]byte(tail[start:]))
	key, ok := event.(uv.KeyPressEvent)
	return ok && n == len(tail)-start && key.String() == "enter"
}

func (r *submissionReader) wait() error {
	r.mu.Lock()
	gate := r.gate
	r.mu.Unlock()
	if gate != nil {
		select {
		case <-gate:
		case <-r.done:
			return cancelreader.ErrCanceled
		case <-r.ctx.Done():
			return cancelreader.ErrCanceled
		}
	}
	select {
	case <-r.done:
		return cancelreader.ErrCanceled
	case <-r.ctx.Done():
		return cancelreader.ErrCanceled
	default:
	}

	return nil
}
func (r *submissionReader) consume(c byte) {
	r.tail += string(c)
	if len(r.tail) > 64 {
		r.tail = r.tail[len(r.tail)-64:]
	}
	if strings.HasSuffix(r.tail, "\x1b[200~") {
		r.paste = true
	}
	if strings.HasSuffix(r.tail, "\x1b[201~") {
		r.paste = false
	}
	if !r.paste && submissionEnter(r.tail) {
		r.mu.Lock()
		r.gate = make(chan struct{})
		r.mu.Unlock()
	}
}

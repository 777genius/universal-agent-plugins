package terminalprompts

import (
	"context"
	"io"
	"sync"

	"github.com/muesli/cancelreader"
)

// submissionReader stops at an Enter boundary until the form either rejects
// that submission or exits. Ultraviolet otherwise reads 4096-byte batches and
// drops answers queued for the next form/plain prompt. It owns no read goroutine
// and no persistent buffer. The six-byte tail only recognizes bracketed paste,
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
	r.mu.Lock()
	gate := r.gate
	r.mu.Unlock()
	if gate != nil {
		select {
		case <-gate:
		case <-r.done:
			return 0, cancelreader.ErrCanceled
		case <-r.ctx.Done():
			return 0, cancelreader.ErrCanceled
		}
	}
	select {
	case <-r.done:
		return 0, cancelreader.ErrCanceled
	case <-r.ctx.Done():
		return 0, cancelreader.ErrCanceled
	default:
	}
	if len(p) == 0 {
		return 0, nil
	}
	n, err := r.reader.Read(p[:1])
	if n == 1 {
		r.tail += string(p[0])
		if len(r.tail) > 6 {
			r.tail = r.tail[len(r.tail)-6:]
		}
		if r.tail == "\x1b[200~" {
			r.paste = true
		}
		if r.tail == "\x1b[201~" {
			r.paste = false
		}
		if !r.paste && p[0] == '\r' {
			r.mu.Lock()
			r.gate = make(chan struct{})
			r.mu.Unlock()
		}
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

package installerui

import (
	"context"
	"errors"
	"io"
	"os"
	"sync"

	"github.com/muesli/cancelreader"
)

type formWriter struct {
	io.Writer
	mu     sync.Mutex
	err    error
	cancel context.CancelFunc
}

func (w *formWriter) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	// Renderer writes run on Bubble Tea's goroutine, outside run's recovery.
	// Convert an injected writer panic into the usual cancellation path, while
	// allowing subsequent writes to restore the terminal. Never expose its value.
	defer func() {
		if recover() != nil {
			n, err = 0, errors.New("terminal output writer panicked")
		}
		if err != nil && w.err == nil {
			w.err = err
			w.cancel()
		}
	}()
	n, err = w.Writer.Write(p)
	if err == nil && n != len(p) {
		err = io.ErrShortWrite
	}
	return n, err
}
func (w *formWriter) Err() error { w.mu.Lock(); defer w.mu.Unlock(); return w.err }

type formFileWriter struct {
	*formWriter
	file *os.File
}

func (w *formFileWriter) Fd() uintptr                { return w.file.Fd() }
func (w *formFileWriter) Read(p []byte) (int, error) { return w.file.Read(p) }
func (w *formFileWriter) Close() error               { return nil } // inherited output remains caller-owned

type formReader struct {
	reader  io.Reader
	mu      sync.Mutex
	err     error
	cancel  context.CancelFunc
	active  sync.WaitGroup
	stopped bool
}

func (r *formReader) Read(p []byte) (int, error) {
	r.mu.Lock()
	if r.stopped {
		r.mu.Unlock()
		return 0, cancelreader.ErrCanceled
	}
	r.active.Add(1)
	r.mu.Unlock()
	defer r.active.Done()
	n, err := r.reader.Read(p)
	if err == nil && n == 0 {
		err = io.ErrNoProgress
	}
	if err != nil && !errors.Is(err, cancelreader.ErrCanceled) {
		r.mu.Lock()
		if r.err == nil {
			r.err = err
		}
		r.mu.Unlock()
		r.cancel()
	}
	return n, err
}
func (r *formReader) Err() error { r.mu.Lock(); defer r.mu.Unlock(); return r.err }

type formFileReader struct {
	*formReader
	file *os.File
}

func (r *formFileReader) Fd() uintptr                 { return r.file.Fd() }
func (r *formFileReader) Write(p []byte) (int, error) { return r.file.Write(p) }
func (r *formFileReader) Close() error                { return nil }

// Omitting Name deliberately keeps the outer library on its fallback reader;
// our inner cancel reader owns wakeups, and stop joins every in-flight read.
func (r *formReader) stop(cancelRead func()) {
	r.mu.Lock()
	r.stopped = true
	r.mu.Unlock()
	cancelRead()
	r.active.Wait()
}

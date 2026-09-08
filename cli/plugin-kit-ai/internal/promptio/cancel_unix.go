//go:build linux || darwin || freebsd || openbsd || netbsd || dragonfly

package promptio

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/muesli/cancelreader"
)

// A per-question cancel reader owns only its wakeup descriptors. It never
// closes the caller's terminal, and its watcher is joined before handoff.
func readCancelable(ctx context.Context, reader io.Reader) (line string, err error) {
	f, ok := reader.(*os.File)
	if !ok {
		return readLine(ctx, reader)
	}
	info, e := f.Stat()
	if e != nil {
		return "", fmt.Errorf("inspect prompt input: %w", e)
	}
	if info.Mode().IsRegular() {
		return readLine(ctx, reader)
	}
	cr, e := cancelreader.NewReader(f)
	if e != nil {
		return "", fmt.Errorf("prepare prompt input: %w", e)
	}
	done := make(chan struct{})
	joined := make(chan struct{})
	go func() {
		defer close(joined)
		select {
		case <-ctx.Done():
			cr.Cancel()
		case <-done:
		}
	}()
	defer func() {
		close(done)
		<-joined
		if e := cr.Close(); err == nil && e != nil {
			line = ""
			err = e
		}
	}()
	line, err = readLine(ctx, cr)
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	return line, err
}

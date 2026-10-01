package pluginkitai

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"
)

// NewCursorObserverPipeIO transfers exclusive ownership of two distinct pipe
// files to a Cursor observer. Neither may be used concurrently or after this
// call. On Linux/macOS originals close during preparation; prepared handles are
// closed when RunCursorObserver returns. This is for Config.IO injection; it
// does not replace injected IO silently. Setup failures surface on read/write.
// Only pipe files that can support deadlines are accepted. Linux/macOS inherited
// blocking pipes are adapted; other systems require already pollable pipes.
// Never pass the same descriptor for both arguments.
func NewCursorObserverPipeIO(stdin, stdout *os.File) CursorObserverIO {
	return newCursorPipeIO(stdin, stdout)
}

type cursorPipeIO struct {
	stdin, stdout     *os.File
	readErr, writeErr error
}

func newCursorProcessIO() *cursorPipeIO { return newCursorPipeIO(os.Stdin, os.Stdout) }

func newCursorPipeIO(stdin, stdout *os.File) *cursorPipeIO {
	in, readErr := prepareCursorPipe(stdin)
	out, writeErr := prepareCursorPipe(stdout)
	return &cursorPipeIO{stdin: in, stdout: out, readErr: readErr, writeErr: writeErr}
}

func (p *cursorPipeIO) close() {
	if p.stdin != nil {
		_ = p.stdin.Close()
	}
	if p.stdout != nil {
		_ = p.stdout.Close()
	}
}

func (p *cursorPipeIO) ReadStdin(ctx context.Context) ([]byte, error) {
	if p.readErr != nil {
		return nil, p.readErr
	}
	stop, err := cursorPipeDeadline(ctx, p.stdin.SetReadDeadline)
	if err != nil {
		return nil, err
	}
	defer stop()
	b, err := io.ReadAll(io.LimitReader(p.stdin, int64(MaxPayloadBytes)+1))
	if err != nil {
		return nil, err
	}
	if len(b) > MaxPayloadBytes {
		return nil, fmt.Errorf("Cursor stdin exceeds max payload size")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return b, nil
}

func (p *cursorPipeIO) WriteStdoutContext(ctx context.Context, b []byte) error {
	if p.writeErr != nil {
		return p.writeErr
	}
	stop, err := cursorPipeDeadline(ctx, p.stdout.SetWriteDeadline)
	if err != nil {
		return err
	}
	defer stop()
	n, err := p.stdout.Write(b)
	if err == nil && n != len(b) {
		return io.ErrShortWrite
	}
	return err
}

func (p *cursorPipeIO) WriteStdout(b []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), cursorObserverOutputReserve)
	defer cancel()
	return p.WriteStdoutContext(ctx, b)
}

func (*cursorPipeIO) WriteStderr(string) error { return nil }

// There is no IO worker goroutine. Cancellation moves the poller deadline;
// stopping joins the short AfterFunc callback before returning/closing files.
func cursorPipeDeadline(ctx context.Context, set func(time.Time) error) (func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	deadline, ok := ctx.Deadline()
	if !ok {
		return nil, fmt.Errorf("Cursor pipe IO requires a deadline")
	}
	if err := set(deadline); err != nil {
		return nil, err
	}
	done := make(chan struct{})
	stop := context.AfterFunc(ctx, func() { _ = set(time.Now()); close(done) })
	return func() {
		if !stop() {
			<-done
		}
	}, nil
}

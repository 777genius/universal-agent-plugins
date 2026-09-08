//go:build darwin

package promptio

import (
	"context"
	"fmt"
	"io"
	"os"

	"golang.org/x/sys/unix"
)

func readUnixTerminalLine(ctx context.Context, reader io.Reader, f *os.File) (line string, err error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	attrs, e := unix.IoctlGetTermios(int(f.Fd()), unix.TIOCGETA)
	if e == unix.ENOTTY {
		return readLine(ctx, reader)
	}
	if e != nil {
		return "", fmt.Errorf("inspect prompt terminal: %w", e)
	}
	if attrs.Lflag&unix.ICANON == 0 {
		return readLine(ctx, reader)
	}
	// Darwin leaves bytes queued in raw mode unprocessed on canonical restore.
	// PENDIN asks the line discipline to reprocess them, without injecting input
	// or flushing valid answers. The existing cancelreader still owns readiness.
	pending := *attrs
	pending.Lflag |= unix.PENDIN
	if e := unix.IoctlSetTermios(int(f.Fd()), unix.TIOCSETA, &pending); e != nil {
		return "", fmt.Errorf("reprocess prompt input: %w", e)
	}
	defer func() {
		if e := unix.IoctlSetTermios(int(f.Fd()), unix.TIOCSETA, attrs); e != nil {
			line = ""
			err = fmt.Errorf("restore prompt terminal: %w", e)
		}
	}()
	// A canonical read stops at one record, including its EOF delimiter. Byte
	// reads on Darwin can leave that delimiter behind for the next owner. Keep
	// reading incomplete records until newline or EOF; partial consent is invalid.
	return readLineBuffer(ctx, reader, make([]byte, 4097))
}

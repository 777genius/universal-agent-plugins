//go:build linux || freebsd || openbsd || netbsd || dragonfly

package promptio

import (
	"context"
	"io"
	"os"
)

func readUnixTerminalLine(ctx context.Context, reader io.Reader, _ *os.File) (string, error) {
	return readLine(ctx, reader)
}

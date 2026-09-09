//go:build !linux && !darwin && !freebsd && !openbsd && !netbsd && !dragonfly && !windows

package promptio

import (
	"context"
	"io"
)

func readCancelable(ctx context.Context, r io.Reader) (string, error) { return readLine(ctx, r) }

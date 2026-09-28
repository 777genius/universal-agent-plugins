//go:build !windows

package terminalprompts

import (
	"context"
	"fmt"
	"os"

	"golang.org/x/term"

	"github.com/777genius/plugin-kit-ai/cli/internal/promptio"
)

func terminalQueuedInput(ctx context.Context, f *os.File) (queued []byte, submitted bool, err error) {
	// Canonical mode can hide an incomplete queued line (notably a lone Space).
	// This short, synchronous ownership interval has no competing form reader.
	restore, err := promptio.SnapshotTerminal(int(f.Fd()))
	if err != nil {
		return nil, false, fmt.Errorf("prepare consent boundary: %w", err)
	}
	defer func() {
		if e := restore(); e != nil {
			queued, submitted, err = nil, false, fmt.Errorf("restore consent boundary: %w", e)
		}
	}()
	if _, err := term.MakeRaw(int(f.Fd())); err != nil {
		return nil, false, fmt.Errorf("prepare consent boundary: %w", err)
	}
	n, err := queuedInputBytes(f)
	if err != nil {
		return nil, false, fmt.Errorf("inspect consent boundary: %w", err)
	}
	return readQueuedInput(ctx, f, n)
}

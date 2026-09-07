//go:build !windows

package scaffold

import (
	"context"
	"os"
)

func renamePackage(_ context.Context, from *os.File, old string, to *os.File, new string, check func() error) error {
	if err := check(); err != nil {
		return err
	}
	return renameExclusive(from, old, to, new)
}

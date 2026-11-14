package ports

import (
	"context"
	"io"
)

// ArchiveExtractor extracts a single binary from a .tar.gz stream (root entries only).
type ArchiveExtractor interface {
	ExtractRootExecutable(ctx context.Context, r io.Reader) (name string, data []byte, err error)
}

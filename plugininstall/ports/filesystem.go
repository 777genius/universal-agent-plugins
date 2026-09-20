package ports

import (
	"context"
	"io"
)

type PathInfo struct {
	Exists bool
	IsDir  bool
}

// FileSystem writes installed binaries and inspects install paths.
type FileSystem interface {
	MkdirAll(ctx context.Context, path string, perm uint32) error
	WriteFileAtomic(ctx context.Context, dir, name string, r io.Reader, size int64, perm uint32) error
	// PathInfo reports whether a path exists and whether it is a directory.
	PathInfo(ctx context.Context, path string) (PathInfo, error)
	// RemoveBestEffort removes path if present; ignores not-exist (best effort before overwrite on Windows).
	RemoveBestEffort(ctx context.Context, path string) error
}

package ports

import "context"

// DirectorySnapshots keeps the existing sealed-tree reader behind an injected
// boundary. CaptureDirectory returns an immutable root and a checked cleanup;
// Digest retains the installed package receipt's tree framing.
type DirectorySnapshots interface {
	CaptureDirectory(context.Context, string) (string, func() error, error)
	Digest(context.Context, string) (string, error)
}

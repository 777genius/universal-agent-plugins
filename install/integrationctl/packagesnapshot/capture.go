package packagesnapshot

import "context"

// CaptureDirectory exposes the existing sealed snapshot without exporting its
// concrete implementation across a client's injected directory-reader boundary.
func (builder Builder) CaptureDirectory(ctx context.Context, root string) (string, func() error, error) {
	snapshot, err := builder.Build(ctx, root)
	if err != nil {
		return "", nil, err
	}
	return snapshot.Root, snapshot.Close, nil
}

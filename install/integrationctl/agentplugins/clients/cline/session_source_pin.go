package cline

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/atomicfile"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
)

const sessionSourceLimit int64 = 1 << 20

// This ephemeral lease observes the original source; it creates no writer lock.
// The native client still opens its documented pathname. It cannot atomically
// exclude arbitrary same-user changes between a check and that native open.
type sessionSourcePin struct {
	path     string
	file     *os.File
	metadata *os.File
	info     os.FileInfo
	ancestry *sessionSourceAncestry
}

// Preserve the missing-FileIO refusal before any physical pathname probe.
// Custom readers must explicitly supply bounded reads; there is no OS fallback.
func authorizedSessionSourcePin(ctx context.Context, kernel nativeconfig.Kernel, path string) (*sessionSourcePin, error) {
	if err := kernel.RequireFileIO(); err != nil {
		return nil, err
	}
	if err := kernel.RequireBoundedRead(); err != nil {
		return nil, err
	}
	return pinSessionSource(ctx, path)
}

func pinSessionSource(ctx context.Context, path string) (selected *sessionSourcePin, resultErr error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if path == "" || !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return nil, fmt.Errorf("cline selected MCP source path must be absolute and clean")
	}
	before, err := os.Lstat(path)
	if err != nil || !sessionSourceRegular(before) {
		return nil, errors.Join(err, nativeconfig.ErrConcurrentChange)
	}
	ancestry, err := captureSessionSourceAncestry(ctx, filepath.Dir(path))
	if err != nil {
		return nil, err
	}
	selected = &sessionSourcePin{path: path, ancestry: ancestry}
	heldPin := selected
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, heldPin.Close())
		}
	}()
	selected.metadata, err = openSessionPinnedPath(path, false)
	if err != nil {
		return nil, err
	}
	metadata, err := selected.metadata.Stat()
	if err != nil || !sessionSourceRegular(metadata) || !sameSessionPhysicalIdentity(before, metadata, false) {
		return nil, errors.Join(err, nativeconfig.ErrConcurrentChange)
	}
	file, err := atomicfile.OpenRootReadFile(ancestry.parentRoot(), filepath.Base(path))
	if err != nil {
		return nil, err
	}
	selected.file = file
	info, err := file.Stat()
	if err != nil || !sessionSourceRegular(info) || !sameSessionPhysicalIdentity(metadata, info, false) {
		return nil, errors.Join(err, nativeconfig.ErrConcurrentChange)
	}
	selected.info = info
	if err := selected.Revalidate(ctx); err != nil {
		return nil, err
	}
	return selected, nil
}

func sessionSourceRegular(info os.FileInfo) bool {
	return info != nil && info.Mode().IsRegular() && info.Size() >= 0 && info.Size() <= sessionSourceLimit
}

func (pin *sessionSourcePin) Revalidate(ctx context.Context) error {
	if err := pin.ancestry.Revalidate(ctx); err != nil {
		return err
	}
	named, err := os.Lstat(pin.path)
	if err != nil || !sameSessionPhysicalIdentity(pin.info, named, false) {
		return errors.Join(err, nativeconfig.ErrConcurrentChange)
	}
	rooted, err := pin.ancestry.parentRoot().Lstat(filepath.Base(pin.path))
	if err != nil || !sameSessionPhysicalIdentity(pin.info, rooted, false) {
		return errors.Join(err, nativeconfig.ErrConcurrentChange)
	}
	metadata, err := pin.metadata.Stat()
	if err != nil || !sameSessionPhysicalIdentity(pin.info, metadata, false) {
		return errors.Join(err, nativeconfig.ErrConcurrentChange)
	}
	held, err := pin.file.Stat()
	if err != nil || !sameSessionPhysicalIdentity(pin.info, held, false) {
		return errors.Join(err, nativeconfig.ErrConcurrentChange)
	}
	return ctx.Err()
}

func (pin *sessionSourcePin) Snapshot(ctx context.Context) (nativeconfig.FileSnapshot, error) {
	if err := pin.Revalidate(ctx); err != nil {
		return nativeconfig.FileSnapshot{}, err
	}
	body, err := io.ReadAll(io.NewSectionReader(pin.file, 0, sessionSourceLimit+1))
	if err != nil {
		return nativeconfig.FileSnapshot{}, err
	}
	if int64(len(body)) > sessionSourceLimit || int64(len(body)) != pin.info.Size() {
		return nativeconfig.FileSnapshot{}, nativeconfig.ErrConcurrentChange
	}
	if err := pin.Revalidate(ctx); err != nil {
		return nativeconfig.FileSnapshot{}, err
	}
	return nativeconfig.FileSnapshot{Body: body, Mode: pin.info.Mode().Perm(), Exists: true}, nil
}

// Matching bytes never replace physical identity, even if timestamps match.
func sameSessionPhysicalIdentity(original, observed os.FileInfo, directory bool) bool {
	if original == nil || observed == nil || !os.SameFile(original, observed) ||
		original.Mode() != observed.Mode() || original.Size() != observed.Size() ||
		!original.ModTime().Equal(observed.ModTime()) {
		return false
	}
	if directory {
		return observed.IsDir() && observed.Mode()&os.ModeSymlink == 0
	}
	return observed.Mode().IsRegular()
}

func (pin *sessionSourcePin) Close() error {
	var result error
	if pin.file != nil {
		result = pin.file.Close()
	}
	if pin.metadata != nil {
		result = errors.Join(result, pin.metadata.Close())
	}
	if pin.ancestry != nil {
		result = errors.Join(result, pin.ancestry.Close())
	}
	return result
}

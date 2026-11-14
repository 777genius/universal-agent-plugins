package fs

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"runtime"

	"github.com/777genius/plugin-kit-ai/plugininstall/domain"
	"github.com/777genius/plugin-kit-ai/plugininstall/ports"
)

// OS implements ports.FileSystem using the real OS.
type OS struct{}

var _ ports.FileSystem = (*OS)(nil)

// PathInfo implements ports.FileSystem.
func (OS) PathInfo(ctx context.Context, path string) (ports.PathInfo, error) {
	select {
	case <-ctx.Done():
		return ports.PathInfo{}, ctx.Err()
	default:
	}
	info, err := os.Stat(path)
	if err == nil {
		return ports.PathInfo{Exists: true, IsDir: info.IsDir()}, nil
	}
	if os.IsNotExist(err) {
		return ports.PathInfo{}, nil
	}
	return ports.PathInfo{}, domain.NewError(domain.ExitFS, "stat: "+err.Error())
}

// RemoveBestEffort implements ports.FileSystem.
func (OS) RemoveBestEffort(ctx context.Context, path string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	err := os.Remove(path)
	if err == nil || os.IsNotExist(err) {
		return nil
	}
	return domain.NewError(domain.ExitFS, "remove: "+err.Error())
}

// MkdirAll implements ports.FileSystem.
func (OS) MkdirAll(ctx context.Context, path string, perm uint32) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	return os.MkdirAll(path, os.FileMode(perm))
}

// WriteFileAtomic writes data to dir/name via a temp file in dir and rename.
func (OS) WriteFileAtomic(ctx context.Context, dir, name string, r io.Reader, size int64, perm uint32) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return domain.NewError(domain.ExitFS, "mkdir: "+err.Error())
	}
	dest := filepath.Join(dir, name)
	f, err := os.CreateTemp(dir, ".plugin-kit-ai-install-*")
	if err != nil {
		return domain.NewError(domain.ExitFS, "temp file: "+err.Error())
	}
	tmpPath := f.Name()
	removeTmp := true
	defer func() {
		if removeTmp {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := io.Copy(f, r); err != nil {
		f.Close()
		return domain.NewError(domain.ExitFS, "write: "+err.Error())
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return domain.NewError(domain.ExitFS, "sync: "+err.Error())
	}
	if err := f.Chmod(os.FileMode(perm)); err != nil {
		f.Close()
		return domain.NewError(domain.ExitFS, "chmod: "+err.Error())
	}
	if err := f.Close(); err != nil {
		return domain.NewError(domain.ExitFS, "close temp: "+err.Error())
	}
	if err := os.Rename(tmpPath, dest); err != nil {
		return domain.NewError(domain.ExitFS, "rename: "+err.Error())
	}
	removeTmp = false
	syncParentDir(dir)
	return nil
}

// syncParentDir flushes directory metadata after rename (best effort; skipped on Windows).
func syncParentDir(dir string) {
	if runtime.GOOS == "windows" {
		return
	}
	d, err := os.Open(dir)
	if err != nil {
		return
	}
	_ = d.Sync()
	_ = d.Close()
}

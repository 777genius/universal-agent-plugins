package processlock

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

// AcquireReadOnly uses the writer's OS mutex without publishing metadata or
// creating a path. Writers retain their lock file after release. If it is
// absent, release validates that no first writer invalidated the observation.
func (lock Lock) AcquireReadOnly(ctx context.Context) (ports.UnlockFunc, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(lock.Path) == "" {
		return nil, fmt.Errorf("agentplugins mutation lock path is required")
	}
	absolute, err := filepath.Abs(lock.Path)
	if err != nil {
		return nil, fmt.Errorf("resolve mutation lock path: %w", err)
	}
	before, err := os.Lstat(absolute)
	if os.IsNotExist(err) {
		return absentLockRelease(absolute), nil
	}
	if err != nil {
		return nil, fmt.Errorf("inspect mutation lock: %w", err)
	}
	if !before.Mode().IsRegular() {
		return nil, fmt.Errorf("mutation lock must be a regular non-symlink file")
	}
	file, err := os.OpenFile(absolute, os.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("open existing mutation lock: %w", err)
	}
	opened, err := file.Stat()
	current, pathErr := os.Lstat(absolute)
	if err != nil || pathErr != nil || !opened.Mode().IsRegular() || !current.Mode().IsRegular() || !os.SameFile(before, opened) || !os.SameFile(opened, current) {
		return nil, errors.Join(fmt.Errorf("mutation lock identity changed before observation"), err, pathErr, file.Close())
	}
	if err := acquireFile(file); err != nil {
		return nil, errors.Join(fmt.Errorf("%w: %w", ErrActive, err), file.Close())
	}
	return readOnlyLockRelease(file, absolute, opened), nil
}

func absentLockRelease(path string) ports.UnlockFunc {
	var once sync.Once
	var result error
	return func() error {
		once.Do(func() {
			current, err := os.Lstat(path)
			if os.IsNotExist(err) {
				return
			}
			if err != nil {
				result = fmt.Errorf("recheck absent mutation lock: %w", err)
			} else if !current.Mode().IsRegular() {
				result = fmt.Errorf("mutation lock must be a regular non-symlink file")
			} else {
				result = ErrActive
			}
		})
		return result
	}
}

func readOnlyLockRelease(file *os.File, path string, opened os.FileInfo) ports.UnlockFunc {
	var once sync.Once
	var result error
	return func() error {
		once.Do(func() {
			current, err := os.Lstat(path)
			if err != nil || !current.Mode().IsRegular() || !os.SameFile(opened, current) {
				result = errors.Join(fmt.Errorf("mutation lock identity changed during observation"), err)
			}
			result = errors.Join(result, releaseFile(file), file.Close())
		})
		return result
	}
}

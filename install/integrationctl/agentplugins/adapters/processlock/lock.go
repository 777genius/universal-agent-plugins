package processlock

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

// ErrActive lets callers report deterministic conflict diagnostics without
// parsing platform-specific advisory-lock errors.
var ErrActive = errors.New("another agentplugins mutation is active")

type Lock struct {
	Path string
	Now  func() time.Time
}

type metadata struct {
	PID        int    `json:"pid"`
	AcquiredAt string `json:"acquired_at"`
}

func (lock Lock) Acquire(ctx context.Context) (ports.UnlockFunc, error) {
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
	parent := filepath.Dir(filepath.Clean(absolute))
	if err := os.MkdirAll(parent, 0o700); err != nil {
		return nil, fmt.Errorf("create mutation lock directory: %w", err)
	}
	if err := os.Chmod(parent, 0o700); err != nil {
		return nil, fmt.Errorf("protect mutation lock directory: %w", err)
	}
	if info, statErr := os.Lstat(absolute); statErr == nil && info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("mutation lock path must not be a symlink")
	} else if statErr != nil && !os.IsNotExist(statErr) {
		return nil, fmt.Errorf("inspect mutation lock path: %w", statErr)
	}
	file, err := os.OpenFile(absolute, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open mutation lock: %w", err)
	}
	closeOnError := func(cause error) (ports.UnlockFunc, error) {
		_ = file.Close()
		return nil, cause
	}
	if err := os.Chmod(absolute, 0o600); err != nil {
		return closeOnError(fmt.Errorf("protect mutation lock: %w", err))
	}
	if err := acquireFile(file); err != nil {
		return closeOnError(fmt.Errorf("%w: %v", ErrActive, err))
	}
	now := time.Now().UTC()
	if lock.Now != nil {
		now = lock.Now().UTC()
	}
	body, err := json.Marshal(metadata{PID: os.Getpid(), AcquiredAt: now.Format(time.RFC3339Nano)})
	if err != nil {
		_ = releaseFile(file)
		return closeOnError(err)
	}
	body = append(body, '\n')
	if err := file.Truncate(0); err != nil {
		_ = releaseFile(file)
		return closeOnError(fmt.Errorf("truncate mutation lock metadata: %w", err))
	}
	if _, err := file.Seek(0, 0); err != nil {
		_ = releaseFile(file)
		return closeOnError(fmt.Errorf("seek mutation lock metadata: %w", err))
	}
	if _, err := file.Write(body); err != nil {
		_ = releaseFile(file)
		return closeOnError(fmt.Errorf("write mutation lock metadata: %w", err))
	}
	if err := file.Sync(); err != nil {
		_ = releaseFile(file)
		return closeOnError(fmt.Errorf("sync mutation lock metadata: %w", err))
	}
	var once sync.Once
	var releaseErr error
	return func() error {
		once.Do(func() {
			if err := releaseFile(file); err != nil {
				releaseErr = err
			}
			if err := file.Close(); err != nil && releaseErr == nil {
				releaseErr = err
			}
		})
		return releaseErr
	}, nil
}

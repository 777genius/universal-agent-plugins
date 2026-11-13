package commands

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/mcpruntime"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/processlock"
)

// The canonical source identity owns the one cross-process dev session lock.
// This file retains its original fail-closed path and release semantics.
func devSessionLockPath(cache, root string) (string, error) {
	if cache == "" || !filepath.IsAbs(cache) {
		return "", &mcpruntime.Error{Code: "runtime_dev_session_lock_unavailable"}
	}
	canonical, err := filepath.Abs(root)
	if err != nil {
		return "", &mcpruntime.Error{Code: "runtime_dev_session_lock_unavailable"}
	}
	canonical, err = filepath.EvalSymlinks(canonical)
	if err != nil {
		return "", &mcpruntime.Error{Code: "runtime_dev_session_lock_unavailable"}
	}
	digest := sha256.Sum256([]byte(filepath.Clean(canonical)))
	return filepath.Join(cache, "agentplugins", "author-dev-locks", hex.EncodeToString(digest[:])+".lock"), nil
}

func acquireDevSession(ctx context.Context, root string) (func() error, error) {
	cache, err := os.UserCacheDir()
	if err != nil {
		return nil, &mcpruntime.Error{Code: "runtime_dev_session_lock_unavailable"}
	}
	lockPath, err := devSessionLockPath(cache, root)
	if err != nil {
		return nil, err
	}
	release, err := (processlock.Lock{Path: lockPath}).Acquire(ctx)
	if err != nil {
		code := "runtime_dev_session_lock_unavailable"
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}
		if errors.Is(err, processlock.ErrActive) {
			code = "runtime_dev_session_conflict"
		}
		return nil, &mcpruntime.Error{Code: code}
	}
	return release, nil
}

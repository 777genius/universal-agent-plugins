package providers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
)

func prepareLockedRuntime(ctx context.Context, dataPath string, config lockedRuntimeConfig, digest string, packageBody, lockBody []byte, runner func(context.Context, string, string, bool) error) (returnErr error) {
	store := filepath.Join(dataPath, "npm-runtime")
	if err := os.Mkdir(store, 0o700); err != nil && !errors.Is(err, os.ErrExist) {
		return err
	}
	if err := requireRealDirectory(store); err != nil {
		return err
	}
	key := strings.TrimPrefix(digest, "sha256:")
	target := filepath.Join(store, key)
	expected := runtimeMarker(config, digest)
	if ready, err := readyLockedRuntime(target, expected); err != nil || ready {
		return err
	}
	lockPath := target + ".lock"
	owner, err := acquireRuntimeLock(ctx, lockPath, target, expected)
	if err != nil {
		return err
	}
	if owner == "" {
		return nil // another process completed the same runtime while we waited
	}
	defer func() {
		if err := releaseRuntimeLock(lockPath, owner); err != nil {
			returnErr = errors.Join(returnErr, err)
		}
	}()
	return installLockedRuntime(ctx, dataPath, store, target, expected, config, packageBody, lockBody, runner)
}

func installLockedRuntime(ctx context.Context, dataPath, store, target string, expected lockedRuntimeMarker, config lockedRuntimeConfig, packageBody, lockBody []byte, runner func(context.Context, string, string, bool) error) error {
	if ready, err := readyLockedRuntime(target, expected); err != nil || ready {
		return err
	}
	temporary, err := os.MkdirTemp(store, ".tmp-"+strings.TrimPrefix(expected.LockDigest, "sha256:")+"-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(temporary) }()
	if err := os.WriteFile(filepath.Join(temporary, "package.json"), packageBody, 0o600); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(temporary, "package-lock.json"), lockBody, 0o600); err != nil {
		return err
	}
	if err := runner(ctx, temporary, dataPath, config.OmitOptional); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := validatePreparedRuntime(temporary, config.Entrypoint); err != nil {
		return err
	}
	if err := writeRuntimeMarker(temporary, expected); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := commitLockedRuntimeTarget(store, target, temporary, expected); err != nil {
		return err
	}
	ready, err := readyLockedRuntime(target, expected)
	if err != nil {
		return fmt.Errorf("prepared npm runtime failed its ready check: %w", err)
	}
	if !ready {
		return fmt.Errorf("prepared npm runtime failed its ready check")
	}
	return nil
}

func validatePreparedRuntime(temporary, entrypointName string) error {
	entrypoint := filepath.Join(temporary, filepath.FromSlash(entrypointName))
	if err := pathpolicy.RequireContainedChild(temporary, entrypoint); err != nil {
		return fmt.Errorf("unsafe locked npm entrypoint: %w", err)
	}
	info, err := os.Stat(entrypoint)
	if err != nil || !info.Mode().IsRegular() {
		return fmt.Errorf("locked npm package did not provide %s", entrypointName)
	}
	binPath := filepath.Join(temporary, "node_modules", ".bin")
	if err := pathpolicy.RequireContainedChild(temporary, binPath); err != nil {
		return fmt.Errorf("unsafe locked npm bin directory: %w", err)
	}
	if err := requireRealDirectory(binPath); err != nil {
		return fmt.Errorf("locked npm package did not provide node_modules/.bin: %w", err)
	}
	return nil
}

func writeRuntimeMarker(temporary string, expected lockedRuntimeMarker) error {
	body, err := json.Marshal(expected)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(temporary, runtimeMarkerName), append(body, '\n'), 0o600); err != nil {
		return err
	}
	return nil
}

func acquireRuntimeLock(ctx context.Context, lockPath, target string, expected lockedRuntimeMarker) (string, error) {
	deadline := time.NewTimer(12 * time.Minute)
	defer deadline.Stop()
	for {
		if err := os.Mkdir(lockPath, 0o700); err == nil {
			return recordRuntimeLockOwner(lockPath)
		} else if !errors.Is(err, os.ErrExist) {
			return "", err
		}
		if ready, err := readyLockedRuntime(target, expected); err != nil || ready {
			return "", err
		}
		if reclaimed, err := reclaimRuntimeLock(lockPath); err != nil {
			return "", err
		} else if reclaimed {
			continue
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-deadline.C:
			return "", fmt.Errorf("locked npm runtime is being prepared by another process; retry after it finishes")
		case <-time.After(250 * time.Millisecond):
		}
	}
}

func releaseRuntimeLock(lockPath, owner string) error {
	before, err := snapshotRuntimeLock(lockPath)
	if err != nil {
		return err
	}
	if before == nil || string(before.owner) != owner {
		return fmt.Errorf("locked npm runtime ownership changed before lock release")
	}
	digest, err := runtimeLockRetirementDigest(lockPath)
	if err != nil {
		return err
	}
	if digest == "" {
		return fmt.Errorf("locked npm runtime disappeared before lock release")
	}
	same, err := sameRuntimeLock(lockPath, before)
	if err != nil {
		return err
	}
	if !same {
		return fmt.Errorf("locked npm runtime ownership changed during lock release")
	}
	return os.Rename(lockPath, lockPath+".retired-"+digest)
}

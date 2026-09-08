package nativeconfig

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

const clineLockStaleAfter = 10 * time.Second

type clineLockTiming struct {
	staleAfter        time.Duration
	pollInterval      time.Duration
	acquireTimeout    time.Duration
	heartbeatInterval time.Duration
}

var defaultClineLockTiming = clineLockTiming{
	staleAfter:        clineLockStaleAfter,
	pollInterval:      25 * time.Millisecond,
	acquireTimeout:    20 * time.Second,
	heartbeatInterval: 2 * time.Second,
}

func (kernel Kernel) acquireCandidateLocks(paths Paths, codec Codec) (func() error, error) {
	pathsToLock := []string{filepath.Clean(paths.JSON)}
	if paths.JSONC != "" {
		pathsToLock = append(pathsToLock, filepath.Clean(paths.JSONC))
	}
	sort.Strings(pathsToLock)
	releases := make([]func() error, 0, len(pathsToLock))
	for _, path := range pathsToLock {
		release, err := kernel.acquireWriteLock(path, codec)
		if err != nil {
			cleanupErr := error(nil)
			for index := len(releases) - 1; index >= 0; index-- {
				cleanupErr = errors.Join(cleanupErr, releases[index]())
			}
			return nil, errors.Join(err, cleanupErr)
		}
		releases = append(releases, release)
	}
	return func() error {
		var result error
		for index := len(releases) - 1; index >= 0; index-- {
			result = errors.Join(result, releases[index]())
		}
		return result
	}, nil
}

type processPathLock struct {
	mutex sync.Mutex
	refs  int
}

var processLocks = struct {
	sync.Mutex
	paths map[string]*processPathLock
}{paths: make(map[string]*processPathLock)}

func acquireProcessPathLock(path string) func() {
	processLocks.Lock()
	lock := processLocks.paths[path]
	if lock == nil {
		lock = &processPathLock{}
		processLocks.paths[path] = lock
	}
	lock.refs++
	processLocks.Unlock()

	lock.mutex.Lock()
	return func() {
		lock.mutex.Unlock()
		processLocks.Lock()
		lock.refs--
		if lock.refs == 0 {
			delete(processLocks.paths, path)
		}
		processLocks.Unlock()
	}
}

func (kernel Kernel) acquireWriteLock(path string, codec Codec) (func() error, error) {
	releaseProcess := acquireProcessPathLock(path)
	switch kernel.files.(type) {
	case osFiles, conditionalOSFiles:
	default:
		return func() error { releaseProcess(); return nil }, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		releaseProcess()
		return nil, fmt.Errorf("create native config parent for lock: %w", err)
	}
	var releaseFile func() error
	var err error
	if codec == CodecCline {
		releaseFile, err = lockClineConfig(path + ".lock")
	} else {
		releaseFile, err = lockNativeConfig(path + ".agentplugins.lock")
	}
	if err != nil {
		releaseProcess()
		return nil, fmt.Errorf("lock native config: %w", err)
	}
	return func() error {
		err := releaseFile()
		releaseProcess()
		return err
	}, nil
}

func lockClineConfig(path string) (func() error, error) {
	return lockClineConfigWithTiming(path, defaultClineLockTiming)
}

type acquiredClineLock struct {
	lockDir   string
	ownerFile string
	token     string
	identity  os.FileInfo
	stop      chan struct{}
	done      chan error
}

func lockClineConfigWithTiming(path string, timing clineLockTiming) (func() error, error) {
	token, err := newClineLockToken()
	if err != nil {
		return nil, err
	}
	deadline := time.Now().Add(timing.acquireTimeout)
	for {
		lock, acquired, err := tryAcquireClineLock(path, token)
		if err != nil {
			return nil, err
		}
		if acquired {
			startClineLockHeartbeat(lock, timing.heartbeatInterval)
			var once sync.Once
			var releaseErr error
			return func() error {
				once.Do(func() { releaseErr = releaseClineLock(lock) })
				return releaseErr
			}, nil
		}
		if _, err := reclaimStaleClineLock(path, timing.staleAfter); err != nil {
			return nil, err
		}
		if !time.Now().Before(deadline) {
			return nil, fmt.Errorf("timed out waiting for Cline native config lock")
		}
		time.Sleep(timing.pollInterval)
	}

}

func newClineLockToken() (string, error) {
	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		return "", fmt.Errorf("create Cline lock owner token: %w", err)
	}
	return fmt.Sprintf("%d.%d.%s", os.Getpid(), time.Now().UnixMilli(), hex.EncodeToString(random)), nil
}

func tryAcquireClineLock(lockDir, token string) (*acquiredClineLock, bool, error) {
	if err := os.MkdirAll(filepath.Dir(lockDir), 0o755); err != nil {
		return nil, false, err
	}
	stagingDir := lockDir + ".tmp." + token
	if err := os.RemoveAll(stagingDir); err != nil {
		return nil, false, err
	}
	if err := os.Mkdir(stagingDir, 0o700); err != nil {
		return nil, false, err
	}
	ownerName := "owner." + token
	ownerFile := filepath.Join(stagingDir, ownerName)
	cleanup := func() { _ = os.RemoveAll(stagingDir) }
	if err := os.WriteFile(ownerFile, []byte(token), 0o600); err != nil {
		cleanup()
		return nil, false, err
	}
	identity, err := os.Lstat(stagingDir)
	if err != nil {
		cleanup()
		return nil, false, err
	}
	if err := os.Rename(stagingDir, lockDir); err != nil {
		cleanup()
		if os.IsExist(err) {
			return nil, false, nil
		}
		_, currentErr := os.Lstat(lockDir)
		if currentErr == nil {
			return nil, false, nil
		}
		if os.IsNotExist(currentErr) {
			return nil, false, err
		}
		return nil, false, currentErr
	}
	return &acquiredClineLock{
		lockDir:   lockDir,
		ownerFile: filepath.Join(lockDir, ownerName),
		token:     token,
		identity:  identity,
		stop:      make(chan struct{}),
		done:      make(chan error, 1),
	}, true, nil
}

func reclaimStaleClineLock(lockDir string, staleAfter time.Duration) (bool, error) {
	current, err := os.Lstat(lockDir)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !current.IsDir() || current.Mode()&os.ModeSymlink != 0 {
		return false, fmt.Errorf("Cline native config lock is not an exact directory")
	}
	if time.Since(current.ModTime()) < staleAfter {
		return false, nil
	}
	token, err := newClineLockToken()
	if err != nil {
		return false, err
	}
	staleDir := lockDir + ".stale." + token
	if err := os.Rename(lockDir, staleDir); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	renamed, err := os.Lstat(staleDir)
	if err != nil {
		return false, err
	}
	if !os.SameFile(current, renamed) {
		return false, fmt.Errorf("Cline stale lock identity changed: %w", ErrConcurrentChange)
	}
	// The owner may have refreshed the directory between our initial stat and
	// rename. Put a lock that proved live back instead of deleting it.
	if time.Since(renamed.ModTime()) < staleAfter {
		if err := os.Rename(staleDir, lockDir); err != nil {
			return false, fmt.Errorf("restore refreshed Cline lock: %w", errors.Join(ErrConcurrentChange, err))
		}
		return false, nil
	}
	if err := os.RemoveAll(staleDir); err != nil {
		return false, err
	}
	return true, nil
}

func verifyClineLockOwnership(lock *acquiredClineLock) error {
	current, err := os.Lstat(lock.lockDir)
	if err != nil {
		return fmt.Errorf("Cline native config lock identity changed: %w", errors.Join(ErrConcurrentChange, err))
	}
	if !current.IsDir() || current.Mode()&os.ModeSymlink != 0 || !os.SameFile(lock.identity, current) {
		return fmt.Errorf("Cline native config lock identity changed: %w", ErrConcurrentChange)
	}
	ownerInfo, err := os.Lstat(lock.ownerFile)
	if err != nil || !ownerInfo.Mode().IsRegular() || ownerInfo.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("Cline native config lock owner changed: %w", errors.Join(ErrConcurrentChange, err))
	}
	owner, err := os.ReadFile(lock.ownerFile)
	if err != nil || string(owner) != lock.token {
		if err == nil {
			err = fmt.Errorf("foreign owner token")
		}
		return fmt.Errorf("Cline native config lock owner changed: %w", errors.Join(ErrConcurrentChange, err))
	}
	return nil
}

func startClineLockHeartbeat(lock *acquiredClineLock, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-lock.stop:
				lock.done <- nil
				return
			default:
			}
			select {
			case <-lock.stop:
				lock.done <- nil
				return
			case now := <-ticker.C:
				if err := verifyClineLockOwnership(lock); err != nil {
					lock.done <- fmt.Errorf("refresh Cline native config lock: %w", err)
					return
				}
				if err := os.Chtimes(lock.lockDir, now, now); err != nil {
					lock.done <- fmt.Errorf("refresh Cline native config lock: %w", err)
					return
				}
			}
		}
	}()
}

func releaseClineLock(lock *acquiredClineLock) error {
	close(lock.stop)
	heartbeatErr := <-lock.done
	if err := verifyClineLockOwnership(lock); err != nil {
		return errors.Join(heartbeatErr, err)
	}
	if err := os.Remove(lock.ownerFile); err != nil {
		return errors.Join(heartbeatErr, err)
	}
	if err := os.Remove(lock.lockDir); err != nil {
		return errors.Join(heartbeatErr, err)
	}
	return heartbeatErr
}

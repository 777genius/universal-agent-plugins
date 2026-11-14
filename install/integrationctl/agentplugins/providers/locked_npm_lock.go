package providers

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"time"
)

// These lock records are shared with the npm bridge launcher. In particular,
// release must retire the directory, not remove owner.json and then the
// directory: a launcher could reclaim that ownerless gap.
const runtimeLockReclaimMarker = ".agentplugins-reclaim.json"

type runtimeLockSnapshot struct {
	identity os.FileInfo
	owner    []byte
}

func runtimeLockNonce() (string, error) {
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(nonce[:]), nil
}

func recordRuntimeLockOwner(lockPath string) (string, error) {
	nonce, err := runtimeLockNonce()
	if err != nil {
		_ = os.Remove(lockPath)
		return "", err
	}
	owner := fmt.Sprintf("{\"pid\":%d,\"token\":%q,\"created_at\":%q}\n", os.Getpid(), nonce, time.Now().UTC().Format(time.RFC3339Nano))
	file, err := os.OpenFile(filepath.Join(lockPath, "owner.json"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		_ = os.Remove(lockPath)
		return "", err
	}
	_, writeErr := file.WriteString(owner)
	closeErr := file.Close()
	if err := errors.Join(writeErr, closeErr); err != nil {
		return "", err
	}
	if _, err := os.Lstat(filepath.Join(lockPath, runtimeLockReclaimMarker)); err == nil {
		return "", fmt.Errorf("locked npm runtime lock was reclaimed during acquisition")
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	snapshot, err := snapshotRuntimeLock(lockPath)
	if err != nil {
		return "", err
	}
	if snapshot == nil || string(snapshot.owner) != owner {
		return "", fmt.Errorf("locked npm runtime ownership changed during acquisition")
	}
	return owner, nil
}

func snapshotRuntimeLock(lockPath string) (*runtimeLockSnapshot, error) {
	identity, err := os.Lstat(lockPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !identity.IsDir() || identity.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("locked npm runtime lock is not a real directory")
	}
	owner, err := os.ReadFile(filepath.Join(lockPath, "owner.json"))
	if errors.Is(err, os.ErrNotExist) {
		return &runtimeLockSnapshot{identity: identity}, nil
	}
	if err != nil {
		return nil, err
	}
	return &runtimeLockSnapshot{identity: identity, owner: owner}, nil
}

func sameRuntimeLock(lockPath string, before *runtimeLockSnapshot) (bool, error) {
	after, err := snapshotRuntimeLock(lockPath)
	if err != nil || after == nil {
		return false, err
	}
	return os.SameFile(before.identity, after.identity) && bytes.Equal(before.owner, after.owner), nil
}

func runtimeLockRetirementDigest(lockPath string) (string, error) {
	nonce, err := runtimeLockNonce()
	if err != nil {
		return "", err
	}
	candidate := fmt.Sprintf("%s.reclaim-%d-%s", lockPath, os.Getpid(), nonce)
	claim := []byte(fmt.Sprintf("{\"pid\":%d,\"token\":%q}\n", os.Getpid(), nonce))
	file, err := os.OpenFile(candidate, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", err
	}
	_, writeErr := file.Write(claim)
	closeErr := file.Close()
	defer func() { _ = os.Remove(candidate) }()
	if err := errors.Join(writeErr, closeErr); err != nil {
		return "", err
	}
	markerPath := filepath.Join(lockPath, runtimeLockReclaimMarker)
	marker := claim
	if err := os.Link(candidate, markerPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		if !errors.Is(err, os.ErrExist) {
			return "", err
		}
		marker, err = os.ReadFile(markerPath)
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		if err != nil {
			return "", err
		}
	}
	sum := sha256.Sum256(marker)
	return hex.EncodeToString(sum[:]), nil
}

func runtimeLockAbandoned(snapshot *runtimeLockSnapshot) bool {
	var owner struct {
		PID int `json:"pid"`
	}
	if json.Unmarshal(snapshot.owner, &owner) == nil && owner.PID > 0 {
		return runtimeLockProcessDead(owner.PID)
	}
	return time.Since(snapshot.identity.ModTime()) >= 30*time.Second
}

func runtimeLockProcessDead(pid int) bool {
	if runtime.GOOS == "windows" {
		// os.FindProcess checks process existence on Windows. If access is
		// denied, retain the lock rather than risk retiring a live owner.
		process, err := os.FindProcess(pid)
		if err != nil {
			return errors.Is(err, syscall.Errno(87))
		}
		_ = process.Release()
		return false
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	defer func() { _ = process.Release() }()
	err = process.Signal(syscall.Signal(0))
	return errors.Is(err, syscall.ESRCH) || errors.Is(err, os.ErrProcessDone)
}

func reclaimRuntimeLock(lockPath string) (bool, error) {
	before, err := snapshotRuntimeLock(lockPath)
	if err != nil || before == nil || !runtimeLockAbandoned(before) {
		return false, err
	}
	digest, err := runtimeLockRetirementDigest(lockPath)
	if err != nil || digest == "" {
		return false, err
	}
	same, err := sameRuntimeLock(lockPath, before)
	if err != nil || !same || !runtimeLockAbandoned(before) {
		return false, err
	}
	if err := os.Rename(lockPath, lockPath+".retired-"+digest); err != nil {
		if errors.Is(err, os.ErrNotExist) || errors.Is(err, os.ErrExist) ||
			errors.Is(err, syscall.ENOTEMPTY) || errors.Is(err, syscall.EPERM) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

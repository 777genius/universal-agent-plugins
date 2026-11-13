//go:build linux

package main

import (
	"context"
	"fmt"
	"os"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

// X.Org LockServer writes/chmods/closes .tX93-lock, then links the final
// name and unlinks the temporary name. The two-link interval is not ready.
// Holding O_PATH prevents identity reuse without reading an aliased file.
type displayLockPublication struct {
	file     *os.File
	original os.FileInfo
}

func (p *displayLockPublication) close() error {
	if p.file == nil {
		return nil
	}
	return p.file.Close()
}
func (p *displayLockPublication) finalInfo(named os.FileInfo) error {
	held, err := p.file.Stat()
	if err != nil || !sameDisplayPublication(p.original, held) || !sameFile(held, named) {
		return fmt.Errorf("display lock changed before final admission")
	}
	return nil
}
func sameDisplayPublication(a, b os.FileInfo) bool {
	aa, aok := a.Sys().(*syscall.Stat_t)
	bb, bok := b.Sys().(*syscall.Stat_t)
	return aok && bok && os.SameFile(a, b) && a.Mode() == b.Mode() && a.Size() == b.Size() && a.ModTime().Equal(b.ModTime()) && aa.Uid == bb.Uid && aa.Gid == bb.Gid
}
func transientDisplayLock(info os.FileInfo, proof isolationProof) bool {
	st, ok := info.Sys().(*syscall.Stat_t)
	return ok && info.Mode().IsRegular() && info.Mode() == 0444 && info.Size() == 11 && st.Nlink == 2 && int64(st.Uid) == int64(proof.UID)
}

// X.Org may briefly expose the pending dentry for the same inode after
// its link count reached one. That is not a final-ready state.
func pendingDisplayLock(held, other os.FileInfo, proof isolationProof) bool {
	return sameDisplayPublication(held, other) &&
		(transientDisplayLock(other, proof) || regular(other, 64))
}

func (p *displayLockPublication) observe(lock string, proof isolationProof) (os.FileInfo, os.FileInfo, error) {
	named, err := anchoredInfo(lock)
	if os.IsNotExist(err) && p.file == nil {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	uid, err := statUID(named)
	if err != nil || int64(uid) != int64(proof.UID) || (!regular(named, 64) && !transientDisplayLock(named, proof)) {
		return nil, nil, fmt.Errorf("display lock publication metadata refused")
	}
	if p.file == nil {
		fd, err := unix.Open(lock, unix.O_PATH|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		if err != nil {
			return nil, nil, err
		}
		p.file, p.original = os.NewFile(uintptr(fd), lock), named
	}
	held, err := p.file.Stat()
	if err != nil {
		return nil, nil, err
	}
	current, err := anchoredInfo(lock)
	if err != nil {
		return nil, nil, err
	}
	if !sameDisplayPublication(p.original, held) || !sameDisplayPublication(held, current) {
		return nil, nil, fmt.Errorf("display lock publication identity changed")
	}
	return current, held, nil
}
func (p *displayLockPublication) single(lock string) (bool, error) {
	held, err := p.file.Stat()
	if err != nil {
		return false, err
	}
	linked, err := anchoredInfo(lock)
	if err != nil {
		return false, err
	}
	if !sameDisplayPublication(p.original, held) || !sameFile(held, linked) || !regular(held, 64) {
		return false, fmt.Errorf("display lock publication pair refused")
	}
	return true, nil
}
func (p *displayLockPublication) ready(lock, temp string, proof isolationProof) (bool, error) {
	current, held, err := p.observe(lock, proof)
	if err != nil || current == nil {
		return false, err
	}
	other, err := anchoredInfo(temp)
	if regular(current, 64) && regular(held, 64) {
		if err == nil && pendingDisplayLock(held, other, proof) {
			return false, nil // Wait until the matching pending name vanishes.
		}
		if !os.IsNotExist(err) {
			return false, fmt.Errorf("display temporary lock collision")
		}
		return p.single(lock)
	}
	if os.IsNotExist(err) {
		// Only a known two-link snapshot may advance to fresh single-link
		// validation. Poll again under the original readiness deadline.
		if !transientDisplayLock(current, proof) && !transientDisplayLock(held, proof) {
			return false, fmt.Errorf("display lock publication links refused")
		}
		_, err := p.single(lock)
		return false, err
	}
	if err != nil {
		return false, err
	}
	if !transientDisplayLock(held, proof) || !transientDisplayLock(current, proof) || !pendingDisplayLock(held, other, proof) {
		return false, fmt.Errorf("display lock publication pair refused")
	}
	return false, nil
}

func requireFreshDisplayLocks(lock, temp, socket string) error {
	for _, name := range []string{lock, temp, socket} {
		if _, err := os.Lstat(name); !os.IsNotExist(err) {
			return fmt.Errorf("display/lock collision; refusing to attach: %s", name)
		}
	}
	return nil
}

// Only the fixed display readiness callback is polled; budgets are the caller's
// original ten-second timer and supervised cancellation, never renewed here.
func waitDisplayReadiness(ctx context.Context, deadline, tick <-chan time.Time, observe func() (bool, error)) error {
	check := func() error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline:
			return fmt.Errorf("display readiness exceeded ten seconds")
		default:
			return nil
		}
	}
	for {
		if err := check(); err != nil {
			return err
		}
		ready, err := observe()
		if err != nil {
			return err
		}
		if ready {
			return check()
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline:
			return fmt.Errorf("display readiness exceeded ten seconds")
		case <-tick:
		}
	}
}

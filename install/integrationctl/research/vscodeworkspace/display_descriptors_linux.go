//go:build linux

package main

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

// Keeping the original inode open prevents immediate unlink/recreate from
// recycling that identity within the filesystem's metadata time quantum.
type heldDisplayAsset struct {
	info os.FileInfo
	file *os.File
}

func (asset heldDisplayAsset) verify(path string) error {
	if asset.file == nil || asset.info == nil {
		return fmt.Errorf("display asset descriptor is missing")
	}
	held, err := asset.file.Stat()
	if err != nil || !sameFile(asset.info, held) {
		return fmt.Errorf("held display asset identity changed or closed")
	}
	named, err := os.Lstat(path)
	if err != nil || !sameFile(held, named) {
		return fmt.Errorf("named display asset differs from held identity")
	}
	return nil
}

func holdDisplayAsset(path string, info os.FileInfo) (heldDisplayAsset, error) {
	fd, err := unix.Open(path, unix.O_PATH|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return heldDisplayAsset{}, err
	}
	asset := heldDisplayAsset{info: info, file: os.NewFile(uintptr(fd), path)}
	if err := asset.verify(path); err != nil {
		return heldDisplayAsset{}, errors.Join(err, asset.file.Close())
	}
	return asset, nil
}

func holdDisplayAssets(lockPath, socketPath string, lockInfo, socketInfo os.FileInfo) (map[string]heldDisplayAsset, error) {
	if lockPath == socketPath || lockInfo == nil || socketInfo == nil || os.SameFile(lockInfo, socketInfo) {
		return nil, fmt.Errorf("display asset identities must be distinct")
	}
	lock, err := holdDisplayAsset(lockPath, lockInfo)
	if err != nil {
		return nil, err
	}
	socket, err := holdDisplayAsset(socketPath, socketInfo)
	if err != nil {
		return nil, errors.Join(err, lock.file.Close())
	}
	if err := errors.Join(lock.verify(lockPath), socket.verify(socketPath)); err != nil {
		return nil, errors.Join(err, lock.file.Close(), socket.file.Close())
	}
	return map[string]heldDisplayAsset{lockPath: lock, socketPath: socket}, nil
}

func (c *controller) closeDisplayAssets() (resultErr error) {
	for _, asset := range c.displayAssets {
		if asset.file == nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("display asset descriptor missing at close"))
			continue
		}
		resultErr = errors.Join(resultErr, asset.file.Close())
	}
	c.displayAssets = nil
	return resultErr
}

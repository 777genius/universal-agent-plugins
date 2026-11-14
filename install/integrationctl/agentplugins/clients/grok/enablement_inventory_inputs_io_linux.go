//go:build linux

package grok

import (
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"os"
	"path/filepath"
	"syscall"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/directoryidentity"
)

func inventoryInputRead(ctx context.Context, path string, limit int) (body []byte, info os.FileInfo, resultErr error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	file, info, err := openProjectPath(path, false, nil)
	if err != nil {
		return nil, nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, file.Close()) }()
	body, err = inventoryInputBytes(ctx, path, file, info, limit)
	return body, info, err
}

func inventoryInputBytes(ctx context.Context, path string, file *os.File, info os.FileInfo, limit int) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !inventoryInputRegular(info) || info.Size() < 0 || limit <= 0 || info.Size() > int64(limit) {
		return nil, errEnablementInventoryInputs
	}
	body, err := io.ReadAll(io.NewSectionReader(file, 0, int64(limit)+1))
	if err != nil {
		return nil, err
	}
	if len(body) > limit || int64(len(body)) != info.Size() {
		return nil, errEnablementInventoryInputs
	}
	held, heldErr := file.Stat()
	named, namedErr := os.Lstat(path)
	if err := errors.Join(heldErr, namedErr, ctx.Err()); err != nil {
		return nil, err
	}
	if !inventoryInputSame(info, held) || !inventoryInputSame(held, named) {
		return nil, errEnablementInventoryInputs
	}
	return body, nil
}

func inventoryInputRegular(info os.FileInfo) bool {
	if info == nil || !info.Mode().IsRegular() {
		return false
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && stat != nil && stat.Nlink == 1 && int64(stat.Uid) == int64(os.Getuid()) &&
		int64(stat.Gid) == int64(os.Getgid()) && stat.Mode&07022 == 0
}

func (lease *enablementInventoryInputs) captureInput(ctx context.Context, path string, directory bool) (enablementInventoryPin, []byte, error) {
	if err := ctx.Err(); err != nil {
		return enablementInventoryPin{}, nil, err
	}
	parent, err := inventoryInputParent(ctx, filepath.Dir(path))
	if errors.Is(err, os.ErrNotExist) {
		return enablementInventoryPin{}, nil, lease.captureAbsent(ctx, path)
	}
	if err != nil {
		return enablementInventoryPin{}, nil, err
	}
	file, info, err := openProjectPath(path, directory, nil)
	if errors.Is(err, os.ErrNotExist) {
		return enablementInventoryPin{}, nil, errors.Join(lease.captureAbsent(ctx, path), directoryidentity.Revalidate(ctx, parent))
	}
	if err != nil {
		return enablementInventoryPin{}, nil, err
	}
	lease.files = append(lease.files, file)
	if err := directoryidentity.Revalidate(ctx, parent); err != nil {
		return enablementInventoryPin{}, nil, err
	}
	pin := enablementInventoryPin{path: path, file: file, info: info, parent: parent}
	var body []byte
	if !directory {
		limit := min(projectManifestLimit, lease.remaining)
		body, err = inventoryInputBytes(ctx, path, file, info, limit)
		if err != nil {
			return enablementInventoryPin{}, nil, err
		}
		lease.remaining -= len(body)
		pin.digest = sha256.Sum256(body)
	}
	if err := pin.revalidate(ctx); err != nil {
		return enablementInventoryPin{}, nil, err
	}
	lease.pins = append(lease.pins, pin)
	return pin, body, nil
}

// An absent input is bound to the first absent component and its original
// existing ancestor. Later creation cannot turn it into a newly adopted input.
func (lease *enablementInventoryInputs) captureAbsent(ctx context.Context, path string) error {
	missing := path
	for range 32 {
		if err := ctx.Err(); err != nil {
			return err
		}
		if _, err := os.Lstat(missing); !errors.Is(err, os.ErrNotExist) {
			return errors.Join(err, errEnablementInventoryInputs)
		}
		parentPath := filepath.Dir(missing)
		var file *os.File
		var err error
		if parentPath == string(filepath.Separator) {
			// The fixed filesystem root has no pathname ancestors or leaf alias.
			file, err = os.Open(parentPath)
		} else {
			file, _, err = openProjectPath(parentPath, true, nil)
		}
		if errors.Is(err, os.ErrNotExist) {
			missing = parentPath
			continue
		}
		if err != nil {
			return err
		}
		lease.files = append(lease.files, file)
		parent, err := directoryidentity.CaptureHeld(ctx, parentPath, file)
		if err != nil {
			return err
		}
		pin := enablementInventoryPin{path: missing, parent: parent}
		if err := pin.revalidate(ctx); err != nil {
			return err
		}
		lease.pins = append(lease.pins, pin)
		return nil
	}
	return errEnablementInventoryInputs
}

func (pin enablementInventoryPin) revalidate(ctx context.Context) error {
	if err := directoryidentity.Revalidate(ctx, pin.parent); err != nil {
		return err
	}
	if pin.file == nil {
		if _, err := os.Lstat(pin.path); !errors.Is(err, os.ErrNotExist) {
			return errors.Join(err, errEnablementInventoryInputs)
		}
		return directoryidentity.Revalidate(ctx, pin.parent)
	}
	held, heldErr := pin.file.Stat()
	named, namedErr := os.Lstat(pin.path)
	if err := errors.Join(heldErr, namedErr, ctx.Err()); err != nil {
		return err
	}
	if !inventoryInputSame(pin.info, held) || !inventoryInputSame(held, named) {
		return errEnablementInventoryInputs
	}
	if !pin.info.IsDir() {
		body, err := inventoryInputBytes(ctx, pin.path, pin.file, pin.info, projectManifestLimit)
		if err != nil {
			return err
		}
		if sha256.Sum256(body) != pin.digest {
			return errEnablementInventoryInputs
		}
	}
	return directoryidentity.Revalidate(ctx, pin.parent)
}

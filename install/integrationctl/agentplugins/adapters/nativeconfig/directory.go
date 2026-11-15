package nativeconfig

import (
	"bytes"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/atomicfile"
)

// WithDirectory borrows an original directory for one native mutation. The
// caller retains the root and its physical publication authority through all
// reads, writer locks, replacement, verification and compensation. It must close
// the root only after this kernel is no longer used. No pathname is recaptured
// as replacement authority. Custom IO/lock authorities are never bypassed.
func (kernel Kernel) WithDirectory(root *os.Root, directory string, revalidate func() error) (Kernel, error) {
	if _, ok := kernel.files.(conditionalOSFiles); !ok || kernel.acquireLocks != nil {
		return Kernel{}, fmt.Errorf("retained directory requires the default native config authority")
	}
	if root == nil || revalidate == nil || !filepath.IsAbs(directory) || filepath.Clean(directory) != directory {
		return Kernel{}, fmt.Errorf("original native config directory authority is required")
	}
	if err := revalidate(); err != nil {
		return Kernel{}, err
	}
	info, err := root.Stat(".")
	if err != nil || !info.IsDir() {
		return Kernel{}, errors.Join(err, ErrConcurrentChange)
	}
	files := &directoryFiles{root: root, path: directory, info: info, revalidate: revalidate}
	if err := files.validate(); err != nil {
		return Kernel{}, err
	}
	kernel.files, kernel.acquireLocks = files, files.lock
	return kernel, nil
}

type directoryFiles struct {
	root       *os.Root
	path       string
	info       os.FileInfo
	revalidate func() error
	lockFile   *os.File
	lockName   string
	lockInfo   os.FileInfo
}

func (files *directoryFiles) validate() error {
	if err := files.revalidate(); err != nil {
		return err
	}
	namedRoot, err := os.Lstat(files.path)
	if err != nil || !namedRoot.IsDir() || !os.SameFile(files.info, namedRoot) || files.info.Mode() != namedRoot.Mode() {
		return errors.Join(err, ErrConcurrentChange)
	}
	if files.lockFile != nil {
		named, err := files.root.Lstat(files.lockName)
		if err != nil || !named.Mode().IsRegular() || !os.SameFile(files.lockInfo, named) || files.lockInfo.Mode() != named.Mode() {
			return errors.Join(err, ErrConcurrentChange)
		}
	}
	return nil
}

func (files *directoryFiles) leaf(path string) (string, error) {
	if filepath.Dir(path) != files.path || filepath.Clean(path) != path {
		return "", fmt.Errorf("native config is outside its original directory")
	}
	if err := files.validate(); err != nil {
		return "", err
	}
	return filepath.Base(path), nil
}

func (files *directoryFiles) ReadNoFollow(path string) (body []byte, mode os.FileMode, exists bool, resultErr error) {
	name, err := files.leaf(path)
	if err != nil {
		return nil, 0, false, err
	}
	defer func() { resultErr = errors.Join(resultErr, files.validate()) }()
	file, err := atomicfile.OpenRootReadFile(files.root, name)
	if os.IsNotExist(err) {
		return nil, 0600, false, nil
	}
	if err != nil {
		return nil, 0, false, err
	}
	defer func() { resultErr = errors.Join(resultErr, file.Close()) }()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxHermesConfigBytes {
		return nil, 0, false, errors.Join(err, fmt.Errorf("retained native config is not bounded and ordinary"))
	}
	body, err = io.ReadAll(io.LimitReader(file, maxHermesConfigBytes+1))
	after, statErr := files.root.Lstat(name)
	if err != nil || statErr != nil || !os.SameFile(info, after) || info.Mode() != after.Mode() ||
		info.Size() != after.Size() || !info.ModTime().Equal(after.ModTime()) || int64(len(body)) != info.Size() || len(body) > maxHermesConfigBytes {
		return nil, 0, false, errors.Join(err, statErr, ErrConcurrentChange)
	}
	return body, info.Mode().Perm(), true, nil
}

func (files *directoryFiles) CompareAndSwap(path string, expected []byte, expectedExists bool, next []byte, mode os.FileMode) error {
	check := func() error {
		body, currentMode, exists, err := files.ReadNoFollow(path)
		if err != nil {
			return err
		}
		if exists != expectedExists || !bytes.Equal(body, expected) || exists && currentMode != mode.Perm() {
			return ErrConcurrentChange
		}
		return nil
	}
	if err := check(); err != nil {
		return err
	}
	return files.write(path, next, mode, check)
}

func (files *directoryFiles) WriteAtomic(path string, body []byte, mode os.FileMode) error {
	return files.write(path, body, mode, files.validate)
}

func (files *directoryFiles) write(path string, body []byte, mode os.FileMode, check func() error) (resultErr error) {
	name, err := files.leaf(path)
	if err != nil {
		return err
	}
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return err
	}
	stage := fmt.Sprintf(".%s.uap-%x", name, nonce)
	file, err := files.root.OpenFile(stage, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode.Perm())
	if err != nil {
		return err
	}
	// Failed staging is retained: uncertainty never authorizes unlinking a
	// replacement that appeared at our temporary name.
	n, writeErr := file.Write(body)
	chmodErr := file.Chmod(mode.Perm())
	syncErr := file.Sync()
	info, statErr := file.Stat()
	closeErr := file.Close()
	if err := errors.Join(writeErr, chmodErr, syncErr, statErr, closeErr); err != nil || n != len(body) {
		return errors.Join(err, fmt.Errorf("retained native config staging incomplete"))
	}
	if err := check(); err != nil {
		return err
	}
	named, err := files.root.Lstat(stage)
	if err != nil || !os.SameFile(info, named) || !named.Mode().IsRegular() || named.Size() != int64(len(body)) {
		return errors.Join(err, ErrConcurrentChange)
	}
	if err := files.root.Rename(stage, name); err != nil {
		return err
	}
	if err := syncNativeDirectory(files.root); err != nil {
		return err
	}
	return files.validate()
}

func (files *directoryFiles) RemoveIfUnchanged(path string, expected []byte) error {
	body, _, exists, err := files.ReadNoFollow(path)
	if err != nil {
		return err
	}
	if !exists || !bytes.Equal(body, expected) {
		return ErrConcurrentChange
	}
	return files.RemoveNoFollow(path)
}

func (files *directoryFiles) RemoveNoFollow(path string) error {
	name, err := files.leaf(path)
	if err != nil {
		return err
	}
	if err := files.root.Remove(name); err != nil {
		return err
	}
	return errors.Join(syncNativeDirectory(files.root), files.validate())
}

func (files *directoryFiles) lock(paths Paths, codec Codec) (func() error, error) {
	// This seam is used for one flat Hermes config. Directory-style Cline
	// locks and variant selection retain their ordinary implementations.
	if codec != CodecHermes || paths.JSONC != "" {
		return nil, fmt.Errorf("retained directory requires one Hermes config")
	}
	name, err := files.leaf(paths.JSON)
	if err != nil {
		return nil, err
	}
	releaseProcess := acquireProcessPathLock(paths.JSON)
	file, err := openRootNativeLock(files.root, name+".agentplugins.lock")
	if err != nil {
		releaseProcess()
		return nil, err
	}
	release, err := lockNativeConfigFile(file)
	if err != nil {
		releaseProcess()
		return nil, err
	}
	files.lockFile, files.lockName = file, name+".agentplugins.lock"
	files.lockInfo, err = file.Stat()
	if err == nil {
		err = files.validate()
	}
	if err != nil {
		releaseErr := release()
		files.lockFile = nil
		releaseProcess()
		return nil, errors.Join(err, releaseErr)
	}
	return func() error {
		err := errors.Join(files.validate(), release())
		files.lockFile = nil
		releaseProcess()
		return err
	}, nil
}

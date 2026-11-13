package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/atomicfile"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
)

const runtimeLimit int64 = 256 << 20

type runtimeLease struct {
	file        *os.File
	info        os.FileInfo
	expectedSHA string
}

func release(leases []runtimeLease) (err error) {
	for index := range leases {
		if leases[index].file != nil {
			err = errors.Join(err, leases[index].file.Close())
			leases[index].file = nil
		}
	}
	return err
}
func openRuntime(path, rootPath string) (*os.File, error) {
	if err := pathpolicy.RequireContainedChild(rootPath, path); err != nil {
		return nil, err
	}
	relative, err := filepath.Rel(rootPath, path)
	if err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		return nil, err
	}
	file, openErr := atomicfile.OpenRootReadFile(root, relative)
	err = errors.Join(openErr, root.Close())
	if err != nil && file != nil {
		err = errors.Join(err, file.Close())
		file = nil
	}
	return file, err
}
func holdRuntime(root string) (leases []runtimeLease, resultErr error) {
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, release(leases))
		}
	}()
	uap, native, probe := paths(root)
	leases = make([]runtimeLease, 0, 5)
	for _, item := range [][3]string{{uap, root, installerSHA}, {native, root, nativeSHA}, {probe, root, probeSHA}, {gitBinary, filepath.Dir(gitBinary), gitSHA}, {goBinary, filepath.Dir(goBinary), goSHA}} {
		file, err := openRuntime(item[0], item[1])
		if err != nil {
			return leases, err
		}
		info, err := file.Stat()
		leases = append(leases, runtimeLease{file: file, info: info, expectedSHA: item[2]})
		if err != nil {
			return leases, err
		}
	}
	return leases, nil
}
func verifyLease(lease runtimeLease) error {
	if lease.file == nil || lease.info == nil || !lease.info.Mode().IsRegular() || lease.info.Size() < 0 || lease.info.Size() > runtimeLimit || len(lease.expectedSHA) != 64 {
		return errors.New("runtime metadata refused")
	}
	info, err := lease.file.Stat()
	linked, linkErr := os.Lstat(lease.file.Name())
	if err != nil || linkErr != nil || !unchanged(lease.info, info) || !unchanged(lease.info, linked) {
		return errors.New("held runtime identity changed")
	}
	return nil
}
func verifyLeases(leases []runtimeLease) error {
	for _, lease := range leases {
		if err := verifyLease(lease); err != nil {
			return err
		}
	}
	return nil
}
func joinRoot(root *os.Root, original os.FileInfo, path string) (resultErr error) {
	parent, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, parent.Close()) }()
	linked, err := parent.Lstat(filepath.Base(path))
	if err != nil {
		return err
	}
	held, err := root.Stat(".")
	if err != nil || !os.SameFile(original, held) || !os.SameFile(original, linked) || original.Mode() != held.Mode() || original.Mode() != linked.Mode() {
		return errors.New("original root changed")
	}
	return nil
}
func hashLease(lease runtimeLease) error {
	if err := verifyLease(lease); err != nil {
		return err
	}
	if _, err := lease.file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	digest := sha256.New()
	size, err := io.Copy(digest, io.LimitReader(lease.file, runtimeLimit+1))
	if err != nil {
		return err
	}
	if err = verifyLease(lease); err != nil {
		return err
	}
	if size != lease.info.Size() {
		return errors.New("runtime byte bound")
	}
	if hex.EncodeToString(digest.Sum(nil)) != lease.expectedSHA {
		return errors.New("runtime hash differs")
	}
	return nil
}
func verifyRuntime(ctx context.Context, root *os.Root, original os.FileInfo, path string, leases []runtimeLease) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(leases) != 5 {
		return errors.New("runtime lease count")
	}
	if err := joinRoot(root, original, path); err != nil {
		return err
	}
	if err := verifyLeases(leases); err != nil {
		return err
	}
	for _, lease := range leases {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := hashLease(lease); err != nil {
			return err
		}
	}
	if err := joinRoot(root, original, path); err != nil {
		return err
	}
	if err := verifyLeases(leases); err != nil {
		return err
	}
	return ctx.Err()
}

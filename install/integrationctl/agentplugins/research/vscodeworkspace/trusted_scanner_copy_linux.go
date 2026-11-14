//go:build linux

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func (c *controller) scannerCopy(target string) (resultErr error) {
	source := filepath.Join(c.request.Base, "executables", "lintai")
	f, before, err := openOrdinary(source, 128<<20)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, f.Close()) }()
	if err := scannerSourceAuthority(f, before); err != nil {
		return err
	}
	parent, err := trustedParent(target)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, parent.Close()) }()
	out, err := trustedOpenHeld(parent, filepath.Base(target), nil, false)
	if err != nil {
		return err
	}
	total, digest, err := copyBounded(c.work, f, out, before.Size())
	if err == nil {
		err = out.Chown(0, 0)
	}
	if err == nil {
		err = out.Chmod(0555)
	}
	after, statErr := out.Stat()
	linked, linkErr := anchoredInfo(target)
	closeErr := out.Close()
	if err != nil || closeErr != nil {
		return errors.Join(err, closeErr)
	}
	if statErr != nil || linkErr != nil || !sameFile(after, linked) {
		return fmt.Errorf("held pinned scanner copy differs")
	}
	return scannerSourceIdentity(f, before, source, total, digest, c.request.Input.Trusted.ScannerSHA256)
}

func scannerSourceAuthority(f *os.File, before os.FileInfo) error {
	uid, err := statUID(before)
	if err != nil || uid != 0 || before.Mode().Perm() != 0555 {
		return fmt.Errorf("sealed scanner source authority differs")
	}
	var head [4]byte
	if _, err := f.ReadAt(head[:], 0); err != nil || string(head[:]) != "\x7fELF" {
		return fmt.Errorf("pinned scanner is not ELF")
	}
	return nil
}

func scannerSourceIdentity(f *os.File, before os.FileInfo, source string, total int64, digest, expected string) error {
	current, err := f.Stat()
	named, nameErr := anchoredInfo(source)
	if total != before.Size() || digest != expected || err != nil || nameErr != nil || !sameFile(before, current) || !sameFile(before, named) {
		return fmt.Errorf("held pinned scanner copy differs")
	}
	return nil
}

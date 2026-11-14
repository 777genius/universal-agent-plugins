package app

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func extractBundleArchiveToDir(archivePath, dest string) error {
	tr, closer, err := openBundleArchive(archivePath)
	if err != nil {
		return err
	}
	defer closer()
	return extractBundleArchive(tr, dest)
}

func extractBundleArchiveEntry(tr *tar.Reader, dest string, hdr *tar.Header, policy *bundleArchivePolicy) error {
	name, err := policy.validate(hdr)
	if err != nil {
		return err
	}
	localName := filepath.FromSlash(name)
	if !filepath.IsLocal(localName) {
		return fmt.Errorf("bundle install refuses non-local archive path %q", name)
	}
	target := filepath.Join(dest, localName)
	switch hdr.Typeflag {
	case tar.TypeDir:
		return os.MkdirAll(target, 0o755)
	case tar.TypeReg:
		return writeBundleArchiveFile(tr, target, hdr.Size, hdr.Mode)
	default:
		return fmt.Errorf("bundle install refuses unsupported archive entry %s", name)
	}
}

func writeBundleArchiveFile(tr *tar.Reader, target string, expectedSize, archiveMode int64) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	body, err := io.ReadAll(io.LimitReader(tr, expectedSize+1))
	if err != nil {
		return err
	}
	if int64(len(body)) != expectedSize {
		return fmt.Errorf("bundle install archive file %s changed size while reading", target)
	}
	mode := os.FileMode(0o644)
	if archiveMode&0o111 != 0 {
		mode = 0o755
	}
	file, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	if _, err := file.Write(body); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

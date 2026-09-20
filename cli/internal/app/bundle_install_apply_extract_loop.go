package app

import (
	"archive/tar"
	"io"
)

func extractBundleArchive(tr *tar.Reader, dest string) error {
	policy := newBundleArchivePolicy()
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if err := extractBundleArchiveEntry(tr, dest, hdr, policy); err != nil {
			return err
		}
	}
}

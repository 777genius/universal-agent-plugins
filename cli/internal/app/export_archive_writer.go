package app

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"time"
)

func openExportArchiveWriter(output string) (*tar.Writer, func() error, error) {
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return nil, nil, err
	}
	f, err := os.Create(output)
	if err != nil {
		return nil, nil, err
	}

	gz := gzip.NewWriter(f)
	gz.Name = ""
	gz.Comment = ""
	gz.ModTime = time.Unix(0, 0)

	tw := tar.NewWriter(gz)
	closeArchive := func() error {
		if err := tw.Close(); err != nil {
			_ = gz.Close()
			_ = f.Close()
			return err
		}
		if err := gz.Close(); err != nil {
			_ = f.Close()
			return err
		}
		return f.Close()
	}
	return tw, closeArchive, nil
}

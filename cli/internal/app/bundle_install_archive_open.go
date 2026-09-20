package app

import (
	"archive/tar"
	"compress/gzip"
	"os"
)

func openBundleArchive(path string) (*tar.Reader, func() error, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	gz, err := gzip.NewReader(f)
	if err != nil {
		_ = f.Close()
		return nil, nil, err
	}
	closeFn := func() error {
		err1 := gz.Close()
		err2 := f.Close()
		if err1 != nil {
			return err1
		}
		return err2
	}
	return tar.NewReader(gz), closeFn, nil
}

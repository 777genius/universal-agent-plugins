package app

import (
	"archive/tar"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

func writeExportArchive(root, output string, files []string, metadata exportMetadata) (err error) {
	tw, closeArchive, err := openExportArchiveWriter(output)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := closeArchive(); err == nil && closeErr != nil {
			err = closeErr
		}
	}()
	if err := writeExportArchiveMetadata(tw, metadata); err != nil {
		return err
	}
	return writeExportArchiveFiles(tw, root, files)
}

func writeArchiveEntry(tw *tar.Writer, rel string, body []byte, mode int64) error {
	name := filepath.ToSlash(filepath.Clean(rel))
	if strings.HasPrefix(name, "../") || name == ".." || filepath.IsAbs(name) {
		return fmt.Errorf("invalid archive path %s", rel)
	}
	hdr := &tar.Header{
		Name:     name,
		Mode:     mode,
		Size:     int64(len(body)),
		ModTime:  time.Unix(0, 0),
		Typeflag: tar.TypeReg,
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	_, err := tw.Write(body)
	return err
}

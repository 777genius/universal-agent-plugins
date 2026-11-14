package app

import (
	"archive/tar"
	"os"
	"path/filepath"
)

func writeExportArchiveFiles(tw *tar.Writer, root string, files []string) error {
	for _, rel := range files {
		if err := writeExportArchiveFile(tw, root, rel); err != nil {
			return err
		}
	}
	return nil
}

func writeExportArchiveFile(tw *tar.Writer, root, rel string) error {
	full := filepath.Join(root, filepath.FromSlash(rel))
	info, err := os.Stat(full)
	if err != nil {
		return err
	}
	body, err := os.ReadFile(full)
	if err != nil {
		return err
	}
	return writeArchiveEntry(tw, rel, body, int64(info.Mode().Perm()))
}

//go:build linux

package pluginkitairepo_test

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"golang.org/x/sys/unix"
)

func geminiCycleWriteArchive(output string, files map[string][]byte, beforeExpose func() error) error {
	if err := projectNativeRuntimeParents(filepath.Join(output, "research.zip")); err != nil {
		return err
	}
	partial := filepath.Join(output, ".research.zip.partial")
	file, err := os.OpenFile(partial, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	return geminiCycleWriteOpenedArchive(output, partial, file, files, beforeExpose)
}

// The production caller supplies its exclusively created ordinary os.File.
// A private io.WriteCloser seam lets inert tests return real write/close errors
// after closing that same owned file; it conveys no completion authority.
func geminiCycleWriteOpenedArchive(output, partial string, file io.WriteCloser, files map[string][]byte, beforeExpose func() error) (result error) {
	committed := false
	defer func() {
		if !committed {
			if err := os.Remove(partial); err != nil && !os.IsNotExist(err) {
				result = errors.Join(result, fmt.Errorf("Gemini owned partial cleanup: %w", err))
			}
		}
	}()
	archive := zip.NewWriter(file)
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		entry, err := archive.Create(name)
		if err == nil {
			_, err = entry.Write(files[name])
		}
		if err != nil {
			result = err
			break
		}
	}
	result = errors.Join(result, archive.Close(), file.Close())
	if result != nil {
		return result
	}
	if beforeExpose != nil {
		if err := beforeExpose(); err != nil {
			return err
		}
	}
	if err := unix.Renameat2(unix.AT_FDCWD, partial, unix.AT_FDCWD, filepath.Join(output, "research.zip"), unix.RENAME_NOREPLACE); err != nil {
		return err
	}
	committed = true
	return nil
}

//go:build windows

package atomicfile

import (
	"errors"
	"io"
	"os"
	"path/filepath"
)

// ReadFile permits atomic replacement while this reader retains its old file.
func ReadFile(path string) ([]byte, error) {
	file, err := openReadFile(path)
	if err != nil {
		return nil, err
	}
	body, err := io.ReadAll(file)
	if closeErr := file.Close(); closeErr != nil {
		return body, errors.Join(err, closeErr)
	}
	return body, err
}

func openReadFile(path string) (*os.File, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(filepath.Dir(absolute))
	if err != nil {
		return nil, err
	}
	file, err := root.Open(filepath.Base(absolute))
	if closeErr := root.Close(); closeErr != nil {
		if file != nil {
			return nil, errors.Join(err, closeErr, file.Close())
		}
		return nil, errors.Join(err, closeErr)
	}
	// Preserve the original PathError for legacy os.IsNotExist callers.
	return file, err
}

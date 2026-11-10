package packagedigest

import (
	"errors"
	"fmt"
	"io"
)

// ErrSnapshotStorage identifies failures writing disposable snapshot output.
// It never indicates that the package input is corrupt. The filesystem cause
// remains available through errors.Is/errors.As, including an ENOENT when the
// temporary directory disappeared or is unavailable.
var ErrSnapshotStorage = errors.New("temporary package snapshot storage failed")

func snapshotStorageError(err error) error {
	return fmt.Errorf("%w: %w", ErrSnapshotStorage, err)
}

type snapshotStorageWriter struct{ io.Writer }

func (writer snapshotStorageWriter) Write(body []byte) (int, error) {
	n, err := writer.Writer.Write(body)
	if err != nil {
		return n, snapshotStorageError(err)
	}
	return n, nil
}

package packagedigest

import (
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestSnapshotCopyDistinguishesInputAndOutputFailure(t *testing.T) {
	source := filepath.Join(t.TempDir(), "source")
	if err := os.WriteFile(source, []byte("payload"), 0o600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(source)
	if err != nil {
		t.Fatal(err)
	}
	item := entry{rel: "source", sourcePath: source, kind: "file", mode: "100644", size: info.Size(), info: info}
	destination := filepath.Join(t.TempDir(), "missing", "output")
	err = copyAndHash(item, destination, sha256.New())
	var pathError *os.PathError
	if !errors.Is(err, ErrSnapshotStorage) || !errors.Is(err, os.ErrNotExist) || !errors.As(err, &pathError) {
		t.Fatalf("output failure lost its storage classification or filesystem cause: %v", err)
	}
	if err := os.Remove(source); err != nil {
		t.Fatal(err)
	}
	err = copyAndHash(item, destination, sha256.New())
	if errors.Is(err, ErrSnapshotStorage) || !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing input incorrectly identified as output storage failure: %v", err)
	}
}

type failedSnapshotWriter struct{ err error }

func (writer failedSnapshotWriter) Write([]byte) (int, error) { return 0, writer.err }

func TestSnapshotOutputWritePreservesFilesystemCause(t *testing.T) {
	cause := &os.PathError{Op: "write", Path: "snapshot/root/file", Err: os.ErrPermission}
	_, err := (snapshotStorageWriter{failedSnapshotWriter{cause}}).Write([]byte("payload"))
	var pathError *os.PathError
	if !errors.Is(err, ErrSnapshotStorage) || !errors.Is(err, os.ErrPermission) || !errors.As(err, &pathError) || pathError != cause {
		t.Fatalf("output write lost its classification or filesystem cause: %v", err)
	}
}

package environmentstore

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestStoreUnavailableTemporaryDirectoryIsOperational(t *testing.T) {
	for _, location := range []string{"outside-package", "inside-package"} {
		t.Run(location, func(t *testing.T) { unavailableSnapshotDirectory(t, location) })
	}
}

func unavailableSnapshotDirectory(t *testing.T, location string) {
	t.Helper()
	store, snapshot, pkg := strictStoreFixture(t)
	metadataPath := filepath.Join(filepath.Dir(pkg.Root), "metadata.json")
	before, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatal(err)
	}
	missingTemp := filepath.Join(t.TempDir(), "missing-temporary-directory")
	if location == "inside-package" {
		missingTemp = filepath.Join(pkg.Root, "missing-temporary-directory")
	}
	for _, key := range []string{"TMPDIR", "TMP", "TEMP"} {
		t.Setenv(key, missingTemp)
	}
	store.TempRoot = ""
	for _, operation := range []string{"get", "snapshot", "put"} {
		var err error
		switch operation {
		case "get":
			_, err = store.Get(t.Context(), snapshot.DigestAlgorithm, snapshot.TreeDigest)
		case "snapshot":
			_, err = store.Snapshot(t.Context(), snapshot.DigestAlgorithm, snapshot.TreeDigest)
		case "put":
			_, err = store.Put(t.Context(), snapshot)
		}
		var pathError *os.PathError
		if !errors.As(err, &pathError) || errors.Is(err, ErrMismatch) || errors.Is(err, ErrMissing) {
			t.Fatalf("%s classified unavailable temporary storage as cache corruption: %v", operation, err)
		}
		if got, readErr := os.ReadFile(metadataPath); readErr != nil || !bytes.Equal(before, got) {
			t.Fatalf("%s changed healthy stored evidence: %v", operation, readErr)
		}
	}
}

func TestStoreMissingPackageInputRemainsCorruption(t *testing.T) {
	store, snapshot, pkg := strictStoreFixture(t)
	if err := os.RemoveAll(pkg.Root); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get(t.Context(), snapshot.DigestAlgorithm, snapshot.TreeDigest); !errors.Is(err, ErrMismatch) {
		t.Fatalf("missing cached package input must remain corruption: %v", err)
	}
}

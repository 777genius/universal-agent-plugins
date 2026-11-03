//go:build !windows

package environmentstore

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestStoreFIFORefusalAndVerifiedRepair(t *testing.T) {
	const childKey = "UAP_STORE_FIFO_CHILD"
	if os.Getenv(childKey) != "1" {
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()
		command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestStoreFIFORefusalAndVerifiedRepair$")
		command.Env = append(os.Environ(), childKey+"=1")
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("FIFO store child failed (%v, context %v): %s", err, ctx.Err(), output)
		}
		return
	}
	store, snapshot, pkg := strictStoreFixture(t)
	path := filepath.Join(filepath.Dir(pkg.Root), "metadata.json")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := unix.Mkfifo(path, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get(t.Context(), snapshot.DigestAlgorithm, snapshot.TreeDigest); !errors.Is(err, ErrMismatch) {
		t.Fatalf("FIFO metadata accepted: %v", err)
	}
	if _, err := store.Put(t.Context(), snapshot); err != nil {
		t.Fatalf("verified repair of FIFO metadata failed: %v", err)
	}
	if _, err := store.Get(t.Context(), snapshot.DigestAlgorithm, snapshot.TreeDigest); err != nil {
		t.Fatalf("repaired entry failed: %v", err)
	}
	if leftovers, err := filepath.Glob(filepath.Join(store.Root, ".staging", "*-corrupt")); err != nil || len(leftovers) != 0 {
		t.Fatalf("repair leaked quarantine: %v, %v", leftovers, err)
	}
}

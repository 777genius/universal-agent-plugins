//go:build linux

package process

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"testing"
)

func TestReadonlySystemHashPreservesBorrowedOffsetAndOriginalMetadata(t *testing.T) {
	original := readonlyBorrowedFile(t)
	info, err := original.Stat()
	if err != nil {
		t.Fatal(err)
	}
	facts, err := readonlyFacts(info)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := original.Seek(4, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	actual, err := hashReadonlyHeld(t.Context(), original, facts)
	want := sha256.Sum256([]byte("original authority"))
	if err != nil || actual != hex.EncodeToString(want[:]) {
		t.Fatal("held original digest differs", actual, err)
	}
	if offset, err := original.Seek(0, io.SeekCurrent); err != nil || offset != 4 {
		t.Fatal("system hash consumed the caller's offset", offset, err)
	}
	if err := os.Chmod(original.Name(), 0400); err != nil {
		t.Fatal(err)
	}
	if _, err := hashReadonlyHeld(t.Context(), original, facts); err == nil {
		t.Fatal("original metadata drift was re-baselined")
	}
	requireReadonlyBorrowStillOpen(t, original)
}

func TestReadonlySystemCancellationPrecedesFilesystemAccess(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	system := &readonlySystemInputs{}
	if _, err := system.runtimeRoot(ctx, "\x00"); !errors.Is(err, context.Canceled) {
		t.Fatal("canceled runtime acquisition reached the filesystem", err)
	}
	if _, err := system.open(ctx, "\x00", false); !errors.Is(err, context.Canceled) {
		t.Fatal("canceled primitive acquisition reached the filesystem", err)
	}
	if err := system.openSelf(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal("canceled bootstrap acquisition reached the filesystem", err)
	}
	if _, err := hashReadonlyHeld(ctx, nil, readonlyObjectFacts{}); !errors.Is(err, context.Canceled) {
		t.Fatal("canceled hash touched an input descriptor", err)
	}
	if len(system.owned) != 0 {
		t.Fatal("canceled acquisition retained descriptor ownership")
	}
}

func TestReadonlySystemCloseRetainsErrorsAndOnlyClosesItsOwnedFiles(t *testing.T) {
	borrowed := readonlyBorrowedFile(t)
	first, err := os.Open(borrowed.Name())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = first.Close() })
	second, err := os.Open(borrowed.Name())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = second.Close() })
	system := &readonlySystemInputs{owned: []*os.File{first, second}}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	firstError := system.close()
	if !errors.Is(firstError, os.ErrClosed) || len(system.owned) != 0 {
		t.Fatal("system close lost its failure or retained owned files", firstError)
	}
	if _, err := second.Stat(); !errors.Is(err, os.ErrClosed) {
		t.Fatal("system close stopped before releasing the second original", err)
	}
	if later := system.close(); !errors.Is(later, firstError) {
		t.Fatal("system close retried an already released descriptor", later)
	}
	var absent *readonlySystemInputs
	if err := absent.close(); err != nil {
		t.Fatal(err)
	}
	requireReadonlyBorrowStillOpen(t, borrowed)
}

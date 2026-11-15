package nativeconfig

import (
	"bytes"
	"errors"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/atomicfile"
)

type unboundedExactTestIO struct {
	FileIO
	calls int
}

func (files *unboundedExactTestIO) ReadNoFollow(string) ([]byte, os.FileMode, bool, error) {
	files.calls++
	return nil, 0, false, errors.New("unbounded reader must not run")
}

type boundedExactTestIO struct {
	FileIO
	body   []byte
	exists bool
	calls  int
	err    error
}

func (files *boundedExactTestIO) ReadNoFollowBounded(string, int64) ([]byte, os.FileMode, bool, error) {
	files.calls++
	return files.body, 0o600, files.exists, files.err
}

func TestBoundedExactReadRequiresExplicitCustomCapability(t *testing.T) {
	files := &unboundedExactTestIO{}
	path := filepath.Join(t.TempDir(), "absent", "config.json")
	_, err := NewWithFileIO(files).ReadExactFileBounded(path, 16)
	if !errors.Is(err, ErrBoundedReadUnavailable) || files.calls != 0 {
		t.Fatalf("unsupported reader was called or replaced with disk authority: calls=%d err=%v", files.calls, err)
	}
	if _, err := os.Lstat(filepath.Dir(path)); !os.IsNotExist(err) {
		t.Fatalf("bounded read created an absent parent: %v", err)
	}
	if err := (Kernel{}).RequireBoundedRead(); err == nil || errors.Is(err, ErrBoundedReadUnavailable) {
		t.Fatalf("missing original FileIO did not retain its own refusal: %v", err)
	}
}

func TestBoundedExactReadDefaultSnapshotAndLimit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	body := []byte("exact owned config\n")
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	before, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	file, err := New().ReadExactFileBounded(path, int64(len(body)))
	if err != nil || !file.Exists || file.Mode != before.Mode().Perm() || !bytes.Equal(file.Body, body) {
		t.Fatalf("bounded exact snapshot differs: %+v %v", file, err)
	}
	if _, err := New().ReadExactFileBounded(path, int64(len(body)-1)); !errors.Is(err, atomicfile.ErrFileTooLarge) {
		t.Fatalf("oversized selected file was not refused: %v", err)
	}
	after, err := os.Lstat(path)
	if err != nil || !os.SameFile(before, after) || before.Mode() != after.Mode() ||
		before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) {
		t.Fatalf("bounded read changed the original file: %v", err)
	}
	current, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(current, body) {
		t.Fatalf("bounded read changed original bytes: %v", err)
	}
}

func TestBoundedExactReadPreservesMissingAndEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	file, err := New().ReadExactFileBounded(path, 0)
	if err != nil || file.Exists || len(file.Body) != 0 || file.Mode != 0o600 {
		t.Fatalf("missing bounded snapshot differs: %+v %v", file, err)
	}
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	file, err = New().ReadExactFileBounded(path, 0)
	if err != nil || !file.Exists || len(file.Body) != 0 {
		t.Fatalf("empty existing bounded snapshot differs: %+v %v", file, err)
	}
}

func TestBoundedExactReadChecksCustomResultAndClonesBytes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "virtual.json")
	shared := []byte("owned")
	files := &boundedExactTestIO{body: shared, exists: true}
	kernel := NewWithFileIO(files)
	file, err := kernel.ReadExactFileBounded(path, 5)
	if err != nil || !file.Exists || !bytes.Equal(file.Body, shared) || files.calls != 1 {
		t.Fatalf("explicit bounded backend was not used: %+v %v", file, err)
	}
	shared[0] = 'X'
	if string(file.Body) != "owned" {
		t.Fatal("caller snapshot aliases mutable backend bytes")
	}
	for _, limit := range []int64{-1, math.MaxInt64} {
		before := files.calls
		if _, err := kernel.ReadExactFileBounded(path, limit); err == nil || files.calls != before {
			t.Fatalf("invalid limit reached custom backend: limit=%d calls=%d err=%v", limit, files.calls, err)
		}
	}
	files.body = []byte("too large")
	if _, err := kernel.ReadExactFileBounded(path, 5); err == nil {
		t.Fatal("oversized custom result accepted")
	}
	files.body, files.exists = []byte("body"), false
	if _, err := kernel.ReadExactFileBounded(path, 5); err == nil {
		t.Fatal("absent custom result with bytes accepted")
	}
	refusal := errors.New("explicit custom refusal")
	files.err = refusal
	if _, err := kernel.ReadExactFileBounded(path, 5); !errors.Is(err, refusal) {
		t.Fatalf("custom refusal lost: %v", err)
	}
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("custom bounded reader silently created disk data: %v", err)
	}
}

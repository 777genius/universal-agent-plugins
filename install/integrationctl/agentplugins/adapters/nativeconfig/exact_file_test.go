package nativeconfig

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestExactFileRejectsConcurrentApplyAndRollback(t *testing.T) {
	for _, boundary := range []string{"apply", "restore", "remove"} {
		t.Run(boundary, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "mcp.json")
			if boundary != "remove" {
				mustWrite(t, path, "original\n")
			}
			files := &boundaryMutationIO{}
			txn, err := NewWithFileIO(files).BeginExactFile(path)
			if err != nil {
				t.Fatal(err)
			}
			defer closeExactFile(t, txn)
			foreign := func(path string) error { return os.WriteFile(path, []byte("foreign\n"), 0600) }
			if boundary == "apply" {
				files.beforeCAS = func(path string, _ []byte, _ bool, _ []byte) error { return foreign(path) }
			}
			err = txn.Apply([]byte("ours\n"))
			if boundary == "apply" {
				if !errors.Is(err, ErrConcurrentChange) {
					t.Fatalf("apply: %v", err)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				if boundary == "restore" {
					files.beforeCAS = func(path string, _ []byte, _ bool, _ []byte) error { return foreign(path) }
				}
				if boundary == "remove" {
					files.beforeRemove = func(path string, _ []byte) error { return foreign(path) }
				}
			}
			err = txn.Rollback()
			if !errors.Is(err, ErrConcurrentChange) || !strings.Contains(err.Error(), path) || txn.Effect() != FileUncertain {
				t.Fatalf("rollback: %v effect %s", err, txn.Effect())
			}
			assertBytes(t, path, "foreign\n")
		})
	}
}

func TestExactFileDoesNotClaimDesiredIdenticalForeignWrite(t *testing.T) {
	for _, existed := range []bool{false, true} {
		t.Run(map[bool]string{false: "new", true: "replacement"}[existed], func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "mcp.json")
			if existed {
				mustWrite(t, path, "original\n")
			}
			files := &boundaryMutationIO{}
			txn, err := NewWithFileIO(files).BeginExactFile(path)
			if err != nil {
				t.Fatal(err)
			}
			defer closeExactFile(t, txn)
			files.beforeCAS = func(path string, _ []byte, _ bool, _ []byte) error {
				return os.WriteFile(path, []byte("ours\n"), 0600)
			}
			if err := txn.Apply([]byte("ours\n")); !errors.Is(err, ErrConcurrentChange) {
				t.Fatalf("foreign write must reject apply: %v", err)
			}
			if err := txn.Rollback(); !errors.Is(err, ErrConcurrentChange) || txn.Effect() != FileUncertain {
				t.Fatalf("foreign write must not be rolled back: %v, effect %s", err, txn.Effect())
			}
			assertBytes(t, path, "ours\n")
		})
	}
}

func TestExactFileVisibleWriteErrorReadbackAndRestore(t *testing.T) {
	for _, existed := range []bool{false, true} {
		t.Run(map[bool]string{false: "absent", true: "present"}[existed], func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "mcp.json")
			if existed {
				mustWrite(t, path, "original\n")
				if err := os.Chmod(path, 0640); err != nil {
					t.Fatal(err)
				}
			}
			files := &faultIO{mode: "error-exact"}
			txn, err := NewWithFileIO(files).BeginExactFile(path)
			if err != nil {
				t.Fatal(err)
			}
			defer closeExactFile(t, txn)
			if err := txn.Apply([]byte("ours\n")); err == nil || txn.Effect() != FileCommitted {
				t.Fatalf("visible failure not read back: %v %s", err, txn.Effect())
			}
			assertBytes(t, path, "ours\n")
			if err := txn.Rollback(); err != nil || txn.Effect() != FileUnchanged {
				t.Fatalf("restore: %v %s", err, txn.Effect())
			}
			if existed {
				assertBytes(t, path, "original\n")
				info, err := os.Stat(path)
				if err != nil || info.Mode().Perm() != 0640 {
					t.Fatalf("restore mode: %v %v", info, err)
				}
			} else if _, err := os.Lstat(path); !os.IsNotExist(err) {
				t.Fatalf("new config not removed: %v", err)
			}
		})
	}
}

func TestExactFileDistinguishesEmptyFileFromAbsence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mcp.json")
	txn, err := New().BeginExactFile(path)
	if err != nil {
		t.Fatal(err)
	}
	defer closeExactFile(t, txn)
	mustWrite(t, path, "")
	if err := txn.Apply([]byte("ours")); !errors.Is(err, ErrConcurrentChange) || txn.Effect() != FileUncertain {
		t.Fatalf("existence drift: %v %s", err, txn.Effect())
	}
	if err := txn.Rollback(); !errors.Is(err, ErrConcurrentChange) {
		t.Fatal(err)
	}
	assertBytes(t, path, "")
}

func TestExactFileNoFollowAndCommittedLockCleanup(t *testing.T) {
	t.Run("symlink", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("symlink privilege")
		}
		root := t.TempDir()
		path, target := filepath.Join(root, "mcp.json"), filepath.Join(root, "foreign.json")
		mustWrite(t, target, "foreign")
		if err := os.Symlink(target, path); err != nil {
			t.Fatal(err)
		}
		if _, err := New().ReadExactFile(path); err == nil {
			t.Fatal("read followed symlink")
		}
		if txn, err := New().BeginExactFile(path); err == nil {
			closeExactFile(t, txn)
			t.Fatal("transaction followed symlink")
		}
		assertBytes(t, target, "foreign")
	})
	t.Run("cleanup", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "mcp.json")
		releaseErr := errors.New("release failure")
		txn, err := NewWithLockAcquirer(func(Paths, Codec) (func() error, error) { return func() error { return releaseErr }, nil }).BeginExactFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := txn.Apply([]byte("ours")); err != nil {
			t.Fatal(err)
		}
		if err := txn.Close(); !errors.Is(err, releaseErr) || txn.Effect() != FileCommitted {
			t.Fatalf("cleanup: %v", err)
		}
		assertBytes(t, path, "ours")
	})
}

func TestExactFileHoldsWriterLockThroughRollback(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mcp.json")
	mustWrite(t, path, `{"mcpServers":{}}`)
	first, err := New().BeginExactFile(path)
	if err != nil {
		t.Fatal(err)
	}
	defer closeExactFile(t, first)
	if err := first.Apply([]byte(`{"mcpServers":{"temporary":{}}}`)); err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	done := make(chan error, 1)
	kernel := NewWithLockAcquirer(func(paths Paths, codec Codec) (func() error, error) {
		close(started)
		return New().acquireCandidateLocks(paths, codec)
	})
	go func() {
		_, err := kernel.Apply(Request{Paths: Paths{JSON: path}, Codec: CodecMCPServers, Action: ActionAdd, Name: "later", Server: Server{Type: "stdio", Command: "fixture"}})
		done <- err
	}()
	<-started
	select {
	case err := <-done:
		t.Fatalf("writer escaped held lock: %v", err)
	case <-time.After(25 * time.Millisecond):
	}
	if err := first.Rollback(); err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("writer lock was not released")
	}
	body := mustRead(t, path)
	if strings.Contains(body, "temporary") || !strings.Contains(body, "later") {
		t.Fatalf("writer used pre-rollback snapshot: %s", body)
	}
}

func closeExactFile(t *testing.T, file *ExactFile) {
	t.Helper()
	if err := file.Close(); err != nil {
		t.Errorf("close exact file: %v", err)
	}
}

//go:build windows

package scaffold

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// A real separate process owns the reader pins. EOF releases them; stdout
// acknowledges acquisition before the publishing process attempts its syscall.
func TestRenamePinProcess(t *testing.T) {
	marker := -1
	for i, arg := range os.Args {
		if arg == "--rename-reader" {
			marker = i
			break
		}
	}
	if marker < 0 {
		return
	}
	var handles []windows.Handle
	for _, path := range os.Args[marker+1:] {
		name, err := windows.UTF16PtrFromString(path)
		if err != nil {
			t.Fatal(err)
		}
		h, err := windows.CreateFile(name, windows.FILE_READ_ATTRIBUTES|windows.FILE_LIST_DIRECTORY,
			windows.FILE_SHARE_READ, nil, windows.OPEN_EXISTING,
			windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
		if err != nil {
			t.Fatal(err)
		}
		handles = append(handles, h)
	}
	fmt.Println("pinned")
	_, _ = io.Copy(io.Discard, os.Stdin)
	for _, h := range handles {
		if err := windows.CloseHandle(h); err != nil {
			t.Fatal(err)
		}
	}
}

func holdRenameReader(t *testing.T, paths ...string) func() {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	args := append([]string{"-test.run=^TestRenamePinProcess$", "--", "--rename-reader"}, paths...)
	cmd := exec.CommandContext(ctx, os.Args[0], args...)
	in, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		cancel()
		t.Fatal(err)
	}
	released := false
	release := func() {
		if released {
			return
		}
		released = true
		_ = in.Close()
		err := cmd.Wait()
		cancel()
		if err != nil {
			t.Errorf("reader process: %v: %s", err, stderr.String())
		}
	}
	t.Cleanup(release)
	line, err := bufio.NewReader(out).ReadString('\n')
	if err != nil || strings.TrimSpace(line) != "pinned" {
		t.Fatalf("reader readiness %q: %v", line, err)
	}
	return release
}

func TestPackageRenameRetryWithReaderProcess(t *testing.T) {
	for _, kind := range []string{"release", "nested-reader", "collision", "cancel", "persistent", "tree", "parent", "stage", "payload"} {
		t.Run(kind, func(t *testing.T) {
			sandbox := t.TempDir()
			parent := filepath.Join(sandbox, "parent")
			if err := os.Mkdir(parent, 0700); err != nil {
				t.Fatal(err)
			}
			dest := filepath.Join(parent, "result")
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var payload, protected string
			blockedTamper := false
			waits := 0
			validate := realValidation(t)
			ops := applyOps{write: writeTree}
			ops.publish = func(ctx context.Context, from *os.File, old string, to *os.File, new string, check func() error) error {
				paths := []string{parent}
				if kind == "nested-reader" {
					// A reader rooted below another init's parent also pins that common
					// ancestor. Distinct immediate-parent lock keys would miss this overlap.
					nested := filepath.Join(parent, "nested")
					if err := os.Mkdir(nested, 0700); err != nil {
						t.Fatal(err)
					}
					paths = append(paths, nested)
				}
				release := holdRenameReader(t, paths...)
				defer release()
				r := &renameRetry{ctx: ctx, check: check, budget: time.Second, attempts: 3}
				r.wait = func(ctx context.Context, _ time.Duration) error {
					waits++ // Reached only after an actual native commit sharing failure.
					if kind == "persistent" {
						return nil
					}
					release()
					switch kind {
					case "collision":
						if err := os.Mkdir(dest, 0700); err != nil {
							t.Fatal(err)
						}
						protected = filepath.Join(dest, "peer")
					case "cancel":
						cancel()
						return ctx.Err()
					case "tree":
						if err := os.WriteFile(filepath.Join(payload, "README.md"), []byte("tampered"), 0600); err != nil {
							t.Fatal(err)
						}
					case "parent", "stage", "payload":
						path := payload
						if kind == "parent" {
							path = parent
						}
						if kind == "stage" {
							path = filepath.Dir(payload)
						}
						before, err := os.Lstat(path)
						if err != nil {
							t.Fatal(err)
						}
						if err := os.Rename(path, path+"-moved"); err != nil {
							// Windows may protect ancestors of the retained source
							// handle from rename. Prove that protection preserved the
							// original object; do not release handles to force tampering.
							if (kind != "parent" && kind != "stage") || !errors.Is(err, windows.ERROR_ACCESS_DENIED) {
								t.Fatal(err)
							}
							after, statErr := os.Lstat(path)
							if statErr != nil || !os.SameFile(before, after) {
								t.Fatalf("blocked tamper changed identity: %v", statErr)
							}
							if _, e := os.Lstat(path + "-moved"); !errors.Is(e, os.ErrNotExist) {
								t.Fatalf("blocked tamper moved source: %v", e)
							}
							blockedTamper = true
							break
						}
						if err := os.Mkdir(path, 0700); err != nil {
							t.Fatal(err)
						}
						protected = filepath.Join(path, "peer")
					}
					if protected != "" {
						if err := os.WriteFile(protected, []byte("untouched"), 0600); err != nil {
							t.Fatal(err)
						}
					}
					return nil
				}
				return renameResult(from, old, to, new, renameWindows(from, old, to, new, r))
			}
			result, err := apply(ctx, planFor(t, "skill"), ApplyOptions{Destination: dest, Validate: func(ctx context.Context, path string, root *os.Root) error {
				payload = path
				return validate(ctx, path, root)
			}}, ops)
			if kind == "release" || kind == "nested-reader" || blockedTamper {
				if err != nil || !result.Committed || waits != 1 {
					t.Fatalf("result=%+v waits=%d err=%v", result, waits, err)
				}
				if err := validate(context.Background(), dest, nil); err != nil {
					t.Fatal(err)
				}
			} else {
				if err == nil || result.Committed {
					t.Fatalf("unsafe success: %+v %v", result, err)
				}
				if kind == "persistent" {
					if waits != 2 || !errors.Is(err, windows.ERROR_SHARING_VIOLATION) {
						t.Fatalf("exhaustion waits=%d err=%v", waits, err)
					}
				} else if waits != 1 {
					t.Fatalf("waits=%d: %v", waits, err)
				}
				if kind == "collision" && !errors.Is(err, os.ErrExist) {
					t.Fatal(err)
				}
				if kind == "cancel" && !errors.Is(err, context.Canceled) {
					t.Fatal(err)
				}
				if protected != "" {
					b, e := os.ReadFile(protected)
					if e != nil || string(b) != "untouched" {
						t.Fatalf("peer changed: %q %v", b, e)
					}
				}
				if kind != "collision" {
					if _, e := os.Lstat(dest); !errors.Is(e, os.ErrNotExist) {
						t.Fatalf("destination exists: %v", e)
					}
				}
			}
			if blockedTamper || (kind != "parent" && kind != "stage" && kind != "payload") {
				entries, e := os.ReadDir(parent)
				if e != nil {
					t.Fatal(e)
				}
				for _, entry := range entries {
					if strings.HasPrefix(entry.Name(), ".authoring-") {
						t.Fatalf("stage leaked: %s", entry.Name())
					}
				}
			}
		})
	}
}

func TestRenameWaitCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := waitRename(ctx, time.Hour); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestPackageRenameRetryEligibility(t *testing.T) {
	for _, kind := range []string{"success", "source-sharing", "collision", "budget"} {
		t.Run(kind, func(t *testing.T) {
			parent := t.TempDir()
			source := filepath.Join(parent, "source")
			if err := os.Mkdir(source, 0700); err != nil {
				t.Fatal(err)
			}
			if kind == "collision" {
				if err := os.Mkdir(filepath.Join(parent, "destination"), 0700); err != nil {
					t.Fatal(err)
				}
			}
			dir, err := os.Open(parent)
			if err != nil {
				t.Fatal(err)
			}
			defer dir.Close()
			if kind == "source-sharing" {
				defer holdRenameReader(t, source)()
			}
			if kind == "budget" {
				defer holdRenameReader(t, parent)()
			}
			waits := 0
			r := &renameRetry{ctx: context.Background(), check: func() error { return nil }, budget: 10 * time.Millisecond, attempts: 32,
				wait: func(ctx context.Context, delay time.Duration) error {
					waits++
					// Exercise actual elapsed-time exhaustion. A no-op wait with
					// a nanosecond budget can fit all attempts in one Windows tick.
					return waitRename(ctx, delay+20*time.Millisecond)
				}}
			started := time.Now()
			err = renameResult(dir, "source", dir, "destination", renameWindows(dir, "source", dir, "destination", r))
			if kind == "budget" {
				if waits > 1 || time.Since(started) < r.budget {
					t.Fatalf("budget not enforced: waits=%d elapsed=%v", waits, time.Since(started))
				}
			} else if waits != 0 {
				t.Fatalf("unexpected retry: %d", waits)
			}
			switch kind {
			case "success":
				if err != nil {
					t.Fatal(err)
				}
			case "collision":
				if !errors.Is(err, os.ErrExist) {
					t.Fatal(err)
				}
			case "source-sharing":
				if !errors.Is(err, windows.ERROR_SHARING_VIOLATION) || !strings.Contains(err.Error(), "open source directory") {
					t.Fatal(err)
				}
			case "budget":
				if !errors.Is(err, windows.ERROR_SHARING_VIOLATION) || !strings.Contains(err.Error(), "commit exclusive directory") {
					t.Fatal(err)
				}
			}
		})
	}
}

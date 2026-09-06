//go:build windows

package scaffold

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"golang.org/x/sys/windows"
)

func TestWindowsRenameNativeClassification(t *testing.T) {
	for _, kind := range []string{"empty", "nonempty", "missing-source", "sharing-violation", "destination-sharing", "destination-metadata"} {
		t.Run(kind, func(t *testing.T) {
			parent := tempRoot(t)
			for _, name := range []string{"source", "dest"} {
				if kind == "sharing-violation" && name == "dest" {
					continue
				}
				if err := os.Mkdir(filepath.Join(parent, name), 0700); err != nil {
					t.Fatal(err)
				}
				if name == "source" || kind == "nonempty" {
					if err := os.WriteFile(filepath.Join(parent, name, "sentinel"), []byte(name), 0600); err != nil {
						t.Fatal(err)
					}
				}
			}
			old, want := "source", windows.STATUS_OBJECT_NAME_COLLISION
			if kind == "missing-source" {
				old, want = "missing", windows.STATUS_OBJECT_NAME_NOT_FOUND
			}
			if kind == "sharing-violation" || kind == "destination-sharing" || kind == "destination-metadata" {
				held, access := "dest", uint32(windows.FILE_READ_ATTRIBUTES|windows.FILE_LIST_DIRECTORY)
				if kind == "sharing-violation" {
					held = old
				}
				if kind == "destination-metadata" {
					access = windows.FILE_READ_ATTRIBUTES
				} else {
					want = windows.STATUS_SHARING_VIOLATION
				}
				handle := holdRenameDirectory(t, filepath.Join(parent, held), access)
				defer windows.CloseHandle(handle)
			}
			root, err := os.Open(parent)
			if err != nil {
				t.Fatal(err)
			}
			defer root.Close()
			before := skillTree(t, parent)
			// Invoke the native no-replace primitive directly; no absent() shortcut.
			err = renameExclusive(root, old, root, "dest")
			collision := want == windows.STATUS_OBJECT_NAME_COLLISION || kind == "destination-sharing"
			var link *os.LinkError
			var status windows.NTStatus
			if !errors.As(err, &link) || link.Old != old || link.New != "dest" || !errors.As(err, &status) || status != want || !errors.Is(err, want.Errno()) || errors.Is(err, os.ErrExist) != collision {
				t.Fatalf("native %s: want status=%#x exists=%v; got %v", kind, uint32(want), collision, err)
			}
			t.Logf("%s: status=%#x win32=%d exists=%v", kind, uint32(status), want.Errno(), collision)
			if !reflect.DeepEqual(before, skillTree(t, parent)) {
				t.Fatal("failed rename changed source/destination paths or contents")
			}
		})
	}
	// Old red reproducer: LinkError and cleanup joins cannot classify raw NTStatus.
	if errors.Is(errors.Join(&os.LinkError{Err: windows.STATUS_OBJECT_NAME_COLLISION}), os.ErrExist) {
		t.Fatal("pinned NTStatus classification changed; re-evaluate the boundary fix")
	}
	for _, status := range []windows.NTStatus{windows.STATUS_ACCESS_DENIED, windows.STATUS_SHARING_VIOLATION} {
		if errors.Is(errors.Join(status, status.Errno()), os.ErrExist) {
			t.Fatalf("noncollision status %#x classified as exists", uint32(status))
		}
	}
}

// FILE_LIST_DIRECTORY participates in sharing; READ_ATTRIBUTES alone does not.
// Deny WRITE and DELETE exactly as packageview's protected directory lease does.
func holdRenameDirectory(t *testing.T, path string, access uint32) windows.Handle {
	t.Helper()
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	h, err := windows.CreateFile(name, access, windows.FILE_SHARE_READ, nil, windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func TestWindowsApplyNativeCollisionCleanup(t *testing.T) {
	for _, kind := range []string{"collision", "destination-sharing", "destination-removed", "source-sharing"} {
		for _, retain := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/retain=%v", kind, retain), func(t *testing.T) {
				parent := tempRoot(t)
				dest := filepath.Join(parent, "dest")
				var container string
				var held windows.Handle
				defer func() {
					if held != 0 {
						if err := windows.CloseHandle(held); err != nil {
							t.Error(err)
						}
					}
				}()
				calls := 0
				ops := applyOps{write: writeTree, rename: func(from *os.File, old string, to *os.File, new string) error {
					calls++
					container = from.Name()
					// Winner appears after Apply's final absence check, without timing.
					if kind != "source-sharing" {
						if err := os.Mkdir(dest, 0700); err != nil {
							t.Fatal(err)
						}
					}
					if kind != "collision" {
						path := dest
						if kind == "source-sharing" {
							path = filepath.Join(from.Name(), old)
						}
						held = holdRenameDirectory(t, path, windows.FILE_READ_ATTRIBUTES|windows.FILE_LIST_DIRECTORY)
					}
					before := skillTree(t, parent)
					var err error
					if kind == "destination-removed" {
						// One real NtSetInformationFile, then remove the winner before
						// the diagnostic observation. Never retry the source rename.
						err = renameWindows(from, old, to, new)
						if !errors.Is(err, windows.STATUS_SHARING_VIOLATION) {
							t.Fatalf("raw destination sharing: %v", err)
						}
						if !reflect.DeepEqual(before, skillTree(t, parent)) {
							t.Fatal("failed native rename changed trees")
						}
						if e := windows.CloseHandle(held); e != nil {
							t.Fatal(e)
						}
						held = 0
						if e := os.Remove(dest); e != nil {
							t.Fatal(e)
						}
						err = &os.LinkError{Op: "rename-exclusive", Old: old, New: new, Err: windowsRenameError(err, to, new)}
					} else {
						err = renameExclusive(from, old, to, new)
						if !reflect.DeepEqual(before, skillTree(t, parent)) {
							t.Fatal("native collision changed staged payload or destination")
						}
					}
					if kind == "source-sharing" {
						if e := windows.CloseHandle(held); e != nil {
							t.Fatal(e)
						}
						held = 0 // Allow normal owned payload cleanup.
					}
					if retain {
						if e := os.WriteFile(filepath.Join(container, "retained"), []byte("retain"), 0600); e != nil {
							t.Fatal(e)
						}
					}
					return err
				}}
				result, err := apply(context.Background(), planFor(t, "skill"), ApplyOptions{Destination: dest, Validate: realValidation(t)}, ops)
				want := windows.STATUS_SHARING_VIOLATION
				if kind == "collision" {
					want = windows.STATUS_OBJECT_NAME_COLLISION
				}
				exists := kind == "collision" || kind == "destination-sharing"
				var cleanup *CleanupError
				var link *os.LinkError
				if calls != 1 || result.Committed || errors.Is(err, os.ErrExist) != exists || !errors.Is(err, want) || !errors.Is(err, want.Errno()) || !errors.As(err, &link) || errors.As(err, &cleanup) != retain {
					t.Fatalf("calls=%d result=%+v err=%v", calls, result, err)
				}
				names := []string{}
				if exists {
					assertOnly(t, dest)
					names = append(names, "dest")
				}
				if retain {
					names = append(names, filepath.Base(container))
					assertOnly(t, container, "retained")
					if b, e := os.ReadFile(filepath.Join(container, "retained")); e != nil || string(b) != "retain" {
						t.Fatalf("cleanup changed unrelated entry: %q %v", b, e)
					}
				}
				assertOnly(t, parent, names...)
			})
		}
	}
}

func TestWindowsRenameObservationFailureAndReparse(t *testing.T) {
	parent := tempRoot(t)
	root, err := os.Open(parent)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	cause := windows.STATUS_SHARING_VIOLATION
	if err := windowsRenameError(windows.STATUS_ACCESS_DENIED, root, "."); errors.Is(err, os.ErrExist) || !errors.Is(err, windows.STATUS_ACCESS_DENIED) {
		t.Fatalf("unrelated failure must not gain existence classification: %v", err)
	}
	closed, err := os.Open(parent)
	if err != nil {
		t.Fatal(err)
	}
	if err := closed.Close(); err != nil {
		t.Fatal(err)
	}
	if err := windowsRenameError(cause, closed, "."); errors.Is(err, os.ErrExist) || !errors.Is(err, cause) || !errors.Is(err, cause.Errno()) {
		t.Fatalf("failed native observation lost cause or claimed existence: %v", err)
	}
	// Invalid name forces the rooted metadata probe to fail, not report exists.
	if err := windowsRenameError(cause, root, "invalid\x00name"); errors.Is(err, os.ErrExist) || !errors.Is(err, cause) || !errors.Is(err, cause.Errno()) {
		t.Fatalf("failed observation lost cause or claimed existence: %v", err)
	}
	for _, target := range []string{filepath.Join(tempRoot(t), "missing"), tempRoot(t)} {
		link := filepath.Join(parent, "link")
		if err := os.Symlink(target, link); err != nil {
			t.Skipf("native symlink prerequisite: %v", err)
		}
		if err := windowsRenameError(cause, root, "link"); !errors.Is(err, os.ErrExist) || !errors.Is(err, cause) {
			t.Fatalf("reparse entry must count without following target: %v", err)
		}
		if err := os.Remove(link); err != nil {
			t.Fatal(err)
		}
	}
}

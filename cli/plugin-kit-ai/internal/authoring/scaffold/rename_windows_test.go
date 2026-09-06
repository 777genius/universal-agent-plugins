//go:build windows

package scaffold

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"golang.org/x/sys/windows"
)

func TestWindowsRenameNativeClassification(t *testing.T) {
	for _, kind := range []string{"empty", "nonempty", "missing-source", "sharing-violation"} {
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
			if kind == "sharing-violation" {
				name, err := windows.UTF16PtrFromString(filepath.Join(parent, old))
				if err != nil {
					t.Fatal(err)
				}
				handle, err := windows.CreateFile(name, windows.FILE_LIST_DIRECTORY, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
				if err != nil {
					t.Fatal(err)
				}
				defer windows.CloseHandle(handle)
				want = windows.STATUS_SHARING_VIOLATION
			}
			root, err := os.Open(parent)
			if err != nil {
				t.Fatal(err)
			}
			defer root.Close()
			before := skillTree(t, parent)
			// Invoke the native no-replace primitive directly; no absent() shortcut.
			err = renameExclusive(root, old, root, "dest")
			collision := want == windows.STATUS_OBJECT_NAME_COLLISION
			var link *os.LinkError
			var status windows.NTStatus
			if !errors.As(err, &link) || link.Old != old || link.New != "dest" || !errors.As(err, &status) || status != want || !errors.Is(err, want.Errno()) || errors.Is(err, os.ErrExist) != collision {
				t.Fatalf("native %s: want status=%#x exists=%v; got %v", kind, uint32(want), collision, err)
			}
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

func TestWindowsApplyNativeCollisionCleanup(t *testing.T) {
	for _, retain := range []bool{false, true} {
		parent := tempRoot(t)
		dest := filepath.Join(parent, "dest")
		var container string
		called := false
		ops := applyOps{write: writeTree, rename: func(from *os.File, old string, to *os.File, new string) error {
			called = true
			container = from.Name()
			// Calibrate at the actual rename seam, after Apply's final absent check.
			if err := os.Mkdir(dest, 0700); err != nil {
				t.Fatal(err)
			}
			before := skillTree(t, parent)
			err := renameExclusive(from, old, to, new)
			if !reflect.DeepEqual(before, skillTree(t, parent)) {
				t.Fatal("native collision changed staged payload or destination")
			}
			if retain {
				if e := os.WriteFile(filepath.Join(from.Name(), "retained"), []byte("retain"), 0600); e != nil {
					t.Fatal(e)
				}
			}
			return err
		}}
		result, err := apply(context.Background(), planFor(t, "skill"), ApplyOptions{Destination: dest, Validate: realValidation(t)}, ops)
		var cleanup *CleanupError
		if !called || result.Committed || !errors.Is(err, os.ErrExist) || !errors.Is(err, windows.STATUS_OBJECT_NAME_COLLISION) || errors.As(err, &cleanup) != retain {
			t.Fatalf("retain=%v called=%v result=%+v err=%v", retain, called, result, err)
		}
		assertOnly(t, dest)
		if !retain {
			assertOnly(t, parent, "dest")
		} else {
			assertOnly(t, container, "retained")
			if b, e := os.ReadFile(filepath.Join(container, "retained")); e != nil || string(b) != "retain" {
				t.Fatalf("cleanup changed unrelated container entry: %q %v", b, e)
			}
		}
	}
}

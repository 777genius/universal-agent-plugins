//go:build windows

package scaffold

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Match packageview's directory protection: first acquire metadata with all
// sharing, then self-open the held object with listing access and no DELETE
// sharing. The second operation must fail before a source walk can descend.
func skillStageReader(path string) (windows.Handle, error) {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	metadata, err := windows.CreateFile(name, windows.FILE_READ_ATTRIBUTES,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return 0, err
	}
	defer windows.CloseHandle(metadata)
	empty, err := windows.NewNTUnicodeString("")
	if err != nil {
		return 0, err
	}
	oa := windows.OBJECT_ATTRIBUTES{RootDirectory: metadata, ObjectName: empty}
	oa.Length = uint32(unsafe.Sizeof(oa))
	var held windows.Handle
	err = windows.NtCreateFile(&held, windows.FILE_READ_ATTRIBUTES|windows.FILE_LIST_DIRECTORY|windows.SYNCHRONIZE,
		&oa, &windows.IO_STATUS_BLOCK{}, nil, 0, windows.FILE_SHARE_READ, windows.FILE_OPEN,
		windows.FILE_OPEN_REPARSE_POINT|windows.FILE_OPEN_NO_RECALL|windows.FILE_SYNCHRONOUS_IO_NONALERT|windows.FILE_OPEN_FOR_BACKUP_INTENT, 0, 0)
	return held, err
}

func TestWindowsSkillStageExcludesReaderDescent(t *testing.T) {
	for _, guarded := range []bool{false, true} {
		name := "ordinary-stage-control"
		if guarded {
			name = "guarded-skill-stage"
		}
		t.Run(name, func(t *testing.T) {
			rootPath := tempRoot(t)
			root, err := os.OpenRoot(rootPath)
			if err != nil {
				t.Fatal(err)
			}
			defer root.Close()
			var parentReader windows.Handle
			if guarded {
				// A losing source gate can already hold this ancestor before
				// the winner creates its private stage.
				parentReader, err = skillStageReader(rootPath)
				if err != nil {
					t.Fatal(err)
				}
				defer func() {
					if parentReader != 0 {
						_ = windows.CloseHandle(parentReader)
					}
				}()
			}
			closeGuard := func() {}
			if guarded {
				guard, err := makePrivateSkillStage(root, "stage")
				if err != nil {
					t.Fatal(err)
				}
				closed := false
				closeGuard = func() {
					if !closed {
						closed = true
						if err := guard.Close(); err != nil {
							t.Error(err)
						}
					}
				}
				defer closeGuard()
				duplicate, err := makePrivateSkillStage(root, "stage")
				if duplicate != nil {
					defer duplicate.Close()
				}
				if err != windows.STATUS_OBJECT_NAME_COLLISION || duplicate != nil {
					t.Fatalf("exclusive creation: guard=%v err=%v", duplicate, err)
				}
			} else if err := makePrivateStage(root, "stage"); err != nil {
				t.Fatal(err)
			}
			if err := root.Mkdir("stage/payload", 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(rootPath, "stage", "payload", "bytes"), []byte("owned bytes"), 0600); err != nil {
				t.Fatalf("write beneath a pinned ancestor: %v", err)
			}
			reader, err := skillStageReader(filepath.Join(rootPath, "stage"))
			var payloadReader windows.Handle
			if guarded {
				if err != windows.STATUS_SHARING_VIOLATION {
					t.Fatalf("guard must reject directory protection before descent: %v", err)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				defer func() {
					if reader != 0 {
						_ = windows.CloseHandle(reader)
					}
				}()
				payloadReader, err = skillStageReader(filepath.Join(rootPath, "stage", "payload"))
				if err != nil {
					t.Fatal(err)
				}
				defer func() {
					if payloadReader != 0 {
						_ = windows.CloseHandle(payloadReader)
					}
				}()
			}
			from, err := root.Open("stage")
			if err != nil {
				t.Fatal(err)
			}
			defer from.Close()
			to, err := root.Open(".")
			if err != nil {
				t.Fatal(err)
			}
			defer to.Close()
			err = renameExclusive(from, "payload", to, "published")
			if guarded {
				if !errors.Is(err, windows.STATUS_SHARING_VIOLATION) || !strings.Contains(err.Error(), "commit exclusive directory rename") {
					t.Fatalf("ancestor protection must block commit after source open succeeds: %v", err)
				}
				if err := windows.CloseHandle(parentReader); err != nil {
					t.Fatal(err)
				}
				parentReader = 0
				err = renameExclusive(from, "payload", to, "published")
			} else {
				if !errors.Is(err, windows.STATUS_SHARING_VIOLATION) || !strings.Contains(err.Error(), "open source directory for exclusive rename") {
					t.Fatalf("unguarded reader must block payload publication: %v", err)
				}
				if err := windows.CloseHandle(payloadReader); err != nil {
					t.Fatal(err)
				}
				payloadReader = 0
				if err := windows.CloseHandle(reader); err != nil {
					t.Fatal(err)
				}
				reader = 0
				err = renameExclusive(from, "payload", to, "published")
			}
			if err != nil {
				t.Fatalf("publication with no payload reader: %v", err)
			}
			if err := from.Close(); err != nil {
				t.Fatal(err)
			}
			// Cleanup issues the removal while the guard is still held. Closing
			// it earlier would reopen a reader-versus-cleanup race.
			if err := root.Remove("stage"); err != nil {
				t.Fatal(err)
			}
			closeGuard()
			if _, err := root.Lstat("stage"); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("stage remains after removal and guard close: %v", err)
			}
			if err := root.Remove("published/bytes"); err != nil {
				t.Fatal(err)
			}
			if err := root.Remove("published"); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// The old filepath.WalkDir oracle opens the guarded directory without DELETE
// sharing and fails before any replacement attack. Snapshot reads must coexist
// with the live guard, retain every entry and leave reader protection unchanged.
func TestWindowsSkillTreeRetainsGuardedStage(t *testing.T) {
	parentPath := tempRoot(t)
	parent, err := os.OpenRoot(parentPath)
	if err != nil {
		t.Fatal(err)
	}
	defer parent.Close()
	guard, err := makePrivateSkillStage(parent, "stage")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := guard.Close(); err != nil {
			t.Error(err)
		}
	}()
	for _, name := range []string{"stage/empty-dir", "stage/payload"} {
		if err := parent.Mkdir(name, 0700); err != nil {
			t.Fatal(err)
		}
	}
	doc := "---\nname: fixture\ndescription: Exact snapshot bytes.\n---\n\nBody.\n"
	raw := string([]byte{0, 255, '\n'})
	files := map[string]string{
		"sibling":                 "unrelated sibling",
		"stage/empty-file":        "",
		"stage/payload/SKILL.md":  doc,
		"stage/payload/raw.bin":   raw,
		"stage/payload/read-only": "read-only bytes",
	}
	for name, body := range files {
		f, err := parent.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			t.Fatal(err)
		}
		_, writeErr := f.WriteString(body)
		if err := errors.Join(writeErr, f.Close()); err != nil {
			t.Fatal(err)
		}
	}
	if err := parent.Chmod("stage/payload/read-only", 0400); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := parent.Chmod("stage/payload/read-only", 0600); err != nil {
			t.Error(err)
		}
	}()
	target := filepath.Join("payload", "raw.bin")
	if err := parent.Symlink(target, "stage/byte-link"); err != nil {
		t.Fatal(err)
	}
	modes := map[string]os.FileMode{}
	for _, name := range []string{".", "sibling", "stage", "stage/empty-dir", "stage/empty-file",
		"stage/payload", "stage/payload/SKILL.md", "stage/payload/raw.bin", "stage/payload/read-only", "stage/byte-link"} {
		info, err := parent.Lstat(name)
		if err != nil {
			t.Fatal(err)
		}
		modes[name] = info.Mode()
	}
	if modes["stage/payload/read-only"].Perm()&0200 != 0 ||
		modes["stage/payload/raw.bin"].Perm()&0200 == 0 ||
		modes["stage/byte-link"]&os.ModeSymlink == 0 {
		t.Fatalf("fixture did not establish distinct file modes and a symlink: %v", modes)
	}
	stagePath := filepath.Join(parentPath, "stage")
	assertOrdinaryReadDenied := func() {
		t.Helper()
		_, err := os.ReadDir(stagePath)
		var native *os.PathError
		if !errors.As(err, &native) || native.Op != "open" || native.Path != stagePath ||
			native.Err != windows.ERROR_SHARING_VIOLATION {
			t.Fatalf("ordinary directory open must fail with exact sharing violation: %v", err)
		}
	}
	assertOrdinaryReadDenied()
	wantStage := map[string]string{
		".":                                   modes["stage"].String(),
		"empty-dir":                           modes["stage/empty-dir"].String(),
		"empty-file":                          modes["stage/empty-file"].String(),
		"payload":                             modes["stage/payload"].String(),
		filepath.Join("payload", "SKILL.md"):  modes["stage/payload/SKILL.md"].String() + doc,
		filepath.Join("payload", "raw.bin"):   modes["stage/payload/raw.bin"].String() + raw,
		filepath.Join("payload", "read-only"): modes["stage/payload/read-only"].String() + "read-only bytes",
		"byte-link":                           modes["stage/byte-link"].String() + target,
	}
	if got := skillTree(t, stagePath); !reflect.DeepEqual(got, wantStage) {
		t.Fatalf("guarded stage snapshot mismatch:\ngot:  %#v\nwant: %#v", got, wantStage)
	}
	wantParent := map[string]string{
		".":       modes["."].String(),
		"sibling": modes["sibling"].String() + "unrelated sibling",
	}
	for name, value := range wantStage {
		key := "stage"
		if name != "." {
			key = filepath.Join(key, name)
		}
		wantParent[key] = value
	}
	if got := skillTree(t, parentPath); !reflect.DeepEqual(got, wantParent) {
		t.Fatalf("parent snapshot omitted or altered guarded entries:\ngot:  %#v\nwant: %#v", got, wantParent)
	}
	assertOrdinaryReadDenied()
	reader, err := skillStageReader(stagePath)
	if err == nil {
		_ = windows.CloseHandle(reader)
		t.Fatal("snapshot released stage protection")
	}
	if err != windows.STATUS_SHARING_VIOLATION {
		t.Fatalf("source-reader protection refusal changed after snapshots: %v", err)
	}
}

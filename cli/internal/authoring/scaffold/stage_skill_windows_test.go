//go:build windows

package scaffold

import (
	"errors"
	"os"
	"path/filepath"
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

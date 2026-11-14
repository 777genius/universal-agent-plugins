//go:build windows && (amd64 || arm64)

package packageview

import (
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

// Mutate the selected leaf once, synchronously, between real metadata queries.
// Both roles see the same operation, with the existing native clock calibration.
func TestWindowsSharedScratchEpochBoundary(t *testing.T) {
	for _, role := range []string{"scratch", "source"} {
		for _, stage := range []string{"stat", "protect"} {
			t.Run(role+"/"+stage, func(t *testing.T) {
				before := winRecordCount()
				root := t.TempDir()
				identity, err := os.Stat(root)
				bootstrapCheck(t, "leaf identity", err)
				fired := false
				var mutationErr error
				var probe *os.File
				open := openSourceWithMetadataStage
				if role == "scratch" {
					open = openTrustedScratchWithMetadataStage
				}
				s, err := open(root, func(f *os.File, at string) {
					info, e := f.Stat()
					if fired || at != stage || e != nil || !os.SameFile(identity, info) {
						return
					}
					fired, probe = true, f
					directoryGuardMetadataAccess(t, f)
					a, e := winMeta(f)
					if e != nil {
						mutationErr = e
						return
					}
					windowsAwaitDirectoryClock(t, a)
					mutationErr = os.Mkdir(filepath.Join(root, "other-init-private"), 0700)
					b, e := winMeta(f)
					if e != nil {
						mutationErr = e
					}
					x, y := a, b
					x.Write, x.Change, y.Write, y.Change = 0, 0, 0, 0
					if x != y || a == b {
						mutationErr = errors.New("fixture did not isolate a directory epoch change")
					}
					t.Logf("role=%s stage=%s before=%+v after=%+v", role, stage, a, b)
				})
				if s != nil {
					defer s.close()
				}
				if !fired || mutationErr != nil {
					t.Fatalf("single mutation fired=%t err=%v", fired, mutationErr)
				}
				if _, e := winMeta(probe); e == nil {
					t.Fatal("metadata probe leaked")
				}
				if role == "source" {
					var safe *Error
					if s != nil || !errors.As(err, &safe) || safe.Code != "root_unreadable" {
						t.Fatalf("source epoch accepted: %v", err)
					}
				} else {
					bootstrapCheck(t, "trusted scratch leaf acquisition", err)
					if s.selectionDepth != winSelectionDepth(root[3:]) {
						t.Fatal("scratch role changed path depth")
					}
					leaf := false
					var handles []*os.File
					for _, o := range s.records {
						handles = append(handles, o.file)
						leaf = leaf || os.SameFile(identity, o.info)
						if !o.traversalOnly || !same(o.info, o.info) {
							t.Fatal("scratch observation lost traversal-only identity")
						}
						data, e := (&pinned{file: o.file, info: o.info}).reopen(true)
						if data != nil {
							data.Close()
						}
						if e == nil {
							t.Fatal("scratch authorized a data upgrade")
						}
					}
					if !leaf {
						t.Fatal("scratch leaf not retained")
					}
					p, e := s.pin("other-init-private", false)
					if p != nil {
						p.file.Close()
					}
					if e == nil {
						t.Fatal("scratch authorized source capture")
					}
					if e := nativeSharedRename(root, root+"-moved"); !errors.Is(e, windows.ERROR_SHARING_VIOLATION) {
						t.Fatalf("protected scratch allowed DELETE: %v", e)
					}
					u, e := windows.UTF16PtrFromString(root)
					bootstrapCheck(t, "scratch spelling", e)
					h, e := windows.CreateFile(u, windows.GENERIC_WRITE, winShare, nil, windows.OPEN_EXISTING,
						windows.FILE_FLAG_OPEN_REPARSE_POINT|windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
					if e == nil {
						windows.CloseHandle(h)
					}
					if !errors.Is(e, windows.ERROR_SHARING_VIOLATION) {
						t.Fatalf("protected scratch allowed reparse setter access: %v", e)
					}
					handles = append(handles, s.anchor)
					bootstrapCheck(t, "scratch close", s.close())
					if s.records != nil || s.anchor != nil {
						t.Fatal("scratch retained ownership after close")
					}
					for _, f := range handles {
						if _, e := winMeta(f); e == nil {
							t.Fatal("scratch handle leaked")
						}
					}
				}
				if winRecordCount() != before {
					t.Fatal("scratch/source metadata map leaked")
				}
				bootstrapCheck(t, "released leaf rename", nativeSharedRename(root, root+"-moved"))
				bootstrapCheck(t, "restore leaf", nativeSharedRename(root+"-moved", root))
			})
		}
	}
}

// The epoch exception also covers rename, so the protected parent-relative name
// check must reject a different directory OR a junction installed at the leaf.
func TestWindowsSharedScratchReplacementRejected(t *testing.T) {
	for _, kind := range []string{"directory", "junction", "reparse"} {
		for _, stage := range []string{"stat", "protect"} {
			t.Run(kind+"/"+stage, func(t *testing.T) {
				before := winRecordCount()
				parent := t.TempDir()
				root, spare, moved := filepath.Join(parent, "scratch"), filepath.Join(parent, "spare"), filepath.Join(parent, "moved")
				bootstrapCheck(t, "scratch fixture", os.Mkdir(root, 0700))
				if kind != "directory" {
					nativeJunction(t, spare, t.TempDir())
				} else {
					bootstrapCheck(t, "replacement fixture", os.Mkdir(spare, 0700))
				}
				var reparseData []byte
				if kind == "reparse" {
					f, e := winOpen(0, `\??\`+spare, false)
					bootstrapCheck(t, "inert junction metadata", e)
					buf := make([]byte, 16*1024)
					var n uint32
					e = windows.DeviceIoControl(windows.Handle(f.Fd()), windows.FSCTL_GET_REPARSE_POINT, nil, 0, &buf[0], uint32(len(buf)), &n, nil)
					f.Close()
					bootstrapCheck(t, "inert junction record", e)
					if n < 8 || n > uint32(len(buf)) || binary.LittleEndian.Uint32(buf) != windows.IO_REPARSE_TAG_MOUNT_POINT || int(binary.LittleEndian.Uint16(buf[4:])) != int(n)-8 {
						t.Fatal("invalid inert junction record")
					}
					reparseData = buf[8:n]
				}
				identity, err := os.Stat(root)
				bootstrapCheck(t, "scratch identity", err)
				fired := false
				var mutationErr error
				s, err := openTrustedScratchWithMetadataStage(root, func(f *os.File, at string) {
					info, e := f.Stat()
					if fired || at != stage || e != nil || !os.SameFile(identity, info) {
						return
					}
					fired = true
					if kind == "reparse" {
						setNativeReparse(t, root, windows.IO_REPARSE_TAG_MOUNT_POINT, reparseData)
						return
					}
					a, e := winMeta(f)
					bootstrapCheck(t, "old leaf metadata", e)
					mutationErr = nativeSharedRename(root, moved)
					if mutationErr == nil {
						mutationErr = nativeSharedRename(spare, root)
					}
					b, e := winMeta(f)
					bootstrapCheck(t, "renamed leaf metadata", e)
					a.Write, a.Change, b.Write, b.Change = 0, 0, 0, 0
					if a != b {
						mutationErr = errors.New("rename changed non-epoch metadata")
					}
				})
				if s != nil {
					s.close()
				}
				var safe *Error
				if !fired || mutationErr != nil || s != nil || !errors.As(err, &safe) || safe.Code != "root_unreadable" {
					t.Fatalf("replacement accepted: fired=%t mutation=%v acquisition=%v", fired, mutationErr, err)
				}
				if winRecordCount() != before {
					t.Fatal("replacement rejection leaked records")
				}
				if kind != "reparse" {
					bootstrapCheck(t, "released old leaf", nativeSharedRename(moved, spare))
				}
				bootstrapCheck(t, "released replacement", nativeSharedRename(root, moved))
			})
		}
	}
}

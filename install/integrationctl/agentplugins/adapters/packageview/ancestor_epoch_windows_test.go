//go:build windows && (amd64 || arm64)

package packageview

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// One synchronous sibling creation, selected by held object identity, occurs
// exactly between the two metadata observations. No race loop, timestamp
// write or mocked metadata. An unchanged native epoch is a fixture failure.
func TestWindowsAncestorEpochBoundary(t *testing.T) {
	for _, boundary := range []string{"outside", "root", "descendant"} {
		for _, stage := range []string{"stat", "protect"} {
			t.Run(boundary+"/"+stage, func(t *testing.T) {
				before := winRecordCount()
				parent := t.TempDir()
				nativeWrite(t, parent, "source/sub/original", "original")
				root := filepath.Join(parent, "source")
				target := map[string]string{"outside": parent, "root": root, "descendant": filepath.Join(root, "sub")}[boundary]
				identity, err := os.Stat(target)
				bootstrapCheck(t, "fixture identity", err)
				fired := false
				var mutationErr error
				hook := func(f *os.File, at string) {
					if fired || at != stage {
						return
					}
					info, err := f.Stat()
					if err != nil || !os.SameFile(identity, info) {
						return
					}
					fired = true
					a, err := winMeta(f)
					if err != nil {
						mutationErr = err
						return
					}
					windowsAwaitDirectoryClock(t, a)
					mutationErr = os.Mkdir(filepath.Join(target, "unrelated-sibling"), 0700)
					if mutationErr != nil {
						return
					}
					b, err := winMeta(f)
					if err != nil {
						mutationErr = err
						return
					}
					// Compare every non-epoch field without depending on the fix helper.
					x, y := a, b
					x.Write, x.Change, y.Write, y.Change = 0, 0, 0, 0
					if x != y || a == b {
						mutationErr = errors.New("native fixture did not isolate a same-object directory epoch change")
					}
					t.Logf("stage=%s boundary=%s before=%+v after=%+v", stage, boundary, a, b)
				}
				s, err := openSourceWithMetadataStage(root, hook)
				if s != nil {
					defer s.close()
				}
				if boundary == "descendant" {
					bootstrapCheck(t, "source acquisition", err)
					p, e := s.pin("sub", false)
					if p != nil {
						p.file.Close()
					}
					err = e
				}
				if !fired || mutationErr != nil {
					t.Fatalf("mutation fired=%t error=%v", fired, mutationErr)
				}
				if boundary == "outside" {
					bootstrapCheck(t, "outside-root sibling must not invalidate acquisition", err)
					// All retained records still participate in global verification.
					for _, o := range s.records {
						if os.SameFile(identity, o.info) {
							data, e := (&pinned{file: o.file, info: o.info}).reopen(true)
							if data != nil {
								data.Close()
							}
							if e == nil {
								t.Fatal("outside identity pin authorized a data upgrade")
							}
						}
						if !same(o.info, o.info) {
							t.Fatal("retained record failed verification")
						}
					}
					p, err := s.pin("sub/original", false)
					bootstrapCheck(t, "contained pin", err)
					data, err := p.reopen(false)
					p.file.Close()
					bootstrapCheck(t, "contained same-object data upgrade", err)
					data.Close()
					if _, err := s.pin("../unrelated-sibling", false); err == nil {
						t.Fatal("ancestry escaped source")
					}
					if err := nativeSharedRename(parent, parent+"-moved"); !errors.Is(err, windows.ERROR_SHARING_VIOLATION) {
						t.Fatalf("outside ancestor lost delete protection: %v", err)
					}
				} else {
					var safe *Error
					want := "source_changed"
					if boundary == "root" {
						want = "root_unreadable"
					}
					if !errors.As(err, &safe) || safe.Code != want {
						t.Fatalf("inside mutation accepted: %v", err)
					}
				}
				if s != nil {
					bootstrapCheck(t, "source close", s.close())
				}
				if got := winRecordCount(); got != before {
					t.Fatalf("record leak: before=%d after=%d", before, got)
				}
			})
		}
	}
}

// Trailing '.' and a real descent followed by '..' must retain strict epochs
// for the actual root from its first acquisition, not just its final spelling.
func TestWindowsAncestorEpochOrderedRoot(t *testing.T) {
	for _, suffix := range []string{`\.`, `\sub\..`} {
		t.Run(suffix, func(t *testing.T) {
			root := t.TempDir()
			nativeWrite(t, root, "sub/original", "original")
			identity, err := os.Stat(root)
			bootstrapCheck(t, "root identity", err)
			fired := false
			var mutationErr error
			s, err := openSourceWithMetadataStage(root+suffix, func(f *os.File, stage string) {
				info, e := f.Stat()
				if fired || stage != "protect" || e != nil || !os.SameFile(identity, info) {
					return
				}
				fired = true
				before, e := winMeta(f)
				bootstrapCheck(t, "ordered root pre-mutation", e)
				windowsAwaitDirectoryClock(t, before)
				mutationErr = os.Mkdir(filepath.Join(root, "late"), 0700)
				after, e := winMeta(f)
				bootstrapCheck(t, "ordered root post-mutation", e)
				x, y := before, after
				x.Write, x.Change, y.Write, y.Change = 0, 0, 0, 0
				if x != y || before == after {
					t.Fatal("ordered root fixture did not isolate directory epoch mutation")
				}
			})
			if s != nil {
				s.close()
			}
			if !fired || mutationErr != nil || err == nil {
				t.Fatalf("ordered root epoch weakened: fired=%t mutation=%v acquisition=%v", fired, mutationErr, err)
			}
		})
	}
}

func TestWindowsAncestorEpochMetadataGuards(t *testing.T) {
	a := winSnapshot{Volume: 1, High: 2, Low: 3, Links: 1, Attributes: windows.FILE_ATTRIBUTE_DIRECTORY, Size: 4096, Creation: 4, Write: 5, Change: 6}
	for _, mutate := range []func(*winSnapshot){
		func(b *winSnapshot) { b.Volume++ }, func(b *winSnapshot) { b.High++ },
		func(b *winSnapshot) { b.Low++ }, func(b *winSnapshot) { b.Links++ },
		func(b *winSnapshot) { b.Attributes |= windows.FILE_ATTRIBUTE_REPARSE_POINT },
		func(b *winSnapshot) { b.Attributes |= windows.FILE_ATTRIBUTE_OFFLINE },
		func(b *winSnapshot) { b.Attributes |= windows.FILE_ATTRIBUTE_RECALL_ON_OPEN },
		func(b *winSnapshot) { b.Size++ }, func(b *winSnapshot) { b.Creation++ },
	} {
		b := a
		mutate(&b)
		if winUnchanged(a, b, true) {
			t.Fatalf("ancestor metadata guard weakened: %+v", b)
		}
	}
	b := a
	b.Write++
	b.Change++
	if !winUnchanged(a, b, true) || winUnchanged(a, b, false) {
		t.Fatal("directory role lost epoch boundary")
	}
	a.Attributes, b.Attributes = 0, 0
	if winUnchanged(a, b, true) {
		t.Fatal("regular file gained ancestor exception")
	}
}

// A rename changes ChangeTime too. The exception must not permit a path ancestor
// to move between its metadata probe and delete-protected HANDLE upgrade.
func TestWindowsAncestorEpochReplacementRejected(t *testing.T) {
	parent := t.TempDir()
	nativeWrite(t, parent, "ancestor/source/original", "original")
	ancestor := filepath.Join(parent, "ancestor")
	identity, err := os.Stat(ancestor)
	bootstrapCheck(t, "ancestor identity", err)
	fired := false
	var mutationErr error
	s, err := openSourceWithMetadataStage(filepath.Join(ancestor, "source"), func(f *os.File, stage string) {
		info, e := f.Stat()
		if fired || stage != "protect" || e != nil || !os.SameFile(identity, info) {
			return
		}
		fired = true
		a, e := winMeta(f)
		if e != nil {
			mutationErr = e
			return
		}
		mutationErr = nativeSharedRename(ancestor, filepath.Join(parent, "moved"))
		if mutationErr == nil {
			mutationErr = os.MkdirAll(filepath.Join(ancestor, "source"), 0700)
		}
		b, e := winMeta(f)
		if e != nil {
			mutationErr = e
			return
		}
		a.Write, a.Change, b.Write, b.Change = 0, 0, 0, 0
		if a != b {
			mutationErr = errors.New("rename fixture changed non-epoch metadata")
		}
	})
	if s != nil {
		s.close()
	}
	var safe *Error
	if !fired || mutationErr != nil || !errors.As(err, &safe) || safe.Code != "root_unreadable" {
		t.Fatalf("ancestor replacement accepted: fired=%t mutation=%v acquisition=%v", fired, mutationErr, err)
	}
}

// NTFS can stamp two consecutive directory mutations with the same kernel clock
// tick. Wait for an observed coarse-clock advance before the SINGLE mutation;
// never retry acquisition/mutation or modify source timestamps. A stopped clock
// fails the fixture after a bounded deadline, and each caller still proves the
// actual filesystem epoch changed without any other metadata changing.
func windowsAwaitDirectoryClock(t *testing.T, snapshot winSnapshot) {
	t.Helper()
	clock := func() int64 {
		var now windows.Filetime
		windows.GetSystemTimeAsFileTime(&now)
		return int64(uint64(now.HighDateTime)<<32 | uint64(now.LowDateTime))
	}
	threshold := max(snapshot.Write, snapshot.Change, clock())
	deadline := time.Now().Add(2 * time.Second)
	for clock() <= threshold {
		if time.Now().After(deadline) {
			t.Fatal("native coarse clock did not advance for directory mutation")
		}
		time.Sleep(time.Millisecond)
	}
}

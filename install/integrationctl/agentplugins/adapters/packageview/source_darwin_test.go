//go:build darwin && arm64

package packageview

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// Every volume is newly created inside t.TempDir, populated while writable,
// then detached and mounted read-only. No environment-provided mount/project is
// accepted. hdiutil is fixture tooling, never production reader behavior.
func nativeFixture(t *testing.T, build func(string)) string {
	t.Helper()
	tmp := t.TempDir()
	mount := filepath.Join(tmp, "mount")
	dmg := filepath.Join(tmp, "fixture.sparseimage")
	if e := os.Mkdir(mount, 0700); e != nil {
		t.Fatal(e)
	}
	run := func(args ...string) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		b, e := exec.CommandContext(ctx, "/usr/bin/hdiutil", args...).CombinedOutput()
		if e != nil {
			t.Fatalf("UNPROVEN APFS fixture prerequisite: hdiutil %v: %v\n%s", args, e, b)
		}
	}
	run("create", "-size", "512m", "-fs", "APFS", "-volname", "packageview-disposable", "-type", "SPARSE", dmg)
	attached := false
	t.Cleanup(func() {
		if attached {
			run("detach", mount)
		}
	})
	attached = true
	run("attach", "-nobrowse", "-noautoopen", "-mountpoint", mount, dmg)
	root := filepath.Join(mount, "source")
	if e := os.Mkdir(root, 0700); e != nil {
		t.Fatal(e)
	}
	build(root)
	run("detach", mount)
	attached = false
	attached = true
	run("attach", "-readonly", "-nobrowse", "-noautoopen", "-mountpoint", mount, dmg)
	return root
}
func TestDarwinWritableProfileAdmission(t *testing.T) {
	root, scratch := writableFixture(t)
	l, e := (Reader{TempDir: scratch}).Open(context.Background(), root)
	if e != nil {
		t.Fatalf("UNPROVEN writable admission: %v", e)
	}
	defer l.Close()
	if _, e = l.Capture(context.Background()); e != nil {
		t.Fatal(e)
	}
}
func TestDarwinSpecialMetadataNeverOpened(t *testing.T) {
	root := nativeFixture(t, func(root string) {
		nativeWrite(t, root, "plugin.json", "core")
		if e := unix.Mkfifo(filepath.Join(root, "fifo"), 0600); e != nil {
			t.Fatal(e)
		}
	})
	s := nativeSource(t, root)
	p, e := s.pin("fifo", true)
	if e != nil {
		t.Fatal(e)
	}
	defer p.file.Close()
	if p.info.Mode()&os.ModeNamedPipe == 0 {
		t.Fatal("FIFO type lost")
	}
	if f, e := p.reopen(false); e == nil {
		f.Close()
		t.Fatal("FIFO data-opened")
	}
}
func TestDarwinDeviceMetadataNeverOpened(t *testing.T) {
	for _, kind := range []uint32{unix.S_IFCHR, unix.S_IFBLK} {
		t.Run(map[uint32]string{unix.S_IFCHR: "character", unix.S_IFBLK: "block"}[kind], func(t *testing.T) {
			root := nativeFixture(t, func(root string) {
				nativeWrite(t, root, "plugin.json", "core")
				// Deliberately unassigned device number in a NEW fixture, never /dev data.
				e := unix.Mknod(filepath.Join(root, "device"), kind|0600, int(unix.Mkdev(255, 255)))
				if errors.Is(e, syscall.EPERM) || errors.Is(e, syscall.EACCES) {
					t.Skipf("UNPROVEN mknod permission gate: %v", e)
				}
				if e != nil {
					t.Fatal(e)
				}
			})
			s := nativeSource(t, root)
			p, e := s.pin("device", true)
			if e != nil {
				t.Fatal(e)
			}
			defer p.file.Close()
			if p.info.Mode()&os.ModeDevice == 0 {
				t.Fatal("device type lost")
			}
			if f, e := p.reopen(false); e == nil {
				f.Close()
				t.Fatal("device data-opened")
			}
		})
	}
}
func TestDarwinReplacementDeniedByProfile(t *testing.T) {
	root := nativeFixture(t, func(root string) { nativeWrite(t, root, "plugin.json", "core") })
	scratch := t.TempDir()
	attempted := false
	l, e := (Reader{TempDir: scratch}).open(context.Background(), root, &captureHooks{afterNameCheck: func(p string) {
		if p != "plugin.json" {
			return
		}
		attempted = true
		e := os.Rename(filepath.Join(root, p), filepath.Join(root, "old"))
		if !errors.Is(e, syscall.EROFS) {
			t.Fatalf("profile allowed replacement: %v", e)
		}
		e = unix.Mkfifo(filepath.Join(root, p), 0600)
		if !errors.Is(e, syscall.EROFS) && !errors.Is(e, syscall.EEXIST) {
			t.Fatalf("unexpected FIFO substitution result: %v", e)
		}
	}})
	if e != nil {
		t.Fatal(e)
	}
	defer l.Close()
	if !attempted || l.Data().Plugin.State != Present {
		t.Fatal("replacement gate did not run")
	}
	if _, e = l.Capture(context.Background()); e != nil {
		t.Fatal(e)
	}
}

func TestDarwinHandleCleanupAndTypeChecks(t *testing.T) {
	root := nativeFixture(t, func(root string) {
		nativeWrite(t, root, "plugin.json", "core")
		if e := unix.Mkfifo(filepath.Join(root, "fifo"), 0600); e != nil {
			t.Fatal(e)
		}
	})
	// Enumerate descriptor names only; no device node is data-opened.
	count := func() int {
		t.Helper()
		entries, e := os.ReadDir("/dev/fd")
		if e != nil {
			t.Fatal(e)
		}
		return len(entries)
	}
	before := count()
	for i := 0; i < 10; i++ {
		s, e := openSource(root, GeneratedStaging{})
		if e != nil {
			t.Fatal(e)
		}
		for _, name := range []string{".", "plugin.json", "fifo"} {
			p, e := s.pin(name, true)
			if e != nil {
				s.close()
				t.Fatal(e)
			}
			// Every mismatch is rejected before opening any target data.
			if f, e := p.reopen(!p.info.IsDir()); e == nil {
				f.Close()
				t.Fatal("wrong-kind reopen succeeded")
			}
			p.file.Close()
		}
		if e := s.close(); e != nil {
			t.Fatal(e)
		}
		if e := s.close(); e != nil {
			t.Fatal(e)
		}
	}
	if after := count(); after != before {
		t.Fatalf("descriptor leak: before=%d after=%d", before, after)
	}
}

func TestDarwinReplacementAtBothReadBoundaries(t *testing.T) {
	root := nativeFixture(t, func(root string) { nativeWrite(t, root, "plugin.json", "core") })
	for _, after := range []bool{false, true} {
		t.Run(map[bool]string{false: "before", true: "after"}[after], func(t *testing.T) {
			attempted := false
			swap := func(name string) {
				if name != "plugin.json" || attempted {
					return
				}
				attempted = true
				if e := os.Rename(filepath.Join(root, name), filepath.Join(root, "old")); !errors.Is(e, syscall.EROFS) {
					t.Fatalf("read-only profile allowed replacement: %v", e)
				}
				if e := os.WriteFile(filepath.Join(root, name), []byte("mutation"), 0600); !errors.Is(e, syscall.EROFS) {
					t.Fatalf("read-only profile allowed mutation: %v", e)
				}
			}
			h := &captureHooks{beforeDataOpen: swap}
			if after {
				h = &captureHooks{afterNameCheck: swap}
			}
			l, e := (Reader{TempDir: t.TempDir()}).open(context.Background(), root, h)
			if e != nil {
				t.Fatal(e)
			}
			defer l.Close()
			if !attempted || string(l.Data().Plugin.Bytes) != "core" {
				t.Fatal("read boundary gate did not preserve original object")
			}
		})
	}
}

func TestDarwinRootSelectionTraversalOrder(t *testing.T) {
	root := nativeFixture(t, func(root string) {
		nativeWrite(t, root, "dir/nested/keep", "data")
		nativeLink(t, root, "dir/nested", "a")
	})
	// Root selection follows ancestors in filesystem order. Cleaning a/.. would
	// choose root itself; kernel traversal must choose root/dir instead.
	s := nativeSource(t, root+"/a/..")
	got, e := s.anchor.Stat()
	if e != nil {
		t.Fatal(e)
	}
	want, e := os.Stat(filepath.Join(root, "dir"))
	if e != nil || !os.SameFile(got, want) {
		t.Fatal("root selection was lexically cleaned", e)
	}
}

// A matching GeneratedStaging proof is the only way an ordinary writable
// local APFS directory is ever accepted; TestDarwinWritableProfileRejected
// above proves the zero-value (untrusted) path still rejects it.
func TestDarwinGeneratedStagingProofAcceptsOwnWritableRoot(t *testing.T) {
	root := t.TempDir()
	nativeWrite(t, root, "plugin.json", "core")
	dir, e := os.OpenRoot(root)
	if e != nil {
		t.Fatal(e)
	}
	defer dir.Close()
	proof, e := NewGeneratedStaging(dir)
	if e != nil {
		t.Fatal(e)
	}
	s, e := openSource(root, proof)
	if e != nil {
		t.Fatal("trusted generated-staging proof rejected on its own writable root:", e)
	}
	defer s.close()
	if !s.trustedWritable {
		t.Fatal("trusted flag not set from a matching proof")
	}
	// The relaxation must still require local APFS and every other check:
	// this exercises the same darwinFS/pin/reopen path the untrusted case uses.
	p, e := s.pin("plugin.json", true)
	if e != nil {
		t.Fatal(e)
	}
	defer p.file.Close()
	f, e := p.reopen(false)
	if e != nil {
		t.Fatal("trusted data reopen failed:", e)
	}
	defer f.Close()
}

// A GeneratedStaging proof built from one directory must never authorize a
// different one: the caller could otherwise mint a proof against a trivially
// creatable writable directory and pass an unrelated path to openSource.
func TestDarwinGeneratedStagingProofMismatchFailsClosed(t *testing.T) {
	a := filepath.Join(t.TempDir(), "a")
	b := filepath.Join(t.TempDir(), "b")
	for _, d := range []string{a, b} {
		if e := os.Mkdir(d, 0700); e != nil {
			t.Fatal(e)
		}
	}
	dirA, e := os.OpenRoot(a)
	if e != nil {
		t.Fatal(e)
	}
	defer dirA.Close()
	proof, e := NewGeneratedStaging(dirA)
	if e != nil {
		t.Fatal(e)
	}
	s, e := openSource(b, proof)
	if e == nil {
		s.close()
		t.Fatal("mismatched generated-staging proof accepted a different directory")
	}
	var safe *Error
	if !errors.As(e, &safe) || safe.Code != "generated_staging_mismatch" {
		t.Fatal(e)
	}
}
func nativeLinkPrivilegeError(e error) bool { return os.IsPermission(e) }

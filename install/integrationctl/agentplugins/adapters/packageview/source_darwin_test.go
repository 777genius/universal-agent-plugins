//go:build darwin && arm64

package packageview

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// Every volume is newly created inside t.TempDir, populated while writable,
// then detached and mounted read-only. No environment-provided mount/project is
// accepted. hdiutil is fixture tooling, never production reader behavior.
func readOnlyFixture(t *testing.T, build func(string)) string {
	t.Helper()
	tmp, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
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
	t.Cleanup(func() {
		if e := detachFixture(dmg, mount); e != nil {
			t.Errorf("UNPROVEN APFS fixture cleanup: %v", e)
		}
	})
	run("attach", "-nobrowse", "-noautoopen", "-mountpoint", mount, dmg)
	root := filepath.Join(mount, "source")
	if e := os.Mkdir(root, 0700); e != nil {
		t.Fatal(e)
	}
	build(root)
	if e := detachFixture(dmg, mount); e != nil {
		t.Fatalf("UNPROVEN APFS fixture prerequisite: %v", e)
	}
	run("attach", "-readonly", "-nobrowse", "-noautoopen", "-mountpoint", mount, dmg)
	return root
}

// Only detach the exact new image at its expected mount. A failed command may
// already have detached it; querying state also makes cleanup idempotent.
func detachFixture(image, mount string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	mounted := func() (bool, error) {
		plist, e := exec.CommandContext(ctx, "/usr/bin/hdiutil", "info", "-plist").CombinedOutput()
		if e != nil {
			return false, fmt.Errorf("hdiutil info: %w\n%s", e, plist)
		}
		convert := exec.CommandContext(ctx, "/usr/bin/plutil", "-convert", "json", "-o", "-", "-")
		convert.Stdin = bytes.NewReader(plist)
		data, e := convert.CombinedOutput()
		if e != nil {
			return false, fmt.Errorf("plutil mount state: %w\n%s", e, data)
		}
		var info struct {
			Images []struct {
				Path     string `json:"image-path"`
				Entities []struct {
					Mount string `json:"mount-point"`
				} `json:"system-entities"`
			} `json:"images"`
		}
		if e := json.Unmarshal(data, &info); e != nil {
			return false, fmt.Errorf("decode mount state: %w", e)
		}
		if info.Images == nil {
			return false, fmt.Errorf("unproven hdiutil image inventory: %s", data)
		}
		ownImage, ownMount := false, false
		for _, attached := range info.Images {
			own := filepath.Clean(attached.Path) == image
			ownImage = ownImage || own
			for _, entity := range attached.Entities {
				if filepath.Clean(entity.Mount) == mount {
					if !own {
						return false, fmt.Errorf("refusing detach: %s belongs to another image", mount)
					}
					ownMount = true
				}
			}
		}
		var fs unix.Statfs_t
		if e := unix.Statfs(mount, &fs); e != nil {
			return false, fmt.Errorf("stat fixture mount: %w", e)
		}
		isMount := unix.ByteSliceToString(fs.Mntonname[:]) == mount
		if ownImage != ownMount || ownMount != isMount {
			return false, fmt.Errorf("unproven fixture mount state for %s: image=%t mount=%t filesystem=%t", image, ownImage, ownMount, isMount)
		}
		return ownMount, nil
	}
	var diagnostics strings.Builder
	for attempt := 1; attempt <= 4; attempt++ {
		attached, e := mounted()
		if e != nil {
			return fmt.Errorf("%sverify fixture before detach: %w", diagnostics.String(), e)
		}
		if !attached {
			return nil
		}
		out, detachErr := exec.CommandContext(ctx, "/usr/bin/hdiutil", "detach", mount).CombinedOutput()
		fmt.Fprintf(&diagnostics, "detach attempt %d: %v\n%s\n", attempt, detachErr, out)
		attached, e = mounted()
		if e != nil {
			return fmt.Errorf("%sverify fixture after detach: %w", diagnostics.String(), e)
		}
		if !attached {
			return nil
		}
		if detachErr == nil || !strings.Contains(strings.ToLower(string(out)), "resource busy") || attempt == 4 {
			return fmt.Errorf("%sfixture still attached at %s", diagnostics.String(), mount)
		}
		timer := time.NewTimer(time.Duration(attempt) * 250 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return fmt.Errorf("%sdetach retry: %w", diagnostics.String(), ctx.Err())
		case <-timer.C:
		}
	}
	return fmt.Errorf("%sdetach attempts exhausted", diagnostics.String())
}

// Ordinary sources are new writable local APFS directories, never user projects.
func nativeFixture(t *testing.T, build func(string)) string {
	t.Helper()
	root := t.TempDir()
	build(root)
	return root
}
func TestDarwinWritableProfileAccepted(t *testing.T) {
	root := nativeFixture(t, func(root string) { nativeWrite(t, root, "plugin.json", "core") })
	l, e := (Reader{TempDir: t.TempDir()}).Open(context.Background(), root)
	if e != nil {
		t.Fatal(e)
	}
	defer l.Close()
	if string(l.Data().Plugin.Bytes) != "core" {
		t.Fatal("writable source not captured")
	}
	if _, e := l.Capture(context.Background()); e != nil {
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
	root := readOnlyFixture(t, func(root string) { nativeWrite(t, root, "plugin.json", "core") })
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
	// Check saved descriptor numbers immediately after close, before reuse.
	assertClosed := func(fd uintptr) {
		t.Helper()
		if _, e := unix.FcntlInt(fd, unix.F_GETFD, 0); !errors.Is(e, unix.EBADF) {
			t.Fatalf("descriptor %d: expected EBADF after close, got %v", fd, e)
		}
	}
	for i := 0; i < 10; i++ {
		s, e := openSource(root, GeneratedStaging{})
		if e != nil {
			t.Fatal(e)
		}
		sourceFD := s.anchor.Fd()
		for _, name := range []string{".", "plugin.json", "fifo"} {
			p, e := s.pin(name, true)
			if e != nil {
				s.close()
				t.Fatal(e)
			}
			pinFD := p.file.Fd()
			// Every mismatch is rejected before opening any target data.
			if f, e := p.reopen(!p.info.IsDir()); e == nil {
				f.Close()
				t.Fatal("wrong-kind reopen succeeded")
			}
			if e := p.file.Close(); e != nil {
				t.Fatal(e)
			}
			assertClosed(pinFD)
		}
		if e := s.close(); e != nil {
			t.Fatal(e)
		}
		assertClosed(sourceFD)
		if e := s.close(); e != nil {
			t.Fatal(e)
		}
		assertClosed(sourceFD)
	}
}

func TestDarwinReplacementAtBothReadBoundaries(t *testing.T) {
	root := readOnlyFixture(t, func(root string) { nativeWrite(t, root, "plugin.json", "core") })
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

// A matching proof remains accepted and an unmatched proof still fails closed.
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

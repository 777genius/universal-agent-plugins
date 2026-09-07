//go:build darwin && arm64

package packageview

import (
	"context"
	"errors"
	"golang.org/x/sys/unix"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// No mounting, cloud, device creation or real-project inputs. Unsupported
// fixture storage is a failed prerequisite, never a native success/skip.
func writableFixture(t *testing.T) (string, string) {
	t.Helper()
	base, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	root, scratch := filepath.Join(base, "parent", "source"), filepath.Join(base, "scratch")
	for _, p := range []string{root, scratch} {
		if e := os.MkdirAll(p, 0700); e != nil {
			t.Fatal(e)
		}
	}
	var fs unix.Statfs_t
	if e := unix.Statfs(root, &fs); e != nil || unix.ByteSliceToString(fs.Fstypename[:]) != "apfs" || fs.Flags&unix.MNT_RDONLY != 0 || fs.Flags&unix.MNT_LOCAL == 0 {
		t.Fatalf("UNPROVEN writable local APFS prerequisite: %+v %v", fs, e)
	}
	t.Logf("writable APFS fsid=%v flags=%x profile=%s", fs.Fsid, fs.Flags, ReadProfile)
	nativeWrite(t, root, "plugin.json", `{"name":"fixture"}`)
	return root, scratch
}
func requireChanged(t *testing.T, e error) {
	t.Helper()
	var safe *Error
	if !errors.As(e, &safe) || safe.Code != "source_changed" {
		t.Fatalf("want source_changed: %v", e)
	}
}
func TestDarwinWritableCapture(t *testing.T) {
	root, scratch := writableFixture(t)
	nativeWrite(t, root, "mcp.json", `{}`)
	nativeWrite(t, root, "skills/a/SKILL.md", "# inert")
	nativeWrite(t, root, "dir/opaque", "payload")
	nativeLink(t, root, "dir/../dir/opaque", "alias")
	l, e := (Reader{TempDir: scratch}).Open(context.Background(), root)
	if e != nil {
		t.Fatal(e)
	}
	if l.Data().Coverage.ComponentsRequested {
		t.Fatal("core authorized components")
	}
	in, e := l.Capture(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	if !in.Coverage.TreeComplete || in.Identity.TreeDigest == "" || ReadProfile != "packageview-local-darwin-v2" {
		t.Fatalf("incomplete capture: %+v", in)
	}
	if e = l.Close(); e != nil {
		t.Fatal(e)
	}
	if e = l.Close(); e != nil {
		t.Fatal(e)
	}
	entries, e := os.ReadDir(scratch)
	if e != nil || len(entries) != 0 {
		t.Fatal("cleanup", entries, e)
	}
}
func TestDarwinWritableStaticOpenBoundary(t *testing.T) {
	root, scratch := writableFixture(t)
	nativeWrite(t, root, "plugin/plugin.yaml", "excluded")
	nativeLink(t, root, "plugin/plugin.yaml", "legacy-alias")
	nativeLink(t, root, "/outside", "absolute")
	nativeLink(t, root, "../outside", "escape")
	nativeLink(t, root, "cycle", "cycle")
	nativeWrite(t, root, "hard", "excluded hardlinks")
	if e := os.Link(filepath.Join(root, "hard"), filepath.Join(root, "hard-alias")); e != nil {
		t.Fatal(e)
	}
	if e := unix.Mkfifo(filepath.Join(root, "fifo"), 0600); e != nil {
		t.Fatal(e)
	}
	fd, e := unix.Socket(unix.AF_UNIX, unix.SOCK_STREAM, 0)
	if e != nil {
		t.Fatal(e)
	}
	defer unix.Close(fd)
	if e := unix.Bind(fd, &unix.SockaddrUnix{Name: filepath.Join(root, "socket")}); e != nil {
		t.Fatal(e)
	}
	opens := map[string]int{}
	l, e := (Reader{TempDir: scratch}).open(context.Background(), root, &captureHooks{nativeOpen: func(n string, flags int) {
		if flags&unix.O_DIRECTORY == 0 {
			opens[n]++
		}
	}})
	if e != nil {
		t.Fatal(e)
	}
	defer l.Close()
	in, e := l.Capture(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	if in.Coverage.TreeComplete || opens["plugin.json"] == 0 {
		t.Fatal("open observer not calibrated", opens)
	}
	for _, n := range []string{"plugin.yaml", "legacy-alias", "absolute", "escape", "cycle", "hard", "hard-alias", "fifo", "socket"} {
		if opens[n] != 0 {
			t.Fatalf("forbidden native openat boundary: %s", n)
		}
	}
}
func TestDarwinWritableObservedChanges(t *testing.T) {
	for _, where := range []string{"before-open", "after-name", "native-open", "after-chunk", "gap-core", "gap-legacy", "gap-link", "gap-directory", "gap-root", "gap-ancestor"} {
		t.Run(where, func(t *testing.T) {
			root, scratch := writableFixture(t)
			nativeWrite(t, root, "plugin/plugin.yaml", "legacy")
			nativeWrite(t, root, "dir/payload", strings.Repeat("x", 65536))
			nativeLink(t, root, "dir/payload", "alias")
			mutated := false
			var mutationErr error
			mutate := func(n string) {
				if n != "plugin.json" || mutated {
					return
				}
				mutated = true
				// Harmless regular substitution even after the final metadata check.
				mutationErr = os.Rename(filepath.Join(root, n), filepath.Join(root, "old-core"))
				if mutationErr == nil {
					mutationErr = os.WriteFile(filepath.Join(root, n), []byte(`{"name":"changed"}`), 0600)
				}
			}
			hooks := &captureHooks{}
			switch where {
			case "before-open":
				hooks.beforeDataOpen = mutate
			case "after-name":
				hooks.afterNameCheck = mutate
			case "native-open":
				hooks.nativeOpen = func(n string, flags int) {
					if flags&unix.O_DIRECTORY == 0 {
						mutate(n)
					}
				}
			case "after-chunk":
				hooks.afterChunk = mutate
			}
			l, e := (Reader{TempDir: scratch}).open(context.Background(), root, hooks)
			if !strings.HasPrefix(where, "gap-") {
				if !mutated || mutationErr != nil {
					t.Fatal("mutation prerequisite", mutated, mutationErr)
				}
				requireChanged(t, e)
				if l != nil {
					l.Close()
					t.Fatal("partial lease")
				}
				return
			}
			if e != nil {
				t.Fatal(e)
			}
			switch where {
			case "gap-core":
				mutationErr = os.Chmod(filepath.Join(root, "plugin.json"), 0400)
			case "gap-legacy":
				mutationErr = os.WriteFile(filepath.Join(root, "plugin/plugin.yaml"), []byte("changed"), 0600)
			case "gap-link":
				mutationErr = os.Remove(filepath.Join(root, "alias"))
				if mutationErr == nil {
					mutationErr = os.Symlink("plugin.json", filepath.Join(root, "alias"))
				}
			case "gap-directory":
				mutationErr = os.WriteFile(filepath.Join(root, "new"), []byte("x"), 0600)
			case "gap-root":
				mutationErr = os.Rename(root, root+"-moved")
			case "gap-ancestor":
				// Scratch is outside this selected ancestor.
				mutationErr = os.Rename(filepath.Dir(root), filepath.Dir(root)+"-moved")
			}
			if mutationErr != nil {
				l.Close()
				t.Fatal(mutationErr)
			}
			in, e := l.Capture(context.Background())
			requireChanged(t, e)
			if in.Identity.Digest != "" || l.Data().Identity.Digest != "" {
				t.Fatal("usable partial evidence")
			}
			entries, e := os.ReadDir(scratch)
			if e != nil || len(entries) != 0 {
				t.Fatal("owned cleanup", e, entries)
			}
		})
	}
}
func TestDarwinWritableOuterSiblingAndPaths(t *testing.T) {
	root, scratch := writableFixture(t)
	nativeWrite(t, root, "a/nested/keep", "x")
	nativeLink(t, root, "a/nested", "linked")
	l, e := (Reader{TempDir: scratch}).Open(context.Background(), root)
	if e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(filepath.Dir(root), "unrelated"), []byte("safe sibling"), 0600); e != nil {
		t.Fatal(e)
	}
	if _, e := l.Capture(context.Background()); e != nil {
		t.Fatal("unrelated sibling rejected", e)
	}
	l.Close()
	s, e := openSource(root + "/linked/..")
	if e != nil {
		t.Fatal(e)
	}
	defer s.close()
	want, e := os.Stat(filepath.Join(root, "a"))
	if e != nil {
		t.Fatal(e)
	}
	got, e := s.anchor.Stat()
	if e != nil || !sameIdentity(got, want) {
		t.Fatal("a/.. was cleaned", e)
	}
	if linked, e := openSource(root + "/linked/"); e == nil {
		linked.close()
		t.Fatal("linked final root accepted")
	}
}
func TestDarwinWritableOverlapAndCloseBinding(t *testing.T) {
	root, scratch := writableFixture(t)
	if l, e := (Reader{TempDir: root}).Open(context.Background(), root); e == nil {
		l.Close()
		t.Fatal("overlap accepted")
	}
	l, e := (Reader{TempDir: scratch}).Open(context.Background(), root)
	if e != nil {
		t.Fatal(e)
	}
	if e := os.Rename(root, root+"-moved"); e != nil {
		t.Fatal(e)
	}
	requireChanged(t, l.Close())
	if l.Data().Identity.Digest != "" {
		t.Fatal("Close retained invalid input")
	}
}

func TestDarwinWritablePanicAndFDs(t *testing.T) {
	root, scratch := writableFixture(t)
	// Warm the real scoped policy path before measuring descriptor ownership.
	l, e := (Reader{TempDir: scratch}).Open(context.Background(), root)
	if e != nil {
		t.Fatal(e)
	}
	if e = l.Close(); e != nil {
		t.Fatal(e)
	}
	count := func() int {
		entries, e := os.ReadDir("/dev/fd")
		if e != nil {
			t.Fatal(e)
		}
		return len(entries)
	}
	before := count()
	held, e := os.Open(filepath.Join(root, "plugin.json"))
	if e != nil {
		t.Fatal(e)
	}
	if count() <= before {
		held.Close()
		t.Fatal("FD observer failed positive control")
	}
	held.Close()
	for i := 0; i < 3; i++ {
		var caught any
		func() {
			defer func() { caught = recover() }()
			_, _ = (Reader{TempDir: scratch}).open(context.Background(), root, &captureHooks{nativeOpen: func(string, int) { panic("owned panic") }})
		}()
		if caught != "owned panic" {
			t.Fatal("panic not relayed", caught)
		}
		if count() != before {
			t.Fatal("descriptor leak after panic")
		}
		entries, e := os.ReadDir(scratch)
		if e != nil || len(entries) != 0 {
			t.Fatal("panic scratch cleanup", e, entries)
		}
	}
}

func TestDarwinWritableWrongCoreAndBounds(t *testing.T) {
	for _, kind := range []string{"fifo", "directory", "legacy", "limit", "cancel"} {
		t.Run(kind, func(t *testing.T) {
			root, scratch := writableFixture(t)
			if e := os.Remove(filepath.Join(root, "plugin.json")); e != nil {
				t.Fatal(e)
			}
			switch kind {
			case "fifo":
				if e := unix.Mkfifo(filepath.Join(root, "plugin.json"), 0600); e != nil {
					t.Fatal(e)
				}
			case "directory":
				if e := os.Mkdir(filepath.Join(root, "plugin.json"), 0700); e != nil {
					t.Fatal(e)
				}
			case "legacy":
				nativeWrite(t, root, "plugin/plugin.yaml", "excluded")
				nativeLink(t, root, "plugin/plugin.yaml", "plugin.json")
			default:
				nativeWrite(t, root, "plugin.json", "oversize")
			}
			opened := 0
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if kind == "cancel" {
				cancel()
			}
			limits := Limits{}
			if kind == "limit" {
				limits.PluginBytes = 1
			}
			l, e := (Reader{TempDir: scratch, Limits: limits}).open(ctx, root, &captureHooks{nativeOpen: func(_ string, flags int) {
				if flags&unix.O_DIRECTORY == 0 {
					opened++
				}
			}})
			if opened != 0 {
				t.Fatal("unsafe/unapproved core data open", opened)
			}
			if kind == "limit" || kind == "cancel" {
				if e == nil || l != nil {
					t.Fatal("expected fatal bound/cancel", e)
				}
				return
			}
			if e != nil {
				t.Fatal(e)
			}
			defer l.Close()
			if l.Data().Plugin.State == Present || l.Data().Coverage.ComponentsRequested {
				t.Fatal("wrong core authorized capture")
			}
			if _, e := l.Capture(ctx); e == nil {
				t.Fatal("wrong core captured components")
			}
		})
	}
}

func TestDarwinWritableReplacedPrivateChild(t *testing.T) {
	root, scratch := writableFixture(t)
	l, e := (Reader{TempDir: scratch}).Open(context.Background(), root)
	if e != nil {
		t.Fatal(e)
	}
	replaced := false
	l.hooks = &captureHooks{cleanup: func() error {
		if e := os.Rename(l.private, l.private+"-owned-moved"); e != nil {
			return e
		}
		if e := os.Mkdir(l.private, 0700); e != nil {
			return e
		}
		if e := os.WriteFile(filepath.Join(l.private, "foreign"), []byte("keep"), 0600); e != nil {
			return e
		}
		replaced = true
		return nil
	}}
	e = l.Close()
	var safe *Error
	if !replaced || !errors.As(e, &safe) || !safe.CleanupFailed {
		t.Fatal("cleanup replacement not detected", e, replaced)
	}
	got, e := os.ReadFile(filepath.Join(l.private, "foreign"))
	if e != nil || string(got) != "keep" {
		t.Fatal("foreign child modified", e)
	}
}

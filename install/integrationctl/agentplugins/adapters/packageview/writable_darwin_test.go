//go:build darwin && arm64

package packageview

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"
)

func TestDarwinWritableReplacementsFailBeforeRead(t *testing.T) {
	for _, stage := range []string{"before", "after"} {
		for _, kind := range []string{"regular", "fifo", "directory", "symlink", "hardlink", "legacy", "growth"} {
			t.Run(stage+"/"+kind, func(t *testing.T) {
				root := nativeFixture(t, func(root string) {
					nativeWrite(t, root, "plugin.json", "core")
					nativeWrite(t, root, "plugin/plugin.yaml", "secret")
					nativeWrite(t, root, "other", "other")
				})
				changed := false
				mutate := func(name string) {
					if name != "plugin.json" || changed {
						return
					}
					changed = true
					name = filepath.Join(root, name)
					if kind == "growth" {
						if e := os.WriteFile(name, []byte("growing"), 0600); e != nil {
							t.Fatal(e)
						}
						return
					}
					if e := os.Remove(name); e != nil {
						t.Fatal(e)
					}
					var e error
					switch kind {
					case "regular":
						e = os.WriteFile(name, []byte("evil"), 0600)
					case "fifo":
						e = unix.Mkfifo(name, 0600)
					case "directory":
						e = os.Mkdir(name, 0700)
					case "symlink":
						e = os.Symlink("other", name)
					case "hardlink":
						e = os.Link(filepath.Join(root, "other"), name)
					case "legacy":
						e = os.Rename(filepath.Join(root, "plugin/plugin.yaml"), name)
					}
					if e != nil {
						t.Fatal(e)
					}
				}
				h := &captureHooks{beforeDataOpen: mutate}
				if stage == "after" {
					h = &captureHooks{afterNameCheck: mutate}
				}
				l, e := (Reader{TempDir: t.TempDir()}).open(context.Background(), root, h)
				accepted := l != nil && l.Data().Plugin.State == Present
				if l != nil {
					l.Close()
				}
				if !changed || (e == nil && accepted) {
					t.Fatalf("replacement accepted: changed=%v err=%v", changed, e)
				}
			})
		}
	}
}

func TestDarwinPinRetainsParentAfterNamespaceReplacement(t *testing.T) {
	root := nativeFixture(t, func(root string) { nativeWrite(t, root, "dir/file", "original") })
	outside := nativeFixture(t, func(root string) { nativeWrite(t, root, "file", "outside") })
	s := nativeSource(t, root)
	p, e := s.pin("dir/file", false)
	if e != nil {
		t.Fatal(e)
	}
	defer p.file.Close()
	if e := os.Rename(filepath.Join(root, "dir"), filepath.Join(root, "old")); e != nil {
		t.Fatal(e)
	}
	if e := os.Symlink(outside, filepath.Join(root, "dir")); e != nil {
		t.Fatal(e)
	}
	f, e := p.reopen(false)
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	b, e := io.ReadAll(f)
	if e != nil || string(b) != "original" {
		t.Fatalf("held parent lost: %q %v", b, e)
	}
	if current, e := s.pin("dir/file", false); e == nil {
		current.file.Close()
		t.Fatal("absolute replacement symlink traversed")
	}
}

func TestDarwinMetadataMatchesNativeStat(t *testing.T) {
	// Keep the disposable pathname below Darwin's Unix socket address limit.
	root, e := os.MkdirTemp("/tmp", "pv-meta-")
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		if e := os.RemoveAll(root); e != nil {
			t.Error(e)
		}
	})
	nativeWrite(t, root, "file", "data")
	nativeLink(t, root, "file", "link")
	if e := os.Mkdir(filepath.Join(root, "dir"), 0700); e != nil {
		t.Fatal(e)
	}
	if e := unix.Mkfifo(filepath.Join(root, "fifo"), 0600); e != nil {
		t.Fatal(e)
	}
	socket, e := net.ListenUnix("unix", &net.UnixAddr{Name: filepath.Join(root, "socket"), Net: "unix"})
	if e != nil {
		t.Fatal(e)
	}
	defer socket.Close()
	if e := os.Chmod(filepath.Join(root, "file"), 0750|os.ModeSetuid|os.ModeSetgid); e != nil {
		t.Fatal(e)
	}
	s := nativeSource(t, root)
	for _, name := range []string{"file", "link", "dir", "fifo", "socket"} {
		p, e := s.pin(name, true)
		if e != nil {
			t.Fatal(e)
		}
		native, e := os.Lstat(filepath.Join(root, name))
		if e != nil {
			t.Fatal(e)
		}
		if !same(p.info, native) || !sameIdentity(p.info, native) {
			t.Fatalf("metadata differs for %s: %v vs %v", name, p.info, native)
		}
		p.file.Close()
	}
}

func TestDarwinCaseInsensitiveScratchOverlap(t *testing.T) {
	outer := t.TempDir()
	root := filepath.Join(outer, "SourceCase")
	if e := os.Mkdir(root, 0700); e != nil {
		t.Fatal(e)
	}
	nativeWrite(t, root, "plugin.json", "core")
	if e := os.Mkdir(filepath.Join(root, "scratch"), 0700); e != nil {
		t.Fatal(e)
	}
	alias := filepath.Join(outer, "sourcecase")
	if _, e := os.Stat(alias); os.IsNotExist(e) {
		t.Skip("case-sensitive APFS: alias does not exist")
	} else if e != nil {
		t.Fatal(e)
	}
	for _, scratch := range []string{alias, filepath.Join(alias, "scratch")} {
		l, e := (Reader{TempDir: scratch}).Open(context.Background(), root)
		if l != nil {
			l.Close()
		}
		var safe *Error
		if !errors.As(e, &safe) || safe.Code != "scratch_overlaps_source" {
			t.Fatalf("scratch alias accepted: %s: %v", scratch, e)
		}
	}
	entries, e := os.ReadDir(filepath.Join(root, "scratch"))
	if e != nil || len(entries) != 0 {
		t.Fatalf("scratch created inside source: %v %v", entries, e)
	}
}

func TestDarwinWritableChangeBetweenCaptureStages(t *testing.T) {
	root := nativeFixture(t, func(root string) { nativeWrite(t, root, "plugin.json", "core") })
	l, e := (Reader{TempDir: t.TempDir()}).Open(context.Background(), root)
	if e != nil {
		t.Fatal(e)
	}
	defer l.Close()
	file := filepath.Join(root, "plugin.json")
	before, e := os.Stat(file)
	if e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(file, []byte("evil"), 0600); e != nil {
		t.Fatal(e)
	}
	if e := os.Chtimes(file, before.ModTime(), before.ModTime()); e != nil {
		t.Fatal(e)
	}
	if _, e := l.Capture(context.Background()); e == nil {
		t.Fatal("changed captured source accepted")
	}
}

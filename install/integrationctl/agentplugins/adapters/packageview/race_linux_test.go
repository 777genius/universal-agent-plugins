//go:build linux

package packageview

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestSpecialEntriesNeverOpenData(t *testing.T) {
	root, r := fixture(t)
	if e := unix.Mkfifo(filepath.Join(root, "mcp.json"), 0600); e != nil {
		t.Fatal(e)
	}
	if e := unix.Mkfifo(filepath.Join(root, "opaque-fifo"), 0600); e != nil {
		t.Fatal(e)
	}
	v := capture(t, open(t, r, root, &captureHooks{beforeDataOpen: func(p string) {
		if p != "plugin.json" {
			t.Fatalf("special data open %q", p)
		}
	}}))
	if v.MCP.State != WrongKind || observation(t, v, "opaque-fifo").State != WrongKind || v.Coverage.TreeComplete {
		t.Fatal("FIFO covered")
	}
}

func TestDeviceMetadataOnly(t *testing.T) {
	root, r := fixture(t)
	// A new disposable device node, never an existing host device. O_PATH gets
	// metadata without dispatching a device open. If mknod is unavailable this
	// particular native device gate is explicitly unproven.
	e := unix.Mknod(filepath.Join(root, "device"), unix.S_IFCHR|0600, int(unix.Mkdev(1, 3)))
	if errors.Is(e, unix.EPERM) || errors.Is(e, unix.EACCES) {
		t.Skip("native device-node gate unproven: fixture mknod denied")
	}
	if e != nil {
		t.Fatal(e)
	}
	v := capture(t, open(t, r, root, &captureHooks{beforeDataOpen: func(p string) {
		if p != "plugin.json" {
			t.Fatal("device data open")
		}
	}}))
	if observation(t, v, "device").State != WrongKind || v.Coverage.TreeComplete {
		t.Fatal("device covered")
	}
}

func TestReplacementRacesNeverFollowSpecialOrOutside(t *testing.T) {
	for _, phase := range []string{"before-name-check", "after-name-check"} {
		for _, replacement := range []string{"fifo", "outside-link", "directory", "regular", "device"} {
			t.Run(phase+"/"+replacement, func(t *testing.T) {
				root, r := fixture(t)
				put(t, root, "mcp.json", mcpBody)
				outside := t.TempDir()
				put(t, outside, "secret", "outside sentinel")
				device := filepath.Join(root, "new-device")
				if replacement == "device" {
					e := unix.Mknod(device, unix.S_IFBLK|0600, int(unix.Mkdev(7, 0)))
					if errors.Is(e, unix.EPERM) || errors.Is(e, unix.EACCES) {
						t.Skip("block-device replacement gate unproven: fixture mknod denied")
					}
					if e != nil {
						t.Fatal(e)
					}
				}
				swap := func(p string) {
					if p != "mcp.json" {
						return
					}
					name := filepath.Join(root, p)
					if e := os.Remove(name); e != nil {
						t.Fatal(e)
					}
					switch replacement {
					case "fifo":
						if e := unix.Mkfifo(name, 0600); e != nil {
							t.Fatal(e)
						}
					case "outside-link":
						link(t, root, p, filepath.Join(outside, "secret"))
					case "directory":
						if e := os.Mkdir(name, 0700); e != nil {
							t.Fatal(e)
						}
					case "regular":
						put(t, root, p, strings.Repeat("x", len(mcpBody)))
					case "device":
						if e := os.Rename(device, name); e != nil {
							t.Fatal(e)
						}
					}
				}
				h := &captureHooks{}
				if phase == "before-name-check" {
					h.beforeDataOpen = swap
				} else {
					h.afterNameCheck = swap
				}
				l := open(t, r, root, h)
				start := time.Now()
				_, e := l.Capture(context.Background())
				code(t, e, "source_changed")
				if time.Since(start) > 2*time.Second {
					t.Fatal("replacement blocked")
				}
				empty(t, r.TempDir)
			})
		}
	}
}

func TestMutationDuringReadAndBetweenStages(t *testing.T) {
	for _, mutation := range []string{"same-size", "growth", "shrink", "chmod", "ancestor"} {
		t.Run(mutation, func(t *testing.T) {
			root, r := fixture(t)
			body := strings.Repeat("a", 100000)
			put(t, root, "data/file", body)
			did := false
			h := &captureHooks{afterChunk: func(p string) {
				if p != "data/file" || did {
					return
				}
				did = true
				name := filepath.Join(root, p)
				switch mutation {
				case "same-size":
					put(t, root, p, strings.Repeat("b", len(body)))
				case "growth":
					put(t, root, p, body+"x")
				case "shrink":
					put(t, root, p, "x")
				case "chmod":
					if e := os.Chmod(name, 0400); e != nil {
						t.Fatal(e)
					}
				case "ancestor":
					if e := os.Rename(filepath.Join(root, "data"), filepath.Join(root, "moved")); e != nil {
						t.Fatal(e)
					}
					put(t, root, p, body)
				}
			}}
			l := open(t, r, root, h)
			_, e := l.Capture(context.Background())
			code(t, e, "source_changed")
			if !did {
				t.Fatal("mutation not scheduled")
			}
			empty(t, r.TempDir)
		})
	}
	t.Run("between core and components", func(t *testing.T) {
		root, r := fixture(t)
		l := open(t, r, root, nil)
		put(t, root, "plugin.json", strings.Repeat("x", len(pluginBody)))
		_, e := l.Capture(context.Background())
		code(t, e, "source_changed")
		empty(t, r.TempDir)
	})
	t.Run("directory after enumeration", func(t *testing.T) {
		root, r := fixture(t)
		put(t, root, "z", "last")
		l := open(t, r, root, &captureHooks{afterChunk: func(p string) {
			if p == "z" {
				put(t, root, "new", "late")
			}
		}})
		_, e := l.Capture(context.Background())
		code(t, e, "source_changed")
		empty(t, r.TempDir)
	})
}

func TestVerifiedHandleReopensPinnedInode(t *testing.T) {
	root, r := fixture(t)
	s, e := openSource(root, GeneratedStaging{})
	if e != nil {
		t.Fatal(e)
	}
	defer s.close()
	p, e := s.pin("plugin.json", false)
	if e != nil {
		t.Fatal(e)
	}
	defer p.file.Close()
	// Rename preserves the pinned inode even if ctime resolution cannot expose
	// the rename. Reopen may reject metadata change; if it succeeds it must select
	// the original regular inode and bytes, despite a FIFO at the source name.
	if e := os.Rename(filepath.Join(root, "plugin.json"), filepath.Join(root, "old")); e != nil {
		t.Fatal(e)
	}
	if e := unix.Mkfifo(filepath.Join(root, "plugin.json"), 0600); e != nil {
		t.Fatal(e)
	}
	f, e := p.reopen(false)
	if e != nil {
		code(t, e, "source_changed")
	} else {
		b, e := io.ReadAll(f)
		ce := f.Close()
		if e != nil || ce != nil || string(b) != pluginBody {
			t.Fatal("reopen lost pinned regular bytes")
		}
	}

	empty(t, r.TempDir)
}

func TestSymlinkExpansionBound(t *testing.T) {
	root, r := fixture(t)
	for i := 0; i < 129; i++ {
		name := fmtLink(i)
		target := fmtLink(i + 1)
		if i == 128 {
			target = "plugin.json"
		}
		link(t, root, name, target)
	}
	link(t, root, "mcp.json", fmtLink(0))
	v := capture(t, open(t, r, root, nil))
	if v.MCP.State != Blocked || v.Coverage.TreeComplete {
		t.Fatal("excessive link chain accepted")
	}
	// Linux caps a pathname resolution at 40 links, stricter than the audit's
	// maximum 128. This is host availability, not a normative profile rule.
}
func fmtLink(i int) string { return "link-" + strings.Repeat("x", i+1) }

func TestSecondByteObservationRejectsChangeWithoutMetadata(t *testing.T) {
	root, _ := fixture(t)
	put(t, root, "data", "second")
	f, e := os.Open(filepath.Join(root, "data"))
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	// No timestamp/identity is passed to this check: same-length changed content
	// must be rejected even if filesystem timestamps could not distinguish it.
	code(t, verifyRead(context.Background(), f, []byte("first!")), "source_changed")
	if e := verifyRead(context.Background(), f, []byte("second")); e != nil {
		t.Fatal(e)
	}
	code(t, verifyRead(context.Background(), f, []byte("short")), "source_changed")
	code(t, verifyRead(context.Background(), f, []byte("too long")), "source_changed")
}

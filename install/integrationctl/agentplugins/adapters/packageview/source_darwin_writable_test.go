//go:build darwin && arm64

package packageview

import (
	"context"
	"errors"
	"golang.org/x/sys/unix"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

// No mounting, cloud, device creation or real-project inputs. Unsupported
// fixture storage is a failed prerequisite, never a native success/skip.
func writableFixture(t *testing.T) (string, string) {
	t.Helper()
	return writableFixtureAt(t, t.TempDir())
}

func writableFixtureAt(t *testing.T, dir string) (string, string) {
	t.Helper()
	base, e := filepath.EvalSymlinks(dir)
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
	// Darwin's sockaddr_un cannot hold a typical testing.T temp path. Own a
	// short directory independently of TMPDIR; never remove the shared root.
	base, e := os.MkdirTemp("/private/tmp", "pv-")
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		if e := os.RemoveAll(base); e != nil {
			t.Error(e)
		}
	})
	root, scratch := writableFixtureAt(t, base)
	socketPath := filepath.Join(root, "socket")
	if len(socketPath) >= len((unix.RawSockaddrUnix{}).Path) {
		t.Fatalf("socket fixture path exceeds host sockaddr_un capacity: %q", socketPath)
	}
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
	if e := unix.Bind(fd, &unix.SockaddrUnix{Name: socketPath}); e != nil {
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
	s, e := openSource(root+"/linked/..", GeneratedStaging{})
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
	if linked, e := openSource(root+"/linked/", GeneratedStaging{}); e == nil {
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
		// Names only: ReadDir may stat volatile /dev/fd entries on Darwin.
		dir, e := os.Open("/dev/fd")
		if e != nil {
			t.Fatal(e)
		}
		observer := strconv.FormatUint(uint64(dir.Fd()), 10)
		names, readErr := dir.Readdirnames(-1)
		closeErr := dir.Close() // Close even on enumeration failure, before comparing.
		if readErr != nil || closeErr != nil {
			t.Fatalf("FD enumeration: read=%v close=%v", readErr, closeErr)
		}
		n := 0
		seenObserver := false
		for _, name := range names {
			if name == observer {
				seenObserver = true
				continue
			}
			if _, e := strconv.ParseUint(name, 10, 64); e != nil {
				t.Fatalf("unexpected FD name %q: %v", name, e)
			}
			n++
		}
		if !seenObserver {
			t.Fatal("FD observer did not enumerate itself")
		}
		return n
	}
	before := count()
	held, e := os.Open(filepath.Join(root, "plugin.json"))
	if e != nil {
		t.Fatal(e)
	}
	defer held.Close()
	if count() != before+1 {
		held.Close()
		t.Fatal("FD observer failed positive control")
	}
	if e := held.Close(); e != nil {
		t.Fatal(e)
	}
	if count() != before {
		t.Fatal("FD observer failed closed control")
	}
	// Keep the original first-native-open schedule: scratch owns an anchor,
	// source binding replay panics, and the lease has not received scratchClose.
	// Later phases exercise the held pin and data FD ownership as well.
	nativeWrite(t, root, "opaque", "inert payload")
	for _, phase := range []string{"scratch-handoff", "core-read", "inventory-read", "final-verification"} {
		for i := 0; i < 3; i++ {
			fired, coreOpens := false, 0
			var l *Lease
			var caught any
			var runErr error
			hooks := &captureHooks{}
			boom := func() { fired = true; panic("owned panic") }
			switch phase {
			case "scratch-handoff":
				hooks.nativeOpen = func(string, int) { boom() }
			case "core-read", "inventory-read":
				hooks.afterChunk = func(n string) {
					if phase == "core-read" && n == "plugin.json" || phase == "inventory-read" && n == "opaque" {
						boom()
					}
				}
			case "final-verification":
				hooks.nativeOpen = func(n string, flags int) {
					if n == "plugin.json" && flags&unix.O_DIRECTORY == 0 {
						coreOpens++
						if coreOpens == 2 {
							boom()
						}
					}
				}
			}
			func() {
				defer func() { caught = recover() }()
				l, runErr = (Reader{TempDir: scratch}).open(context.Background(), root, hooks)
				if runErr == nil && l != nil {
					_, runErr = l.Capture(context.Background())
				}
			}()
			if !fired || caught != "owned panic" || runErr != nil {
				t.Fatalf("%s panic not relayed: fired=%v panic=%v err=%v", phase, fired, caught, runErr)
			}
			if count() != before {
				t.Fatalf("descriptor leak after panic: phase=%s", phase)
			}
			if l != nil {
				if !reflect.DeepEqual(l.Data(), Input{}) {
					t.Fatal("panic retained partial Input")
				}
				if e := l.Close(); e != nil {
					t.Fatal("panic Close", e)
				}
				if e := l.Close(); e != nil {
					t.Fatal("repeated panic Close", e)
				}
			}
			entries, e := os.ReadDir(scratch)
			if e != nil || len(entries) != 0 {
				t.Fatal("panic scratch cleanup", e, entries)
			}
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

// darwinTraversalDeadlineCause is synthetic cause-propagation evidence, not a
// wall-clock deadline test. Its controlled clock reaches the fixed deadline
// when the traversal hook closes done. Deadline and Done stay unchanged, and
// Err transitions exactly once with that channel; no timer races filesystem I/O.
type darwinTraversalDeadlineCause struct{ done <-chan struct{} }

func (c darwinTraversalDeadlineCause) Deadline() (time.Time, bool) { return time.Unix(1, 0), true }
func (c darwinTraversalDeadlineCause) Done() <-chan struct{}       { return c.done }
func (c darwinTraversalDeadlineCause) Value(any) any               { return nil }
func (c darwinTraversalDeadlineCause) Err() error {
	select {
	case <-c.done:
		return context.DeadlineExceeded
	default:
		return nil
	}
}

func TestDarwinWritableTraversalCancellation(t *testing.T) {
	for _, cause := range []error{context.Canceled, context.DeadlineExceeded} {
		for _, phase := range []string{"after-metadata", "descendant-replay", "scratch-resolution", "post-open-guard", "final-guard"} {
			t.Run(cause.Error()+"/"+phase, func(t *testing.T) {
				root, scratch := writableFixture(t)
				nativeWrite(t, root, "skills/a/SKILL.md", "# inert")
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				if cause == context.DeadlineExceeded {
					ctx = darwinTraversalDeadlineCause{done: ctx.Done()}
					t.Log("synthetic deadline cause propagation; no wall-clock expiry claim")
				}
				done := ctx.Done()
				deadline, hasDeadline := ctx.Deadline()
				if done == nil || ctx.Err() != nil {
					t.Fatal("context must start live with a stable Done channel")
				}
				fired, armed, coreOpens := false, false, 0
				stop := func() { fired = true; cancel() }
				hooks := &captureHooks{}
				switch phase {
				case "after-metadata":
					hooks.beforeDataOpen = func(n string) {
						if n == "plugin.json" {
							stop()
						}
					}
				case "descendant-replay":
					hooks.metadata = func(n string) error { armed = n == "skills/a/SKILL.md"; return nil }
					hooks.nativeOpen = func(n string, flags int) {
						if armed && n == "a" && flags&unix.O_DIRECTORY != 0 {
							stop()
						}
					}
				case "scratch-resolution":
					// Opens the real scratch directory; a subsequent prefix replay
					// step returns cancellation before ownership reaches the lease.
					hooks.scratchOpen = func(n string, flags int) {
						if n == "scratch" && flags&unix.O_DIRECTORY != 0 {
							stop()
						}
					}
				case "post-open-guard", "final-guard":
					hooks.nativeOpen = func(n string, flags int) {
						if n == "plugin.json" && flags&unix.O_DIRECTORY == 0 {
							coreOpens++
							if phase == "post-open-guard" || coreOpens == 2 {
								stop()
							}
						}
					}
				}
				l, e := (Reader{TempDir: scratch}).open(ctx, root, hooks)
				if phase == "descendant-replay" || phase == "final-guard" {
					if e != nil {
						t.Fatal("Open prerequisite", e)
					}
					defer l.Close()
					in, captureErr := l.Capture(ctx)
					e = captureErr
					if !reflect.DeepEqual(in, Input{}) || !reflect.DeepEqual(l.Data(), Input{}) {
						t.Fatal("usable partial Input")
					}
				} else if l != nil {
					l.Close()
					t.Fatal("partial Open lease")
				}
				var safe *Error
				if !fired || !errors.Is(e, cause) || !errors.As(e, &safe) || safe.Code != "canceled" || safe.CleanupFailed {
					t.Fatalf("traversal cancellation lost: fired=%v err=%v", fired, e)
				}
				if after, ok := ctx.Deadline(); after != deadline || ok != hasDeadline || ctx.Done() != done || ctx.Err() != cause {
					t.Fatal("context contract changed at traversal boundary")
				}
				select {
				case <-done:
				default:
					t.Fatal("original Done channel did not close")
				}
				entries, e := os.ReadDir(scratch)
				if e != nil || len(entries) != 0 {
					t.Fatal("owned cleanup", entries, e)
				}
			})
		}
	}
}

func TestDarwinWritableCaptureContextHandoff(t *testing.T) {
	root, scratch := writableFixture(t)
	openCtx, cancelOpen := context.WithCancel(context.Background())
	defer cancelOpen()
	l, e := (Reader{TempDir: scratch}).Open(openCtx, root)
	if e != nil {
		t.Fatal("Open prerequisite", e)
	}
	defer l.Close()
	// A resolver retaining Open's context must fail this otherwise live Capture.
	// Merely observing Capture's argument at a shared polling site cannot pass.
	cancelOpen()
	captureCtx, cancelCapture := context.WithCancel(context.Background())
	defer cancelCapture()
	in, e := l.Capture(captureCtx)
	if e != nil || !in.Coverage.TreeComplete || in.Identity.TreeDigest == "" {
		t.Fatalf("Capture retained stale Open context: %v", e)
	}
	if openCtx.Err() != context.Canceled || captureCtx.Err() != nil {
		t.Fatal("distinct phase context prerequisite")
	}
	if e := l.Close(); e != nil {
		t.Fatal(e)
	}
	entries, e := os.ReadDir(scratch)
	if e != nil || len(entries) != 0 {
		t.Fatal("owned cleanup", entries, e)
	}
}

func TestDarwinWritableScratchFinalSymlink(t *testing.T) {
	root, scratch := writableFixture(t)
	external := filepath.Join(filepath.Dir(scratch), "scratch-alias")
	inside := filepath.Join(filepath.Dir(scratch), "inside-alias")
	sourceAlias := filepath.Join(filepath.Dir(scratch), "source-alias")
	nativeWrite(t, root, "inside/keep", "source sentinel")
	for alias, target := range map[string]string{external: scratch, inside: filepath.Join(root, "inside"), sourceAlias: root} {
		if e := os.Symlink(target, alias); e != nil {
			t.Fatal(e)
		}
	}
	for _, temp := range []string{scratch, external} {
		l, e := (Reader{TempDir: temp}).Open(context.Background(), root)
		if e != nil {
			t.Fatal("trusted scratch rejected", temp, e)
		}
		if filepath.Dir(l.private) != scratch {
			t.Fatal("private child is not physical", l.private)
		}
		if _, e := l.Capture(context.Background()); e != nil {
			t.Fatal(e)
		}
		if e := l.Close(); e != nil {
			t.Fatal(e)
		}
		entries, e := os.ReadDir(scratch)
		if e != nil || len(entries) != 0 {
			t.Fatal("scratch cleanup", entries, e)
		}
	}
	for _, temp := range []string{inside, sourceAlias} {
		l, e := (Reader{TempDir: temp}).Open(context.Background(), root)
		if l != nil {
			l.Close()
			t.Fatal("overlap accepted")
		}
		var safe *Error
		if !errors.As(e, &safe) || safe.Code != "scratch_overlaps_source" {
			t.Fatal("physical overlap not rejected", e)
		}
	}
	if l, e := (Reader{TempDir: external}).Open(context.Background(), sourceAlias); e == nil || l != nil {
		if l != nil {
			l.Close()
		}
		t.Fatal("final source symlink accepted")
	}
	got, e := os.ReadFile(filepath.Join(root, "inside/keep"))
	if e != nil || string(got) != "source sentinel" {
		t.Fatal("source changed", e)
	}
}

func TestDarwinWritableObservedChangePrecedesCancellation(t *testing.T) {
	root, scratch := writableFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	fired := false
	var mutationErr error
	l, e := (Reader{TempDir: scratch}).open(ctx, root, &captureHooks{nativeOpen: func(n string, flags int) {
		if !fired && n == "parent" && flags&unix.O_DIRECTORY != 0 {
			fired = true
			mutationErr = os.Chmod(filepath.Dir(root), 0750)
			cancel()
		}
	}})
	if l != nil {
		l.Close()
		t.Fatal("partial lease")
	}
	if !fired || mutationErr != nil {
		t.Fatal("mutation prerequisite", fired, mutationErr)
	}
	// p.open observes the changed mode before directory/verifyBinding unwind.
	// A later ctx.Err probe must not overwrite that already observed change.
	requireChanged(t, e)
	if errors.Is(e, context.Canceled) {
		t.Fatal("observed change masked by cancellation")
	}
	entries, e := os.ReadDir(scratch)
	if e != nil || len(entries) != 0 {
		t.Fatal("owned cleanup", entries, e)
	}
}

package scaffold

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
)

func TestFailureCancellationAndCleanup(t *testing.T) {
	for _, kind := range []string{"validation", "cancel-before", "cancel-validation", "partial-write", "commit-failure", "validator-edited", "validator-extra", "validator-missing", "validator-symlink"} {
		t.Run(kind, func(t *testing.T) {
			parent := tempRoot(t)
			dest := filepath.Join(parent, "output")
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			sentinel := errors.New("injected operation failure")
			validate := realValidation(t)
			ops := applyOps{write: writeTree, rename: renameExclusive}
			if kind == "cancel-before" {
				cancel()
			}
			if kind == "partial-write" {
				ops.write = func(ctx context.Context, r *os.Root, files []File) error {
					f, err := r.OpenFile("partial", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
					if err != nil {
						return err
					}
					_, err = f.Write([]byte("part"))
					return errors.Join(err, f.Close(), sentinel)
				}
			}
			if kind == "commit-failure" {
				ops.rename = func(*os.File, string, *os.File, string) error { return sentinel }
			}
			result, err := apply(ctx, planFor(t, "skill"), ApplyOptions{Destination: dest, Validate: func(ctx context.Context, s string) error {
				if err := validate(ctx, s); err != nil {
					return err
				}
				switch kind {
				case "validation":
					return sentinel
				case "cancel-validation":
					cancel()
				case "validator-edited":
					return os.WriteFile(filepath.Join(s, "README.md"), []byte("changed"), 0644)
				case "validator-extra":
					return os.WriteFile(filepath.Join(s, "extra"), []byte("extra"), 0644)
				case "validator-missing":
					return os.Remove(filepath.Join(s, "README.md"))
				case "validator-symlink":
					if err := os.Remove(filepath.Join(s, "README.md")); err != nil {
						return err
					}
					if err := os.Symlink(filepath.Join(parent, "outside"), filepath.Join(s, "README.md")); err != nil {
						t.Skipf("native symlink prerequisite: %v", err)
					}
				}
				return nil
			}}, ops)
			if err == nil || result.Committed {
				t.Fatalf("failure was success: %v %v", result, err)
			}
			if kind == "cancel-before" || kind == "cancel-validation" {
				if !errors.Is(err, context.Canceled) {
					t.Fatal(err)
				}
			}
			if kind == "validation" || kind == "partial-write" || kind == "commit-failure" {
				if !errors.Is(err, sentinel) {
					t.Fatal(err)
				}
			}
			assertOnly(t, parent)
		})
	}
}

func TestRequiredValidationAndZeroPlan(t *testing.T) {
	root := tempRoot(t)
	dest := filepath.Join(root, "out")
	p := planFor(t, "skill")
	if _, err := Apply(context.Background(), p, ApplyOptions{Destination: dest}); err == nil {
		t.Fatal("nil validator accepted")
	}
	if _, err := Apply(context.Background(), Plan{}, ApplyOptions{Destination: dest, Validate: realValidation(t)}); err == nil {
		t.Fatal("zero plan accepted")
	}
	if _, err := Apply(nil, p, ApplyOptions{Destination: dest, Validate: realValidation(t)}); err == nil {
		t.Fatal("nil context accepted")
	}
	assertOnly(t, root)
}
func TestMissingParentSymlinkParentAndSourceOverlap(t *testing.T) {
	root := tempRoot(t)
	source := filepath.Join(root, "source")
	if err := os.Mkdir(source, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "sentinel"), []byte("untouched"), 0644); err != nil {
		t.Fatal(err)
	}
	p := planFor(t, "skill")
	validate := realValidation(t)
	for _, o := range []ApplyOptions{
		{Destination: filepath.Join(root, "missing", "out")},
		{Destination: filepath.Join(source, "out"), SourceRoots: []string{source}},
		{Destination: source, SourceRoots: []string{source}},
		{Destination: root, SourceRoots: []string{source}},
		{Destination: "relative"},
		{Destination: root + string(filepath.Separator) + ".." + string(filepath.Separator) + "out"},
		{Destination: filepath.Join(root, "CON.txt")},
		{Destination: filepath.Join(root, "out"), SourceRoots: []string{"relative"}},
	} {
		o.Validate = validate
		if result, err := Apply(context.Background(), p, o); err == nil || result.Committed {
			t.Fatalf("accepted unsafe apply: %+v", o)
		}
	}
	alias := filepath.Join(root, "alias")
	if err := os.Symlink(source, alias); err != nil {
		t.Skipf("native symlink prerequisite: %v", err)
	}
	if _, err := Apply(context.Background(), p, ApplyOptions{Destination: filepath.Join(alias, "out"), Validate: validate}); err == nil {
		t.Fatal("followed symlink parent")
	}
	if _, err := Apply(context.Background(), p, ApplyOptions{Destination: filepath.Join(source, "out"), SourceRoots: []string{alias}, Validate: validate}); err == nil {
		t.Fatal("source alias overlap")
	}
	assertOnly(t, source, "sentinel")
	b, _ := os.ReadFile(filepath.Join(source, "sentinel"))
	if string(b) != "untouched" {
		t.Fatal("source changed")
	}
}
func TestExecutableModeAndNoExecution(t *testing.T) {
	p := planFor(t, "skill")
	p.files = append(p.files, File{"bin/helper", []byte("#!/bin/sh\nexit 97\n"), 0755})
	root := tempRoot(t)
	dest := filepath.Join(root, "out")
	result, err := Apply(context.Background(), p, ApplyOptions{Destination: dest, Validate: realValidation(t)})
	if err != nil || !result.Committed {
		t.Fatalf("%+v %v", result, err)
	}
	info, err := os.Stat(filepath.Join(dest, "bin", "helper"))
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0755 {
		t.Fatalf("executable mode lost: %v", info.Mode())
	}
}
func TestStagingReplacementRefusesForeignCleanup(t *testing.T) {
	parent := tempRoot(t)
	dest := filepath.Join(parent, "out")
	validate := realValidation(t)
	var original, replaced string
	result, err := Apply(context.Background(), planFor(t, "skill"), ApplyOptions{Destination: dest, Validate: func(ctx context.Context, s string) error {
		if err := validate(ctx, s); err != nil {
			return err
		}
		original = filepath.Dir(s)
		replaced = original + "-moved"
		if err := os.Rename(original, replaced); err != nil {
			t.Skipf("native directory handle rename prerequisite: %v", err)
		}
		if err := os.Mkdir(original, 0700); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(original, "sentinel"), []byte("foreign"), 0600); err != nil {
			return err
		}
		return nil
	}})
	if err == nil || result.Committed {
		t.Fatal("accepted replaced staging")
	}
	b, e := os.ReadFile(filepath.Join(original, "sentinel"))
	if e != nil || string(b) != "foreign" {
		t.Fatalf("foreign stage removed: %v", e)
	}
	// Owned payload cleanup still addresses its pinned container, wherever moved.
	assertOnly(t, replaced)
	if _, e = os.Lstat(dest); !errors.Is(e, fs.ErrNotExist) {
		t.Fatal("partial destination")
	}
}
func TestParentReplacementRefusesCommitAndCleansOwnedStage(t *testing.T) {
	grand := tempRoot(t)
	parent := filepath.Join(grand, "parent")
	moved := filepath.Join(grand, "moved")
	os.Mkdir(parent, 0755)
	validate := realValidation(t)
	result, err := Apply(context.Background(), planFor(t, "skill"), ApplyOptions{Destination: filepath.Join(parent, "out"), Validate: func(ctx context.Context, s string) error {
		if err := validate(ctx, s); err != nil {
			return err
		}
		if err := os.Rename(parent, moved); err != nil {
			t.Skipf("native directory handle rename prerequisite: %v", err)
		}
		if err := os.Mkdir(parent, 0755); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(parent, "sentinel"), []byte("foreign"), 0644)
	}})
	if err == nil || result.Committed {
		t.Fatal("accepted replaced parent")
	}
	assertOnly(t, parent, "sentinel")
	assertOnly(t, moved)
}
func TestDeniedParent(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permissions fixture; Windows ACL denial needs native CI fixture")
	}
	parent := tempRoot(t)
	if err := os.Chmod(parent, 0500); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(parent, 0700)
	// Root/capability-bearing environments cannot establish this fixture.
	probe := filepath.Join(parent, "permission-probe")
	if err := os.Mkdir(probe, 0700); err == nil {
		os.Remove(probe)
		t.Skip("environment bypasses directory write permissions")
	}
	if r, err := Apply(context.Background(), planFor(t, "skill"), ApplyOptions{Destination: filepath.Join(parent, "out"), Validate: realValidation(t)}); err == nil || r.Committed {
		t.Fatal("denied parent accepted")
	}
}

// Cancel after one chunk of a larger file, without signals or timing sleeps.
type writeCancelContext struct {
	context.Context
	calls int
}

func (c *writeCancelContext) Err() error {
	c.calls++
	if c.calls >= 3 {
		return context.Canceled
	}
	return nil
}
func TestInterruptedChunkWriteCleanup(t *testing.T) {
	parent := tempRoot(t)
	p := planFor(t, "skill")
	ops := applyOps{rename: renameExclusive, write: func(ctx context.Context, r *os.Root, _ []File) error {
		return writeTree(&writeCancelContext{Context: ctx}, r, []File{{"partial", make([]byte, 128*1024), 0644}})
	}}
	result, err := apply(context.Background(), p, ApplyOptions{Destination: filepath.Join(parent, "out"), Validate: realValidation(t)}, ops)
	if !errors.Is(err, context.Canceled) || result.Committed {
		t.Fatalf("%+v %v", result, err)
	}
	assertOnly(t, parent)
}
func TestPayloadReplacementDoesNotDeleteForeignTree(t *testing.T) {
	parent := tempRoot(t)
	validate := realValidation(t)
	var foreign, moved string
	result, err := Apply(context.Background(), planFor(t, "skill"), ApplyOptions{Destination: filepath.Join(parent, "out"), Validate: func(ctx context.Context, s string) error {
		if err := validate(ctx, s); err != nil {
			return err
		}
		foreign = s
		moved = filepath.Join(parent, "owned-moved")
		if err := os.Rename(s, moved); err != nil {
			t.Skipf("native directory handle rename prerequisite: %v", err)
		}
		if err := os.Mkdir(s, 0755); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(s, "sentinel"), []byte("foreign"), 0644)
	}})
	if err == nil || result.Committed {
		t.Fatal("accepted replaced payload")
	}
	b, e := os.ReadFile(filepath.Join(foreign, "sentinel"))
	if e != nil || string(b) != "foreign" {
		t.Fatalf("foreign payload removed: %v", e)
	}
	assertOnly(t, moved)
}

// Fault the exact write/chmod/close seam while retaining real private files.
type faultOutput struct {
	*os.File
	kind   string
	closed *bool
}

func (f faultOutput) Write(b []byte) (int, error) {
	switch f.kind {
	case "disk-full":
		return 0, syscall.ENOSPC
	case "short-write":
		return 0, nil
	}
	return f.File.Write(b)
}
func (f faultOutput) Chmod(mode fs.FileMode) error {
	if f.kind == "chmod" {
		return fs.ErrPermission
	}
	return f.File.Chmod(mode)
}
func (f faultOutput) Close() error {
	*f.closed = true
	err := f.File.Close()
	if f.kind == "close" {
		return errors.Join(err, fs.ErrPermission)
	}
	return err
}
func TestWriteFaultsCloseHandlesAndCleanStage(t *testing.T) {
	for _, kind := range []string{"open", "disk-full", "short-write", "chmod", "close"} {
		t.Run(kind, func(t *testing.T) {
			parent := tempRoot(t)
			closed := false
			ops := applyOps{rename: renameExclusive, write: func(ctx context.Context, r *os.Root, files []File) error {
				return writeTreeWith(ctx, r, files, func(root *os.Root, name string) (outputFile, error) {
					if kind == "open" {
						return nil, fs.ErrPermission
					}
					f, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
					if err != nil {
						return nil, err
					}
					return faultOutput{f, kind, &closed}, nil
				})
			}}
			result, err := apply(context.Background(), planFor(t, "skill"), ApplyOptions{Destination: filepath.Join(parent, "out"), Validate: realValidation(t)}, ops)
			if err == nil || result.Committed {
				t.Fatalf("%+v %v", result, err)
			}
			if kind != "open" && !closed {
				t.Fatal("output handle not closed on failure")
			}
			if kind == "disk-full" && !errors.Is(err, syscall.ENOSPC) {
				t.Fatal(err)
			}
			if kind == "short-write" && !errors.Is(err, io.ErrShortWrite) {
				t.Fatal(err)
			}
			assertOnly(t, parent)
		})
	}
}

func TestOneCharacterDestination(t *testing.T) {
	dest := filepath.Join(tempRoot(t), "x")
	result, err := Apply(context.Background(), planFor(t, "skill"), ApplyOptions{Destination: dest, Validate: realValidation(t)})
	if err != nil || !result.Committed {
		t.Fatalf("%+v %v", result, err)
	}
}

func TestPostCommitCleanupErrorRetainsCommittedResult(t *testing.T) {
	parent := tempRoot(t)
	dest := filepath.Join(parent, "out")
	validate := realValidation(t)
	var container string
	ops := applyOps{write: writeTree, rename: func(from *os.File, old string, to *os.File, new string) error {
		if err := renameExclusive(from, old, to, new); err != nil {
			return err
		}
		// A distinct container entry prevents empty-container removal. Cleanup must
		// report it without undoing the already committed destination.
		if err := os.WriteFile(filepath.Join(container, "retained"), []byte("retain"), 0600); err != nil {
			t.Fatal(err)
		}
		return nil
	}}
	result, err := apply(context.Background(), planFor(t, "skill"), ApplyOptions{Destination: dest, Validate: func(ctx context.Context, s string) error { container = filepath.Dir(s); return validate(ctx, s) }}, ops)
	var cleanup *CleanupError
	if !errors.As(err, &cleanup) || !result.Committed || result.Destination != dest {
		t.Fatalf("lost committed result: %+v %v", result, err)
	}
	if err := validate(context.Background(), dest); err != nil {
		t.Fatal(err)
	}
	b, readErr := os.ReadFile(filepath.Join(container, "retained"))
	if readErr != nil || string(b) != "retain" {
		t.Fatal("cleanup deleted unrelated container entry")
	}
}

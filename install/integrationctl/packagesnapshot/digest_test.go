package packagesnapshot

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Detect format changes independently of Build, including empty directories and executable intent.
func TestDigestKnownFormatParityAndNoOutput(t *testing.T) {
	t.Parallel()
	source := t.TempDir()
	mustWrite(t, filepath.Join(source, "plugin.json"), []byte("{}"), 0o644)
	mustWrite(t, filepath.Join(source, "bin", "run"), []byte("#!/bin/sh\n"), 0o755)
	if err := os.Mkdir(filepath.Join(source, "empty"), 0o755); err != nil {
		t.Fatal(err)
	}
	want := "sha256:1b3b199aac618cf73b63f83b6faac401f72100f859a0caca75a0903aa5c61156"
	if runtime.GOOS == "windows" {
		// Go reports regular Windows files without executable bits.
		want = "sha256:49a284b86cfbae3cc04367adaacd1fd4e03b280694aea345a8c2c3f64508eefb"
	}
	output := t.TempDir()
	builder := Builder{TempRoot: output}
	for range 2 {
		got, err := builder.Digest(context.Background(), source)
		if err != nil || got != want {
			t.Fatalf("Digest = %q, %v; want %q", got, err, want)
		}
	}
	entries, err := os.ReadDir(output)
	if err != nil || len(entries) != 0 {
		t.Fatalf("Digest output = %v, %v", entries, err)
	}
	snapshot, err := builder.Build(context.Background(), source)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := snapshot.Close(); err != nil {
			t.Errorf("close snapshot: %v", err)
		}
	})
	if snapshot.Digest != want {
		t.Fatalf("Build digest = %q, want %q", snapshot.Digest, want)
	}
	builder.TempRoot = filepath.Join(source, "plugin.json")
	if got, err := builder.Digest(context.Background(), source); err != nil || got != want {
		t.Fatalf("Digest with unusable TempRoot = %q, %v", got, err)
	}
}

func TestDigestFreshBytesAndExecutableIntent(t *testing.T) {
	t.Parallel()
	source := t.TempDir()
	path := filepath.Join(source, "run")
	mustWrite(t, path, []byte("one"), 0o644)
	digest := func() string {
		t.Helper()
		got, err := (Builder{}).Digest(context.Background(), source)
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	first := digest()
	mustWrite(t, path, []byte("two"), 0o644)
	second := digest()
	if first == second {
		t.Fatal("same-size byte change ignored")
	}
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatal(err)
	}
	// Windows Chmod changes only the read-only attribute, not executable intent.
	if runtime.GOOS != "windows" && digest() == second {
		t.Fatal("executable intent change ignored")
	}
}

// Both consumers must refuse unsafe substitutions and size drift after inspection.
func TestBuildAndDigestRejectSourceChanges(t *testing.T) {
	for _, operation := range []string{"Build", "Digest"} {
		for _, change := range []string{"identity", "grow", "shrink", "hardlink", "symlink", "ancestor symlink"} {
			t.Run(operation+"/"+change, func(t *testing.T) {
				if runtime.GOOS == "windows" && strings.Contains(change, "symlink") {
					t.Skip("symlinks may require elevation")
				}
				source := t.TempDir()
				path := filepath.Join(source, "nested", "file")
				mustWrite(t, path, []byte("original"), 0o644)
				builder := Builder{beforeCopy: func(string) error {
					switch change {
					case "identity":
						if err := os.Rename(path, path+"-old"); err != nil {
							return err
						}
						return os.WriteFile(path, []byte("original"), 0o644)
					case "grow":
						return os.WriteFile(path, []byte("original plus"), 0o644)
					case "shrink":
						return os.WriteFile(path, []byte("tiny"), 0o644)
					case "hardlink":
						return os.Link(path, filepath.Join(t.TempDir(), "link"))
					case "symlink":
						if err := os.Rename(path, path+"-old"); err != nil {
							return err
						}
						return os.Symlink(path+"-old", path)
					case "ancestor symlink":
						directory := filepath.Dir(path)
						if err := os.Rename(directory, directory+"-old"); err != nil {
							return err
						}
						return os.Symlink(directory+"-old", directory)
					}
					return nil
				}}
				var err error
				if operation == "Build" {
					var snapshot Snapshot
					snapshot, err = builder.Build(context.Background(), source)
					if err == nil {
						t.Cleanup(func() {
							if closeErr := snapshot.Close(); closeErr != nil {
								t.Errorf("close snapshot: %v", closeErr)
							}
						})
					}
				} else {
					_, err = builder.Digest(context.Background(), source)
				}
				if err == nil {
					t.Fatalf("%s accepted %s", operation, change)
				}
				messages := map[string]string{"identity": "changed identity", "grow": "grew", "shrink": "changed size", "hardlink": "hardlinks are not allowed"}
				if message := messages[change]; message != "" && !strings.Contains(err.Error(), message) {
					t.Fatalf("error = %v, want %q", err, message)
				}
			})
		}
	}
}

func TestDigestLimitsAndCancellation(t *testing.T) {
	t.Parallel()
	source := t.TempDir()
	mustWrite(t, filepath.Join(source, "nested", "one"), []byte("12"), 0o644)
	mustWrite(t, filepath.Join(source, "two"), []byte("34"), 0o644)
	for _, limits := range []Limits{{MaxFiles: 1}, {MaxFileBytes: 1}, {MaxTreeBytes: 3}, {MaxDepth: 1}} {
		if _, err := (Builder{Limits: limits}).Digest(context.Background(), source); err == nil {
			t.Fatalf("limits ignored: %+v", limits)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := (Builder{}).Digest(ctx, source); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled Digest = %v", err)
	}
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	if _, err := (Builder{beforeCopy: func(string) error { cancel(); return nil }}).Digest(ctx, source); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation before read = %v", err)
	}
}

type cancelOnWrite struct{ cancel context.CancelFunc }

func (writer cancelOnWrite) Write(body []byte) (int, error) { writer.cancel(); return len(body), nil }

func TestSharedReadCancellationDuringRead(t *testing.T) {
	t.Parallel()
	source := t.TempDir()
	mustWrite(t, filepath.Join(source, "large"), bytes.Repeat([]byte("x"), 128<<10), 0o644)
	entries, _, _, err := inspectTree(context.Background(), source, (Builder{}).limits())
	if err != nil {
		t.Fatal(err)
	}
	file, err := openValidatedRegular(source, entries[0])
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := file.Close(); err != nil {
			t.Errorf("close source file: %v", err)
		}
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	written, err := readRegularFile(ctx, file, entries[0], cancelOnWrite{cancel: cancel})
	if !errors.Is(err, context.Canceled) || written >= entries[0].size {
		t.Fatalf("canceled read = %d bytes, %v", written, err)
	}
}

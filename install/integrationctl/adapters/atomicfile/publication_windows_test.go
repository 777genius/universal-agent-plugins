//go:build windows

package atomicfile

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWindowsAtomicPublicationHeldReaders(t *testing.T) {
	t.Run("ordinary_reader_remains_a_refusal", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "state.json")
		if err := Write(path, []byte("old complete bytes"), 0600); err != nil {
			t.Fatal(err)
		}
		reader, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		closePublicationReader(t, reader)
		if err := Write(path, []byte("new complete bytes"), 0600); err == nil {
			t.Fatal("publication bypassed an ordinary reader's delete denial")
		}
		requirePublicationBody(t, path, "old complete bytes")
		requirePublicationReader(t, reader, "old complete bytes")
		requireNoPublicationTemps(t, filepath.Dir(path))
	})
	t.Run("production_reader_keeps_old_file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "state.json")
		if err := Write(path, []byte("old complete bytes"), 0600); err != nil {
			t.Fatal(err)
		}
		reader, err := openReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		closePublicationReader(t, reader)
		legacy := filepath.Join(filepath.Dir(path), "legacy-source")
		if err := os.WriteFile(legacy, []byte("legacy candidate"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(legacy, path); err == nil {
			t.Fatal("legacy rename unexpectedly replaced a held destination")
		} else {
			t.Logf("controlled legacy replacement refusal: %v", err)
		}
		requirePublicationBody(t, path, "old complete bytes")
		if err := os.Remove(legacy); err != nil {
			t.Fatal(err)
		}
		if err := Write(path, []byte("new complete bytes"), 0600); err != nil {
			t.Fatal(err)
		}
		requirePublicationReader(t, reader, "old complete bytes")
		requirePublicationBody(t, path, "new complete bytes")
		requireNoPublicationTemps(t, filepath.Dir(path))
	})
}

func TestWindowsAtomicPublicationRefusals(t *testing.T) {
	root := t.TempDir()
	for _, path := range []string{filepath.Join(root, "missing"), filepath.Join(root, "missing-parent", "file")} {
		_, err := ReadFile(path)
		if !os.IsNotExist(err) || !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("missing-read compatibility changed: %T %v", err, err)
		}
	}
	readonly := filepath.Join(root, "readonly")
	if err := Write(readonly, []byte("protected bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(readonly, 0400); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(readonly, 0600); err != nil {
			t.Error(err)
		}
	})
	if err := Write(readonly, []byte("must not replace"), 0600); err == nil {
		t.Fatal("publication bypassed the target's readonly attribute")
	}
	requirePublicationBody(t, readonly, "protected bytes")
	directory := filepath.Join(root, "directory")
	if err := os.Mkdir(directory, 0700); err != nil {
		t.Fatal(err)
	}
	if err := Write(directory, []byte("must not replace"), 0600); err == nil {
		t.Fatal("directory target was accepted")
	}
	info, err := os.Stat(directory)
	if err != nil || !info.IsDir() {
		t.Fatal("directory target changed after refusal")
	}
	source := filepath.Join(root, "source")
	if err := os.WriteFile(source, []byte("source remains"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := Replace(source, filepath.Join(directory, "outside-parent")); err == nil {
		t.Fatal("publication escaped the source parent")
	}
	requirePublicationBody(t, source, "source remains")
	requireNoPublicationTemps(t, root)
}

func TestWindowsAtomicPublicationUnicodeLongPath(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	parent := filepath.Join(strings.Repeat("long-parent-", 10), strings.Repeat("second-parent-", 9), "unicode-é-插件")
	if err := os.MkdirAll(parent, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(parent, "状態.json")
	absolute, err := filepath.Abs(path)
	if err != nil || len(absolute) <= 260 {
		t.Fatal("fixture did not establish a long absolute path")
	}
	if err := Write(path, []byte("old unicode bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	reader, err := openReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	closePublicationReader(t, reader)
	if err := Write(path, []byte("new unicode bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	requirePublicationReader(t, reader, "old unicode bytes")
	requirePublicationBody(t, path, "new unicode bytes")
	requirePublicationBody(t, absolute, "new unicode bytes")
	requireNoPublicationTemps(t, parent)
}

func closePublicationReader(t *testing.T, reader *os.File) {
	t.Helper()
	t.Cleanup(func() {
		if err := reader.Close(); err != nil {
			t.Error(err)
		}
	})
}

func requirePublicationReader(t *testing.T, reader *os.File, want string) {
	t.Helper()
	body, err := io.ReadAll(reader)
	if err != nil || string(body) != want {
		t.Fatalf("retained reader lost complete old bytes: %q %v", body, err)
	}
}

func requirePublicationBody(t *testing.T, path, want string) {
	t.Helper()
	body, err := ReadFile(path)
	if err != nil || string(body) != want {
		t.Fatalf("named file is not the complete expected version: %q %v", body, err)
	}
}

func requireNoPublicationTemps(t *testing.T, parent string) {
	t.Helper()
	entries, err := os.ReadDir(parent)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.Contains(entry.Name(), ".tmp-") {
			t.Fatal("publication left a temporary file")
		}
	}
}

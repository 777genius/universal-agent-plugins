package jsonmaint

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func errorCode(t *testing.T, err error) string {
	t.Helper()
	var target *Error
	if !errors.As(err, &target) {
		t.Fatalf("expected typed error, got %v", err)
	}
	return target.Code
}

func TestBuildCanonicalPreservesNumbersAndRejectsDuplicates(t *testing.T) {
	plan, err := Build("plugin.json", []byte(`{"z":1e+03,"a":{"n":9007199254740993123456789,"m":-0.00}}`))
	if err != nil {
		t.Fatal(err)
	}
	want := "{\n  \"a\": {\n    \"m\": -0.00,\n    \"n\": 9007199254740993123456789\n  },\n  \"z\": 1e+03\n}\n"
	if string(plan.after) != want || !plan.Changed() || plan.Document != "plugin.json" || len(plan.BeforeSHA256) != 64 || len(plan.AfterSHA256) != 64 {
		t.Fatalf("unexpected plan: %+v\n%s", plan, plan.after)
	}
	for _, body := range []string{`{"a":{"x":1,"x":2}}`, `[{"x":1,"x":2}]`} {
		if code := errorCode(t, func() error { _, err := Decode([]byte(body)); return err }()); code != "json_duplicate_key" {
			t.Fatal(code)
		}
	}
	for _, body := range []string{`{"x":1,}`, `// comment\n{"x":1}`} {
		if code := errorCode(t, func() error { _, err := Decode([]byte(body)); return err }()); code != "json_malformed" {
			t.Fatal(code)
		}
	}
}

func TestApplyNoopMtimeSymlinkConcurrentAndRollback(t *testing.T) {
	ctx := context.Background()
	t.Run("noop", func(t *testing.T) {
		root := t.TempDir()
		path := filepath.Join(root, "plugin.json")
		body := []byte("{\n  \"a\": 1\n}\n")
		if err := os.WriteFile(path, body, 0640); err != nil {
			t.Fatal(err)
		}
		old := time.Unix(123456789, 0)
		if err := os.Chtimes(path, old, old); err != nil {
			t.Fatal(err)
		}
		plan, _ := Build("plugin.json", body)
		called := false
		committed, err := Apply(ctx, plan, ApplyOptions{Root: root, Validate: func(context.Context) error { called = true; return nil }})
		info, statErr := os.Stat(path)
		if err != nil || statErr != nil || committed || called || !info.ModTime().Equal(old) {
			t.Fatalf("commit=%t called=%t err=%v mtime=%v", committed, called, err, info.ModTime())
		}
	})
	t.Run("symlink", func(t *testing.T) {
		root := t.TempDir()
		target := filepath.Join(root, "target")
		body := []byte(`{"b":2,"a":1}`)
		if err := os.WriteFile(target, body, 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink("target", filepath.Join(root, "plugin.json")); err != nil {
			t.Fatal(err)
		}
		plan, _ := Build("plugin.json", body)
		if code := errorCode(t, Verify(root, plan)); code != "document_unavailable" {
			t.Fatal(code)
		}
		if committed, err := Apply(ctx, plan, ApplyOptions{Root: root, Validate: func(context.Context) error { return nil }}); committed || errorCode(t, err) != "source_changed" {
			t.Fatalf("commit=%t err=%v", committed, err)
		}
	})
	t.Run("concurrent", func(t *testing.T) {
		root := t.TempDir()
		path := filepath.Join(root, "plugin.json")
		original := []byte(`{"b":2,"a":1}`)
		user := []byte(`{"user":true}`)
		if err := os.WriteFile(path, original, 0600); err != nil {
			t.Fatal(err)
		}
		plan, _ := Build("plugin.json", original)
		if err := os.WriteFile(path, user, 0600); err != nil {
			t.Fatal(err)
		}
		committed, err := Apply(ctx, plan, ApplyOptions{Root: root, Validate: func(context.Context) error { return nil }})
		got, _ := os.ReadFile(path)
		if committed || errorCode(t, err) != "source_changed" || string(got) != string(user) {
			t.Fatalf("commit=%t err=%v body=%s", committed, err, got)
		}
	})
	t.Run("final replacement window restores detected writer", func(t *testing.T) {
		root := t.TempDir()
		path := filepath.Join(root, "plugin.json")
		original := []byte(`{"b":2,"a":1}`)
		user := []byte(`{"external_writer":true}`)
		if err := os.WriteFile(path, original, 0640); err != nil {
			t.Fatal(err)
		}
		plan, _ := Build("plugin.json", original)
		committed, err := apply(ctx, plan, ApplyOptions{Root: root, Validate: func(context.Context) error { return nil }}, func() {
			if removeErr := os.Remove(path); removeErr != nil {
				t.Fatal(removeErr)
			}
			if writeErr := os.WriteFile(path, user, 0600); writeErr != nil {
				t.Fatal(writeErr)
			}
		})
		got, readErr := os.ReadFile(path)
		entries, entriesErr := os.ReadDir(root)
		if committed || errorCode(t, err) != "source_changed" || readErr != nil || entriesErr != nil || !bytes.Equal(got, user) || len(entries) != 1 {
			t.Fatalf("commit=%t err=%v read=%v entries_err=%v body=%s entries=%v", committed, err, readErr, entriesErr, got, entries)
		}
	})
	t.Run("success preserves supported permission bits", func(t *testing.T) {
		root := t.TempDir()
		path := filepath.Join(root, "plugin.json")
		original := []byte(`{"b":2,"a":1}`)
		if err := os.WriteFile(path, original, 0640); err != nil {
			t.Fatal(err)
		}
		plan, _ := Build("plugin.json", original)
		committed, err := Apply(ctx, plan, ApplyOptions{Root: root, Validate: func(context.Context) error { return nil }})
		got, _ := os.ReadFile(path)
		info, _ := os.Stat(path)
		entries, _ := os.ReadDir(root)
		modePreserved := runtime.GOOS == "windows" || info.Mode().Perm() == 0640
		if err != nil || !committed || string(got) != "{\n  \"a\": 1,\n  \"b\": 2\n}\n" || !modePreserved || len(entries) != 1 {
			t.Fatalf("commit=%t err=%v body=%s mode=%o entries=%v", committed, err, got, info.Mode().Perm(), entries)
		}
	})
	t.Run("rollback", func(t *testing.T) {
		root := t.TempDir()
		path := filepath.Join(root, "plugin.json")
		original := []byte(`{"b":2,"a":1}`)
		if err := os.WriteFile(path, original, 0750); err != nil {
			t.Fatal(err)
		}
		plan, _ := Build("plugin.json", original)
		sentinel := errors.New("validation")
		committed, err := Apply(ctx, plan, ApplyOptions{Root: root, Validate: func(context.Context) error { return sentinel }})
		got, _ := os.ReadFile(path)
		info, _ := os.Stat(path)
		entries, _ := os.ReadDir(root)
		modePreserved := runtime.GOOS == "windows" || info.Mode().Perm() == 0750
		if committed || !errors.Is(err, sentinel) || string(got) != string(original) || !modePreserved || len(entries) != 1 || strings.HasPrefix(entries[0].Name(), ".authoring-json-") {
			t.Fatalf("commit=%t err=%v body=%s mode=%o entries=%v", committed, err, got, info.Mode().Perm(), entries)
		}
	})
}

//go:build linux

package pluginkitairepo_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestCursorProjectCaptureCommandAuthority(t *testing.T) {
	f := &cursorProjectAttempt{base: &nativeFixture{Project: "/owned/A", Package: "/owned/source"}, client: "/pin/dist-package/cursor-agent", installer: "/owned/installer", git: "/usr/bin/git", other: "/owned/B", outside: "/owned/outside"}
	want := []cursorProjectCommand{
		{"git-a", "/owned/A", []string{"/usr/bin/git", "init", "--quiet", "--initial-branch=main"}},
		{"git-b", "/owned/B", []string{"/usr/bin/git", "init", "--quiet", "--initial-branch=main"}},
		{"version", "/owned/outside", []string{"/pin/dist-package/cursor-agent", "--version"}},
		{"empty-a-mcp", "/owned/A", []string{"/pin/dist-package/cursor-agent", "mcp", "list"}},
		{"empty-b-mcp", "/owned/B", []string{"/pin/dist-package/cursor-agent", "mcp", "list"}},
		{"empty-outside-mcp", "/owned/outside", []string{"/pin/dist-package/cursor-agent", "mcp", "list"}},
		{"add-a", "/owned/A", []string{"/owned/installer", "add", "/owned/source", "--no-save", "--scope", "project", "--project-root", "/owned/A", "--target", "cursor", "--format", "json"}},
		{"native-attempt", "/owned/A", []string{"/pin/dist-package/cursor-agent", "mcp", "list"}},
		{"control-b-mcp", "/owned/B", []string{"/pin/dist-package/cursor-agent", "mcp", "list"}},
		{"control-outside-mcp", "/owned/outside", []string{"/pin/dist-package/cursor-agent", "mcp", "list"}},
	}
	if !reflect.DeepEqual(f.commands(), want) {
		t.Fatal("Cursor first component call authority changed")
	}
}
func TestCursorProjectCaptureHeldAndAbsentAuthority(t *testing.T) {
	for _, change := range []string{"bytes", "replacement", "alias", "hardlink", "other-source"} {
		t.Run(change, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "held")
			projectNativeRuntimeWrite(t, path, []byte("held bytes"), 0600)
			f := &cursorProjectAttempt{held: map[string]cursorProjectHeld{}, absent: []string{filepath.Join(root, "absent")}}
			if err := f.hold(path); err != nil {
				t.Fatal(err)
			}
			if err := f.check(); err != nil {
				t.Fatal(err)
			}
			switch change {
			case "bytes":
				if err := os.WriteFile(path, []byte("changed"), 0600); err != nil {
					t.Fatal(err)
				}
			case "replacement", "alias":
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				if change == "replacement" {
					projectNativeRuntimeWrite(t, path, []byte("held bytes"), 0600)
				} else if err := os.Symlink(f.absent[0], path); err != nil {
					t.Fatal(err)
				}
			case "hardlink":
				if err := os.Link(path, filepath.Join(root, "alias")); err != nil {
					t.Fatal(err)
				}
			case "other-source":
				projectNativeRuntimeWrite(t, f.absent[0], []byte("{}"), 0600)
			}
			if f.check() == nil {
				t.Fatal("changed Cursor held/other-source authority accepted")
			}
		})
	}
}
func TestCursorProjectCaptureRuntimeClosure(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "member")
	projectNativeRuntimeWrite(t, path, []byte("runtime"), 0600)
	f := &cursorProjectAttempt{runtimeRoot: root, runtimeMembers: map[string]string{path: "file"}}
	if err := f.runtimeClosure(); err != nil {
		t.Fatal(err)
	}
	unknown := filepath.Join(root, "unknown")
	projectNativeRuntimeWrite(t, unknown, []byte("extra"), 0600)
	if f.runtimeClosure() == nil {
		t.Fatal("unknown runtime file accepted")
	}
	if err := os.Remove(unknown); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(unknown, path); err != nil {
		t.Fatal(err)
	}
	if f.runtimeClosure() == nil {
		t.Fatal("aliased runtime member accepted")
	}
}

func TestCursorProjectCaptureMaterializedDirectoryAuthority(t *testing.T) {
	root := t.TempDir()
	bundle, leaf := filepath.Join(root, "bundle"), filepath.Join(root, "bundle", "client")
	empty := filepath.Join(root, "bundle", "empty")
	archive := map[string]string{bundle: "directory", leaf: "file", empty: "directory", filepath.Join(empty, "nested"): "directory"}
	expected := map[string]string{bundle: "directory", leaf: "file"}
	members, err := cursorProjectMaterializedMembers(root, archive)
	if err != nil || !reflect.DeepEqual(members, expected) {
		t.Fatal("materialized authority differs from literal file/parent set", err)
	}
	projectNativeRuntimeWrite(t, leaf, []byte("inert runtime"), 0600)
	f := &cursorProjectAttempt{runtimeRoot: root, runtimeMembers: members}
	if err := f.runtimeClosure(); err != nil {
		t.Fatal("unmaterialized empty archive directories were required", err)
	}
	if err := os.Mkdir(empty, 0700); err != nil {
		t.Fatal(err)
	}
	if f.runtimeClosure() == nil {
		t.Fatal("observed empty archive directory was adopted into authority")
	}
	if err := os.Remove(empty); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(leaf); err != nil {
		t.Fatal(err)
	}
	if f.runtimeClosure() == nil {
		t.Fatal("removed required runtime file accepted")
	}
	for _, unsafe := range []string{root, filepath.Dir(root)} {
		if _, err := cursorProjectMaterializedMembers(root, map[string]string{unsafe: "directory"}); err == nil {
			t.Fatal("root or parent archive directory was silently ignored")
		}
	}
	for _, kind := range []string{"", "file"} {
		archive[bundle] = kind
		if _, err := cursorProjectMaterializedMembers(root, archive); err == nil {
			t.Fatal("missing/non-directory trusted parent accepted")
		}
	}
}

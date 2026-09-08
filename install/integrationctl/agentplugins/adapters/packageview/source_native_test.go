//go:build (darwin && arm64) || (windows && amd64)

package packageview

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func nativeWrite(t *testing.T, root, name, body string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(name))
	if e := os.MkdirAll(filepath.Dir(p), 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(p, []byte(body), 0600); e != nil {
		t.Fatal(e)
	}
}
func nativeLink(t *testing.T, root, target, name string) {
	t.Helper()
	if e := os.Symlink(filepath.FromSlash(target), filepath.Join(root, name)); e != nil {
		if nativeLinkPrivilegeError(e) {
			t.Skipf("UNPROVEN native symlink privilege gate: %v", e)
		}
		t.Fatal(e)
	}
}
func nativeSource(t *testing.T, root string) *source {
	t.Helper()
	s, e := openSource(root)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		if e := s.close(); e != nil {
			t.Error(e)
		}
	})
	return s
}
func TestNativeCaptureAndCleanup(t *testing.T) {
	root := nativeFixture(t, func(root string) {
		nativeWrite(t, root, "plugin.json", `{"name":"fixture"}`)
		nativeWrite(t, root, "mcp.json", `{}`)
		nativeWrite(t, root, "skills/a/SKILL.md", "# inert\n")
		nativeWrite(t, root, "opaque", "bytes")
	})
	scratch := t.TempDir()
	l, e := (Reader{TempDir: scratch}).Open(context.Background(), root)
	if e != nil {
		t.Fatal(e)
	}
	defer l.Close()
	if l.Data().Plugin.State != Present || l.Data().Coverage.ComponentsRequested {
		t.Fatalf("bad core stage: %+v", l.Data())
	}
	in, e := l.Capture(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	if in.MCP.State != Present || len(in.Skills) != 1 || !in.Coverage.InventoryComplete || !in.Coverage.TreeComplete {
		t.Fatalf("incomplete capture: %+v", in)
	}
	if e := l.Close(); e != nil {
		t.Fatal(e)
	}
	if e := l.Close(); e != nil {
		t.Fatal(e)
	}
	entries, e := os.ReadDir(scratch)
	if e != nil || len(entries) != 0 {
		t.Fatalf("scratch not cleaned: %v %v", entries, e)
	}
}
func TestNativeTraversalOrderAndBoundedLinks(t *testing.T) {
	root := nativeFixture(t, func(root string) {
		nativeWrite(t, root, "plugin.json", "core")
		nativeWrite(t, root, "dir/target", "contained")
		nativeLink(t, root, "dir/target", "ok")
		nativeLink(t, root, "dir/../plugin.json", "up")
		nativeLink(t, root, "../outside", "escape")
		nativeLink(t, root, ".", "a")
		nativeLink(t, root, "a/../plugin.json", "order")
		nativeLink(t, root, "cycle", "cycle")
		nativeLink(t, root, filepath.Join(root, "plugin.json"), "absolute")
	})
	s := nativeSource(t, root)
	for _, n := range []string{"ok", "up"} {
		p, e := s.pin(n, false)
		if e != nil {
			t.Fatalf("contained %s: %v", n, e)
		}
		f, e := p.reopen(false)
		p.file.Close()
		if e != nil {
			t.Fatal(e)
		}
		b, e := io.ReadAll(io.LimitReader(f, 100))
		f.Close()
		if e != nil || len(b) == 0 {
			t.Fatal("empty contained file", e)
		}
	}
	for _, n := range []string{"escape", "order", "cycle", "absolute", "../plugin.json"} {
		p, e := s.pin(n, false)
		if e == nil {
			p.file.Close()
			t.Fatalf("escaped/followed %s", n)
		}
		if stateOf(e) != Blocked {
			t.Fatalf("%s: expected blocked, got %v", n, e)
		}
	}
	p, e := s.pin("ok", true)
	if e != nil {
		t.Fatal(e)
	}
	defer p.file.Close()
	target, e := p.link(100)
	if e != nil || filepath.ToSlash(target) != "dir/target" {
		t.Fatalf("link = %q, %v", target, e)
	}
	if _, e := p.link(2); e == nil {
		t.Fatal("link limit ignored")
	}
	if f, e := p.reopen(false); e == nil {
		f.Close()
		t.Fatal("link acquired data handle")
	}
}
func TestNativeLegacyMetadataAndHardlinks(t *testing.T) {
	root := nativeFixture(t, func(root string) {
		nativeWrite(t, root, "plugin/plugin.yaml", "DO NOT READ LEGACY")
		nativeWrite(t, root, "plugin.json", "core")
		nativeWrite(t, root, "ordinary", "data")
		if e := os.Link(filepath.Join(root, "plugin", "plugin.yaml"), filepath.Join(root, "mcp.json")); e != nil {
			t.Fatal(e)
		}
		if e := os.Link(filepath.Join(root, "ordinary"), filepath.Join(root, "alias")); e != nil {
			t.Fatal(e)
		}
	})
	scratch := t.TempDir()
	opened := map[string]bool{}
	l, e := (Reader{TempDir: scratch}).open(context.Background(), root, &captureHooks{beforeDataOpen: func(p string) { opened[p] = true }})
	if e != nil {
		t.Fatal(e)
	}
	defer l.Close()
	if l.Data().Legacy != Present {
		t.Fatal("legacy metadata missing")
	}
	in, e := l.Capture(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	if in.MCP.State != Blocked || in.Coverage.TreeComplete {
		t.Fatalf("legacy/hardlink capture: %+v", in)
	}
	for _, n := range []string{"plugin/plugin.yaml", "mcp.json", "ordinary", "alias"} {
		if opened[n] {
			t.Fatalf("data-opened excluded %s", n)
		}
	}
}
func TestNativeLegacySymlinkAlias(t *testing.T) {
	root := nativeFixture(t, func(root string) {
		nativeWrite(t, root, "plugin/plugin.yaml", "DO NOT READ LEGACY")
		nativeLink(t, root, "plugin/plugin.yaml", "plugin.json")
	})
	l, e := (Reader{TempDir: t.TempDir()}).open(context.Background(), root, &captureHooks{beforeDataOpen: func(p string) { t.Fatalf("data-opened legacy alias %s", p) }})
	if e != nil {
		t.Fatal(e)
	}
	defer l.Close()
	if l.Data().Plugin.State != Blocked || l.Data().Legacy != Present {
		t.Fatalf("legacy alias: %+v", l.Data())
	}
}
func TestNativeFailureCleanup(t *testing.T) {
	root := nativeFixture(t, func(root string) { nativeWrite(t, root, "plugin.json", "too long") })
	scratch := t.TempDir()
	_, e := (Reader{TempDir: scratch, Limits: Limits{PluginBytes: 2}}).Open(context.Background(), root)
	var safe *Error
	if !errors.As(e, &safe) || safe.Code != "byte_limit" {
		t.Fatalf("limit: %v", e)
	}
	entries, e := os.ReadDir(scratch)
	if e != nil || len(entries) != 0 {
		t.Fatal("failure leaked scratch", e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, e = (Reader{TempDir: scratch}).Open(ctx, root)
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
func TestNativeRootLinkRejected(t *testing.T) {
	root := nativeFixture(t, func(root string) { nativeWrite(t, root, "plugin.json", "core") })
	link := filepath.Join(t.TempDir(), "root-link")
	if e := os.Symlink(root, link); e != nil {
		if nativeLinkPrivilegeError(e) {
			t.Skipf("UNPROVEN root symlink privilege gate: %v", e)
		}
		t.Fatal(e)
	}
	for _, p := range []string{link, link + string(filepath.Separator)} {
		s, e := openSource(p)
		if e == nil {
			s.close()
			t.Fatal("accepted root link", p)
		}
	}
}

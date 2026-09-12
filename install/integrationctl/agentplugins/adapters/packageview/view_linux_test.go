//go:build linux

package packageview

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"golang.org/x/sys/unix"
)

const pluginBody = `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"fixture","version":"not-semver"}`
const mcpBody = `{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"docs":{"type":"streamable-http","url":"https://fixture.invalid/mcp"}}}`
const skillBody = "---\nname: helper\ndescription: Fixture only\n---\nOffline instructions.\n"

func put(t *testing.T, root, name, body string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(name))
	if e := os.MkdirAll(filepath.Dir(p), 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(p, []byte(body), 0600); e != nil {
		t.Fatal(e)
	}
}
func link(t *testing.T, root, name, target string) {
	t.Helper()
	if e := os.Symlink(target, filepath.Join(root, name)); e != nil {
		t.Fatal(e)
	}
}
func fixture(t *testing.T) (string, Reader) {
	t.Helper()
	root := t.TempDir()
	put(t, root, "plugin.json", pluginBody)
	return root, Reader{TempDir: t.TempDir()}
}
func open(t *testing.T, r Reader, root string, h *captureHooks) *Lease {
	t.Helper()
	l, e := r.open(context.Background(), root, h)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		if e := l.Close(); e != nil {
			t.Error(e)
		}
	})
	return l
}
func capture(t *testing.T, l *Lease) Input {
	t.Helper()
	v, e := l.Capture(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	return v
}
func code(t *testing.T, e error, want string) {
	t.Helper()
	var v *Error
	if !errors.As(e, &v) || v.Code != want {
		t.Fatalf("error = %v, want %s", e, want)
	}
}
func empty(t *testing.T, p string) {
	t.Helper()
	v, e := os.ReadDir(p)
	if e != nil || len(v) != 0 {
		t.Fatalf("private parent not empty: %v, %v", v, e)
	}
}
func observation(t *testing.T, v Input, p string) Observation {
	t.Helper()
	for _, o := range v.Inventory {
		if o.Path == p {
			return o
		}
	}
	t.Fatalf("no observation for %q", p)
	return Observation{}
}
func finding(v Input, code string) bool {
	for _, f := range v.Findings {
		if f.Code == code {
			return true
		}
	}
	return false
}

func TestRegularCaptureLeaseAndSourceIndependence(t *testing.T) {
	root, r := fixture(t)
	put(t, root, "mcp.json", mcpBody)
	put(t, root, "skills/helper/SKILL.md", skillBody)
	put(t, root, "skills/empty/asset", "opaque")
	put(t, root, "skills/helper/deeper/SKILL.md", "not discovered")
	put(t, root, "bin/helper", "never executed")
	if e := os.Chmod(filepath.Join(root, "bin/helper"), 0700); e != nil {
		t.Fatal(e)
	}
	profile := t.TempDir()
	put(t, profile, "client/state", "unchanged")
	before := fixtureState(t, root)
	l := open(t, r, root, nil)
	initial := l.Data()
	if initial.Coverage.ComponentsRequested || initial.MCP.State != Blocked || initial.Plugin.State != Present || initial.Identity.TreeDigest != "" {
		t.Fatalf("eager capture: %+v", initial.Coverage)
	}
	if !finding(initial, "complete_tree_identity_unavailable") {
		t.Fatal("initial partial identity not disclosed")
	}
	v := capture(t, l)
	if string(v.Plugin.Bytes) != pluginBody || string(v.MCP.Bytes) != mcpBody || len(v.Skills) != 2 || v.Skills[0].Document.State != Absent || string(v.Skills[1].Document.Bytes) != skillBody {
		t.Fatal("wrong document records")
	}
	if !v.Coverage.TreeComplete || !v.Coverage.InventoryComplete || !v.Coverage.SkillsEnumerated || v.Identity.TreeAlgorithm != TreeAlgorithm || v.Identity.Digest == initial.Identity.Digest {
		t.Fatalf("coverage/identity: %+v %+v", v.Coverage, v.Identity)
	}
	if finding(v, "complete_tree_identity_unavailable") {
		t.Fatal("complete capture retained stale warning")
	}
	if !observation(t, v, "bin/helper").Executable {
		t.Fatal("lost executable mode")
	}
	// Source file bytes, modes, sizes and timestamps are unchanged, including atime.
	if after := fixtureState(t, root); !reflect.DeepEqual(before, after) {
		t.Fatal("source changed during capture")
	}
	b, e := os.ReadFile(filepath.Join(profile, "client/state"))
	if e != nil || string(b) != "unchanged" {
		t.Fatal("fixture profile changed")
	}
	private := l.private
	info, e := os.Stat(private)
	if e != nil || info.Mode().Perm() != 0500 {
		t.Fatal("private lease not sealed")
	}
	files, e := os.ReadDir(private)
	if e != nil || len(files) == 0 {
		t.Fatal("missing private bytes")
	}
	for _, f := range files {
		info, e := f.Info()
		if e != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0400 {
			t.Fatal("unsafe private materialization")
		}
	}
	original := l.Data()
	v.Plugin.Bytes[0] = '!'
	v.Skills[1].Document.Bytes[0] = '!'
	v.Inventory[0].Path = "changed"
	if !reflect.DeepEqual(original, l.Data()) {
		t.Fatal("Data exposed mutable storage")
	}
	put(t, root, "plugin.json", "changed after capture")
	again := capture(t, l)
	if !reflect.DeepEqual(again, original) {
		t.Fatal("completed capture reopened source")
	}
	if e := l.Close(); e != nil {
		t.Fatal(e)
	}
	if e := l.Close(); e != nil {
		t.Fatal(e)
	}
	if _, e := os.Lstat(private); !os.IsNotExist(e) {
		t.Fatal("private root remains")
	}
	empty(t, r.TempDir)
	if !reflect.DeepEqual(l.Data(), original) {
		t.Fatal("close invalidated returned byte evidence")
	}
	_, e = l.Capture(context.Background())
	code(t, e, "lease_closed")
}

// fixtureState only reads fixture files. Atime is observed after bytes are read;
// the capture itself must use O_NOATIME. Directories are compared independently
// of test enumeration atime.
func fixtureState(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	e := filepath.WalkDir(root, func(p string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if !d.Type().IsRegular() {
			return nil
		}
		f, e := os.OpenFile(p, os.O_RDONLY|unix.O_NOATIME, 0)
		if e != nil {
			return e
		}
		b, e := io.ReadAll(f)
		ce := f.Close()
		if ce != nil {
			return ce
		}
		if e != nil {
			return e
		}
		i, e := os.Stat(p)
		if e != nil {
			return e
		}
		s := i.Sys().(*syscall.Stat_t)
		out[strings.TrimPrefix(p, root)] = fmt.Sprintf("%x/%v/%v/%v/%v", b, i.Mode(), s.Mtim, s.Ctim, s.Atim)
		return nil
	})
	if e != nil {
		t.Fatal(e)
	}
	return out
}

func TestExactRootAndCoreFirst(t *testing.T) {
	parent := t.TempDir()
	put(t, parent, "plugin.json", pluginBody)
	root := filepath.Join(parent, "selected")
	if e := os.Mkdir(root, 0700); e != nil {
		t.Fatal(e)
	}
	put(t, root, "nested/plugin.json", pluginBody)
	put(t, root, ".codex-plugin/plugin.json", pluginBody)
	put(t, root, "plugin/plugin.yaml", "SECRET LEGACY")
	r := Reader{TempDir: t.TempDir()}
	l := open(t, r, root, &captureHooks{metadata: func(p string) error {
		if p != "plugin.json" {
			t.Fatalf("eager metadata %q", p)
		}
		return nil
	}, beforeDataOpen: func(p string) { t.Fatalf("unexpected read %q", p) }})
	v := l.Data()
	if v.Plugin.State != Absent || v.Legacy != Present || v.Identity.ScopeID != ScopeID || v.Coverage.ComponentsRequested {
		t.Fatal("discovery or fallback")
	}
	_, e := l.Capture(context.Background())
	code(t, e, "core_unavailable")
	if l.Data().Identity.Digest != "" {
		t.Fatal("failed capture retained stale identity")
	}
	empty(t, r.TempDir)
	// Unsupported/malformed core bytes are deliberately not parsed by the adapter.
	// A core decoder can stop here without triggering broad caches or components.
	put(t, root, "plugin.json", `{"$schema":"future"}`)
	l = open(t, r, root, &captureHooks{metadata: func(p string) error {
		if p != "plugin.json" {
			t.Fatalf("eager metadata %q", p)
		}
		return nil
	}})
	if len(l.contents) != 1 {
		t.Fatal("core open scanned tree")
	}
	if e := l.Close(); e != nil {
		t.Fatal(e)
	}
}

func TestDocumentAvailabilityPreservesSiblings(t *testing.T) {
	for _, state := range []State{Absent, Unreadable, WrongKind, Blocked, Present} {
		t.Run(string(state), func(t *testing.T) {
			root, r := fixture(t)
			put(t, root, "skills/helper/SKILL.md", skillBody)
			h := &captureHooks{}
			switch state {
			case Unreadable:
				put(t, root, "mcp.json", mcpBody)
				h.metadata = func(p string) error {
					if p == "mcp.json" {
						return &os.PathError{Op: "stat", Path: "/secret/absolute", Err: syscall.EACCES}
					}
					return nil
				}
			case WrongKind:
				if e := os.Mkdir(filepath.Join(root, "mcp.json"), 0700); e != nil {
					t.Fatal(e)
				}
			case Blocked:
				link(t, root, "mcp.json", "../escape")
			case Present:
				put(t, root, "mcp.json", "malformed bytes remain facts")
			}
			v := capture(t, open(t, r, root, h))
			if v.MCP.State != state || v.Plugin.State != Present || len(v.Skills) != 1 || v.Skills[0].Document.State != Present {
				t.Fatalf("wrong availability: %s", v.MCP.State)
			}
			if state != Absent && state != Present && !finding(v, "document_"+string(state)) {
				t.Fatal("missing host finding")
			}
			if state == Unreadable && (v.Coverage.TreeComplete || v.Coverage.InventoryComplete) {
				t.Fatal("unreadable covered")
			}
			b, e := json.Marshal(v)
			if e != nil || bytes.Contains(b, []byte("/secret")) || bytes.Contains(b, []byte(root)) || bytes.Contains(b, []byte("malformed bytes")) {
				t.Fatal("private material leaked")
			}
		})
	}
}

func TestRootAndComponentLinksUseTraversalOrder(t *testing.T) {
	t.Run("root link rejected", func(t *testing.T) {
		root, r := fixture(t)
		alias := filepath.Join(t.TempDir(), "alias")
		if e := os.Symlink(root, alias); e != nil {
			t.Fatal(e)
		}
		_, e := r.Open(context.Background(), alias)
		code(t, e, "root_wrong_kind")
		_, e = r.Open(context.Background(), alias+"/")
		code(t, e, "root_wrong_kind")
		empty(t, r.TempDir)
	})
	t.Run("contained", func(t *testing.T) {
		root, r := fixture(t)
		if e := os.Rename(filepath.Join(root, "plugin.json"), filepath.Join(root, "core")); e != nil {
			t.Fatal(e)
		}
		put(t, root, "components/mcp", mcpBody)
		put(t, root, "assets/helper/SKILL.md", skillBody)
		link(t, root, "plugin.json", "core")
		link(t, root, "mcp.json", "components/mcp")
		link(t, root, "skills", "assets")
		v := capture(t, open(t, r, root, nil))
		if v.Plugin.State != Present || v.MCP.State != Present || len(v.Skills) != 1 || v.Skills[0].Document.State != Present || !v.Coverage.TreeComplete {
			t.Fatal("contained links rejected")
		}
		if observation(t, v, "plugin.json").Target != "core" {
			t.Fatal("lost physical link text")
		}
	})
	t.Run("counterexample", func(t *testing.T) {
		root, r := fixture(t)
		put(t, root, "x", "inside")
		put(t, filepath.Dir(root), "x", "outside sentinel")
		link(t, root, "a", ".")
		link(t, root, "b", "a/../x")
		link(t, root, "mcp.json", "b")
		v := capture(t, open(t, r, root, nil))
		if v.MCP.State != Blocked || observation(t, v, "b").State != Blocked || v.Coverage.TreeComplete {
			t.Fatal("lexical containment accepted escape")
		}
	})
	t.Run("escaping ancestor", func(t *testing.T) {
		root, r := fixture(t)
		outside := t.TempDir()
		put(t, outside, "helper/SKILL.md", "outside")
		link(t, root, "skills", outside)
		v := capture(t, open(t, r, root, nil))
		if v.SkillsRoot != Blocked || v.Coverage.SkillsEnumerated {
			t.Fatal("escaping skills root accepted")
		}
	})
	t.Run("ordered root ancestor", func(t *testing.T) {
		parent := t.TempDir()
		put(t, parent, "x/plugin.json", pluginBody)
		if e := os.Mkdir(filepath.Join(parent, "sub"), 0700); e != nil {
			t.Fatal(e)
		}
		link(t, parent, "sub/a", ".")
		// Go's Abs/Clean would incorrectly select sub/x. Kernel traversal selects x.
		exact := parent + "/sub/a/../x"
		r := Reader{TempDir: t.TempDir()}
		v := capture(t, open(t, r, exact, nil))
		if v.Plugin.State != Present {
			t.Fatal("root lexically cleaned")
		}
	})
	for _, target := range []string{"missing", "cycle", "plain/child"} {
		t.Run(target, func(t *testing.T) {
			root, r := fixture(t)
			put(t, root, "plain", "file")
			link(t, root, "cycle", "cycle")
			link(t, root, "mcp.json", target)
			v := capture(t, open(t, r, root, nil))
			if v.MCP.State == Present || v.Coverage.TreeComplete {
				t.Fatal("unsafe link covered")
			}
		})
	}
}

func TestLegacyNeverOpenedIncludingAliases(t *testing.T) {
	for _, kind := range []string{"regular", "fifo", "symlink", "hardlink", "ancestor-link"} {
		t.Run(kind, func(t *testing.T) {
			root, r := fixture(t)
			if e := os.Mkdir(filepath.Join(root, "plugin"), 0700); e != nil {
				t.Fatal(e)
			}
			switch kind {
			case "fifo":
				if e := unix.Mkfifo(filepath.Join(root, "plugin/plugin.yaml"), 0600); e != nil {
					t.Fatal(e)
				}
			case "symlink":
				put(t, root, "secret", "LEGACY SECRET")
				link(t, root, "plugin/plugin.yaml", "../secret")
			case "ancestor-link":
				if e := os.Remove(filepath.Join(root, "plugin")); e != nil {
					t.Fatal(e)
				}
				put(t, root, "legacy/plugin.yaml", "LEGACY SECRET")
				link(t, root, "plugin", "legacy")
			default:
				put(t, root, "plugin/plugin.yaml", "LEGACY SECRET")
			}
			link(t, root, "mcp.json", "plugin/plugin.yaml")
			link(t, root, "alias", "plugin/plugin.yaml")
			if kind == "hardlink" {
				if e := os.Link(filepath.Join(root, "plugin/plugin.yaml"), filepath.Join(root, "hard")); e != nil {
					t.Fatal(e)
				}
			}
			v := capture(t, open(t, r, root, &captureHooks{beforeDataOpen: func(p string) {
				if p != "plugin.json" {
					t.Fatalf("legacy alias data open: %q", p)
				}
			}}))
			if v.Coverage.TreeComplete || v.Identity.TreeDigest != "" || !finding(v, "legacy_manifest_ignored") || !finding(v, "complete_tree_identity_unavailable") {
				t.Fatal("legacy incorrectly covered")
			}
			if v.MCP.State == Present {
				t.Fatal("alias used legacy bytes")
			}
		})
	}
	t.Run("escaping legacy metadata", func(t *testing.T) {
		root, r := fixture(t)
		link(t, root, "plugin", t.TempDir())
		l := open(t, r, root, &captureHooks{beforeDataOpen: func(string) { t.Fatal("ambiguous legacy guard read data") }})
		if l.Data().Legacy != Blocked || l.Data().Plugin.State != Blocked {
			t.Fatal("unsafe legacy metadata accepted")
		}
	})
}

func TestOpaqueInstallerRestrictionsKeepDocuments(t *testing.T) {
	for _, name := range []string{"CON.txt", "trailing.", "a:b", "README", "e\u0301", "bad\xff", "line\nbreak", "nested/.git/file", ".plugin-kit-ai.lock/child", "pointer"} {
		t.Run(fmt.Sprintf("%x", name), func(t *testing.T) {
			root, r := fixture(t)
			body := "opaque"
			if name == "README" {
				put(t, root, "readme", "collision")
			}
			if name == "pointer" {
				body = "version https://git-lfs.github.com/spec/v1\noid sha256:fixture\n"
			}
			put(t, root, name, body)
			put(t, root, "mcp.json", mcpBody)
			put(t, root, "skills/helper/SKILL.md", skillBody)
			v := capture(t, open(t, r, root, nil))
			if v.Plugin.State != Present || v.MCP.State != Present || v.Skills[0].Document.State != Present || !observation(t, v, name).Captured {
				t.Fatal("policy erased safe facts")
			}
			if v.Coverage.TreeComplete || v.Identity.TreeDigest != "" || !finding(v, "tree_digest_policy_unavailable") {
				t.Fatal("installer restriction not separate")
			}
		})
	}
}

func TestBudgetsCancellationAndCleanup(t *testing.T) {
	for _, tc := range []struct {
		name   string
		limits Limits
		setup  func(*testing.T, string)
		want   string
	}{
		{"core", Limits{PluginBytes: 1}, nil, "byte_limit"},
		{"file", Limits{FileBytes: int64(len(pluginBody))}, func(t *testing.T, r string) { put(t, r, "opaque", strings.Repeat("x", len(pluginBody)+1)) }, "byte_limit"},
		{"tree", Limits{TotalBytes: int64(len(pluginBody))}, func(t *testing.T, r string) { put(t, r, "opaque", "x") }, "byte_limit"},
		{"docs", Limits{DocumentBytes: int64(len(pluginBody))}, func(t *testing.T, r string) { put(t, r, "mcp.json", "x") }, "byte_limit"},
		{"entries", Limits{Entries: 2}, func(t *testing.T, r string) { put(t, r, "cache/a", "x"); put(t, r, "cache/b", "x") }, "entry_limit"},
		{"depth", Limits{Depth: 1}, func(t *testing.T, r string) { put(t, r, "cache/a", "x") }, "depth_limit"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, r := fixture(t)
			r.Limits = tc.limits
			if tc.setup != nil {
				tc.setup(t, root)
			}
			l, e := r.Open(context.Background(), root)
			if e == nil {
				_, e = l.Capture(context.Background())
			}
			code(t, e, tc.want)
			empty(t, r.TempDir)
		})
	}
	t.Run("exact byte budgets", func(t *testing.T) {
		root, r := fixture(t)
		r.Limits = Limits{PluginBytes: int64(len(pluginBody)), FileBytes: int64(len(pluginBody)), TotalBytes: int64(len(pluginBody)), DocumentBytes: int64(len(pluginBody)), Entries: 1, Depth: 1}
		if !capture(t, open(t, r, root, nil)).Coverage.TreeComplete {
			t.Fatal("exact budget rejected")
		}
	})
	t.Run("cancel before open", func(t *testing.T) {
		root, r := fixture(t)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, e := r.Open(ctx, root)
		if !errors.Is(e, context.Canceled) {
			t.Fatal(e)
		}
		empty(t, r.TempDir)
	})
	t.Run("cancel during read", func(t *testing.T) {
		root, r := fixture(t)
		ctx, cancel := context.WithCancel(context.Background())
		_, e := r.open(ctx, root, &captureHooks{afterChunk: func(string) { cancel() }})
		if !errors.Is(e, context.Canceled) {
			t.Fatal(e)
		}
		empty(t, r.TempDir)
	})
	t.Run("cancel inventory", func(t *testing.T) {
		root, r := fixture(t)
		ctx, cancel := context.WithCancel(context.Background())
		l := open(t, r, root, &captureHooks{metadata: func(p string) error {
			if p == "." {
				cancel()
			}
			return nil
		}})
		_, e := l.Capture(ctx)
		if !errors.Is(e, context.Canceled) {
			t.Fatal(e)
		}
		empty(t, r.TempDir)
	})
	t.Run("panic unwind open", func(t *testing.T) {
		root, r := fixture(t)
		func() {
			defer func() {
				if recover() == nil {
					t.Error("panic swallowed")
				}
			}()
			_, _ = r.open(context.Background(), root, &captureHooks{afterChunk: func(string) { panic("fixture") }})
		}()
		empty(t, r.TempDir)
	})
	t.Run("scratch overlap", func(t *testing.T) {
		root, r := fixture(t)
		r.TempDir = root
		_, e := r.Open(context.Background(), root)
		code(t, e, "scratch_overlaps_source")
		if len(fixtureState(t, root)) != 1 {
			t.Fatal("wrote to source")
		}
	})
}

func TestCleanupAuthorityAndErrors(t *testing.T) {
	var zero Lease
	if e := zero.Close(); e != nil {
		t.Fatal(e)
	}
	root, r := fixture(t)
	l, e := r.Open(context.Background(), root)
	if e != nil {
		t.Fatal(e)
	}
	owned := l.private
	displaced := owned + "-displaced"
	if e := os.Rename(owned, displaced); e != nil {
		t.Fatal(e)
	}
	if e := os.Mkdir(owned, 0700); e != nil {
		t.Fatal(e)
	}
	put(t, owned, "unowned", "preserve")
	first := l.Close()
	code(t, first, "cleanup_failed")
	if l.Close() != first {
		t.Fatal("non-idempotent cleanup")
	}
	if b, e := os.ReadFile(filepath.Join(owned, "unowned")); e != nil || string(b) != "preserve" {
		t.Fatal("removed replacement directory")
	}
	if e := os.RemoveAll(displaced); e != nil {
		t.Fatal(e)
	}
	root, r = fixture(t)
	l, e = r.open(context.Background(), root, &captureHooks{cleanup: func() error { return errors.New("/secret/raw error") }})
	if e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, e = l.Capture(ctx)
	var safe *Error
	if !errors.As(e, &safe) || !safe.CleanupFailed || !errors.Is(e, context.Canceled) || strings.Contains(e.Error(), "/secret") {
		t.Fatal("lost or leaked cleanup failure")
	}
	if e := os.RemoveAll(l.private); e != nil {
		t.Fatal(e)
	}
}

func TestFullDigestParityOnSafePrivateBytes(t *testing.T) {
	root, r := fixture(t)
	put(t, root, "mcp.json", mcpBody)
	put(t, root, "skills/helper/SKILL.md", skillBody)
	put(t, root, "a", "")
	put(t, root, "a-prefix", "prefix")
	put(t, root, "bin/x", "x")
	if e := os.Chmod(filepath.Join(root, "bin/x"), 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.Mkdir(filepath.Join(root, "empty"), 0700); e != nil {
		t.Fatal(e)
	}
	link(t, root, "link", "a")
	put(t, root, ".git/never-read", "excluded")
	link(t, root, ".plugin-kit-ai.lock", "../never-resolve")
	l := open(t, r, root, nil)
	v := capture(t, l)
	if !v.Coverage.TreeComplete {
		t.Fatalf("no full digest: %+v", v.Findings)
	}
	// Build a safe private normal tree from captured bytes only. The existing
	// digester must never see the mutable source, including in parity tests.
	private := t.TempDir()
	for _, o := range v.Inventory {
		switch o.Kind {
		case "directory":
			if e := os.MkdirAll(filepath.Join(private, o.Path), 0700); e != nil {
				t.Fatal(e)
			}
		case "file":
			put(t, private, o.Path, string(l.contents[o.Path]))
			if o.Executable {
				if e := os.Chmod(filepath.Join(private, o.Path), 0700); e != nil {
					t.Fatal(e)
				}
			}
		case "symlink":
			link(t, private, o.Path, o.Target)
		}
	}
	snapshot, e := (packagedigest.Builder{TempRoot: t.TempDir()}).Snapshot(context.Background(), private, domain.SourceIdentity{})
	if e != nil {
		t.Fatal(e)
	}
	defer func() {
		if e := packagedigest.Remove(snapshot); e != nil {
			t.Error(e)
		}
	}()
	if snapshot.TreeDigest != v.Identity.TreeDigest {
		t.Fatalf("digest drift: %s != %s", snapshot.TreeDigest, v.Identity.TreeDigest)
	}
	// Capture identity excludes root/temp names and timestamps, but includes scope.
	other := capture(t, open(t, Reader{TempDir: t.TempDir()}, private, nil))
	if other.Identity != v.Identity {
		t.Fatalf("non-reproducible identities: %+v %+v", v.Identity, other.Identity)
	}
}

func TestReadOpenFailureDistinctFromAbsentAndRedacted(t *testing.T) {
	root, r := fixture(t)
	put(t, root, "mcp.json", mcpBody)
	put(t, root, "skills/helper/SKILL.md", skillBody)
	calls := 0
	l := open(t, r, root, &captureHooks{dataOpenError: func(p string) error {
		if p == "mcp.json" {
			calls++
			return &os.PathError{Op: "open", Path: "/private/secret-kernel-path", Err: syscall.EACCES}
		}
		return nil
	}})
	v := capture(t, l)
	if calls == 0 || v.MCP.State != Unreadable || v.Plugin.State != Present || v.Skills[0].Document.State != Present || v.Coverage.TreeComplete {
		t.Fatal("read-open failure lost boundary")
	}
	if !finding(v, "document_unreadable") || observation(t, v, "mcp.json").State != Unreadable {
		t.Fatal("read-open failure became absence")
	}
	b, e := json.Marshal(v)
	if e != nil || bytes.Contains(b, []byte("secret-kernel-path")) {
		t.Fatal("raw read-open error leaked")
	}
}

func TestInventoryPanicClosesPins(t *testing.T) {
	root, r := fixture(t)
	put(t, root, "opaque", "inert payload")
	warm, e := r.Open(context.Background(), root)
	if e != nil {
		t.Fatal(e)
	}
	if e := warm.Close(); e != nil {
		t.Fatal(e)
	}
	count := func() int {
		entries, e := os.ReadDir("/proc/self/fd")
		if e != nil {
			t.Fatal(e)
		}
		return len(entries)
	}
	before := count()
	held, e := os.Open(filepath.Join(root, "opaque"))
	if e != nil {
		t.Fatal(e)
	}
	defer held.Close()
	if count() != before+1 {
		t.Fatal("FD observer failed positive control")
	}
	if e := held.Close(); e != nil {
		t.Fatal(e)
	}
	if count() != before {
		t.Fatal("FD observer failed closed control")
	}
	for i := 0; i < 3; i++ {
		fired := false
		l, e := r.open(context.Background(), root, &captureHooks{afterChunk: func(n string) {
			if n == "opaque" {
				fired = true
				panic("inventory panic")
			}
		}})
		if e != nil {
			t.Fatal(e)
		}
		var caught any
		func() {
			defer func() { caught = recover() }()
			_, _ = l.Capture(context.Background())
		}()
		if !fired || caught != "inventory panic" {
			t.Fatal("panic prerequisite", fired, caught)
		}
		if count() != before {
			t.Fatal("descriptor leak after inventory panic")
		}
		if !reflect.DeepEqual(l.Data(), Input{}) {
			t.Fatal("usable partial Input")
		}
		if e := l.Close(); e != nil {
			t.Fatal(e)
		}
		if e := l.Close(); e != nil {
			t.Fatal(e)
		}
		empty(t, r.TempDir)
	}
}

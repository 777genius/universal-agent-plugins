package project

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packageview"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/conformance"
)

const core = `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"demo"}`

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
func empty(t *testing.T, root string) {
	t.Helper()
	e, err := os.ReadDir(root)
	if err != nil || len(e) != 0 {
		t.Fatalf("scratch not cleaned: %v %v", e, err)
	}
}
func writableNative(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "linux" && !(runtime.GOOS == "windows" && runtime.GOARCH == "amd64") {
		t.Skip("writable native authoring requires Linux or Windows amd64")
	}
}

func TestCoreGateAndExactRoot(t *testing.T) {
	writableNative(t)
	scratch := t.TempDir()
	s := Service{Scratch: scratch}
	for _, body := range []string{`{`, strings.Replace(core, "1.0.0", "8.0.0", 1), strings.Replace(core, `"demo"`, `9`, 1)} {
		root := t.TempDir()
		put(t, root, "plugin.json", body)
		// Component capture would exceed its budget. Core/schema rejection must
		// preserve early-fatal precedence rather than surfacing that later failure.
		put(t, root, "mcp.json", strings.Repeat("x", 8192))
		bounded := s
		bounded.Limits.MCPBytes = 16
		p, e := bounded.Read(context.Background(), root)
		if e != nil || p.Facts.Package != nil || p.Input.Coverage.ComponentsRequested || len(p.Input.Inventory) != 0 {
			t.Fatalf("fatal core read components: %+v %v", p.Input, e)
		}
		empty(t, scratch)
	}
	for _, name := range []string{"plugin/plugin.yaml", ".codex-plugin/plugin.json", "nested/plugin.json"} {
		root := t.TempDir()
		put(t, root, name, core)
		p, e := s.Read(context.Background(), root)
		if e != nil || p.Facts.Package != nil || p.Input.Plugin.State != packageview.Absent {
			t.Fatalf("fallback %s: %+v %v", name, p.Input, e)
		}
	}
	root := t.TempDir()
	put(t, root, "plugin.json", core)
	linked := filepath.Join(t.TempDir(), "alias")
	if e := os.Symlink(root, linked); e != nil {
		t.Fatal(e)
	}
	if _, e := s.Read(context.Background(), linked); e == nil {
		t.Fatal("symlink root accepted")
	}
	empty(t, scratch)
}
func TestLegacyMetadataAndImmutableIdentity(t *testing.T) {
	writableNative(t)
	root, scratch := t.TempDir(), t.TempDir()
	s := Service{Scratch: scratch}
	put(t, root, "plugin.json", core)
	put(t, root, "plugin/plugin.yaml", "ordinary-fixture-marker")
	if e := os.Symlink("plugin/plugin.yaml", filepath.Join(root, "alias")); e != nil {
		t.Fatal(e)
	}
	p, e := s.Read(context.Background(), root)
	if e != nil {
		t.Fatal(e)
	}
	if p.Facts.Package == nil || p.Input.Identity.Digest == "" || p.Input.Coverage.TreeComplete || p.Input.Identity.TreeDigest != "" {
		t.Fatalf("legacy identity overclaimed: %+v", p.Input)
	}
	for _, o := range p.Input.Inventory {
		if (o.Path == "alias" || o.Path == "plugin/plugin.yaml") && o.Captured {
			t.Fatal("legacy alias captured")
		}
	}
	oldDigest := p.Input.Identity.Digest
	put(t, root, "plugin/plugin.yaml", "a different harmless fixture")
	p2, e := s.Read(context.Background(), root)
	if e != nil {
		t.Fatal(e)
	}
	// Metadata size may change captured-input identity. Neither observation hashes
	// or exposes the YAML bytes; changing later source never changes prior facts.
	_ = p2
	if p.Input.Identity.Digest != oldDigest || string(p.Input.Plugin.Bytes) != core {
		t.Fatal("mutable result")
	}
	empty(t, scratch)
}

type countingContext struct {
	context.Context
	cancel context.CancelFunc
	calls  atomic.Int64
	at     int64
}

func (c *countingContext) Err() error {
	if c.calls.Add(1) == c.at {
		c.cancel()
	}
	return c.Context.Err()
}
func TestCleanupOnCaptureFailureAndCancellation(t *testing.T) {
	writableNative(t)
	root, scratch := t.TempDir(), t.TempDir()
	put(t, root, "plugin.json", core)
	put(t, root, "data", strings.Repeat("a", 1<<20))
	s := Service{Scratch: scratch}
	limited := s
	limited.Limits.FileBytes = 128
	p, e := limited.Read(context.Background(), root)
	if e == nil || p.Input.Identity.Digest != "" {
		t.Fatalf("capture failure retained stale identity: %+v %v", p, e)
	}
	empty(t, scratch)
	for _, at := range []int64{1, 5, 20, 60, 120} {
		ctx, cancel := context.WithCancel(context.Background())
		counted := &countingContext{Context: ctx, cancel: cancel, at: at}
		_, e = s.Read(counted, root)
		cancel()
		if !errors.Is(e, context.Canceled) {
			t.Fatalf("cancel at %d calls=%d: %v", at, counted.calls.Load(), e)
		}
		empty(t, scratch)
	}
}
func TestCapturedCommandContainment(t *testing.T) {
	writableNative(t)
	for _, tc := range []struct {
		value, field string
		entries      []packageview.Observation
		want         conformance.Outcome
	}{
		{"./bin/tool", "command", []packageview.Observation{{Path: "bin", Kind: "directory", State: packageview.Present, Captured: true}, {Path: "bin/tool", Kind: "file", State: packageview.Present, Captured: true}}, conformance.Pass},
		{"./missing/../bin/tool", "command", []packageview.Observation{{Path: "bin", Kind: "directory", State: packageview.Present, Captured: true}, {Path: "bin/tool", Kind: "file", State: packageview.Present, Captured: true}}, conformance.NotEvaluated},
		{"./regular/../bin", "cwd", []packageview.Observation{{Path: "regular", Kind: "file", State: packageview.Present, Captured: true}, {Path: "bin", Kind: "directory", State: packageview.Present, Captured: true}}, conformance.NotEvaluated},
		{"./../outside", "command", nil, conformance.Fail},
		{"${PLUGIN_DATA}/later", "cwd", nil, conformance.NotEvaluated},
		{"${PLUGIN_ROOT}/bin", "cwd", []packageview.Observation{{Path: "bin", Kind: "directory", State: packageview.Present, Captured: true}}, conformance.Pass},
		{"./alias", "command", []packageview.Observation{{Path: "alias", Kind: "symlink", Target: "tool", State: packageview.Present, Captured: true}, {Path: "tool", Kind: "file", State: packageview.Present, Captured: true}}, conformance.Pass},
	} {
		if got := observePath(packageview.Input{Inventory: tc.entries}, tc.value, tc.field); got != tc.want {
			t.Fatalf("%s %s: got %s want %s", tc.field, tc.value, got, tc.want)
		}
	}
	// Integration from inventory and exact decoded facts; no packageview.Paths API.
	for _, tc := range []struct {
		cmd, cwd string
		want     conformance.Outcome
	}{{"./bin/tool", "${PLUGIN_ROOT}/bin", conformance.Pass}, {"missing-fixture-command", "", conformance.Pass}, {"./absent", "", conformance.NotEvaluated}, {"missing-fixture-command", "${PLUGIN_DATA}/later", conformance.NotEvaluated}} {
		root, scratch := t.TempDir(), t.TempDir()
		put(t, root, "plugin.json", core)
		put(t, root, "bin/tool", "inert fixture")
		cwd := ""
		if tc.cwd != "" {
			cwd = `,"cwd":"` + tc.cwd + `"`
		}
		put(t, root, "mcp.json", `{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"s":{"type":"stdio","command":"`+tc.cmd+`"`+cwd+`}}}`)
		p, e := (Service{Scratch: scratch}).Read(context.Background(), root)
		if e != nil || p.Facts.Conformance != tc.want {
			t.Fatalf("command facts %s %s: %+v %v", tc.cmd, tc.cwd, p.Facts, e)
		}
		empty(t, scratch)
	}
}

//go:build windows && (amd64 || arm64)

package packageview

import (
	"context"
	"os"
	"testing"
)

func TestWindowsOrdinaryPostOpenObservation(t *testing.T) {
	root := nativeFixture(t, func(root string) { nativeWrite(t, root, "plugin.json", "ordinary") })
	s := nativeSource(t, root)
	p, err := s.pin("plugin.json", false)
	if err != nil {
		t.Fatal(err)
	}
	defer p.file.Close()
	f, err := p.reopen(false)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	raw, err := f.Stat()
	if err != nil {
		t.Fatal(err)
	}
	before := winRecordCount()
	observed, err := p.postOpenInfo(f)
	if err != nil || multipleLinks(observed) {
		t.Fatalf("ordinary post-open observation rejected: %v", err)
	}
	if !multipleLinks(raw) {
		t.Fatal("unregistered File.Stat must remain fail-closed")
	}
	if got := winRecordCount(); got != before {
		t.Fatalf("post-open observation registered metadata: before=%d after=%d", before, got)
	}
}

func TestWindowsOrdinaryReadStages(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		t.Run(map[bool]string{false: "without-legacy", true: "with-legacy"}[legacy], func(t *testing.T) {
			before := winRecordCount()
			const core = `{"name":"ordinary"}`
			const mcp = `{}`
			root := nativeFixture(t, func(root string) {
				nativeWrite(t, root, "plugin.json", core)
				nativeWrite(t, root, "mcp.json", mcp)
				nativeWrite(t, root, "opaque", "ordinary bytes")
				if legacy {
					nativeWrite(t, root, "plugin/plugin.yaml", "DO NOT READ LEGACY")
				}
			})
			scratch := t.TempDir()
			l, err := (Reader{TempDir: scratch}).Open(context.Background(), root)
			if err != nil {
				t.Fatalf("ordinary Reader.Open: %v", err)
			}
			defer l.Close()
			coreInput := l.Data()
			if coreInput.Plugin.State != Present || string(coreInput.Plugin.Bytes) != core || coreInput.Coverage.ComponentsRequested {
				t.Fatalf("ordinary core capture: %+v", coreInput)
			}
			// Capture must pass the separate verifyCaptured post-open guard for
			// every retained regular file, including the already captured core.
			in, err := l.Capture(context.Background())
			if err != nil {
				t.Fatalf("ordinary Capture/final verification: %v", err)
			}
			if in.Plugin.State != Present || string(in.Plugin.Bytes) != core || in.MCP.State != Present || string(in.MCP.Bytes) != mcp {
				t.Fatalf("ordinary document bytes: %+v", in)
			}
			wantLegacy := Absent
			if legacy {
				wantLegacy = Present
			}
			if in.Legacy != wantLegacy || !in.Coverage.ComponentsRequested || !in.Coverage.InventoryComplete || in.Coverage.TreeComplete != !legacy {
				t.Fatalf("ordinary capture coverage: %+v", in)
			}
			if string(l.contents["opaque"]) != "ordinary bytes" {
				t.Fatal("ordinary inventory file was not captured")
			}
			if _, captured := l.contents["plugin/plugin.yaml"]; captured {
				t.Fatal("legacy bytes were captured")
			}
			if err := l.Close(); err != nil {
				t.Fatalf("ordinary Close: %v", err)
			}
			if err := l.Close(); err != nil {
				t.Fatalf("idempotent Close: %v", err)
			}
			if got := winRecordCount(); got != before {
				t.Fatalf("metadata records after Close: got %d, want %d", got, before)
			}
			entries, err := os.ReadDir(scratch)
			if err != nil || len(entries) != 0 {
				t.Fatalf("ordinary scratch cleanup: entries=%d, err=%v", len(entries), err)
			}
		})
	}
}

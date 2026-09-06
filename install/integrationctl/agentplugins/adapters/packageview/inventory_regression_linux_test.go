//go:build linux

package packageview

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestCapturedTraversalRepresentation(t *testing.T) {
	for _, tc := range []struct {
		name, target string
		dirs         []string
		links        map[string]string
		complete     bool
	}{
		{"excluded", ".git/../data", []string{".git"}, nil, false},
		{"retained", "kept/../data", []string{"kept"}, nil, true},
		{"excluded-marker-intermediary", ".plugin-kit-ai.lock/../data", []string{"kept"}, map[string]string{".plugin-kit-ai.lock": "kept"}, false},
		{"nested-excluded", "kept/nested/../../data", []string{"kept", ".git", "other/inner"}, map[string]string{"kept/nested": "../.git/../other/inner"}, false},
		{"nested-retained", "kept/nested/../../data", []string{"kept", "other/inner"}, map[string]string{"kept/nested": "../other/inner"}, true},
		{"expanded-excluded-intermediary", "kept/hop/../data", []string{"kept", ".git"}, map[string]string{"kept/hop": "../.git"}, false},
		{"expanded-retained-intermediary", "kept/hop/../data", []string{"kept", "other"}, map[string]string{"kept/hop": "../other"}, true},
		{"retained-dotgit-substring", "kept.git/../data", []string{"kept.git"}, nil, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, r := fixture(t)
			put(t, root, "data", mcpBody)
			// Keep the inherited lexical installer check satisfied independently.
			put(t, root, "kept/data", mcpBody)
			for _, dir := range tc.dirs {
				if err := os.MkdirAll(filepath.Join(root, dir), 0700); err != nil {
					t.Fatal(err)
				}
			}
			for name, target := range tc.links {
				link(t, root, name, target)
			}
			link(t, root, "mcp.json", tc.target)
			l := open(t, r, root, nil)
			v := capture(t, l)
			if v.Plugin.State != Present || string(v.Plugin.Bytes) != pluginBody || v.MCP.State != Present || string(v.MCP.Bytes) != mcpBody || v.Identity.Digest == "" || !v.Coverage.InventoryComplete {
				t.Fatalf("lost safe captured facts: %+v", v.Coverage)
			}
			if v.Coverage.TreeComplete != tc.complete || (v.Identity.TreeDigest != "") != tc.complete || (v.Identity.TreeAlgorithm != "") != tc.complete {
				t.Fatalf("tree identity = %+v, coverage = %+v", v.Identity, v.Coverage)
			}
			if finding(v, "complete_tree_identity_unavailable") == tc.complete {
				t.Fatal("incorrect unavailable finding")
			}
			for _, f := range v.Findings {
				if f.Code == "complete_tree_identity_unavailable" && f.Layer != "host" {
					t.Fatal("normative finding")
				}
			}
			if err := l.Close(); err != nil {
				t.Fatal(err)
			}
			empty(t, r.TempDir)
		})
	}
}

func TestCapturedLFSPolicyPreservesFacts(t *testing.T) {
	for name, suffix := range map[string]string{"LF": "\n", "CRLF": "\r\n", "doubleCRLF": "\r\r\n", "doubleCREOF": "\r\r"} {
		t.Run(name, func(t *testing.T) {
			root, r := fixture(t)
			put(t, root, "mcp.json", mcpBody)
			put(t, root, "skills/helper/SKILL.md", skillBody)
			put(t, root, "pointer", "version https://git-lfs.github.com/spec/v1"+suffix)
			l := open(t, r, root, nil)
			v, err := l.Capture(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if v.Plugin.State != Present || string(v.Plugin.Bytes) != pluginBody || v.MCP.State != Present || string(v.MCP.Bytes) != mcpBody || len(v.Skills) != 1 || v.Skills[0].Document.State != Present || string(v.Skills[0].Document.Bytes) != skillBody || v.Identity.Digest == "" || !v.Coverage.InventoryComplete {
				t.Fatal("policy rejection lost safe facts")
			}
			if v.Coverage.TreeComplete || v.Identity.TreeDigest != "" || v.Identity.TreeAlgorithm != "" || !finding(v, "complete_tree_identity_unavailable") || !finding(v, "tree_digest_policy_unavailable") {
				t.Fatal("missing policy downgrade")
			}
			for _, f := range v.Findings {
				if f.Code == "tree_digest_policy_unavailable" && f.Layer != "installer_policy" {
					t.Fatal("normative policy finding")
				}
			}
			if err := l.Close(); err != nil {
				t.Fatal(err)
			}
			empty(t, r.TempDir)
		})
	}
}

func TestRetainedTraversalIntermediariesAndBounds(t *testing.T) {
	retained := map[string]Observation{
		"dir":   {Kind: "directory", State: Present, Captured: true},
		"file":  {Kind: "file", State: Present, Captured: true},
		"cycle": {Kind: "symlink", Target: "cycle", State: Present, Captured: true},
		"alias": {Kind: "symlink", Target: "dir", State: Present, Captured: true},
	}
	for _, tc := range []struct {
		target string
		want   bool
	}{
		{"missing/../file", false}, {"file/../file", false}, {"file/", false},
		{"file/.", false}, {"dir/../file", true}, {"alias/../file", true},
		{"cycle/../file", false}, {"../file", false}, {"/file", false}, {"dir/", true},
	} {
		retained["link"] = Observation{Kind: "symlink", Target: tc.target, State: Present, Captured: true}
		if got := retainedLinkResolves("link", retained); got != tc.want {
			t.Errorf("%q = %v", tc.target, got)
		}
	}
}

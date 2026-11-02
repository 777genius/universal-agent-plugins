package project

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packageview"
)

// Optional absence is a leaf, but malformed or unavailable declarations must
// block readiness; target references are deliberately outside package authority.
func TestReadPackageDependencyDiagnostics(t *testing.T) {
	writableNative(t)
	for _, tc := range []struct {
		name, body, code string
		present          bool
	}{
		{name: "absent"},
		{name: "version only", body: "schema_version = 1\n", present: true},
		{name: "canonical without targets", body: "schema_version = 1\n[dependencies]\n\"acme/context\" = \"^2.0\"\n", present: true},
		{name: "author target references ignored", body: "schema_version = 1\n[targets]\nunknown = [\"missing\"]\n", present: true},
		{name: "empty", present: true, code: "dependency_manifest_invalid"},
		{name: "syntax", body: "schema_version = [", present: true, code: "dependency_manifest_invalid"},
		{name: "short source", body: "schema_version = 1\n[dependencies]\ncontext = \"^2.0\"\n", present: true, code: "dependency_manifest_invalid"},
		{name: "invalid target shape", body: "schema_version = 1\n[targets]\nunknown = [2]\n", present: true, code: "dependency_manifest_invalid"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, scratch := t.TempDir(), t.TempDir()
			put(t, root, "plugin.json", core)
			// Nested sidecars never supply the root's declarations.
			put(t, root, "nested/uap.toml", "invalid nested sidecar")
			if tc.present {
				put(t, root, "uap.toml", tc.body)
			}
			p, err := (Service{Scratch: scratch}).Read(context.Background(), root)
			if err != nil || p.Facts.Package == nil {
				t.Fatalf("read: %+v %v", p, err)
			}
			if tc.code == "" {
				if len(p.DependencyDiagnostics) != 0 {
					t.Fatalf("diagnostics = %+v", p.DependencyDiagnostics)
				}
			} else if len(p.DependencyDiagnostics) != 1 || p.DependencyDiagnostics[0].Code != tc.code || !strings.HasPrefix(p.DependencyDiagnostics[0].Location, "uap.toml:") {
				t.Fatalf("diagnostics = %+v, want %s", p.DependencyDiagnostics, tc.code)
			}
			empty(t, scratch)
		})
	}
	root := t.TempDir()
	put(t, root, "plugin.json", core)
	if err := os.Mkdir(filepath.Join(root, "uap.toml"), 0700); err != nil {
		t.Fatal(err)
	}
	p, err := (Service{Scratch: t.TempDir()}).Read(context.Background(), root)
	if err != nil || len(p.DependencyDiagnostics) != 1 || p.DependencyDiagnostics[0].Code != "dependency_manifest_wrong_kind" {
		t.Fatalf("directory: %+v %v", p.DependencyDiagnostics, err)
	}
}

// Mutation after capture must not influence author validation. This catches a
// second source read even when normal quiescent-source Read tests would pass.
func TestDependencyValidationUsesCapturedContent(t *testing.T) {
	writableNative(t)
	root, scratch := t.TempDir(), t.TempDir()
	put(t, root, "plugin.json", core)
	put(t, root, "sidecar", "schema_version = 1\n[dependencies]\n\"acme/context\" = \"^2.0\"\n")
	if err := os.Symlink("sidecar", filepath.Join(root, "uap.toml")); err != nil {
		t.Skipf("symlink privilege unavailable: %v", err)
	}
	lease, err := (packageview.Reader{TempDir: scratch}).Open(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := lease.Close(); err != nil {
			t.Error(err)
		}
	}()
	input, err := lease.Capture(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	// Close before modifying the source, preserving the native reader's
	// quiescent-source contract; retained content remains immutable after Close.
	if err := lease.Close(); err != nil {
		t.Fatal(err)
	}
	empty(t, scratch)
	put(t, root, "sidecar", "invalid mutated content")
	if diagnostics := dependencyDiagnostics(nil, input, lease); len(diagnostics) != 0 {
		t.Fatalf("captured valid bytes lost: %+v", diagnostics)
	}
	// Without retained content the declaration is unavailable, not optional.
	for i := range input.Inventory {
		if input.Inventory[i].Path == "sidecar" {
			input.Inventory[i].Captured = false
		}
	}
	if diagnostics := dependencyDiagnostics(nil, input, lease); len(diagnostics) != 1 || diagnostics[0].Code != "dependency_manifest_unavailable" {
		t.Fatalf("uncaptured declaration accepted: %+v", diagnostics)
	}
	for _, state := range []packageview.State{packageview.Absent, packageview.Unreadable, packageview.Blocked, packageview.WrongKind} {
		unavailable := input
		unavailable.Inventory = []packageview.Observation{{Path: "uap.toml", Kind: "file", State: state}}
		if diagnostics := dependencyDiagnostics(nil, unavailable, lease); len(diagnostics) != 1 || diagnostics[0].Code != "dependency_manifest_unavailable" {
			t.Fatalf("%s declaration accepted: %+v", state, diagnostics)
		}
	}
	input.Inventory = nil
	input.Coverage.InventoryComplete = false
	if diagnostics := dependencyDiagnostics(nil, input, lease); len(diagnostics) != 1 {
		t.Fatal("incomplete inventory asserted absence")
	}
}

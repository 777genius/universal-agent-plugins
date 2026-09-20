package targetcontracts

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestAllIncludesNativeDocPathsForCodexTargets(t *testing.T) {
	packageEntry, ok := Lookup("codex-package")
	if !ok {
		t.Fatal("missing codex-package entry")
	}
	if got := packageEntry.NativeDocPaths["interface"]; got != filepath.Join("plugin", "targets", "codex-package", "interface.json") {
		t.Fatalf("codex-package native_doc_paths[interface] = %q", got)
	}
	if got := packageEntry.NativeDocPaths["package_metadata"]; got != filepath.Join("plugin", "targets", "codex-package", "package.yaml") {
		t.Fatalf("codex-package native_doc_paths[package_metadata] = %q", got)
	}
	if got := packageEntry.NativeSurfaceTiers["interface"]; got != "stable" {
		t.Fatalf("codex-package native_surface_tiers[interface] = %q", got)
	}
	if got := packageEntry.NativeSurfaceTiers["hooks"]; got != "stable" {
		t.Fatalf("codex-package native_surface_tiers[hooks] = %q", got)
	}
	if got := packageEntry.NativeSurfaceTiers["app_manifest"]; got != "beta" {
		t.Fatalf("codex-package native_surface_tiers[app_manifest] = %q", got)
	}
	var foundAppRule, foundHooksRule, foundMCPRule bool
	for _, rule := range packageEntry.ManagedArtifactRules {
		switch {
		case rule.Path == ".app.json" && rule.Condition == "when app_manifest is enabled":
			foundAppRule = true
		case rule.Path == "hooks/**" && rule.Condition == "when hooks are authored":
			foundHooksRule = true
		case rule.Path == ".mcp.json" && rule.Condition == "when portable MCP is authored":
			foundMCPRule = true
		}
	}
	if !foundAppRule || !foundHooksRule || !foundMCPRule {
		t.Fatalf("codex-package managed_artifact_rules = %+v", packageEntry.ManagedArtifactRules)
	}

	runtimeEntry, ok := Lookup("codex-runtime")
	if !ok {
		t.Fatal("missing codex-runtime entry")
	}
	if got := runtimeEntry.NativeDocPaths["config_extra"]; got != filepath.Join("plugin", "targets", "codex-runtime", "config.extra.toml") {
		t.Fatalf("codex-runtime native_doc_paths[config_extra] = %q", got)
	}
	if got := runtimeEntry.NativeDocPaths["package_metadata"]; got != filepath.Join("plugin", "targets", "codex-runtime", "package.yaml") {
		t.Fatalf("codex-runtime native_doc_paths[package_metadata] = %q", got)
	}
	if got := runtimeEntry.NativeSurfaceTiers["config_extra"]; got != "stable" {
		t.Fatalf("codex-runtime native_surface_tiers[config_extra] = %q", got)
	}
	if got := runtimeEntry.NativeSurfaceTiers["commands"]; got != "beta" {
		t.Fatalf("codex-runtime native_surface_tiers[commands] = %q", got)
	}
	var foundCommandsRule bool
	for _, rule := range runtimeEntry.ManagedArtifactRules {
		if rule.Path == "commands/**" && rule.Condition == "when commands are authored" {
			foundCommandsRule = true
		}
	}
	if !foundCommandsRule {
		t.Fatalf("codex-runtime managed_artifact_rules = %+v", runtimeEntry.ManagedArtifactRules)
	}
	if runtimeEntry.PortableComponentKinds == nil {
		t.Fatal("codex-runtime portable_component_kinds must be an empty slice, not nil")
	}
}

func TestMarkdownStaysInSyncWithGeneratedDoc(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
	body, err := os.ReadFile(filepath.Join(root, "docs", "generated", "target_support_matrix.md"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(body)
	want := string(Markdown(All()))
	if got != want {
		t.Fatalf("target support matrix drifted\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

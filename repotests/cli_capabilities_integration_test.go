package pluginkitairepo_test

import (
	"encoding/json"
	"os/exec"
	"strings"
	"testing"
)

func TestPluginKitAICapabilities(t *testing.T) {
	pluginKitAIBin := buildPluginKitAI(t)

	tableCmd := exec.Command(pluginKitAIBin, "capabilities")
	tableOut, err := tableCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("capabilities table: %v\n%s", err, tableOut)
	}
	table := string(tableOut)
	if !strings.Contains(table, "claude") || !strings.Contains(table, "codex-package") || !strings.Contains(table, "codex-runtime") || !strings.Contains(table, "gemini") || !strings.Contains(table, "cursor") || !strings.Contains(table, "TARGET") || !strings.Contains(table, "CLASS") {
		t.Fatalf("unexpected capabilities table output:\n%s", table)
	}

	jsonCmd := exec.Command(pluginKitAIBin, "capabilities", "--mode", "runtime", "--format", "json", "--platform", "claude")
	jsonOut, err := jsonCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("capabilities json: %v\n%s", err, jsonOut)
	}
	var entries []map[string]any
	if err := json.Unmarshal(jsonOut, &entries); err != nil {
		t.Fatalf("parse capabilities json: %v\n%s", err, jsonOut)
	}
	if len(entries) == 0 {
		t.Fatal("expected at least one capabilities entry")
	}
	for _, entry := range entries {
		if entry["platform"] != "claude" {
			t.Fatalf("unexpected platform entry: %+v", entry)
		}
		if entry["maturity"] == "" {
			t.Fatalf("missing maturity entry: %+v", entry)
		}
		if entry["contract_class"] == "" {
			t.Fatalf("missing contract_class entry: %+v", entry)
		}
		if entry["scaffold_support"] != true || entry["validate_support"] != true {
			t.Fatalf("expected scaffold/validate support in entry: %+v", entry)
		}
	}

	geminiTableCmd := exec.Command(pluginKitAIBin, "capabilities", "--mode", "runtime", "--platform", "gemini")
	geminiTableOut, err := geminiTableCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("gemini capabilities table: %v\n%s", err, geminiTableOut)
	}
	geminiTable := string(geminiTableOut)
	for _, want := range []string{"LIVE_TEST", "gemini_extension", "SessionStart", "BeforeModel", "BeforeToolSelection", "make test-gemini-runtime", "make test-gemini-runtime-live"} {
		if !strings.Contains(geminiTable, want) {
			t.Fatalf("gemini runtime table missing %q:\n%s", want, geminiTable)
		}
	}

	targetJSONCmd := exec.Command(pluginKitAIBin, "capabilities", "--mode", "targets", "--format", "json", "--platform", "codex-package")
	targetJSONOut, err := targetJSONCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("target capabilities json: %v\n%s", err, targetJSONOut)
	}
	var targetEntries []map[string]any
	if err := json.Unmarshal(targetJSONOut, &targetEntries); err != nil {
		t.Fatalf("parse target capabilities json: %v\n%s", err, targetJSONOut)
	}
	if len(targetEntries) != 1 {
		t.Fatalf("expected one target capabilities entry, got %d: %s", len(targetEntries), targetJSONOut)
	}
	target := targetEntries[0]
	if target["target"] != "codex-package" {
		t.Fatalf("unexpected target entry: %+v", target)
	}
	nativeDocs, ok := target["native_docs"].([]any)
	if !ok || len(nativeDocs) == 0 {
		t.Fatalf("missing native_docs entry: %+v", target)
	}
	nativeDocPaths, ok := target["native_doc_paths"].(map[string]any)
	if !ok {
		t.Fatalf("missing native_doc_paths entry: %+v", target)
	}
	if nativeDocPaths["interface"] != authoredRel("targets", "codex-package", "interface.json") {
		t.Fatalf("native_doc_paths[interface] = %v", nativeDocPaths["interface"])
	}
	if nativeDocPaths["package_metadata"] != authoredRel("targets", "codex-package", "package.yaml") {
		t.Fatalf("native_doc_paths[package_metadata] = %v", nativeDocPaths["package_metadata"])
	}
	nativeSurfaceTiers, ok := target["native_surface_tiers"].(map[string]any)
	if !ok {
		t.Fatalf("missing native_surface_tiers entry: %+v", target)
	}
	if nativeSurfaceTiers["interface"] != "stable" {
		t.Fatalf("native_surface_tiers[interface] = %v", nativeSurfaceTiers["interface"])
	}
	if nativeSurfaceTiers["app_manifest"] != "beta" {
		t.Fatalf("native_surface_tiers[app_manifest] = %v", nativeSurfaceTiers["app_manifest"])
	}
	managedArtifactRules, ok := target["managed_artifact_rules"].([]any)
	if !ok {
		t.Fatalf("missing managed_artifact_rules entry: %+v", target)
	}
	var foundAppRule, foundMCPRule bool
	for _, raw := range managedArtifactRules {
		rule, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("managed_artifact_rules entry = %#v", raw)
		}
		switch {
		case rule["path"] == ".app.json" && rule["condition"] == "when app_manifest is enabled":
			foundAppRule = true
		case rule["path"] == ".mcp.json" && rule["condition"] == "when portable MCP is authored":
			foundMCPRule = true
		}
	}
	if !foundAppRule || !foundMCPRule {
		t.Fatalf("managed_artifact_rules = %+v", managedArtifactRules)
	}

	runtimeTargetJSONCmd := exec.Command(pluginKitAIBin, "capabilities", "--mode", "targets", "--format", "json", "--platform", "codex-runtime")
	runtimeTargetJSONOut, err := runtimeTargetJSONCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("runtime target capabilities json: %v\n%s", err, runtimeTargetJSONOut)
	}
	targetEntries = nil
	if err := json.Unmarshal(runtimeTargetJSONOut, &targetEntries); err != nil {
		t.Fatalf("parse runtime target capabilities json: %v\n%s", err, runtimeTargetJSONOut)
	}
	if len(targetEntries) != 1 {
		t.Fatalf("expected one runtime target capabilities entry, got %d: %s", len(targetEntries), runtimeTargetJSONOut)
	}
	runtimeTarget := targetEntries[0]
	if runtimeTarget["target"] != "codex-runtime" {
		t.Fatalf("unexpected runtime target entry: %+v", runtimeTarget)
	}
	portables, ok := runtimeTarget["portable_component_kinds"].([]any)
	if !ok {
		t.Fatalf("portable_component_kinds should decode as an array: %+v", runtimeTarget)
	}
	if len(portables) != 0 {
		t.Fatalf("portable_component_kinds = %+v, want empty array", portables)
	}
	runtimeRules, ok := runtimeTarget["managed_artifact_rules"].([]any)
	if !ok {
		t.Fatalf("missing runtime managed_artifact_rules entry: %+v", runtimeTarget)
	}
	var foundCommandsRule bool
	for _, raw := range runtimeRules {
		rule, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("runtime managed_artifact_rules entry = %#v", raw)
		}
		if rule["path"] == "commands/**" && rule["condition"] == "when commands are authored" {
			foundCommandsRule = true
		}
	}
	if !foundCommandsRule {
		t.Fatalf("runtime managed_artifact_rules = %+v", runtimeRules)
	}
}

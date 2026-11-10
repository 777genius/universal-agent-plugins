//go:build linux

package pluginkitairepo_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/process"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

// This is run from the exact pinned real-Grok cycle on disposable hosted Linux.
// The Project remains untrusted; no model invocation, server launch or consent
// is simulated. It verifies source-to-UAP-to-native plugin MCP discovery.
func grokProjectNativeMCPCycle(t *testing.T, cycle *grokProjectCycle) {
	t.Helper()
	f := cycle.f
	cycle.check(t)
	project := filepath.Join(f.base.Root, "grok-project-mcp-only")
	source := filepath.Join(f.base.Root, "grok-project-mcp-source")
	const name = "uap-grok-mcp-only"
	for _, root := range []string{project, source} {
		if err := os.Mkdir(root, 0700); err != nil {
			t.Fatal(err)
		}
	}
	grokProjectMCPRun(t, f, project, f.git, "-c", "core.hooksPath=/dev/null", "init", "--quiet")
	manifest, _ := json.Marshal(map[string]any{
		"$schema": domain.PluginSchemaV1, "name": name,
		"version": "1.0.0", "description": "Inert Project MCP discovery",
	})
	projectNativeRuntimeWrite(t, filepath.Join(source, "plugin.json"), manifest, 0600)
	config, _ := json.Marshal(map[string]any{
		"$schema": domain.MCPSchemaV1,
		"mcpServers": map[string]any{
			"uap-grok-mcp-probe": map[string]any{
				"type": "streamable-http", "url": "https://inert.test/mcp",
			},
		},
	})
	projectNativeRuntimeWrite(t, filepath.Join(source, "mcp.json"), config, 0600)
	add := grokProjectMCPRun(t, f, project, f.installer, "add", source, "--no-save",
		"--scope", "project", "--project-root", project, "--target", "grok", "--format", "json")
	if err := nativeValidateInstallerJSON(add); err != nil {
		t.Fatal("real Grok Project MCP-only installation response invalid", err)
	}
	active := grokProjectMCPReceipt(t, project, name)
	grokProjectMCPNativeInspect(t, f, project, name, active, true)
	cycle.check(t)
	remove := grokProjectMCPRun(t, f, project, f.installer, "remove", name, "--target", "grok",
		"--scope", "project", "--project-root", project, "--format", "json")
	if err := nativeValidateInstallerJSON(remove); err != nil {
		t.Fatal("real Grok Project MCP removal response invalid", err)
	}
	if _, err := os.Lstat(active); !os.IsNotExist(err) {
		t.Fatalf("removed MCP-only project artifact survives: %v", err)
	}
	grokProjectMCPNativeInspect(t, f, project, name, active, false)
	cycle.check(t)
	t.Log("real Grok 1.0.70: Project MCP source installed, native candidate observed and removed; MCP tools/call not qualified")
}

func grokProjectMCPRun(t *testing.T, f *projectNativeCLIAttempt, cwd string, argv ...string) []byte {
	t.Helper()
	f.check(t)
	ctx, cancel := context.WithTimeout(f.ctx, 30*time.Second)
	defer cancel()
	result, err := (process.OS{}).RunWithDescendantExitGrace(ctx, ports.Command{
		Argv: argv, Env: f.env(), Dir: cwd, StdoutLimitBytes: 1 << 20,
	}, 5*time.Second)
	if err != nil || ctx.Err() != nil || result.ExitCode != 0 || len(result.Stdout) > 1<<20 {
		t.Fatalf("Grok native project MCP command failed (exit=%d, err=%v)", result.ExitCode, err)
	}
	f.check(t)
	return result.Stdout
}

func grokProjectMCPReceipt(t *testing.T, project, name string) string {
	t.Helper()
	raw, err := projectNativeRuntimeRead(filepath.Join(project, ".uap", "state-v2.json"), 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	var state domain.StateFileV2
	if json.Unmarshal(raw, &state) != nil || len(state.Installations) != 1 ||
		state.Installations[0].DeclaredName != name || len(state.Installations[0].Clients) != 1 {
		t.Fatal("real MCP-only project does not have one exact owned installation")
	}
	for _, binding := range state.Installations[0].Clients {
		active := binding.TargetLocator
		if binding.Scope != "project" || binding.ClientID != string(domain.ClientGrok) ||
			binding.Activation != domain.ActivationPrepared || binding.Verification != domain.VerificationPackageValid ||
			binding.NativeProfileRoot != filepath.Join(project, ".grok") ||
			filepath.Dir(active) != filepath.Join(project, ".grok", "plugins") ||
			pathpolicy.ValidateLeafID(filepath.Base(active)) != nil {
			t.Fatalf("real MCP-only project escaped prepared authority: %+v", binding)
		}
		expected := map[string]any{"name": name, "version": "1.0.0", "description": "Inert Project MCP discovery", "mcpServers": "./.mcp.json"}
		rawManifest, err := projectNativeRuntimeRead(filepath.Join(active, ".grok-plugin", "plugin.json"), 1<<20)
		if err != nil {
			t.Fatal(err)
		}
		var manifest map[string]any
		if json.Unmarshal(rawManifest, &manifest) != nil || !mapsEqualProjectGrokMCP(manifest, expected) {
			t.Fatalf("native Grok MCP manifest differs: %s", rawManifest)
		}
		rawMCP, err := projectNativeRuntimeRead(filepath.Join(active, ".mcp.json"), 1<<20)
		if err != nil {
			t.Fatal(err)
		}
		var document struct {
			Servers map[string]map[string]any `json:"mcpServers"`
		}
		if json.Unmarshal(rawMCP, &document) != nil || len(document.Servers) != 1 ||
			document.Servers["uap-grok-mcp-probe"]["type"] != "http" ||
			document.Servers["uap-grok-mcp-probe"]["url"] != "https://inert.test/mcp" {
			t.Fatalf("native Grok MCP sidecar differs: %s", rawMCP)
		}
		if _, err := os.Stat(filepath.Join(active, "mcp.json")); err != nil {
			t.Fatal("Grok Project lost its standard portable MCP source", err)
		}
		return active
	}
	t.Fatal("missing owned MCP-only binding")
	return ""
}

func mapsEqualProjectGrokMCP(actual, expected map[string]any) bool {
	if len(actual) != len(expected) {
		return false
	}
	for key, want := range expected {
		if actual[key] != want {
			return false
		}
	}
	return true
}

func grokProjectMCPNativeInspect(t *testing.T, f *projectNativeCLIAttempt, project, name, active string, present bool) {
	t.Helper()
	body := grokProjectMCPRun(t, f, project, f.client, "inspect", "--json")
	value, err := projectNativeRuntimeJSON(body)
	if err != nil {
		t.Fatal(err)
	}
	obj, ok := value.(map[string]any)
	if !ok || !grokProjectMCPUnambiguousFields(obj, "grokVersion", "cwd", "projectRoot", "projectTrusted", "mcpServers", "plugins") ||
		obj["grokVersion"] != "1.0.70" || obj["cwd"] != project ||
		(obj["projectRoot"] != project && obj["projectRoot"] != project+"/") ||
		obj["projectTrusted"] != false {
		t.Fatal("real Grok native inspect returned different project/version/folder trust")
	}
	// This is a distinct, newly created Git project. It must not inherit
	// trust from the original project or an ambient user profile, and the
	// prepared plugin must not activate MCP by itself.
	servers, ok := obj["mcpServers"].([]any)
	if !ok || len(servers) != 0 {
		t.Fatal("untrusted Grok project unexpectedly activated MCP")
	}
	plugins, ok := obj["plugins"].([]any)
	if !ok {
		t.Fatal("real Grok native inspect lacks plugin inventory")
	}
	if err := grokProjectMCPInventory(plugins, name, active, present); err != nil {
		t.Fatal(err)
	}
}

// Native identity fields must not have Unicode-folded aliases. Unknown fields
// remain allowed for forward-compatible inspect metadata.
func grokProjectMCPUnambiguousFields(object map[string]any, fields ...string) bool {
	for key := range object {
		for _, field := range fields {
			if key != field && shared.FoldJSONKey(key) == shared.FoldJSONKey(field) {
				return false
			}
		}
	}
	return true
}

// The observed native inventory must not contain a case-folded alternative to
// the selected owned plugin, even if the exact spelling is also present.
func grokProjectMCPInventory(plugins []any, name, active string, present bool) error {
	matched := false
	for _, raw := range plugins {
		entry, ok := raw.(map[string]any)
		if !ok || !grokProjectMCPUnambiguousFields(entry, "name", "scope", "path", "enabled", "provides") {
			return fmt.Errorf("real Grok native plugin inventory has unrecognized or ambiguous entry")
		}
		candidate, ok := entry["name"].(string)
		if !ok || strings.TrimSpace(candidate) == "" {
			return fmt.Errorf("real Grok native plugin has invalid name")
		}
		if candidate != name {
			if shared.FoldJSONKey(candidate) == shared.FoldJSONKey(name) {
				return fmt.Errorf("real Grok native plugin has a folded alias of the selected name")
			}
			continue
		}
		if matched || entry["scope"] != "project" || entry["path"] != active || entry["enabled"] != false {
			return fmt.Errorf("real Grok MCP project entry is foreign, duplicated or unexpectedly trusted")
		}
		provides, ok := entry["provides"].(map[string]any)
		if !ok || !grokProjectMCPUnambiguousFields(provides, "mcpServers") {
			return fmt.Errorf("real Grok did not report unambiguous plugin MCP discovery")
		}
		count, ok := provides["mcpServers"].(json.Number)
		if !ok || count != "1" {
			return fmt.Errorf("real Grok did not discover exactly one numeric plugin MCP server")
		}
		matched = true
	}
	if matched != present {
		return fmt.Errorf("real Grok native MCP plugin discovery mismatch: present=%t observed=%t", present, matched)
	}
	return nil
}

func TestGrokProjectMCPInventoryRejectsFoldedAliasesAndUnexpectedTrust(t *testing.T) {
	const name = "uap-grok-mcp-only"
	const active = "/fixture/.grok/plugins/owned"
	plugin := func(candidate string) map[string]any {
		return map[string]any{"name": candidate, "scope": "project", "path": active,
			"enabled": false, "provides": map[string]any{"mcpServers": json.Number("1")}}
	}
	valid := plugin(name)
	foreign := plugin("other-project-plugin")
	for _, tc := range []struct {
		name             string
		plugins          []any
		present, refused bool
	}{
		{"exact selected", []any{foreign, valid}, true, false},
		{"absent with unrelated plugin", []any{foreign}, false, false},
		{"missing selected", []any{foreign}, true, true},
		{"duplicate exact", []any{valid, valid}, true, true},
		{"folded alias before selected", []any{plugin("UAP-grok-mcp-only"), valid}, true, true},
		{"folded alias after selected", []any{valid, plugin("UAP-grok-mcp-only")}, true, true},
		{"folded alias after removal", []any{plugin("UAP-grok-mcp-only")}, false, true},
		{"malformed entry", []any{"invalid"}, false, true},
		{"ambiguous selected name", []any{map[string]any{
			"name": name, "Name": "not-the-selected-plugin", "scope": "project",
			"path": active, "enabled": false, "provides": map[string]any{"mcpServers": json.Number("1")},
		}}, true, true},
		{"ambiguous foreign scope", []any{map[string]any{
			"name": "foreign", "scope": "user", "Scope": "project", "path": "/foreign",
			"enabled": false,
		}}, false, true},
		{"ambiguous MCP count", []any{map[string]any{
			"name": name, "scope": "project", "path": active, "enabled": false,
			"provides": map[string]any{"mcpServers": json.Number("1"), "McpServers": json.Number("2")},
		}}, true, true},
		{"missing native name", []any{map[string]any{"scope": "project"}}, false, true},
		{"unexpected trust", []any{map[string]any{"name": name, "scope": "project", "path": active,
			"enabled": true, "provides": map[string]any{"mcpServers": json.Number("1")}}}, true, true},
		{"quoted MCP count", []any{map[string]any{"name": name, "scope": "project", "path": active,
			"enabled": false, "provides": map[string]any{"mcpServers": "1"}}}, true, true},
		{"fractional MCP count", []any{map[string]any{"name": name, "scope": "project", "path": active,
			"enabled": false, "provides": map[string]any{"mcpServers": json.Number("1.0")}}}, true, true},
		{"missing MCP discovery", []any{map[string]any{"name": name, "scope": "project", "path": active,
			"enabled": false, "provides": map[string]any{}}}, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := grokProjectMCPInventory(tc.plugins, name, active, tc.present)
			if (err != nil) != tc.refused {
				t.Fatalf("native inventory admission mismatch (refused=%t): %v", tc.refused, err)
			}
		})
	}
}

func TestGrokProjectMCPUnambiguousInspectIdentity(t *testing.T) {
	for _, tc := range []struct {
		name string
		fields map[string]any
		valid bool
	}{
		{"canonical", map[string]any{"projectTrusted": false, "plugins": []any{}}, true},
		{"unrelated extension", map[string]any{"projectTrusted": false, "futureVendorField": 1}, true},
		{"ASCII alias", map[string]any{"projectTrusted": false, "ProjectTrusted": true}, false},
		{"Unicode folded alias", map[string]any{"plugins": []any{}, "plu\u212ains": []any{}}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := grokProjectMCPUnambiguousFields(tc.fields, "projectTrusted", "plugins")
			if got != tc.valid {
				t.Fatalf("canonical inspect field admission: got %t, want %t", got, tc.valid)
			}
		})
	}
}

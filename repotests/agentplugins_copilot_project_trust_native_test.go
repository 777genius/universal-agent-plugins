//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestAgentpluginsCopilotNativeProjectTrustInstallation(t *testing.T) {
	if os.Getenv("AGENTPLUGINS_COPILOT_PROJECT_TRUST_RUNTIME") != "1" {
		t.Skip("opt-in genuine hosted Copilot project trust installation")
	}
	testCopilotProjectTrustInstallation(t)
}

// This independent native session starts without A/B trust. The established
// complete-cycle fixture retains its original capture and regression sequence.
func testCopilotProjectTrustInstallation(t *testing.T) {
	t.Helper()
	t.Setenv("AGENTPLUGINS_PROJECT_NATIVE_OUTPUT", t.TempDir())
	f := newCopilotProjectAttempt(t)
	defer f.cancel()
	config := filepath.Join(f.base.Home, ".copilot", "config.json")
	foreign := filepath.Join(f.base.Root, "foreign-trusted-folder")
	if err := os.Mkdir(foreign, 0700); err != nil {
		t.Fatal(err)
	}
	otherConfig := filepath.Join(f.other, ".github", "mcp.json")
	otherBody := []byte(`{"mcpServers":{"untrusted-project-b":{"type":"http","url":"http://127.0.0.1:9/mcp","tools":["*"]}}}`)
	projectNativeRuntimeWrite(t, otherConfig, otherBody, 0600)
	f.protected[otherConfig] = copilotProjectConfigHash(otherBody)
	original := map[string]any{
		"firstLaunchAt":  f.started.UTC().Format(time.RFC3339Nano),
		"trustedFolders": []any{foreign},
		"allowedTools":   []any{}, "deniedTools": []any{},
		"uapTrustSentinel": map[string]any{"preserve": "original", "enabled": true},
	}
	f.preparePackageWithConfig(t, func(t *testing.T) error {
		body, err := json.Marshal(original)
		if err != nil {
			return err
		}
		projectNativeRuntimeWrite(t, config, append(body, '\n'), 0600)
		return nil
	})
	copilotTrustConfigEqual(t, config, original)
	f.setupPrepared(t, "--yes")
	expected := make(map[string]any, len(original))
	for key, value := range original {
		expected[key] = value
	}
	expected["trustedFolders"] = []any{foreign, f.base.Project}
	copilotTrustConfigEqual(t, config, expected)
	// Freeze only after exact independently authored semantic comparison. No B,
	// parent-directory, foreign-setting or tool-permission change is admitted.
	body, err := projectNativeRuntimeRead(config, copilotProjectBodyLimit)
	if err != nil {
		t.Fatal(err)
	}
	f.protected[config] = copilotProjectConfigHash(body)
	copilotTrustMcpList(t, f, "A", f.base.Project, true)
	copilotTrustMcpList(t, f, "B", f.other, false)
	session := copilotTrustNativeCall(t, f)
	copilotTrustConfigEqual(t, config, expected)
	proof := map[string]any{"schema_version": 1, "native_session": session,
		"initial_projects_untrusted": true, "selected_project_remembered": true,
		"other_project_untrusted": true, "foreign_settings_preserved": true,
		"tool_permissions_preserved": true, "native_skills_catalog_verified": true,
		"mcp_initialize": true, "mcp_tools_list": true, "mcp_tools_call": true,
		"vendor_authentication_qualified": false, "real_model_qualified": false}
	encoded, err := json.Marshal(proof)
	if err != nil {
		t.Fatal(err)
	}
	// The existing frontend receipt retains bounded -test.v output, including
	// this proof, while the original cycle's closed archive stays unchanged.
	t.Logf("Copilot remembered trust evidence: %s", encoded)
	t.Log("Fresh pinned Copilot loaded installed project Skills/MCP and completed initialize/list/call after public --yes installation; B and foreign settings remained unchanged.")
}

func copilotTrustConfigEqual(t *testing.T, path string, expected map[string]any) {
	t.Helper()
	body, err := projectNativeRuntimeRead(path, copilotProjectBodyLimit)
	if err != nil {
		t.Fatal(err)
	}
	value, err := projectNativeRuntimeJSON(body)
	if err != nil || !reflect.DeepEqual(value, expected) {
		t.Fatal("Copilot selected trust or original foreign configuration differs", err)
	}
}

func copilotTrustMcpList(t *testing.T, f *copilotProjectAttempt, label, project string, installed bool) {
	t.Helper()
	r := f.command(t, "trust-list-"+label, project, f.env(), []string{f.client, "mcp", "list", "--json"}, 30*time.Second)
	value, err := projectNativeRuntimeJSON(r.Stdout)
	if err != nil || len(r.Stderr) != 0 {
		t.Fatal("fresh Copilot MCP listing did not return clean JSON", err)
	}
	document, ok := value.(map[string]any)
	if !ok {
		t.Fatal("Copilot MCP listing is not an object")
	}
	servers, ok := document["mcpServers"].(map[string]any)
	if !ok {
		t.Fatal("Copilot MCP listing has no server object")
	}
	if !installed && len(servers) != 0 {
		t.Fatal("Copilot loaded an MCP from the untrusted sibling project")
	}
	if installed {
		server, ok := servers["uapcopilota"].(map[string]any)
		if !ok || server["enabled"] != true || server["source"] != "workspace" || server["sourcePath"] != filepath.Join(project, ".github", "mcp.json") {
			t.Fatal("Copilot did not load the installed MCP from the exact trusted project")
		}
	}
	f.check(t)
}

func copilotTrustNativeCall(t *testing.T, f *copilotProjectAttempt) string {
	t.Helper()
	p := newCopilotProjectProvider("uapcopilota-inspect_runtime", f.base.Nonce, f.validateFacts)
	if err := p.start(f.ctx); err != nil {
		t.Fatal(err)
	}
	defer p.server.Close()
	env := append(f.env(), "COPILOT_PROVIDER_TYPE=openai", "COPILOT_PROVIDER_BASE_URL="+p.server.URL+"/v1", "COPILOT_PROVIDER_WIRE_API=completions", "COPILOT_MODEL=uap-fixture-model")
	names := bytes.ReplaceAll([]byte(copilotReadinessDisabled[len("Disabled tools: "):]), []byte(", "), []byte(","))
	args := []string{f.client, "-p", copilotReadinessPrompt, "--output-format", "json", "--no-auto-update", "--disable-client-telemetry", "--disable-builtin-mcps",
		"--excluded-tools=" + string(names), "--allow-tool=uapcopilota(inspect_runtime)",
		"--deny-tool=shell", "--deny-tool=write", "--deny-tool=read", "--deny-tool=url", "--deny-tool=memory"}
	result := f.command(t, "native-attempt", f.base.Project, env, args, 60*time.Second)
	p.server.Close()
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.err != nil || !p.returned || len(p.requests) != 2 || len(result.Stderr) != 0 {
		t.Fatalf("Copilot project trust native exchange incomplete: %v", p.err)
	}
	// Existing strict readiness checks join the exact project skill catalog,
	// workspace server, provider exchanges, and three actual probe events.
	log, err := projectNativeRuntimeRead(f.base.Events, copilotProjectBodyLimit)
	if err != nil {
		t.Fatal(err)
	}
	session, err := f.readinessBytesOrder(result.Stdout, log, p, true)
	if err != nil {
		t.Fatal(fmt.Errorf("Copilot installed trust runtime proof: %w", err))
	}
	f.check(t)
	return session
}

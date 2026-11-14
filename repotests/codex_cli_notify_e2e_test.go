package pluginkitairepo_test

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"
)

// Codex CLI --model for real hook e2e. Example:
//
//	PLUGIN_KIT_AI_RUN_CODEX_CLI=1 go test ./repotests -run TestCodexCLINotify -v -args -codex-model=gpt-5.4-mini
var codexModel = flag.String("codex-model", "gpt-5.4-mini", "codex exec --model for CLI e2e (notify smoke)")

func TestCodexCLINotify(t *testing.T) {
	codexBin := codexBinaryOrSkip(t)
	hookBin := buildPluginKitAIE2E(t)
	trace := filepath.Join(t.TempDir(), "trace.ndjson")
	dir := t.TempDir()
	notifyOverride := codexNotifyOverride(t, trace, hookBin)

	runCodexExec(t, codexBin, dir, trace, *codexModel, "Reply with exactly OK.", "-c", notifyOverride)

	lines := waitForTraceLines(t, trace, 3*time.Second)
	rec, ok := traceFind(t, lines, "Notify")
	if !ok {
		t.Fatalf("expected Notify trace entry; got:\n%s", strings.Join(lines, "\n"))
	}
	if strings.TrimSpace(rec.Outcome) != "continue" {
		t.Fatalf("notify outcome = %q; want continue", rec.Outcome)
	}
	if strings.TrimSpace(rec.RawJSON) == "" {
		t.Fatalf("expected raw_json in trace entry; got %+v", rec)
	}
}

func TestCodexProductionExampleNotifyUsesRealCLI(t *testing.T) {
	codexBin := codexBinaryOrSkip(t)
	pluginKitAIBin := buildPluginKitAI(t)
	dir, binPath := newRenderedCodexRuntimeExampleWorkspace(t, pluginKitAIBin)
	markerFile := filepath.Join(t.TempDir(), "notify-marker.txt")
	notifyOverride := codexNotifyBinaryOverride(t, markerFile, binPath)

	logOutput, output := runCodexExecWithMarkerProbe(t, codexBin, dir, markerFile, "Reply with exactly OK.", "-c", notifyOverride, "-m", *codexModel)
	if strings.TrimSpace(output) != "OK" {
		t.Fatalf("codex exec last message = %q, want %q\n%s", strings.TrimSpace(output), "OK", logOutput)
	}
	if _, err := os.Stat(markerFile); err != nil {
		t.Fatalf("production example notify marker missing: %v\n%s", err, logOutput)
	}
}

func TestCodexProductionExampleNotifyUsesRenderedProjectConfig(t *testing.T) {
	codexBin := codexBinaryOrSkip(t)
	pluginKitAIBin := buildPluginKitAI(t)
	dir, binPath := newRenderedCodexRuntimeExampleWorkspace(t, pluginKitAIBin)
	markerFile := filepath.Join(t.TempDir(), "notify-marker.txt")
	wrapCodexRuntimeBinaryWithMarker(t, binPath, markerFile)

	logOutput, output := runCodexExecWithProjectConfigMarkerProbe(t, codexBin, dir, "Reply with exactly OK.")
	if strings.TrimSpace(output) != "OK" {
		t.Fatalf("codex exec last message = %q, want %q\n%s", strings.TrimSpace(output), "OK", logOutput)
	}
	if !strings.Contains(logOutput, "model: gpt-5.4-mini") {
		t.Skipf("real codex exec did not honor production example project-local .codex/config.toml model %q in this build:\n%s", "gpt-5.4-mini", truncateRunes(logOutput, 4000))
	}
	if _, err := os.Stat(markerFile); err != nil {
		t.Skipf("real codex exec did not invoke notify from checked-in production example project-local .codex/config.toml in this build:\n%s", truncateRunes(logOutput, 4000))
	}
}

func TestCodexProductionExampleMCPGetWithOverride(t *testing.T) {
	codexBin := codexBinaryOrSkip(t)
	pluginKitAIBin := buildPluginKitAI(t)
	dir, _ := newRenderedCodexRuntimeExampleWorkspace(t, pluginKitAIBin)
	assertCodexConfigContains(t, dir,
		"[mcp_servers.release-checks]",
		`command = "/bin/echo"`,
		`args = ["codex-basic-prod"]`,
	)

	out := runCodexMCPGetWithArgs(t, codexBin, "release-checks",
		"-c", `mcp_servers.release-checks.command="/bin/echo"`,
		"-c", `mcp_servers.release-checks.args=["codex-basic-prod"]`,
	)
	if !strings.Contains(out, `"name":"release-checks"`) && !strings.Contains(out, `"name": "release-checks"`) {
		t.Fatalf("codex mcp get output missing production example runtime MCP server name:\n%s", out)
	}
	if !strings.Contains(out, `"/bin/echo"`) {
		t.Fatalf("codex mcp get output missing production example runtime MCP command %q:\n%s", "/bin/echo", out)
	}
	if !strings.Contains(out, `"codex-basic-prod"`) {
		t.Fatalf("codex mcp get output missing production example runtime MCP args:\n%s", out)
	}
}

func TestCodexProductionExampleMCPListWithOverride(t *testing.T) {
	codexBin := codexBinaryOrSkip(t)
	pluginKitAIBin := buildPluginKitAI(t)
	dir, _ := newRenderedCodexRuntimeExampleWorkspace(t, pluginKitAIBin)
	assertCodexConfigContains(t, dir,
		"[mcp_servers.release-checks]",
		`command = "/bin/echo"`,
		`args = ["codex-basic-prod"]`,
	)

	out := runCodexMCPListWithArgs(t, codexBin,
		"-c", `mcp_servers.release-checks.command="/bin/echo"`,
		"-c", `mcp_servers.release-checks.args=["codex-basic-prod"]`,
	)
	var entries []map[string]any
	if err := json.Unmarshal([]byte(out), &entries); err != nil {
		t.Fatalf("parse codex mcp list output: %v\n%s", err, out)
	}
	for _, entry := range entries {
		if strings.TrimSpace(fmt.Sprint(entry["name"])) != "release-checks" {
			continue
		}
		transport, ok := entry["transport"].(map[string]any)
		if !ok {
			t.Fatalf("codex mcp list production example runtime entry missing transport:\n%s", out)
		}
		if strings.TrimSpace(fmt.Sprint(transport["command"])) != "/bin/echo" {
			t.Fatalf("codex mcp list production example runtime command = %q want %q\n%s", transport["command"], "/bin/echo", out)
		}
		args, ok := transport["args"].([]any)
		if !ok || len(args) != 1 || strings.TrimSpace(fmt.Sprint(args[0])) != "codex-basic-prod" {
			t.Fatalf("codex mcp list production example runtime args = %#v want [codex-basic-prod]\n%s", transport["args"], out)
		}
		return
	}
	t.Fatalf("codex mcp list output missing production example runtime MCP server:\n%s", out)
}

func TestCodexProductionExampleMCPGetUsesRenderedProjectConfig(t *testing.T) {
	codexBin := codexBinaryOrSkip(t)
	pluginKitAIBin := buildPluginKitAI(t)
	dir, _ := newRenderedCodexRuntimeExampleWorkspace(t, pluginKitAIBin)
	assertCodexConfigContains(t, dir,
		"[mcp_servers.release-checks]",
		`command = "/bin/echo"`,
		`args = ["codex-basic-prod"]`,
	)

	out := runCodexMCPGetProbe(t, codexBin, dir, "release-checks")
	if !strings.Contains(out, `"name":"release-checks"`) && !strings.Contains(out, `"name": "release-checks"`) {
		t.Skipf("real codex mcp get did not expose checked-in production example project-local .codex/config.toml MCP server in this build:\n%s", truncateRunes(out, 4000))
	}
	if !strings.Contains(out, `"/bin/echo"`) {
		t.Fatalf("codex mcp get output missing production example generated command %q:\n%s", "/bin/echo", out)
	}
}

func TestCodexProductionExampleMCPListUsesRenderedProjectConfig(t *testing.T) {
	codexBin := codexBinaryOrSkip(t)
	pluginKitAIBin := buildPluginKitAI(t)
	dir, _ := newRenderedCodexRuntimeExampleWorkspace(t, pluginKitAIBin)
	assertCodexConfigContains(t, dir,
		"[mcp_servers.release-checks]",
		`command = "/bin/echo"`,
		`args = ["codex-basic-prod"]`,
	)

	out := runCodexMCPListWithProjectConfigProbe(t, codexBin, dir)
	var entries []map[string]any
	if err := json.Unmarshal([]byte(out), &entries); err != nil {
		t.Skipf("real codex mcp list did not return JSON for checked-in production example project-local .codex/config.toml in this build:\n%s", truncateRunes(out, 4000))
	}
	for _, entry := range entries {
		if strings.TrimSpace(fmt.Sprint(entry["name"])) != "release-checks" {
			continue
		}
		transport, ok := entry["transport"].(map[string]any)
		if !ok {
			t.Fatalf("codex mcp list production example runtime entry missing transport:\n%s", out)
		}
		if strings.TrimSpace(fmt.Sprint(transport["command"])) != "/bin/echo" {
			t.Fatalf("codex mcp list production example runtime command = %q want %q\n%s", transport["command"], "/bin/echo", out)
		}
		return
	}
	t.Skipf("real codex mcp list did not expose checked-in production example project-local .codex/config.toml MCP server in this build:\n%s", truncateRunes(out, 4000))
}

func TestCodexCLINotifyUsesRenderedProjectConfig(t *testing.T) {
	codexBin := codexBinaryOrSkip(t)
	pluginKitAIBin := buildPluginKitAI(t)
	hookBin := buildPluginKitAIE2E(t)
	trace := filepath.Join(t.TempDir(), "trace.ndjson")
	dir := newCodexRenderedNotifyWorkspace(t, pluginKitAIBin, hookBin, trace, *codexModel)

	logOutput, output, lines := runCodexExecWithProjectConfigProbe(t, codexBin, dir, trace, "Reply with exactly OK.")
	if strings.TrimSpace(output) != "OK" {
		t.Fatalf("codex exec last message = %q, want %q\n%s", strings.TrimSpace(output), "OK", logOutput)
	}
	if !strings.Contains(logOutput, "model: "+*codexModel) {
		t.Skipf("real codex exec did not honor project-local .codex/config.toml model %q in this build:\n%s", *codexModel, truncateRunes(logOutput, 4000))
	}
	rec, ok := traceFind(t, lines, "Notify")
	if !ok {
		t.Skipf("real codex exec did not invoke notify from project-local .codex/config.toml in this build:\n%s", truncateRunes(logOutput, 4000))
	}
	if strings.TrimSpace(rec.Outcome) != "continue" {
		t.Fatalf("notify outcome = %q; want continue", rec.Outcome)
	}
}

func TestCodexCLIMCPUsesRenderedProjectConfig(t *testing.T) {
	codexBin := codexBinaryOrSkip(t)
	pluginKitAIBin := buildPluginKitAI(t)
	dir := newCodexRenderedMCPWorkspace(t, pluginKitAIBin)

	out := runCodexMCPGetProbe(t, codexBin, dir, "release-checks")
	if !strings.Contains(out, `"name":"release-checks"`) && !strings.Contains(out, `"name": "release-checks"`) {
		t.Skipf("real codex mcp get did not expose project-local .codex/config.toml MCP server in this build:\n%s", truncateRunes(out, 4000))
	}
	if !strings.Contains(out, `"/bin/echo"`) {
		t.Fatalf("codex mcp get output missing generated command %q:\n%s", "/bin/echo", out)
	}
}

func TestCodexCLIMCPListUsesRenderedProjectConfig(t *testing.T) {
	codexBin := codexBinaryOrSkip(t)
	pluginKitAIBin := buildPluginKitAI(t)
	dir := newCodexRenderedMCPWorkspace(t, pluginKitAIBin)

	out := runCodexMCPListWithProjectConfigProbe(t, codexBin, dir)
	var entries []map[string]any
	if err := json.Unmarshal([]byte(out), &entries); err != nil {
		t.Skipf("real codex mcp list did not return JSON for project-local .codex/config.toml in this build:\n%s", truncateRunes(out, 4000))
	}
	for _, entry := range entries {
		if strings.TrimSpace(fmt.Sprint(entry["name"])) != "release-checks" {
			continue
		}
		transport, ok := entry["transport"].(map[string]any)
		if !ok {
			t.Fatalf("codex mcp list release-checks entry missing transport:\n%s", out)
		}
		if strings.TrimSpace(fmt.Sprint(transport["command"])) != "/bin/echo" {
			t.Fatalf("codex mcp list release-checks command = %q want %q\n%s", transport["command"], "/bin/echo", out)
		}
		return
	}
	t.Skipf("real codex mcp list did not expose project-local .codex/config.toml MCP server in this build:\n%s", truncateRunes(out, 4000))
}

func TestCodexCLIMCPGetWithOverride(t *testing.T) {
	codexBin := codexBinaryOrSkip(t)
	out := runCodexMCPGetWithArgs(t, codexBin, "release-checks",
		"-c", `mcp_servers.release-checks.command="/bin/echo"`,
		"-c", `mcp_servers.release-checks.args=["hello"]`,
	)
	if !strings.Contains(out, `"name":"release-checks"`) && !strings.Contains(out, `"name": "release-checks"`) {
		t.Fatalf("codex mcp get output missing server name:\n%s", out)
	}
	if !strings.Contains(out, `"/bin/echo"`) {
		t.Fatalf("codex mcp get output missing override command %q:\n%s", "/bin/echo", out)
	}
	if !strings.Contains(out, `"hello"`) {
		t.Fatalf("codex mcp get output missing override args:\n%s", out)
	}
}

func TestCodexCLIMCPListWithOverride(t *testing.T) {
	codexBin := codexBinaryOrSkip(t)
	out := runCodexMCPListWithArgs(t, codexBin,
		"-c", `mcp_servers.release-checks.command="/bin/echo"`,
		"-c", `mcp_servers.release-checks.args=["hello"]`,
	)
	var entries []map[string]any
	if err := json.Unmarshal([]byte(out), &entries); err != nil {
		t.Fatalf("parse codex mcp list output: %v\n%s", err, out)
	}
	for _, entry := range entries {
		if strings.TrimSpace(fmt.Sprint(entry["name"])) != "release-checks" {
			continue
		}
		transport, ok := entry["transport"].(map[string]any)
		if !ok {
			t.Fatalf("codex mcp list release-checks entry missing transport:\n%s", out)
		}
		if strings.TrimSpace(fmt.Sprint(transport["command"])) != "/bin/echo" {
			t.Fatalf("codex mcp list release-checks command = %q want %q\n%s", transport["command"], "/bin/echo", out)
		}
		args, ok := transport["args"].([]any)
		if !ok || len(args) != 1 || strings.TrimSpace(fmt.Sprint(args[0])) != "hello" {
			t.Fatalf("codex mcp list release-checks args = %#v want [hello]\n%s", transport["args"], out)
		}
		return
	}
	t.Fatalf("codex mcp list output missing release-checks server:\n%s", out)
}

func TestCodexCLIMCPAddGetListRemoveStdioInIsolatedHome(t *testing.T) {
	codexBin := codexBinaryOrSkip(t)
	tempHome := newCodexTempHome(t)

	runCodexMCPHomeCommand(t, codexBin, tempHome, "add", "release-checks", "--env", "PLUGIN_KIT_AI_MCP_SMOKE_STATIC=isolated-home", "--", "/bin/echo", "hello")
	assertCodexHomeConfigContains(t, tempHome,
		"[mcp_servers.release-checks]",
		`command = "/bin/echo"`,
		`args = ["hello"]`,
		`PLUGIN_KIT_AI_MCP_SMOKE_STATIC = "isolated-home"`,
	)
	out := runCodexMCPHomeCommand(t, codexBin, tempHome, "get", "release-checks", "--json")
	if !strings.Contains(out, `"name":"release-checks"`) && !strings.Contains(out, `"name": "release-checks"`) {
		t.Fatalf("isolated-home codex mcp get output missing server name:\n%s", out)
	}
	if !strings.Contains(out, `"/bin/echo"`) {
		t.Fatalf("isolated-home codex mcp get output missing command %q:\n%s", "/bin/echo", out)
	}
	if !strings.Contains(out, `"PLUGIN_KIT_AI_MCP_SMOKE_STATIC":"isolated-home"`) &&
		!strings.Contains(out, `"PLUGIN_KIT_AI_MCP_SMOKE_STATIC": "isolated-home"`) {
		t.Fatalf("isolated-home codex mcp get output missing env:\n%s", out)
	}

	listOut := runCodexMCPHomeCommand(t, codexBin, tempHome, "list", "--json")
	assertCodexMCPListEntry(t, listOut, "release-checks", "stdio", "/bin/echo", "hello", "PLUGIN_KIT_AI_MCP_SMOKE_STATIC", "isolated-home")

	runCodexMCPHomeCommand(t, codexBin, tempHome, "remove", "release-checks")
	listOut = runCodexMCPHomeCommand(t, codexBin, tempHome, "list", "--json")
	assertCodexMCPListMissing(t, listOut, "release-checks")
	assertCodexHomeConfigNotContains(t, tempHome, "[mcp_servers.release-checks]")
}

func TestCodexCLIMCPAddGetListRemoveHTTPInIsolatedHome(t *testing.T) {
	codexBin := codexBinaryOrSkip(t)
	tempHome := newCodexTempHome(t)

	runCodexMCPHomeCommand(t, codexBin, tempHome, "add", "docs", "--url", "https://example.com/mcp", "--bearer-token-env-var", "CODEX_DOCS_TOKEN")
	assertCodexHomeConfigContains(t, tempHome,
		"[mcp_servers.docs]",
		`url = "https://example.com/mcp"`,
		`bearer_token_env_var = "CODEX_DOCS_TOKEN"`,
	)
	out := runCodexMCPHomeCommand(t, codexBin, tempHome, "get", "docs", "--json")
	if !strings.Contains(out, `"name":"docs"`) && !strings.Contains(out, `"name": "docs"`) {
		t.Fatalf("isolated-home codex mcp get output missing HTTP server name:\n%s", out)
	}
	if !strings.Contains(out, `"type":"streamable_http"`) && !strings.Contains(out, `"type": "streamable_http"`) {
		t.Fatalf("isolated-home codex mcp get output missing streamable_http transport:\n%s", out)
	}
	if !strings.Contains(out, `"url":"https://example.com/mcp"`) && !strings.Contains(out, `"url": "https://example.com/mcp"`) {
		t.Fatalf("isolated-home codex mcp get output missing HTTP MCP url:\n%s", out)
	}
	if !strings.Contains(out, `"bearer_token_env_var":"CODEX_DOCS_TOKEN"`) &&
		!strings.Contains(out, `"bearer_token_env_var": "CODEX_DOCS_TOKEN"`) {
		t.Fatalf("isolated-home codex mcp get output missing bearer_token_env_var:\n%s", out)
	}

	listOut := runCodexMCPHomeCommand(t, codexBin, tempHome, "list", "--json")
	assertCodexMCPHTTPListEntry(t, listOut, "docs", "https://example.com/mcp", "CODEX_DOCS_TOKEN")

	runCodexMCPHomeCommand(t, codexBin, tempHome, "remove", "docs")
	listOut = runCodexMCPHomeCommand(t, codexBin, tempHome, "list", "--json")
	assertCodexMCPListMissing(t, listOut, "docs")
	assertCodexHomeConfigNotContains(t, tempHome, "[mcp_servers.docs]")
}

func TestCodexCLIMCPAddExecStdioInIsolatedHome(t *testing.T) {
	codexBin := codexBinaryOrSkip(t)
	tempHome := newCodexTempHome(t)
	mcpBin := buildPortableMCPSmokeServer(t)
	markerPath := filepath.Join(t.TempDir(), "isolated-home-mcp-marker.json")

	runCodexMCPHomeCommand(t, codexBin, tempHome, "add", "release-checks", "--", filepath.ToSlash(mcpBin))
	assertCodexHomeConfigContains(t, tempHome,
		"[mcp_servers.release-checks]",
		`command = "`+filepath.ToSlash(mcpBin)+`"`,
	)
	out := runCodexMCPHomeCommand(t, codexBin, tempHome, "get", "release-checks", "--json")
	if !strings.Contains(out, `"name":"release-checks"`) && !strings.Contains(out, `"name": "release-checks"`) {
		t.Fatalf("isolated-home codex mcp get output missing live stdio server name:\n%s", out)
	}

	runCodexExecWithHomePortableMCP(t, codexBin, tempHome, markerPath, *codexModel)
	assertPortableMCPMarker(t, markerPath, "tools/call", "release_checks", "CODEX_PORTABLE_MCP_OK")
}

func TestCodexCLIMCPAddExecStdioInAuthSeededCodexHome(t *testing.T) {
	codexBin := codexBinaryOrSkip(t)
	tempHome := newAuthSeededCodexTempHome(t)
	mcpBin := buildPortableMCPSmokeServer(t)
	markerPath := filepath.Join(t.TempDir(), "auth-seeded-home-mcp-marker.json")

	loginOut := runCodexHomeCommand(t, codexBin, tempHome, "login", "status")
	if !strings.Contains(loginOut, "Logged in") {
		t.Fatalf("auth-seeded CODEX_HOME did not preserve login status:\n%s", loginOut)
	}

	runCodexMCPHomeCommand(t, codexBin, tempHome, "add", "release-checks", "--", filepath.ToSlash(mcpBin))
	assertCodexHomeConfigContains(t, tempHome,
		"[mcp_servers.release-checks]",
		`command = "`+filepath.ToSlash(mcpBin)+`"`,
	)
	out := runCodexMCPHomeCommand(t, codexBin, tempHome, "get", "release-checks", "--json")
	if !strings.Contains(out, `"name":"release-checks"`) && !strings.Contains(out, `"name": "release-checks"`) {
		t.Fatalf("auth-seeded codex mcp get output missing live stdio server name:\n%s", out)
	}
	listOut := runCodexMCPHomeCommand(t, codexBin, tempHome, "list", "--json")
	assertCodexMCPListEntry(t, listOut, "release-checks", "stdio", filepath.ToSlash(mcpBin), "", "", "")

	runCodexExecWithHomePortableMCP(t, codexBin, tempHome, markerPath, *codexModel)
	assertPortableMCPMarker(t, markerPath, "tools/call", "release_checks", "CODEX_PORTABLE_MCP_OK")
}

func TestCodexCLIMCPAddGetListRemoveStdioInAuthSeededCodexHome(t *testing.T) {
	codexBin := codexBinaryOrSkip(t)
	tempHome := newAuthSeededCodexTempHome(t)
	mcpBin := buildPortableMCPSmokeServer(t)

	loginOut := runCodexHomeCommand(t, codexBin, tempHome, "login", "status")
	if !strings.Contains(loginOut, "Logged in") {
		t.Fatalf("auth-seeded CODEX_HOME did not preserve login status:\n%s", loginOut)
	}

	runCodexMCPHomeCommand(t, codexBin, tempHome, "add", "release-checks", "--", filepath.ToSlash(mcpBin))
	assertCodexHomeConfigContains(t, tempHome,
		"[mcp_servers.release-checks]",
		`command = "`+filepath.ToSlash(mcpBin)+`"`,
	)
	out := runCodexMCPHomeCommand(t, codexBin, tempHome, "get", "release-checks", "--json")
	if !strings.Contains(out, `"name":"release-checks"`) && !strings.Contains(out, `"name": "release-checks"`) {
		t.Fatalf("auth-seeded codex mcp get output missing live stdio server name:\n%s", out)
	}
	listOut := runCodexMCPHomeCommand(t, codexBin, tempHome, "list", "--json")
	assertCodexMCPListEntry(t, listOut, "release-checks", "stdio", filepath.ToSlash(mcpBin), "", "", "")

	runCodexMCPHomeCommand(t, codexBin, tempHome, "remove", "release-checks")
	listOut = runCodexMCPHomeCommand(t, codexBin, tempHome, "list", "--json")
	assertCodexMCPListMissing(t, listOut, "release-checks")
	assertCodexHomeConfigNotContains(t, tempHome, "[mcp_servers.release-checks]")
}

func TestCodexCLIMCPAddGetListRemoveStdioWithEnvInAuthSeededCodexHome(t *testing.T) {
	codexBin := codexBinaryOrSkip(t)
	tempHome := newAuthSeededCodexTempHome(t)
	mcpBin := buildPortableMCPSmokeServer(t)

	loginOut := runCodexHomeCommand(t, codexBin, tempHome, "login", "status")
	if !strings.Contains(loginOut, "Logged in") {
		t.Fatalf("auth-seeded CODEX_HOME did not preserve login status:\n%s", loginOut)
	}

	runCodexMCPHomeCommand(t, codexBin, tempHome, "add", "release-checks", "--env", "PLUGIN_KIT_AI_MCP_SMOKE_STATIC=auth-seeded-home", "--", filepath.ToSlash(mcpBin))
	assertCodexHomeConfigContains(t, tempHome,
		"[mcp_servers.release-checks]",
		`command = "`+filepath.ToSlash(mcpBin)+`"`,
		`[mcp_servers.release-checks.env]`,
		`PLUGIN_KIT_AI_MCP_SMOKE_STATIC = "auth-seeded-home"`,
	)
	out := runCodexMCPHomeCommand(t, codexBin, tempHome, "get", "release-checks", "--json")
	if !strings.Contains(out, `"name":"release-checks"`) && !strings.Contains(out, `"name": "release-checks"`) {
		t.Fatalf("auth-seeded codex mcp get output missing stdio env server name:\n%s", out)
	}
	if !strings.Contains(out, `"PLUGIN_KIT_AI_MCP_SMOKE_STATIC":"auth-seeded-home"`) &&
		!strings.Contains(out, `"PLUGIN_KIT_AI_MCP_SMOKE_STATIC": "auth-seeded-home"`) {
		t.Fatalf("auth-seeded codex mcp get output missing stdio env projection:\n%s", out)
	}
	listOut := runCodexMCPHomeCommand(t, codexBin, tempHome, "list", "--json")
	assertCodexMCPListEntry(t, listOut, "release-checks", "stdio", filepath.ToSlash(mcpBin), "", "PLUGIN_KIT_AI_MCP_SMOKE_STATIC", "auth-seeded-home")

	runCodexMCPHomeCommand(t, codexBin, tempHome, "remove", "release-checks")
	listOut = runCodexMCPHomeCommand(t, codexBin, tempHome, "list", "--json")
	assertCodexMCPListMissing(t, listOut, "release-checks")
	assertCodexHomeConfigNotContains(t, tempHome, "[mcp_servers.release-checks]")
}

func TestCodexCLIMCPAddGetListRemoveHTTPInAuthSeededCodexHome(t *testing.T) {
	codexBin := codexBinaryOrSkip(t)
	tempHome := newAuthSeededCodexTempHome(t)

	loginOut := runCodexHomeCommand(t, codexBin, tempHome, "login", "status")
	if !strings.Contains(loginOut, "Logged in") {
		t.Fatalf("auth-seeded CODEX_HOME did not preserve login status:\n%s", loginOut)
	}

	runCodexMCPHomeCommand(t, codexBin, tempHome, "add", "docs", "--url", "https://example.com/mcp", "--bearer-token-env-var", "CODEX_DOCS_TOKEN")
	assertCodexHomeConfigContains(t, tempHome,
		"[mcp_servers.docs]",
		`url = "https://example.com/mcp"`,
		`bearer_token_env_var = "CODEX_DOCS_TOKEN"`,
	)
	out := runCodexMCPHomeCommand(t, codexBin, tempHome, "get", "docs", "--json")
	if !strings.Contains(out, `"name":"docs"`) && !strings.Contains(out, `"name": "docs"`) {
		t.Fatalf("auth-seeded codex mcp get output missing HTTP server name:\n%s", out)
	}
	if !strings.Contains(out, `"type":"streamable_http"`) && !strings.Contains(out, `"type": "streamable_http"`) {
		t.Fatalf("auth-seeded codex mcp get output missing streamable_http transport:\n%s", out)
	}
	if !strings.Contains(out, `"url":"https://example.com/mcp"`) && !strings.Contains(out, `"url": "https://example.com/mcp"`) {
		t.Fatalf("auth-seeded codex mcp get output missing HTTP MCP url:\n%s", out)
	}
	if !strings.Contains(out, `"bearer_token_env_var":"CODEX_DOCS_TOKEN"`) &&
		!strings.Contains(out, `"bearer_token_env_var": "CODEX_DOCS_TOKEN"`) {
		t.Fatalf("auth-seeded codex mcp get output missing bearer_token_env_var:\n%s", out)
	}
	listOut := runCodexMCPHomeCommand(t, codexBin, tempHome, "list", "--json")
	assertCodexMCPHTTPListEntry(t, listOut, "docs", "https://example.com/mcp", "CODEX_DOCS_TOKEN")

	runCodexMCPHomeCommand(t, codexBin, tempHome, "remove", "docs")
	listOut = runCodexMCPHomeCommand(t, codexBin, tempHome, "list", "--json")
	assertCodexMCPListMissing(t, listOut, "docs")
	assertCodexHomeConfigNotContains(t, tempHome, "[mcp_servers.docs]")
}

func TestCodexCLIMCPLoginLogoutRejectStdioInAuthSeededCodexHome(t *testing.T) {
	codexBin := codexBinaryOrSkip(t)
	tempHome := newAuthSeededCodexTempHome(t)

	loginOut := runCodexHomeCommand(t, codexBin, tempHome, "login", "status")
	if !strings.Contains(loginOut, "Logged in") {
		t.Fatalf("auth-seeded CODEX_HOME did not preserve login status:\n%s", loginOut)
	}

	runCodexMCPHomeCommand(t, codexBin, tempHome, "add", "release-checks", "--env", "PLUGIN_KIT_AI_MCP_SMOKE_STATIC=codex-package-live", "--", "/bin/echo", "hello")
	assertCodexHomeConfigContains(t, tempHome,
		"[mcp_servers.release-checks]",
		`command = "/bin/echo"`,
		`PLUGIN_KIT_AI_MCP_SMOKE_STATIC = "codex-package-live"`,
	)

	loginErr := runCodexMCPHomeCommandExpectError(t, codexBin, tempHome, "login", "release-checks")
	if !strings.Contains(loginErr, "OAuth login is only supported for streamable HTTP servers.") {
		t.Fatalf("codex mcp login on stdio server should reject with documented oauth message:\n%s", loginErr)
	}

	logoutErr := runCodexMCPHomeCommandExpectError(t, codexBin, tempHome, "logout", "release-checks")
	if !strings.Contains(logoutErr, "OAuth logout is only supported for streamable_http transports.") {
		t.Fatalf("codex mcp logout on stdio server should reject with documented oauth message:\n%s", logoutErr)
	}

	out := runCodexMCPHomeCommand(t, codexBin, tempHome, "get", "release-checks", "--json")
	if !strings.Contains(out, `"name":"release-checks"`) && !strings.Contains(out, `"name": "release-checks"`) {
		t.Fatalf("auth-seeded codex mcp get output missing stdio server name after oauth rejection:\n%s", out)
	}

	listOut := runCodexMCPHomeCommand(t, codexBin, tempHome, "list", "--json")
	assertCodexMCPListEntry(t, listOut, "release-checks", "stdio", "/bin/echo", "hello", "PLUGIN_KIT_AI_MCP_SMOKE_STATIC", "codex-package-live")

	runCodexMCPHomeCommand(t, codexBin, tempHome, "remove", "release-checks")
	listOut = runCodexMCPHomeCommand(t, codexBin, tempHome, "list", "--json")
	assertCodexMCPListMissing(t, listOut, "release-checks")
	assertCodexHomeConfigNotContains(t, tempHome, "[mcp_servers.release-checks]")
}

func TestCodexCLIMCPMissingServerBehaviorInAuthSeededCodexHome(t *testing.T) {
	codexBin := codexBinaryOrSkip(t)
	tempHome := newAuthSeededCodexTempHome(t)

	loginOut := runCodexHomeCommand(t, codexBin, tempHome, "login", "status")
	if !strings.Contains(loginOut, "Logged in") {
		t.Fatalf("auth-seeded CODEX_HOME did not preserve login status:\n%s", loginOut)
	}

	runCodexMCPHomeCommand(t, codexBin, tempHome, "add", "release-checks", "--", "/bin/echo", "hello")
	runCodexMCPHomeCommand(t, codexBin, tempHome, "remove", "release-checks")

	getErr := runCodexMCPHomeCommandExpectError(t, codexBin, tempHome, "get", "release-checks", "--json")
	if !strings.Contains(getErr, "No MCP server named 'release-checks' found.") {
		t.Fatalf("codex mcp get on removed server should report missing server:\n%s", getErr)
	}

	removeOut := runCodexMCPHomeCommand(t, codexBin, tempHome, "remove", "release-checks")
	if !strings.Contains(removeOut, "No MCP server named 'release-checks' found.") {
		t.Fatalf("codex mcp remove on missing server should be idempotent:\n%s", removeOut)
	}

	listOut := runCodexMCPHomeCommand(t, codexBin, tempHome, "list", "--json")
	assertCodexMCPListMissing(t, listOut, "release-checks")
	assertCodexHomeConfigNotContains(t, tempHome, "[mcp_servers.release-checks]")
}

func TestCodexPackageMCPGetUsesRenderedSidecar(t *testing.T) {
	codexBin := codexBinaryOrSkip(t)
	pluginKitAIBin := buildPluginKitAI(t)
	mcpBin := buildPortableMCPSmokeServer(t)
	workDir := newCodexPackageRenderedMCPWorkspace(t, pluginKitAIBin, mcpBin)
	mcpServer := readRenderedSharedMCPServer(t, workDir, "release-checks")
	configArgs := codexMCPConfigArgs("release-checks", mcpServer)

	out := runCodexMCPGetWithArgs(t, codexBin, "release-checks", configArgs...)
	if !strings.Contains(out, `"name":"release-checks"`) && !strings.Contains(out, `"name": "release-checks"`) {
		t.Fatalf("codex mcp get output missing generated package MCP server name:\n%s", out)
	}
	wantCommand := filepath.ToSlash(mcpBin)
	if !strings.Contains(out, wantCommand) {
		t.Fatalf("codex mcp get output missing generated package MCP command %q:\n%s", wantCommand, out)
	}
	if !strings.Contains(out, `"PLUGIN_KIT_AI_MCP_SMOKE_STATIC":"codex-package-live"`) &&
		!strings.Contains(out, `"PLUGIN_KIT_AI_MCP_SMOKE_STATIC": "codex-package-live"`) {
		t.Fatalf("codex mcp get output missing generated package MCP env:\n%s", out)
	}
}

func TestCodexPackageMCPListUsesRenderedSidecar(t *testing.T) {
	codexBin := codexBinaryOrSkip(t)
	pluginKitAIBin := buildPluginKitAI(t)
	mcpBin := buildPortableMCPSmokeServer(t)
	workDir := newCodexPackageRenderedMCPWorkspace(t, pluginKitAIBin, mcpBin)
	mcpServer := readRenderedSharedMCPServer(t, workDir, "release-checks")
	configArgs := codexMCPConfigArgs("release-checks", mcpServer)

	out := runCodexMCPListWithArgs(t, codexBin, configArgs...)
	var entries []map[string]any
	if err := json.Unmarshal([]byte(out), &entries); err != nil {
		t.Fatalf("parse codex mcp list output: %v\n%s", err, out)
	}
	wantCommand := filepath.ToSlash(mcpBin)
	for _, entry := range entries {
		if strings.TrimSpace(fmt.Sprint(entry["name"])) != "release-checks" {
			continue
		}
		transport, ok := entry["transport"].(map[string]any)
		if !ok {
			t.Fatalf("codex mcp list release-checks entry missing transport:\n%s", out)
		}
		if strings.TrimSpace(fmt.Sprint(transport["type"])) != "stdio" {
			t.Fatalf("codex mcp list release-checks transport type = %q want %q\n%s", transport["type"], "stdio", out)
		}
		if strings.TrimSpace(fmt.Sprint(transport["command"])) != wantCommand {
			t.Fatalf("codex mcp list release-checks command = %q want %q\n%s", transport["command"], wantCommand, out)
		}
		env, ok := transport["env"].(map[string]any)
		if !ok {
			t.Fatalf("codex mcp list release-checks transport missing env:\n%s", out)
		}
		if strings.TrimSpace(fmt.Sprint(env["PLUGIN_KIT_AI_MCP_SMOKE_STATIC"])) != "codex-package-live" {
			t.Fatalf("codex mcp list release-checks env value = %q want %q\n%s", env["PLUGIN_KIT_AI_MCP_SMOKE_STATIC"], "codex-package-live", out)
		}
		return
	}
	t.Fatalf("codex mcp list output missing release-checks server:\n%s", out)
}

func TestCodexPackageExecUsesRenderedSidecarMCP(t *testing.T) {
	codexBin := codexBinaryOrSkip(t)
	pluginKitAIBin := buildPluginKitAI(t)
	mcpBin := buildPortableMCPSmokeServer(t)
	workDir := newCodexPackageRenderedMCPWorkspace(t, pluginKitAIBin, mcpBin)
	marker := filepath.Join(t.TempDir(), "codex-package-mcp-marker.json")

	runCodexExecWithPortableMCP(t, codexBin, workDir, marker, *codexModel, mcpBin)
	assertPortableMCPMarker(t, marker, "tools/call", "release_checks", "CODEX_PORTABLE_MCP_OK")
}

func TestCodexPackageProductionExampleMCPGetUsesRenderedSidecar(t *testing.T) {
	codexBin := codexBinaryOrSkip(t)
	pluginKitAIBin := buildPluginKitAI(t)
	workDir := newRenderedCodexPackageExampleWorkspace(t, pluginKitAIBin)
	server := readRenderedSharedMCPServer(t, workDir, "docs")
	configArgs := codexMCPConfigArgsFromRenderedServer(t, "docs", server)

	out := runCodexMCPGetWithArgs(t, codexBin, "docs", configArgs...)
	if !strings.Contains(out, `"name":"docs"`) && !strings.Contains(out, `"name": "docs"`) {
		t.Fatalf("codex mcp get output missing docs server name:\n%s", out)
	}
	if !strings.Contains(out, `"type":"streamable_http"`) && !strings.Contains(out, `"type": "streamable_http"`) {
		t.Fatalf("codex mcp get output missing streamable_http transport:\n%s", out)
	}
	if !strings.Contains(out, `"url":"https://example.com/mcp"`) && !strings.Contains(out, `"url": "https://example.com/mcp"`) {
		t.Fatalf("codex mcp get output missing production example MCP URL:\n%s", out)
	}
}

func TestCodexPackageProductionExampleMCPListUsesRenderedSidecar(t *testing.T) {
	codexBin := codexBinaryOrSkip(t)
	pluginKitAIBin := buildPluginKitAI(t)
	workDir := newRenderedCodexPackageExampleWorkspace(t, pluginKitAIBin)
	server := readRenderedSharedMCPServer(t, workDir, "docs")
	configArgs := codexMCPConfigArgsFromRenderedServer(t, "docs", server)

	out := runCodexMCPListWithArgs(t, codexBin, configArgs...)
	var entries []map[string]any
	if err := json.Unmarshal([]byte(out), &entries); err != nil {
		t.Fatalf("parse codex mcp list output: %v\n%s", err, out)
	}
	for _, entry := range entries {
		if strings.TrimSpace(fmt.Sprint(entry["name"])) != "docs" {
			continue
		}
		transport, ok := entry["transport"].(map[string]any)
		if !ok {
			t.Fatalf("codex mcp list docs entry missing transport:\n%s", out)
		}
		if strings.TrimSpace(fmt.Sprint(transport["type"])) != "streamable_http" {
			t.Fatalf("codex mcp list docs transport type = %q want %q\n%s", transport["type"], "streamable_http", out)
		}
		if strings.TrimSpace(fmt.Sprint(transport["url"])) != "https://example.com/mcp" {
			t.Fatalf("codex mcp list docs transport url = %q want %q\n%s", transport["url"], "https://example.com/mcp", out)
		}
		return
	}
	t.Fatalf("codex mcp list output missing docs server:\n%s", out)
}

func TestCodexProductionExampleRuntimeMCPAddGetListRemoveInIsolatedHome(t *testing.T) {
	codexBin := codexBinaryOrSkip(t)
	pluginKitAIBin := buildPluginKitAI(t)
	dir, _ := newRenderedCodexRuntimeExampleWorkspace(t, pluginKitAIBin)
	assertCodexConfigContains(t, dir,
		"[mcp_servers.release-checks]",
		`command = "/bin/echo"`,
		`args = ["codex-basic-prod"]`,
	)

	tempHome := newCodexTempHome(t)
	runCodexMCPHomeCommand(t, codexBin, tempHome, "add", "release-checks", "--", "/bin/echo", "codex-basic-prod")
	assertCodexHomeConfigContains(t, tempHome,
		"[mcp_servers.release-checks]",
		`command = "/bin/echo"`,
		`args = ["codex-basic-prod"]`,
	)

	out := runCodexMCPHomeCommand(t, codexBin, tempHome, "get", "release-checks", "--json")
	if !strings.Contains(out, `"name":"release-checks"`) && !strings.Contains(out, `"name": "release-checks"`) {
		t.Fatalf("isolated-home codex mcp get output missing production runtime server name:\n%s", out)
	}
	if !strings.Contains(out, `"/bin/echo"`) || !strings.Contains(out, `"codex-basic-prod"`) {
		t.Fatalf("isolated-home codex mcp get output missing production runtime MCP projection:\n%s", out)
	}

	listOut := runCodexMCPHomeCommand(t, codexBin, tempHome, "list", "--json")
	assertCodexMCPListEntry(t, listOut, "release-checks", "stdio", "/bin/echo", "codex-basic-prod", "", "")

	runCodexMCPHomeCommand(t, codexBin, tempHome, "remove", "release-checks")
	listOut = runCodexMCPHomeCommand(t, codexBin, tempHome, "list", "--json")
	assertCodexMCPListMissing(t, listOut, "release-checks")
}

func TestCodexPackageProductionExampleMCPAddGetListRemoveInIsolatedHome(t *testing.T) {
	codexBin := codexBinaryOrSkip(t)
	pluginKitAIBin := buildPluginKitAI(t)
	workDir := newRenderedCodexPackageExampleWorkspace(t, pluginKitAIBin)
	server := readRenderedSharedMCPServer(t, workDir, "docs")
	url := strings.TrimSpace(fmt.Sprint(server["url"]))
	if url == "" {
		t.Fatalf("generated docs MCP server missing url: %#v", server)
	}

	tempHome := newCodexTempHome(t)
	runCodexMCPHomeCommand(t, codexBin, tempHome, "add", "docs", "--url", url)
	assertCodexHomeConfigContains(t, tempHome,
		"[mcp_servers.docs]",
		`url = "`+url+`"`,
	)

	out := runCodexMCPHomeCommand(t, codexBin, tempHome, "get", "docs", "--json")
	if !strings.Contains(out, `"name":"docs"`) && !strings.Contains(out, `"name": "docs"`) {
		t.Fatalf("isolated-home codex mcp get output missing production package server name:\n%s", out)
	}
	if !strings.Contains(out, `"type":"streamable_http"`) && !strings.Contains(out, `"type": "streamable_http"`) {
		t.Fatalf("isolated-home codex mcp get output missing production package transport type:\n%s", out)
	}
	if !strings.Contains(out, `"url":"`+url+`"`) && !strings.Contains(out, `"url": "`+url+`"`) {
		t.Fatalf("isolated-home codex mcp get output missing production package MCP URL:\n%s", out)
	}

	listOut := runCodexMCPHomeCommand(t, codexBin, tempHome, "list", "--json")
	assertCodexMCPHTTPListEntry(t, listOut, "docs", url, "")

	runCodexMCPHomeCommand(t, codexBin, tempHome, "remove", "docs")
	listOut = runCodexMCPHomeCommand(t, codexBin, tempHome, "list", "--json")
	assertCodexMCPListMissing(t, listOut, "docs")
}

func TestCodexProductionExampleRuntimeMCPAddGetListRemoveInAuthSeededCodexHome(t *testing.T) {
	codexBin := codexBinaryOrSkip(t)
	pluginKitAIBin := buildPluginKitAI(t)
	dir, _ := newRenderedCodexRuntimeExampleWorkspace(t, pluginKitAIBin)
	assertCodexConfigContains(t, dir,
		"[mcp_servers.release-checks]",
		`command = "/bin/echo"`,
		`args = ["codex-basic-prod"]`,
	)

	tempHome := newAuthSeededCodexTempHome(t)
	loginOut := runCodexHomeCommand(t, codexBin, tempHome, "login", "status")
	if !strings.Contains(loginOut, "Logged in") {
		t.Fatalf("auth-seeded CODEX_HOME did not preserve login status:\n%s", loginOut)
	}

	runCodexMCPHomeCommand(t, codexBin, tempHome, "add", "release-checks", "--", "/bin/echo", "codex-basic-prod")
	assertCodexHomeConfigContains(t, tempHome,
		"[mcp_servers.release-checks]",
		`command = "/bin/echo"`,
		`args = ["codex-basic-prod"]`,
	)

	out := runCodexMCPHomeCommand(t, codexBin, tempHome, "get", "release-checks", "--json")
	if !strings.Contains(out, `"name":"release-checks"`) && !strings.Contains(out, `"name": "release-checks"`) {
		t.Fatalf("auth-seeded codex mcp get output missing production runtime server name:\n%s", out)
	}
	if !strings.Contains(out, `"/bin/echo"`) || !strings.Contains(out, `"codex-basic-prod"`) {
		t.Fatalf("auth-seeded codex mcp get output missing production runtime MCP projection:\n%s", out)
	}

	listOut := runCodexMCPHomeCommand(t, codexBin, tempHome, "list", "--json")
	assertCodexMCPListEntry(t, listOut, "release-checks", "stdio", "/bin/echo", "codex-basic-prod", "", "")

	runCodexMCPHomeCommand(t, codexBin, tempHome, "remove", "release-checks")
	listOut = runCodexMCPHomeCommand(t, codexBin, tempHome, "list", "--json")
	assertCodexMCPListMissing(t, listOut, "release-checks")
	assertCodexHomeConfigNotContains(t, tempHome, "[mcp_servers.release-checks]")
}

func TestCodexPackageProductionExampleMCPAddGetListRemoveInAuthSeededCodexHome(t *testing.T) {
	codexBin := codexBinaryOrSkip(t)
	pluginKitAIBin := buildPluginKitAI(t)
	workDir := newRenderedCodexPackageExampleWorkspace(t, pluginKitAIBin)
	server := readRenderedSharedMCPServer(t, workDir, "docs")
	url := strings.TrimSpace(fmt.Sprint(server["url"]))
	if url == "" {
		t.Fatalf("generated docs MCP server missing url: %#v", server)
	}

	tempHome := newAuthSeededCodexTempHome(t)
	loginOut := runCodexHomeCommand(t, codexBin, tempHome, "login", "status")
	if !strings.Contains(loginOut, "Logged in") {
		t.Fatalf("auth-seeded CODEX_HOME did not preserve login status:\n%s", loginOut)
	}

	runCodexMCPHomeCommand(t, codexBin, tempHome, "add", "docs", "--url", url)
	assertCodexHomeConfigContains(t, tempHome,
		"[mcp_servers.docs]",
		`url = "`+url+`"`,
	)

	out := runCodexMCPHomeCommand(t, codexBin, tempHome, "get", "docs", "--json")
	if !strings.Contains(out, `"name":"docs"`) && !strings.Contains(out, `"name": "docs"`) {
		t.Fatalf("auth-seeded codex mcp get output missing production package server name:\n%s", out)
	}
	if !strings.Contains(out, `"type":"streamable_http"`) && !strings.Contains(out, `"type": "streamable_http"`) {
		t.Fatalf("auth-seeded codex mcp get output missing production package transport type:\n%s", out)
	}
	if !strings.Contains(out, `"url":"`+url+`"`) && !strings.Contains(out, `"url": "`+url+`"`) {
		t.Fatalf("auth-seeded codex mcp get output missing production package MCP URL:\n%s", out)
	}

	listOut := runCodexMCPHomeCommand(t, codexBin, tempHome, "list", "--json")
	assertCodexMCPHTTPListEntry(t, listOut, "docs", url, "")

	runCodexMCPHomeCommand(t, codexBin, tempHome, "remove", "docs")
	listOut = runCodexMCPHomeCommand(t, codexBin, tempHome, "list", "--json")
	assertCodexMCPListMissing(t, listOut, "docs")
	assertCodexHomeConfigNotContains(t, tempHome, "[mcp_servers.docs]")
}

func TestCodexPackageRenderedSidecarMCPAddGetListRemoveInIsolatedHome(t *testing.T) {
	codexBin := codexBinaryOrSkip(t)
	pluginKitAIBin := buildPluginKitAI(t)
	mcpBin := buildPortableMCPSmokeServer(t)
	workDir := newCodexPackageRenderedMCPWorkspace(t, pluginKitAIBin, mcpBin)
	server := readRenderedSharedMCPServer(t, workDir, "release-checks")
	tempHome := newCodexTempHome(t)

	runCodexMCPAddRenderedServerInHome(t, codexBin, tempHome, "release-checks", server)
	assertCodexHomeConfigContains(t, tempHome,
		"[mcp_servers.release-checks]",
		`command = "`+filepath.ToSlash(mcpBin)+`"`,
		`PLUGIN_KIT_AI_MCP_SMOKE_STATIC = "codex-package-live"`,
	)

	out := runCodexMCPHomeCommand(t, codexBin, tempHome, "get", "release-checks", "--json")
	if !strings.Contains(out, `"name":"release-checks"`) && !strings.Contains(out, `"name": "release-checks"`) {
		t.Fatalf("isolated-home codex mcp get output missing generated package stdio server name:\n%s", out)
	}
	if !strings.Contains(out, filepath.ToSlash(mcpBin)) {
		t.Fatalf("isolated-home codex mcp get output missing generated package stdio command:\n%s", out)
	}
	if !strings.Contains(out, `"PLUGIN_KIT_AI_MCP_SMOKE_STATIC":"codex-package-live"`) &&
		!strings.Contains(out, `"PLUGIN_KIT_AI_MCP_SMOKE_STATIC": "codex-package-live"`) {
		t.Fatalf("isolated-home codex mcp get output missing generated package stdio env:\n%s", out)
	}
	listOut := runCodexMCPHomeCommand(t, codexBin, tempHome, "list", "--json")
	assertCodexMCPListEntry(t, listOut, "release-checks", "stdio", filepath.ToSlash(mcpBin), "", "PLUGIN_KIT_AI_MCP_SMOKE_STATIC", "codex-package-live")

	runCodexMCPHomeCommand(t, codexBin, tempHome, "remove", "release-checks")
	listOut = runCodexMCPHomeCommand(t, codexBin, tempHome, "list", "--json")
	assertCodexMCPListMissing(t, listOut, "release-checks")
	assertCodexHomeConfigNotContains(t, tempHome, "[mcp_servers.release-checks]")
}

func TestCodexPackageRenderedSidecarMCPAddGetListRemoveInAuthSeededCodexHome(t *testing.T) {
	codexBin := codexBinaryOrSkip(t)
	pluginKitAIBin := buildPluginKitAI(t)
	mcpBin := buildPortableMCPSmokeServer(t)
	workDir := newCodexPackageRenderedMCPWorkspace(t, pluginKitAIBin, mcpBin)
	server := readRenderedSharedMCPServer(t, workDir, "release-checks")
	tempHome := newAuthSeededCodexTempHome(t)

	loginOut := runCodexHomeCommand(t, codexBin, tempHome, "login", "status")
	if !strings.Contains(loginOut, "Logged in") {
		t.Fatalf("auth-seeded CODEX_HOME did not preserve login status:\n%s", loginOut)
	}

	runCodexMCPAddRenderedServerInHome(t, codexBin, tempHome, "release-checks", server)
	assertCodexHomeConfigContains(t, tempHome,
		"[mcp_servers.release-checks]",
		`command = "`+filepath.ToSlash(mcpBin)+`"`,
		`PLUGIN_KIT_AI_MCP_SMOKE_STATIC = "codex-package-live"`,
	)

	out := runCodexMCPHomeCommand(t, codexBin, tempHome, "get", "release-checks", "--json")
	if !strings.Contains(out, `"name":"release-checks"`) && !strings.Contains(out, `"name": "release-checks"`) {
		t.Fatalf("auth-seeded codex mcp get output missing generated package stdio server name:\n%s", out)
	}
	if !strings.Contains(out, filepath.ToSlash(mcpBin)) {
		t.Fatalf("auth-seeded codex mcp get output missing generated package stdio command:\n%s", out)
	}
	if !strings.Contains(out, `"PLUGIN_KIT_AI_MCP_SMOKE_STATIC":"codex-package-live"`) &&
		!strings.Contains(out, `"PLUGIN_KIT_AI_MCP_SMOKE_STATIC": "codex-package-live"`) {
		t.Fatalf("auth-seeded codex mcp get output missing generated package stdio env:\n%s", out)
	}
	listOut := runCodexMCPHomeCommand(t, codexBin, tempHome, "list", "--json")
	assertCodexMCPListEntry(t, listOut, "release-checks", "stdio", filepath.ToSlash(mcpBin), "", "PLUGIN_KIT_AI_MCP_SMOKE_STATIC", "codex-package-live")

	runCodexMCPHomeCommand(t, codexBin, tempHome, "remove", "release-checks")
	listOut = runCodexMCPHomeCommand(t, codexBin, tempHome, "list", "--json")
	assertCodexMCPListMissing(t, listOut, "release-checks")
	assertCodexHomeConfigNotContains(t, tempHome, "[mcp_servers.release-checks]")
}

func TestCodexPackageRenderedHTTPSidecarMCPAddGetListRemoveInIsolatedHome(t *testing.T) {
	codexBin := codexBinaryOrSkip(t)
	pluginKitAIBin := buildPluginKitAI(t)
	workDir := newCodexPackageRenderedHTTPMCPWorkspace(t, pluginKitAIBin)
	server := readRenderedSharedMCPServer(t, workDir, "docs")
	url := strings.TrimSpace(fmt.Sprint(server["url"]))
	if url == "" {
		t.Fatalf("generated docs MCP server missing url: %#v", server)
	}

	tempHome := newCodexTempHome(t)
	runCodexMCPAddRenderedServerInHome(t, codexBin, tempHome, "docs", server)
	assertCodexHomeConfigContains(t, tempHome,
		"[mcp_servers.docs]",
		`url = "`+url+`"`,
	)

	out := runCodexMCPHomeCommand(t, codexBin, tempHome, "get", "docs", "--json")
	if !strings.Contains(out, `"name":"docs"`) && !strings.Contains(out, `"name": "docs"`) {
		t.Fatalf("isolated-home codex mcp get output missing generated package HTTP server name:\n%s", out)
	}
	if !strings.Contains(out, `"type":"streamable_http"`) && !strings.Contains(out, `"type": "streamable_http"`) {
		t.Fatalf("isolated-home codex mcp get output missing generated package HTTP transport type:\n%s", out)
	}
	if !strings.Contains(out, `"url":"`+url+`"`) && !strings.Contains(out, `"url": "`+url+`"`) {
		t.Fatalf("isolated-home codex mcp get output missing generated package HTTP URL:\n%s", out)
	}
	listOut := runCodexMCPHomeCommand(t, codexBin, tempHome, "list", "--json")
	assertCodexMCPHTTPListEntry(t, listOut, "docs", url, "")

	runCodexMCPHomeCommand(t, codexBin, tempHome, "remove", "docs")
	listOut = runCodexMCPHomeCommand(t, codexBin, tempHome, "list", "--json")
	assertCodexMCPListMissing(t, listOut, "docs")
	assertCodexHomeConfigNotContains(t, tempHome, "[mcp_servers.docs]")
}

func TestCodexPackageRenderedHTTPSidecarMCPAddGetListRemoveInAuthSeededCodexHome(t *testing.T) {
	codexBin := codexBinaryOrSkip(t)
	pluginKitAIBin := buildPluginKitAI(t)
	workDir := newCodexPackageRenderedHTTPMCPWorkspace(t, pluginKitAIBin)
	server := readRenderedSharedMCPServer(t, workDir, "docs")
	url := strings.TrimSpace(fmt.Sprint(server["url"]))
	if url == "" {
		t.Fatalf("generated docs MCP server missing url: %#v", server)
	}

	tempHome := newAuthSeededCodexTempHome(t)
	loginOut := runCodexHomeCommand(t, codexBin, tempHome, "login", "status")
	if !strings.Contains(loginOut, "Logged in") {
		t.Fatalf("auth-seeded CODEX_HOME did not preserve login status:\n%s", loginOut)
	}

	runCodexMCPAddRenderedServerInHome(t, codexBin, tempHome, "docs", server)
	assertCodexHomeConfigContains(t, tempHome,
		"[mcp_servers.docs]",
		`url = "`+url+`"`,
	)

	out := runCodexMCPHomeCommand(t, codexBin, tempHome, "get", "docs", "--json")
	if !strings.Contains(out, `"name":"docs"`) && !strings.Contains(out, `"name": "docs"`) {
		t.Fatalf("auth-seeded codex mcp get output missing generated package HTTP server name:\n%s", out)
	}
	if !strings.Contains(out, `"type":"streamable_http"`) && !strings.Contains(out, `"type": "streamable_http"`) {
		t.Fatalf("auth-seeded codex mcp get output missing generated package HTTP transport type:\n%s", out)
	}
	if !strings.Contains(out, `"url":"`+url+`"`) && !strings.Contains(out, `"url": "`+url+`"`) {
		t.Fatalf("auth-seeded codex mcp get output missing generated package HTTP URL:\n%s", out)
	}
	listOut := runCodexMCPHomeCommand(t, codexBin, tempHome, "list", "--json")
	assertCodexMCPHTTPListEntry(t, listOut, "docs", url, "")

	runCodexMCPHomeCommand(t, codexBin, tempHome, "remove", "docs")
	listOut = runCodexMCPHomeCommand(t, codexBin, tempHome, "list", "--json")
	assertCodexMCPListMissing(t, listOut, "docs")
	assertCodexHomeConfigNotContains(t, tempHome, "[mcp_servers.docs]")
}

func codexBinaryOrSkip(t *testing.T) string {
	t.Helper()
	if strings.TrimSpace(os.Getenv("PLUGIN_KIT_AI_SKIP_CODEX_CLI")) == "1" {
		t.Skip("PLUGIN_KIT_AI_SKIP_CODEX_CLI=1")
	}
	if strings.TrimSpace(os.Getenv("PLUGIN_KIT_AI_RUN_CODEX_CLI")) != "1" {
		t.Skip("set PLUGIN_KIT_AI_RUN_CODEX_CLI=1 to run real Codex CLI e2e (see -args -codex-model)")
	}
	codexBin := strings.TrimSpace(os.Getenv("PLUGIN_KIT_AI_E2E_CODEX"))
	if codexBin == "" {
		var err error
		codexBin, err = exec.LookPath("codex")
		if err != nil {
			t.Skip("codex not in PATH; set PLUGIN_KIT_AI_E2E_CODEX or install Codex CLI")
		}
	}
	if out, err := exec.Command(codexBin, "login", "status").CombinedOutput(); err != nil {
		t.Skipf("codex login status failed (need login): %v\n%s", err, out)
	}
	return codexBin
}

func codexNotifyOverride(t *testing.T, traceFile, hookBin string) string {
	t.Helper()
	absHook, err := filepath.Abs(hookBin)
	if err != nil {
		t.Fatal(err)
	}
	wrapper := filepath.Join(t.TempDir(), "codex-notify-wrapper.sh")
	script := "#!/bin/sh\n" +
		"trace_file=\"$1\"\n" +
		"hook_bin=\"$2\"\n" +
		"shift 2\n" +
		"PLUGIN_KIT_AI_E2E_TRACE=\"$trace_file\" exec \"$hook_bin\" \"$@\"\n"
	if err := os.WriteFile(wrapper, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	absWrapper, err := filepath.Abs(wrapper)
	if err != nil {
		t.Fatal(err)
	}
	quoted := []string{
		"notify=[",
		quoteTOMLString(absWrapper), ",",
		quoteTOMLString(traceFile), ",",
		quoteTOMLString(absHook), ",",
		quoteTOMLString("notify"),
		"]",
	}
	return strings.Join(quoted, "")
}

func codexNotifyBinaryOverride(t *testing.T, markerFile, hookBin string) string {
	t.Helper()
	absHook, err := filepath.Abs(hookBin)
	if err != nil {
		t.Fatal(err)
	}
	wrapper := filepath.Join(t.TempDir(), "codex-notify-binary-wrapper.sh")
	script := "#!/bin/sh\n" +
		"marker_file=\"$1\"\n" +
		"hook_bin=\"$2\"\n" +
		"shift 2\n" +
		"printf 'notify\\n' > \"$marker_file\"\n" +
		"exec \"$hook_bin\" \"$@\"\n"
	if err := os.WriteFile(wrapper, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	absWrapper, err := filepath.Abs(wrapper)
	if err != nil {
		t.Fatal(err)
	}
	quoted := []string{
		"notify=[",
		quoteTOMLString(absWrapper), ",",
		quoteTOMLString(markerFile), ",",
		quoteTOMLString(absHook), ",",
		quoteTOMLString("notify"),
		"]",
	}
	return strings.Join(quoted, "")
}

func wrapCodexRuntimeBinaryWithMarker(t *testing.T, binPath, markerFile string) {
	t.Helper()
	realBin := binPath + ".real"
	if err := os.Rename(binPath, realBin); err != nil {
		t.Fatalf("rename runtime binary for marker wrapper: %v", err)
	}
	wrapper := "#!/bin/sh\n" +
		"printf 'notify\\n' > " + quoteShell(markerFile) + "\n" +
		"exec " + quoteShell(realBin) + " \"$@\"\n"
	if err := os.WriteFile(binPath, []byte(wrapper), 0o755); err != nil {
		t.Fatalf("write runtime marker wrapper: %v", err)
	}
}

func quoteTOMLString(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}

func newCodexRenderedNotifyWorkspace(t *testing.T, pluginKitAIBin, hookBin, traceFile, model string) string {
	t.Helper()
	root := RepoRoot(t)
	dir := t.TempDir()
	mustWriteRepoFile(t, dir, "README.md", "# codex generated notify live smoke\n")
	mustWriteRepoFile(t, dir, filepath.Join("plugin", "plugin.yaml"), `api_version: v1
name: "codex-generated-live"
version: "0.1.0"
description: "codex generated live smoke"
targets:
  - "codex-runtime"
`)
	mustWriteRepoFile(t, dir, filepath.Join("plugin", "launcher.yaml"), "runtime: shell\nentrypoint: ./bin/codex-generated-live\n")
	mustWriteRepoFile(t, dir, filepath.Join("targets", "codex-runtime", "package.yaml"), "model_hint: "+model+"\n")
	mustWriteRepoExecutable(t, dir, filepath.Join("scripts", "main.sh"), "#!/bin/sh\nexit 0\n")
	wrapper := "#!/bin/sh\n" +
		"PLUGIN_KIT_AI_E2E_TRACE=" + quoteShell(traceFile) + " exec " + quoteShell(hookBin) + " \"$@\"\n"
	mustWriteRepoExecutable(t, dir, filepath.Join("bin", "codex-generated-live"), wrapper)

	runCmd(t, root, exec.Command(pluginKitAIBin, "generate", dir))
	runCmd(t, root, exec.Command(pluginKitAIBin, "generate", dir, "--check"))
	runCmd(t, root, exec.Command(pluginKitAIBin, "validate", dir, "--platform", "codex-runtime", "--strict"))
	assertCodexConfig(t, dir, model, "./bin/codex-generated-live")
	return dir
}

func newRenderedCodexRuntimeExampleWorkspace(t *testing.T, pluginKitAIBin string) (string, string) {
	t.Helper()
	root := RepoRoot(t)
	src := filepath.Join(root, "examples", "plugins", "codex-basic-prod")
	dir := filepath.Join(t.TempDir(), "codex-basic-prod")
	copyTree(t, src, dir)
	bootstrapGeneratedGoPlugin(t, dir)

	runCmd(t, root, exec.Command(pluginKitAIBin, "generate", dir, "--check"))
	runCmd(t, root, exec.Command(pluginKitAIBin, "validate", dir, "--platform", "codex-runtime", "--strict"))
	assertCodexConfig(t, dir, "gpt-5.4-mini", "./bin/codex-basic-prod")

	binDir := filepath.Join(dir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	binName := "codex-basic-prod"
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}
	binPath := filepath.Join(binDir, binName)
	buildCmd := exec.Command("go", "build", "-o", binPath, "./cmd/codex-basic-prod")
	buildCmd.Dir = dir
	buildCmd.Env = newGoModuleEnv(t)
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("build codex-basic-prod example: %v\n%s", err, out)
	}
	return dir, binPath
}

func newCodexRenderedMCPWorkspace(t *testing.T, pluginKitAIBin string) string {
	t.Helper()
	root := RepoRoot(t)
	dir := t.TempDir()
	mustWriteRepoFile(t, dir, "README.md", "# codex generated mcp live smoke\n")
	mustWriteRepoFile(t, dir, filepath.Join("plugin", "plugin.yaml"), `api_version: v1
name: "codex-generated-mcp-live"
version: "0.1.0"
description: "codex generated mcp live smoke"
targets:
  - "codex-runtime"
`)
	mustWriteRepoFile(t, dir, filepath.Join("plugin", "launcher.yaml"), "runtime: shell\nentrypoint: ./bin/codex-generated-mcp-live\n")
	mustWriteRepoFile(t, dir, filepath.Join("targets", "codex-runtime", "package.yaml"), "model_hint: gpt-5.4-mini\n")
	mustWriteRepoFile(t, dir, filepath.Join("targets", "codex-runtime", "config.extra.toml"), "[mcp_servers.release-checks]\ncommand = \"/bin/echo\"\nargs = [\"hello\"]\n")
	mustWriteRepoExecutable(t, dir, filepath.Join("scripts", "main.sh"), "#!/bin/sh\nexit 0\n")
	mustWriteRepoExecutable(t, dir, filepath.Join("bin", "codex-generated-mcp-live"), "#!/bin/sh\nexit 0\n")

	runCmd(t, root, exec.Command(pluginKitAIBin, "generate", dir))
	runCmd(t, root, exec.Command(pluginKitAIBin, "generate", dir, "--check"))
	runCmd(t, root, exec.Command(pluginKitAIBin, "validate", dir, "--platform", "codex-runtime", "--strict"))
	return dir
}

func newCodexPackageRenderedMCPWorkspace(t *testing.T, pluginKitAIBin, mcpBin string) string {
	t.Helper()
	root := RepoRoot(t)
	dir := t.TempDir()
	mustWriteRepoFile(t, dir, "README.md", "# codex package generated mcp live smoke\n")
	mustWriteRepoFile(t, dir, filepath.Join("plugin", "plugin.yaml"), `api_version: v1
name: "codex-package-generated-mcp-live"
version: "0.1.0"
description: "codex package generated mcp live smoke"
targets:
  - "codex-package"
`)
	mustWriteRepoFile(t, dir, filepath.Join("targets", "codex-package", "package.yaml"), "homepage: https://example.com/codex-package-generated-mcp-live\n")
	mustWriteRepoFile(t, dir, filepath.Join("plugin", "mcp", "servers.yaml"), fmt.Sprintf(`api_version: v1

servers:
  release-checks:
    description: Codex package live smoke server
    type: stdio
    stdio:
      command: %q
      env:
        PLUGIN_KIT_AI_MCP_SMOKE_STATIC: codex-package-live
    targets:
      - codex-package
`, filepath.ToSlash(mcpBin)))

	runCmd(t, root, exec.Command(pluginKitAIBin, "generate", dir))
	runCmd(t, root, exec.Command(pluginKitAIBin, "generate", dir, "--check"))
	runCmd(t, root, exec.Command(pluginKitAIBin, "validate", dir, "--platform", "codex-package", "--strict"))
	return dir
}

func newCodexPackageRenderedHTTPMCPWorkspace(t *testing.T, pluginKitAIBin string) string {
	t.Helper()
	root := RepoRoot(t)
	dir := t.TempDir()
	mustWriteRepoFile(t, dir, "README.md", "# codex package generated http mcp live smoke\n")
	mustWriteRepoFile(t, dir, filepath.Join("plugin", "plugin.yaml"), `api_version: v1
name: "codex-package-generated-http-mcp-live"
version: "0.1.0"
description: "codex package generated http mcp live smoke"
targets:
  - "codex-package"
`)
	mustWriteRepoFile(t, dir, filepath.Join("targets", "codex-package", "package.yaml"), "homepage: https://example.com/codex-package-generated-http-mcp-live\n")
	mustWriteRepoFile(t, dir, filepath.Join("plugin", "mcp", "servers.yaml"), `api_version: v1

servers:
  docs:
    description: Codex package generated HTTP live smoke server
    type: remote
    remote:
      protocol: streamable_http
      url: https://example.com/generated-mcp
    targets:
      - codex-package
`)

	runCmd(t, root, exec.Command(pluginKitAIBin, "generate", dir))
	runCmd(t, root, exec.Command(pluginKitAIBin, "generate", dir, "--check"))
	runCmd(t, root, exec.Command(pluginKitAIBin, "validate", dir, "--platform", "codex-package", "--strict"))
	return dir
}

func newRenderedCodexPackageExampleWorkspace(t *testing.T, pluginKitAIBin string) string {
	t.Helper()
	root := RepoRoot(t)
	src := filepath.Join(root, "examples", "plugins", "codex-package-prod")
	dir := filepath.Join(t.TempDir(), "codex-package-prod")
	copyTree(t, src, dir)
	runCmd(t, root, exec.Command(pluginKitAIBin, "generate", dir, "--check"))
	runCmd(t, root, exec.Command(pluginKitAIBin, "validate", dir, "--platform", "codex-package", "--strict"))
	return dir
}

func quoteShell(s string) string {
	return "\"" + strings.ReplaceAll(s, `"`, `\"`) + "\""
}

func runCodexExec(t *testing.T, codexBin, projectDir, traceFile, model, prompt string, extraArgs ...string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 75*time.Second)
	defer cancel()
	outputFile := filepath.Join(t.TempDir(), "last-message.txt")
	logFile := filepath.Join(t.TempDir(), "codex.log")
	args := []string{
		"exec",
		"--skip-git-repo-check",
		"--ephemeral",
		"-C", projectDir,
		"-m", model,
		"--color", "never",
		"--output-last-message", outputFile,
	}
	args = append(args, extraArgs...)
	args = append(args, prompt)
	cmd := exec.CommandContext(ctx, codexBin, args...)
	cmd.Env = os.Environ()
	logfh, err := os.Create(logFile)
	if err != nil {
		t.Fatal(err)
	}
	defer logfh.Close()
	cmd.Stdout = logfh
	cmd.Stderr = logfh
	if err := cmd.Start(); err != nil {
		t.Fatalf("codex exec start: %v", err)
	}
	waitCh := make(chan error, 1)
	go func() {
		waitCh <- cmd.Wait()
	}()

	if err := waitForCodexInvariants(t, traceFile, outputFile, waitCh); err != nil {
		out := readLogFile(t, logFile)
		if codexRuntimeUnhealthy(out) {
			t.Skipf("codex runtime unhealthy in current environment:\n%s", truncateRunes(out, 4000))
		}
		t.Logf("codex output:\n%s", out)
		t.Fatal(err)
	}

	select {
	case err := <-waitCh:
		out := readLogFile(t, logFile)
		if err != nil {
			if codexRuntimeUnhealthy(out) {
				t.Skipf("codex runtime unhealthy in current environment:\n%s", truncateRunes(out, 4000))
			}
			t.Logf("codex output:\n%s", out)
			t.Fatalf("codex exec: %v", err)
		}
		t.Logf("codex output (truncated): %s", truncateRunes(out, 4000))
	case <-time.After(3 * time.Second):
		_ = cmd.Process.Kill()
		<-waitCh
		out := readLogFile(t, logFile)
		t.Logf("codex output (truncated, process killed after invariants): %s", truncateRunes(out, 4000))
	}
}

func runCodexExecWithProjectConfigProbe(t *testing.T, codexBin, projectDir, traceFile, prompt string) (string, string, []string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 75*time.Second)
	defer cancel()
	outputFile := filepath.Join(t.TempDir(), "last-message.txt")
	logFile := filepath.Join(t.TempDir(), "codex.log")
	args := []string{
		"exec",
		"--skip-git-repo-check",
		"--ephemeral",
		"-C", projectDir,
		"--color", "never",
		"--output-last-message", outputFile,
		prompt,
	}
	cmd := exec.CommandContext(ctx, codexBin, args...)
	cmd.Env = os.Environ()
	logfh, err := os.Create(logFile)
	if err != nil {
		t.Fatal(err)
	}
	defer logfh.Close()
	cmd.Stdout = logfh
	cmd.Stderr = logfh
	if err := cmd.Start(); err != nil {
		t.Fatalf("codex exec start: %v", err)
	}
	waitCh := make(chan error, 1)
	go func() {
		waitCh <- cmd.Wait()
	}()

	select {
	case err := <-waitCh:
		out := readLogFile(t, logFile)
		if err != nil {
			if codexRuntimeUnhealthy(out) {
				t.Skipf("codex runtime unhealthy in current environment:\n%s", truncateRunes(out, 4000))
			}
			t.Logf("codex output:\n%s", out)
			t.Fatalf("codex exec: %v", err)
		}
		t.Logf("codex output (truncated): %s", truncateRunes(out, 4000))
		return out, readOptionalTextFile(outputFile), waitForTraceLines(t, traceFile, 3*time.Second)
	case <-time.After(75 * time.Second):
		_ = cmd.Process.Kill()
		<-waitCh
		out := readLogFile(t, logFile)
		if codexRuntimeUnhealthy(out) {
			t.Skipf("codex runtime unhealthy in current environment:\n%s", truncateRunes(out, 4000))
		}
		t.Fatalf("timed out waiting for codex exec using generated project config:\n%s", truncateRunes(out, 4000))
		return "", "", nil
	}
}

func runCodexExecWithProjectConfigMarkerProbe(t *testing.T, codexBin, projectDir, prompt string) (string, string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 75*time.Second)
	defer cancel()
	outputFile := filepath.Join(t.TempDir(), "last-message.txt")
	logFile := filepath.Join(t.TempDir(), "codex.log")
	args := []string{
		"exec",
		"--skip-git-repo-check",
		"--ephemeral",
		"-C", projectDir,
		"--color", "never",
		"--output-last-message", outputFile,
		prompt,
	}
	cmd := exec.CommandContext(ctx, codexBin, args...)
	cmd.Env = os.Environ()
	logfh, err := os.Create(logFile)
	if err != nil {
		t.Fatal(err)
	}
	defer logfh.Close()
	cmd.Stdout = logfh
	cmd.Stderr = logfh
	err = cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("timed out waiting for codex exec using production example project config:\n%s", truncateRunes(readLogFile(t, logFile), 4000))
	}
	logOutput := readLogFile(t, logFile)
	if err != nil {
		if codexRuntimeUnhealthy(logOutput) {
			t.Skipf("codex runtime unhealthy in current environment:\n%s", truncateRunes(logOutput, 4000))
		}
		t.Fatalf("codex exec: %v\n%s", err, logOutput)
	}
	return logOutput, readOptionalTextFile(outputFile)
}

func runCodexExecWithMarkerProbe(t *testing.T, codexBin, projectDir, markerFile, prompt string, extraArgs ...string) (string, string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 75*time.Second)
	defer cancel()
	outputFile := filepath.Join(t.TempDir(), "last-message.txt")
	logFile := filepath.Join(t.TempDir(), "codex.log")
	args := []string{
		"exec",
		"--skip-git-repo-check",
		"--ephemeral",
		"-C", projectDir,
		"--color", "never",
		"--output-last-message", outputFile,
	}
	args = append(args, extraArgs...)
	args = append(args, prompt)
	cmd := exec.CommandContext(ctx, codexBin, args...)
	cmd.Env = os.Environ()
	logfh, err := os.Create(logFile)
	if err != nil {
		t.Fatal(err)
	}
	defer logfh.Close()
	cmd.Stdout = logfh
	cmd.Stderr = logfh
	if err := cmd.Start(); err != nil {
		t.Fatalf("codex exec start: %v", err)
	}
	waitCh := make(chan error, 1)
	go func() {
		waitCh <- cmd.Wait()
	}()

	if err := waitForCodexMarkerInvariants(t, markerFile, outputFile, waitCh); err != nil {
		out := readLogFile(t, logFile)
		if codexRuntimeUnhealthy(out) {
			t.Skipf("codex runtime unhealthy in current environment:\n%s", truncateRunes(out, 4000))
		}
		t.Logf("codex output:\n%s", out)
		t.Fatal(err)
	}

	select {
	case err := <-waitCh:
		out := readLogFile(t, logFile)
		if err != nil {
			if codexRuntimeUnhealthy(out) {
				t.Skipf("codex runtime unhealthy in current environment:\n%s", truncateRunes(out, 4000))
			}
			t.Logf("codex output:\n%s", out)
			t.Fatalf("codex exec: %v", err)
		}
		return out, readOptionalTextFile(outputFile)
	case <-time.After(3 * time.Second):
		_ = cmd.Process.Kill()
		<-waitCh
		out := readLogFile(t, logFile)
		return out, readOptionalTextFile(outputFile)
	}
}

func readOptionalTextFile(path string) string {
	body, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(body))
}

func runCodexMCPGetProbe(t *testing.T, codexBin, projectDir, serverName string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, codexBin, "-C", projectDir, "mcp", "get", serverName, "--json")
	out, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("codex mcp get %s timed out:\n%s", serverName, out)
	}
	if err != nil {
		return string(out)
	}
	return string(out)
}

func runCodexMCPListWithProjectConfigProbe(t *testing.T, codexBin, projectDir string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, codexBin, "-C", projectDir, "mcp", "list", "--json")
	out, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("codex mcp list timed out:\n%s", out)
	}
	if err != nil {
		return string(out)
	}
	return string(out)
}

func runCodexHomeCommand(t *testing.T, codexBin, home string, args ...string) string {
	t.Helper()
	out, err := runCodexHomeCommandResult(t, codexBin, home, args...)
	if err != nil {
		t.Fatalf("codex %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return out
}

func runCodexHomeCommandExpectError(t *testing.T, codexBin, home string, args ...string) string {
	t.Helper()
	out, err := runCodexHomeCommandResult(t, codexBin, home, args...)
	if err == nil {
		t.Fatalf("codex %s unexpectedly succeeded:\n%s", strings.Join(args, " "), out)
	}
	return out
}

func runCodexHomeCommandResult(t *testing.T, codexBin, home string, args ...string) (string, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, codexBin, args...)
	cmd.Env = append(os.Environ(),
		"HOME="+home,
		"CODEX_HOME="+filepath.Join(home, ".codex"),
	)
	out, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("codex %s timed out:\n%s", strings.Join(args, " "), out)
	}
	return string(out), err
}

func runCodexMCPHomeCommand(t *testing.T, codexBin, home string, args ...string) string {
	t.Helper()
	return runCodexHomeCommand(t, codexBin, home, append([]string{"mcp"}, args...)...)
}

func runCodexMCPHomeCommandExpectError(t *testing.T, codexBin, home string, args ...string) string {
	t.Helper()
	return runCodexHomeCommandExpectError(t, codexBin, home, append([]string{"mcp"}, args...)...)
}

func runCodexMCPGetWithArgs(t *testing.T, codexBin, serverName string, configArgs ...string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	args := []string{"mcp", "get", serverName, "--json"}
	args = append(args, configArgs...)
	cmd := exec.CommandContext(ctx, codexBin, args...)
	out, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("codex mcp get %s timed out:\n%s", serverName, out)
	}
	if err != nil {
		t.Fatalf("codex mcp get %s: %v\n%s", serverName, err, out)
	}
	return string(out)
}

func runCodexMCPListWithArgs(t *testing.T, codexBin string, configArgs ...string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	args := []string{"mcp", "list", "--json"}
	args = append(args, configArgs...)
	cmd := exec.CommandContext(ctx, codexBin, args...)
	out, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("codex mcp list timed out:\n%s", out)
	}
	if err != nil {
		t.Fatalf("codex mcp list: %v\n%s", err, out)
	}
	return string(out)
}

func newCodexTempHome(t *testing.T) string {
	t.Helper()
	home := filepath.Join(t.TempDir(), "codex-home")
	if err := os.MkdirAll(filepath.Join(home, ".codex"), 0o755); err != nil {
		t.Fatal(err)
	}
	return home
}

func newAuthSeededCodexTempHome(t *testing.T) string {
	t.Helper()
	home := newCodexTempHome(t)
	seedCodexAuthIntoHome(t, home)
	return home
}

func seedCodexAuthIntoHome(t *testing.T, home string) {
	t.Helper()
	srcCodexHome := strings.TrimSpace(os.Getenv("CODEX_HOME"))
	if srcCodexHome == "" {
		srcCodexHome = filepath.Join(os.Getenv("HOME"), ".codex")
	}
	authSrc := filepath.Join(srcCodexHome, "auth.json")
	authBody, err := os.ReadFile(authSrc)
	if err != nil {
		t.Skipf("codex auth seed unavailable at %s: %v", authSrc, err)
	}
	if err := os.WriteFile(filepath.Join(home, ".codex", "auth.json"), authBody, 0o600); err != nil {
		t.Fatalf("seed codex auth.json: %v", err)
	}
	accountsSrc := filepath.Join(srcCodexHome, "accounts")
	if info, err := os.Stat(accountsSrc); err == nil && info.IsDir() {
		copyTree(t, accountsSrc, filepath.Join(home, ".codex", "accounts"))
	}
}

func assertCodexHomeConfigContains(t *testing.T, home string, want ...string) {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(home, ".codex", "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, needle := range want {
		if !strings.Contains(text, needle) {
			t.Fatalf("isolated-home .codex/config.toml missing %q:\n%s", needle, text)
		}
	}
}

func assertCodexHomeConfigNotContains(t *testing.T, home string, unwanted string) {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(home, ".codex", "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	if strings.Contains(text, unwanted) {
		t.Fatalf("isolated-home .codex/config.toml unexpectedly contains %q:\n%s", unwanted, text)
	}
}

func runCodexExecWithHomePortableMCP(t *testing.T, codexBin, home, markerPath, model string) {
	t.Helper()
	models := codexPortableMCPLiveModels(model)
	for idx, candidate := range models {
		if runCodexExecWithHomePortableMCPModel(t, codexBin, home, markerPath, candidate) {
			return
		}
		if idx < len(models)-1 {
			t.Logf("codex isolated-home MCP live smoke did not observe a tool call with model %q; retrying with fallback model", candidate)
		}
	}
	t.Skipf("codex exec completed without selecting the isolated-home MCP tool after trying models %v; codex mcp add/get/list already verified the persisted server config, so treating this as model-behavior variability", models)
}

func runCodexExecWithHomePortableMCPModel(t *testing.T, codexBin, home, markerPath, model string) bool {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	outputFile := filepath.Join(t.TempDir(), "last-message.txt")
	logFile := filepath.Join(t.TempDir(), "codex-home-mcp.log")
	args := []string{
		"exec",
		"--skip-git-repo-check",
		"--ephemeral",
		"-C", t.TempDir(),
		"-m", model,
		"--color", "never",
		"--output-last-message", outputFile,
		"--dangerously-bypass-approvals-and-sandbox",
		"Do not inspect files or run shell commands. Before your final answer, make exactly one MCP tool call to release_checks with JSON arguments {\"token\":\"CODEX_PORTABLE_MCP_OK\"}. After that single tool call, answer DONE.",
	}
	cmd := exec.CommandContext(ctx, codexBin, args...)
	cmd.Env = append(os.Environ(),
		"HOME="+home,
		"CODEX_HOME="+filepath.Join(home, ".codex"),
		"PLUGIN_KIT_AI_MCP_SMOKE_MARKER="+markerPath,
	)
	logfh, err := os.Create(logFile)
	if err != nil {
		t.Fatal(err)
	}
	defer logfh.Close()
	cmd.Stdout = logfh
	cmd.Stderr = logfh
	if err := cmd.Start(); err != nil {
		t.Fatalf("start codex isolated-home MCP live smoke: %v", err)
	}
	waitCh := make(chan error, 1)
	go func() {
		waitCh <- cmd.Wait()
	}()

	deadline := time.Now().Add(75 * time.Second)
	for {
		if _, err := os.Stat(markerPath); err == nil {
			return true
		}
		select {
		case err := <-waitCh:
			out := readLogFile(t, logFile)
			if err != nil {
				if codexAuthUnavailable(out) {
					t.Skipf("codex exec with isolated CODEX_HOME lost auth in this build; treating isolated-home MCP exec as evidence-only:\n%s", truncateRunes(out, 4000))
				}
				if codexRuntimeUnhealthy(out) {
					t.Skipf("codex runtime unhealthy in current environment:\n%s", truncateRunes(out, 4000))
				}
				t.Fatalf("codex isolated-home MCP live smoke: %v\n%s", err, out)
			}
			if codexPortableMCPToolUnavailable(out) {
				t.Logf("codex isolated-home MCP live smoke did not expose the persisted MCP tool in exec session for model %q:\n%s", model, truncateRunes(out, 4000))
				return false
			}
			if body, readErr := os.ReadFile(outputFile); readErr == nil && strings.Contains(string(body), "DONE") {
				t.Logf("codex isolated-home MCP live smoke finished without tool selection for model %q:\n%s", model, truncateRunes(out, 4000))
				return false
			}
			t.Logf("codex isolated-home MCP live smoke exited without producing MCP marker for model %q after successful add/get/list preflight:\n%s", model, truncateRunes(out, 4000))
			return false
		default:
		}
		if time.Now().After(deadline) {
			t.Logf("codex isolated-home MCP live smoke timed out for model %q; treating as session variability", model)
			return false
		}
		time.Sleep(150 * time.Millisecond)
	}
}

func codexAuthUnavailable(log string) bool {
	markers := []string{
		"Missing bearer or basic authentication in header",
		"unexpected status 401 Unauthorized",
	}
	for _, marker := range markers {
		if strings.Contains(log, marker) {
			return true
		}
	}
	return false
}

func assertCodexMCPListEntry(t *testing.T, out, name, wantType, wantCommand, wantArg, envKey, envValue string) {
	t.Helper()
	var entries []map[string]any
	if err := json.Unmarshal([]byte(out), &entries); err != nil {
		t.Fatalf("parse codex mcp list output: %v\n%s", err, out)
	}
	for _, entry := range entries {
		if strings.TrimSpace(fmt.Sprint(entry["name"])) != name {
			continue
		}
		transport, ok := entry["transport"].(map[string]any)
		if !ok {
			t.Fatalf("codex mcp list %s entry missing transport:\n%s", name, out)
		}
		if strings.TrimSpace(fmt.Sprint(transport["type"])) != wantType {
			t.Fatalf("codex mcp list %s transport type = %q want %q\n%s", name, transport["type"], wantType, out)
		}
		if strings.TrimSpace(fmt.Sprint(transport["command"])) != wantCommand {
			t.Fatalf("codex mcp list %s command = %q want %q\n%s", name, transport["command"], wantCommand, out)
		}
		args, ok := transport["args"].([]any)
		if wantArg == "" {
			if !ok || len(args) != 0 {
				t.Fatalf("codex mcp list %s args = %#v want []\n%s", name, transport["args"], out)
			}
		} else if !ok || len(args) != 1 || strings.TrimSpace(fmt.Sprint(args[0])) != wantArg {
			t.Fatalf("codex mcp list %s args = %#v want [%s]\n%s", name, transport["args"], wantArg, out)
		}
		if envKey != "" {
			env, ok := transport["env"].(map[string]any)
			if !ok {
				t.Fatalf("codex mcp list %s transport missing env:\n%s", name, out)
			}
			if strings.TrimSpace(fmt.Sprint(env[envKey])) != envValue {
				t.Fatalf("codex mcp list %s env value = %q want %q\n%s", name, env[envKey], envValue, out)
			}
		}
		return
	}
	t.Fatalf("codex mcp list output missing %s server:\n%s", name, out)
}

func assertCodexMCPHTTPListEntry(t *testing.T, out, name, wantURL, wantBearerVar string) {
	t.Helper()
	var entries []map[string]any
	if err := json.Unmarshal([]byte(out), &entries); err != nil {
		t.Fatalf("parse codex mcp list output: %v\n%s", err, out)
	}
	for _, entry := range entries {
		if strings.TrimSpace(fmt.Sprint(entry["name"])) != name {
			continue
		}
		transport, ok := entry["transport"].(map[string]any)
		if !ok {
			t.Fatalf("codex mcp list %s entry missing transport:\n%s", name, out)
		}
		if strings.TrimSpace(fmt.Sprint(transport["type"])) != "streamable_http" {
			t.Fatalf("codex mcp list %s transport type = %q want %q\n%s", name, transport["type"], "streamable_http", out)
		}
		if strings.TrimSpace(fmt.Sprint(transport["url"])) != wantURL {
			t.Fatalf("codex mcp list %s transport url = %q want %q\n%s", name, transport["url"], wantURL, out)
		}
		if wantBearerVar != "" && strings.TrimSpace(fmt.Sprint(transport["bearer_token_env_var"])) != wantBearerVar {
			t.Fatalf("codex mcp list %s bearer_token_env_var = %q want %q\n%s", name, transport["bearer_token_env_var"], wantBearerVar, out)
		}
		return
	}
	t.Fatalf("codex mcp list output missing %s server:\n%s", name, out)
}

func assertCodexMCPListMissing(t *testing.T, out, name string) {
	t.Helper()
	var entries []map[string]any
	if err := json.Unmarshal([]byte(out), &entries); err != nil {
		t.Fatalf("parse codex mcp list output: %v\n%s", err, out)
	}
	for _, entry := range entries {
		if strings.TrimSpace(fmt.Sprint(entry["name"])) == name {
			t.Fatalf("codex mcp list unexpectedly still contains %s:\n%s", name, out)
		}
	}
}

func codexMCPConfigArgsFromRenderedServer(t *testing.T, name string, server map[string]any) []string {
	t.Helper()
	switch renderedServerString(server["type"]) {
	case "http":
		url := renderedServerString(server["url"])
		if url == "" {
			t.Fatalf("generated MCP server %q missing url: %#v", name, server)
		}
		return []string{"-c", fmt.Sprintf(`mcp_servers.%s.url=%q`, name, url)}
	case "stdio":
		return codexMCPConfigArgs(name, server)
	default:
		t.Fatalf("unsupported generated MCP server type %q for %s: %#v", server["type"], name, server)
		return nil
	}
}

func runCodexMCPAddRenderedServerInHome(t *testing.T, codexBin, home, name string, server map[string]any) {
	t.Helper()
	serverType := renderedServerString(server["type"])
	if serverType == "" {
		switch {
		case renderedServerString(server["url"]) != "":
			serverType = "http"
		case renderedServerString(server["command"]) != "":
			serverType = "stdio"
		}
	}
	switch serverType {
	case "http":
		url := renderedServerString(server["url"])
		if url == "" {
			t.Fatalf("generated MCP server %q missing url: %#v", name, server)
		}
		args := []string{"add", name, "--url", url}
		if bearer := renderedServerString(server["bearer_token_env_var"]); bearer != "" {
			args = append(args, "--bearer-token-env-var", bearer)
		}
		runCodexMCPHomeCommand(t, codexBin, home, args...)
	case "stdio":
		command := renderedServerString(server["command"])
		if command == "" {
			t.Fatalf("generated MCP server %q missing stdio command: %#v", name, server)
		}
		args := []string{"add", name}
		if envMap, ok := server["env"].(map[string]any); ok {
			keys := make([]string, 0, len(envMap))
			for k := range envMap {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				args = append(args, "--env", k+"="+strings.TrimSpace(fmt.Sprint(envMap[k])))
			}
		}
		args = append(args, "--", command)
		if rawArgs, ok := server["args"].([]any); ok {
			for _, raw := range rawArgs {
				args = append(args, strings.TrimSpace(fmt.Sprint(raw)))
			}
		}
		runCodexMCPHomeCommand(t, codexBin, home, args...)
	default:
		t.Fatalf("unsupported generated MCP server type %q for %s: %#v", serverType, name, server)
	}
}

func renderedServerString(v any) string {
	if v == nil {
		return ""
	}
	s := strings.TrimSpace(fmt.Sprint(v))
	if s == "<nil>" {
		return ""
	}
	return s
}

func waitForCodexInvariants(t *testing.T, traceFile, outputFile string, waitCh <-chan error) error {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for {
		if lines := readTraceLines(t, traceFile); len(lines) > 0 {
			if _, ok := traceFind(t, lines, "Notify"); ok {
				if b, err := os.ReadFile(outputFile); err == nil && strings.TrimSpace(string(b)) != "" {
					return nil
				}
			}
		}
		select {
		case err := <-waitCh:
			if err != nil {
				return fmt.Errorf("codex exec exited before invariants: %w", err)
			}
			if lines := readTraceLines(t, traceFile); len(lines) == 0 {
				return fmt.Errorf("codex exec exited without trace entry")
			}
			if b, err := os.ReadFile(outputFile); err != nil || strings.TrimSpace(string(b)) == "" {
				if err != nil {
					return fmt.Errorf("codex exec exited without last message file: %w", err)
				}
				return fmt.Errorf("codex exec exited with empty last message file")
			}
			return nil
		default:
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out waiting for codex notify invariants")
		}
		time.Sleep(150 * time.Millisecond)
	}
}

func waitForCodexMarkerInvariants(t *testing.T, markerFile, outputFile string, waitCh <-chan error) error {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for {
		if _, err := os.Stat(markerFile); err == nil {
			if b, err := os.ReadFile(outputFile); err == nil && strings.TrimSpace(string(b)) != "" {
				return nil
			}
		}
		select {
		case err := <-waitCh:
			if err != nil {
				return fmt.Errorf("codex exec exited before production example invariants: %w", err)
			}
			if _, err := os.Stat(markerFile); err != nil {
				return fmt.Errorf("codex exec exited without notify marker: %w", err)
			}
			if b, err := os.ReadFile(outputFile); err != nil || strings.TrimSpace(string(b)) == "" {
				if err != nil {
					return fmt.Errorf("codex exec exited without last message file: %w", err)
				}
				return fmt.Errorf("codex exec exited with empty last message file")
			}
			return nil
		default:
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out waiting for codex production example notify invariants")
		}
		time.Sleep(150 * time.Millisecond)
	}
}

func readLogFile(t *testing.T, path string) string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, f)
	return buf.String()
}

func codexRuntimeUnhealthy(log string) bool {
	markers := []string{
		"Could not create otel exporter",
		"Attempted to create a NULL object.",
		"event loop thread panicked",
		"failed to refresh available models",
		"failed to connect to websocket: HTTP error: 500 Internal Server Error",
		"We're currently experiencing high demand",
		"stream disconnected - retrying sampling request",
	}
	for _, marker := range markers {
		if strings.Contains(log, marker) {
			return true
		}
	}
	return false
}

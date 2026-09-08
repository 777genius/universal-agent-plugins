package providers

import (
	"context"
	"encoding/json"
	"os"

	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/managedstdio"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers/nativeconfig"
)

func TestWindsurfStagerProjectsResolvedMCPAndExactOwnership(t *testing.T) {
	t.Parallel()
	envelope := windsurfTestEnvelope(t, "one")
	plan := stagingPlan(t, domain.ClientWindsurf, domain.PackagePrepared)
	plan.NativeRegistryRoot = filepath.Join(t.TempDir(), ".codeium", "windsurf")
	plan.Components = []domain.ComponentDecision{
		{Kind: domain.ComponentMCPServer, Name: "local", Support: domain.SupportPrepared},
		{Kind: domain.ComponentMCPServer, Name: "docs", Support: domain.SupportPrepared},
	}
	dataRoot := filepath.Join(t.TempDir(), "plugin-data")
	delivery, err := windsurfFixtureStager(t).StageWithPluginData(context.Background(), envelope, plan, "windsurf-stage", domain.CompatibilityHints{}, dataRoot)
	if err != nil {
		t.Fatal(err)
	}
	document := readObject(t, filepath.Join(delivery.StagingPath, "mcp.json"))
	servers := document["mcpServers"].(map[string]any)
	local := servers["local"].(map[string]any)
	args := local["args"].([]any)
	if args[7] != filepath.Join(plan.ActivePath, "runtime", "server.js") || args[8] != filepath.Join(dataRoot, "cache") {
		t.Fatalf("Windsurf placeholders were not bound: %+v", args)
	}
	env := local["env"].(map[string]any)
	if len(env) != 2 || env["PLUGIN_ROOT"] != plan.ActivePath || env["PLUGIN_DATA"] != dataRoot {
		t.Fatalf("Windsurf stdio environment = %+v", env)
	}
	if _, exists := local["cwd"]; exists {
		t.Fatalf("Windsurf projection invented unsupported cwd: %+v", local)
	}
	if got := servers["docs"].(map[string]any)["url"]; got != "https://docs.test/one" {
		t.Fatalf("remote URL = %v", got)
	}
	objects := windsurfObjects(delivery.NativeObjects)
	if len(objects) != 2 {
		t.Fatalf("Windsurf native ownership = %+v", delivery.NativeObjects)
	}
	for _, object := range objects {
		if object.Path != filepath.Join(plan.NativeRegistryRoot, "mcp_config.json") || !strings.HasPrefix(object.ManagedDigest, "sha256:") {
			t.Fatalf("invalid Windsurf object = %+v", object)
		}
	}
}

func TestWindsurfNativeLifecyclePreservesForeignEntries(t *testing.T) {
	t.Parallel()
	configRoot := filepath.Join(t.TempDir(), ".codeium", "windsurf")
	configPath := filepath.Join(configRoot, "mcp_config.json")
	writeTestFile(t, configPath, `{"theme":"night","mcpServers":{"foreign":{"url":"https://foreign.test"}}}`)

	first := stagedWindsurfDelivery(t, configRoot, "one", "first")
	request := windsurfActivationRequest(first, configRoot, nil)
	outcome, err := (Activator{}).Activate(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Activation != domain.ActivationActive || outcome.Verification != domain.VerificationInstalled {
		t.Fatalf("add outcome = %+v", outcome)
	}
	assertWindsurfConfig(t, configPath, "one", true)

	verified := request
	verified.VerifyOnly = true
	if outcome, err := (Activator{}).Activate(context.Background(), verified); err != nil || outcome.Verification != domain.VerificationInstalled {
		t.Fatalf("verify outcome = %+v, %v", outcome, err)
	}

	second := stagedWindsurfDelivery(t, configRoot, "two", "second")
	update := windsurfActivationRequest(second, configRoot, first.NativeObjects)
	update.Replacing = true
	if _, err := (Activator{}).Activate(context.Background(), update); err != nil {
		t.Fatal(err)
	}
	assertWindsurfConfig(t, configPath, "two", true)

	if err := removeWindsurfEntryForTest(configPath, "docs"); err != nil {
		t.Fatal(err)
	}
	repair := update
	repair.PreviousNativeObjects = second.NativeObjects
	if _, err := (Activator{}).Activate(context.Background(), repair); err != nil {
		t.Fatalf("repair exact absent entry: %v", err)
	}
	assertWindsurfConfig(t, configPath, "two", true)

	remove := domain.DeactivationRequest{
		Client:       domain.DetectedClient{ClientID: domain.ClientWindsurf, Status: domain.DetectionDetected, ConfigRoot: configRoot},
		DeclaredName: "demo", CurrentActivation: domain.ActivationActive, Confirmed: true,
		NativeObjects: second.NativeObjects,
	}
	removed, err := (Activator{}).Deactivate(context.Background(), remove)
	if err != nil {
		t.Fatal(err)
	}
	if !removed.ArtifactRemovalAllowed || !removed.ExternalRemovalComplete {
		t.Fatalf("remove outcome = %+v", removed)
	}
	assertWindsurfConfig(t, configPath, "", true)
	if _, err := (Activator{}).Deactivate(context.Background(), remove); err != nil {
		t.Fatalf("idempotent removal after exact absence: %v", err)
	}
}

func TestWindsurfCollisionAndTamperFailClosed(t *testing.T) {
	t.Parallel()
	configRoot := filepath.Join(t.TempDir(), ".codeium", "windsurf-next")
	configPath := filepath.Join(configRoot, "mcp_config.json")
	writeTestFile(t, configPath, `{"mcpServers":{"docs":{"url":"https://unmanaged.test"}}}`)
	delivery := stagedWindsurfDelivery(t, configRoot, "owned", "collision")
	request := windsurfActivationRequest(delivery, configRoot, nil)
	before, _ := os.ReadFile(configPath)
	if _, err := (Activator{}).Activate(context.Background(), request); err == nil {
		t.Fatal("unmanaged collision was overwritten")
	}
	after, _ := os.ReadFile(configPath)
	if string(after) != string(before) {
		t.Fatalf("collision changed config: %s", after)
	}

	if err := os.WriteFile(configPath, []byte(`{"mcpServers":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := (Activator{}).Activate(context.Background(), request); err != nil {
		t.Fatalf("add after exact absence: %v", err)
	}
	writeTestFile(t, configPath, `{"mcpServers":{"docs":{"url":"https://tampered.test"}}}`)
	request.PreviousNativeObjects = delivery.NativeObjects
	request.Replacing = true
	if _, err := (Activator{}).Activate(context.Background(), request); err == nil {
		t.Fatal("tampered managed entry was overwritten")
	}
}

func TestWindsurfSkillsOnlyAndDevinOnlyRemainManual(t *testing.T) {
	t.Parallel()
	request := activationRequest(t, domain.ClientWindsurf)
	request.Client.ConfigRoot = ""
	request.Client.ExecutablePath = "/test/bin/devin"
	request.Plan.Components = []domain.ComponentDecision{{Kind: domain.ComponentSkill, Name: "guide", Support: domain.SupportPrepared}}
	outcome, err := (Activator{}).Activate(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	userActions := strings.Join(outcome.UserActions, " ")
	localActions := strings.Join(outcome.LocalActions, " ")
	if outcome.Activation != domain.ActivationManual ||
		!strings.Contains(userActions, "does not claim automatic skill activation") ||
		strings.Contains(userActions, "MCP") ||
		!strings.Contains(localActions, filepath.Join(request.Delivery.ActivePath, "skills")) ||
		!strings.Contains(localActions, "never changed automatically") {
		t.Fatalf("Devin-only activation = %+v", outcome)
	}
}

func TestWindsurfSSEUsesLegacyURLKey(t *testing.T) {
	t.Parallel()
	configPath := filepath.Join(t.TempDir(), ".codeium", "windsurf-insiders", "mcp_config.json")
	server, err := windsurfServer(domain.MCPServer{
		Type:    "sse",
		Decoded: map[string]any{"type": "sse", "url": "https://events.test/sse"},
	}, "/test/package", "/test/data")
	if err != nil {
		t.Fatal(err)
	}
	if server.RemoteTransport != "sse" {
		t.Fatalf("remote transport = %q", server.RemoteTransport)
	}
	if _, err := nativeconfig.New().Apply(nativeconfig.Request{
		Paths: nativeconfig.Paths{JSON: configPath}, Codec: nativeconfig.CodecWindsurf,
		Action: nativeconfig.ActionAdd, Name: "events", Server: server,
	}); err != nil {
		t.Fatal(err)
	}
	entry := readObject(t, configPath)["mcpServers"].(map[string]any)["events"].(map[string]any)
	if entry["url"] != "https://events.test/sse" || entry["serverUrl"] != nil {
		t.Fatalf("SSE native shape = %+v", entry)
	}
}

func TestWindsurfExpandsOnlyPortableStdioValues(t *testing.T) {
	t.Parallel()
	packageRoot := filepath.Join(t.TempDir(), "plugin", "${PLUGIN_DATA}")
	dataRoot := filepath.Join(t.TempDir(), "data")
	stdio, err := windsurfServer(domain.MCPServer{
		Type: "stdio",
		Decoded: map[string]any{
			"type":    "stdio",
			"command": "${PLUGIN_ROOT}",
			"args":    []any{"${PLUGIN_ROOT}/run.js", "${PLUGIN_CACHE}/literal"},
			"env":     map[string]any{"DATA": "${PLUGIN_DATA}/state", "UNKNOWN": "${HOME}"},
		},
	}, packageRoot, dataRoot)
	if err != nil {
		t.Fatal(err)
	}
	if stdio.Args[6] != "${PLUGIN_ROOT}" || stdio.Args[7] != filepath.Join(packageRoot, "run.js") || stdio.Args[8] != "${PLUGIN_CACHE}/literal" || stdio.Env["DATA"] != filepath.Join(dataRoot, "state") || stdio.Env["UNKNOWN"] != "${HOME}" {
		t.Fatalf("Windsurf stdio placeholder projection = %+v", stdio)
	}
	if strings.Contains(stdio.Args[7], dataRoot) {
		t.Fatalf("Windsurf recursively expanded replacement text: %+v", stdio.Args)
	}

	remote, err := windsurfServer(domain.MCPServer{
		Type: "streamable-http",
		Decoded: map[string]any{
			"type":    "streamable-http",
			"url":     "https://example.test/${PLUGIN_ROOT}",
			"headers": map[string]any{"X-Path": "${PLUGIN_DATA}"},
		},
	}, packageRoot, dataRoot)
	if err != nil {
		t.Fatal(err)
	}
	if remote.URL != "https://example.test/${PLUGIN_ROOT}" || remote.Headers["X-Path"] != "${PLUGIN_DATA}" {
		t.Fatalf("Windsurf remote literals were expanded: %+v", remote)
	}
}

func TestWindsurfStagerRejectsMissingCWDWithoutChangingProjection(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	projectionPath := filepath.Join(root, "mcp.json")
	writeTestFile(t, projectionPath, "sentinel\n")
	envelope := windsurfTestEnvelope(t, "cwd")
	local := envelope.MCP.Servers["local"]
	local.Decoded["cwd"] = "${PLUGIN_ROOT}/workspace"
	envelope.MCP.Servers["local"] = local
	plan := stagingPlan(t, domain.ClientWindsurf, domain.PackagePrepared)
	plan.Components = []domain.ComponentDecision{{Kind: domain.ComponentMCPServer, Name: "local", Support: domain.SupportPrepared}}

	err := projectWindsurfMCP(root, envelope, plan, filepath.Join(t.TempDir(), "data"))
	if err == nil || !strings.Contains(err.Error(), "cwd unavailable") {
		t.Fatalf("Windsurf cwd projection error = %v", err)
	}
	body, readErr := os.ReadFile(projectionPath)
	if readErr != nil || string(body) != "sentinel\n" {
		t.Fatalf("rejected Windsurf cwd changed projection = %q, %v", body, readErr)
	}
}

func TestWindsurfStandardProjectionNeverDropsCWD(t *testing.T) {
	t.Parallel()
	_, err := standardWindsurfServer(nativeconfig.Server{Type: "stdio", Command: "node", CWD: "/workspace"}, "stdio")
	if err == nil || !strings.Contains(err.Error(), "does not support cwd") {
		t.Fatalf("Windsurf standard projection accepted cwd: %v", err)
	}
}

func TestWindsurfRejectsReservedStdioEnvWithoutChangingProjection(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	projectionPath := filepath.Join(root, "mcp.json")
	writeTestFile(t, projectionPath, "sentinel\n")
	envelope := windsurfTestEnvelope(t, "collision")
	local := envelope.MCP.Servers["local"]
	authorEnv := map[string]any{"KEEP": "value", "PLUGIN_DATA": "/attacker"}
	local.Decoded["env"] = authorEnv
	envelope.MCP.Servers["local"] = local
	plan := stagingPlan(t, domain.ClientWindsurf, domain.PackagePrepared)
	plan.Components = []domain.ComponentDecision{{Kind: domain.ComponentMCPServer, Name: "local", Support: domain.SupportPrepared}}

	err := projectWindsurfMCP(root, envelope, plan, filepath.Join(t.TempDir(), "data"))
	if err == nil || !strings.Contains(err.Error(), "PLUGIN_DATA is reserved") {
		t.Fatalf("reserved Windsurf env error = %v", err)
	}
	body, readErr := os.ReadFile(projectionPath)
	if readErr != nil || string(body) != "sentinel\n" {
		t.Fatalf("rejected Windsurf env changed projection = %q, %v", body, readErr)
	}
	if len(authorEnv) != 2 || authorEnv["KEEP"] != "value" || authorEnv["PLUGIN_DATA"] != "/attacker" {
		t.Fatalf("rejected Windsurf projection mutated package env: %#v", authorEnv)
	}
}

func TestWindsurfBundledCommandUsesManagedPluginRoot(t *testing.T) {
	t.Parallel()
	activeRoot := t.TempDir()
	commandPath := filepath.Join(activeRoot, "bin", "server")
	writeTestFile(t, commandPath, "#!/bin/sh\nprintf 'managed-root\\n'\n")
	if err := os.Chmod(commandPath, 0o700); err != nil {
		t.Fatal(err)
	}

	envelope := windsurfTestEnvelope(t, "bundled")
	local := envelope.MCP.Servers["local"]
	local.Decoded = cloneObject(local.Decoded)
	local.Decoded["command"] = "./bin/server"
	local.Decoded["args"] = []any{}
	envelope.MCP.Servers["local"] = local
	plan := stagingPlan(t, domain.ClientWindsurf, domain.PackagePrepared)
	plan.ActivePath = activeRoot
	plan.NativeRegistryRoot = filepath.Join(t.TempDir(), ".codeium", "windsurf")
	plan.Components = []domain.ComponentDecision{{Kind: domain.ComponentMCPServer, Name: "local", Support: domain.SupportPrepared}}

	if err := projectWindsurfMCP(activeRoot, envelope, plan, filepath.Join(t.TempDir(), "data")); err != nil {
		t.Fatal(err)
	}
	objects, err := buildWindsurfNativeObjects(activeRoot, plan)
	if err != nil {
		t.Fatal(err)
	}
	if err := applyWindsurfNativeMutation(plan.NativeRegistryRoot, activeRoot, nil, objects); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(plan.NativeRegistryRoot, "mcp_config.json")
	entry := readObject(t, configPath)["mcpServers"].(map[string]any)["local"].(map[string]any)
	if got := entry["command"]; got != filepath.Join(activeRoot, filepath.FromSlash(managedstdio.RelativeDirectory), managedstdio.ExecutableName) {
		t.Fatalf("bundled Windsurf command = %v, want %s", got, commandPath)
	}
	if got := envelope.MCP.Servers["local"].Decoded["command"]; got != "./bin/server" {
		t.Fatalf("Windsurf projection mutated source command: %v", got)
	}
}

func TestWindsurfBundledCommandTraversalFailsBeforeProjectionMutation(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	projectionPath := filepath.Join(root, "mcp.json")
	writeTestFile(t, projectionPath, "sentinel\n")
	envelope := windsurfTestEnvelope(t, "escape")
	local := envelope.MCP.Servers["local"]
	local.Decoded = cloneObject(local.Decoded)
	local.Decoded["command"] = "./../outside"
	envelope.MCP.Servers["local"] = local
	plan := stagingPlan(t, domain.ClientWindsurf, domain.PackagePrepared)
	plan.ActivePath = filepath.Join(t.TempDir(), "managed-plugin")
	plan.Components = []domain.ComponentDecision{{Kind: domain.ComponentMCPServer, Name: "local", Support: domain.SupportPrepared}}

	err := projectWindsurfMCP(root, envelope, plan, filepath.Join(t.TempDir(), "data"))
	if err == nil || !strings.Contains(err.Error(), "escapes PLUGIN_ROOT") {
		t.Fatalf("Windsurf traversal projection error = %v", err)
	}
	body, readErr := os.ReadFile(projectionPath)
	if readErr != nil || string(body) != "sentinel\n" {
		t.Fatalf("rejected bundled command changed projection = %q, %v", body, readErr)
	}
	if got := envelope.MCP.Servers["local"].Decoded["command"]; got != "./../outside" {
		t.Fatalf("rejected Windsurf projection mutated source command: %v", got)
	}
}

func stagedWindsurfDelivery(t *testing.T, configRoot, revision, operation string) domain.StagedDelivery {
	t.Helper()
	envelope := windsurfTestEnvelope(t, revision)
	plan := stagingPlan(t, domain.ClientWindsurf, domain.PackagePrepared)
	plan.NativeRegistryRoot = configRoot
	plan.Components = []domain.ComponentDecision{
		{Kind: domain.ComponentMCPServer, Name: "local", Support: domain.SupportPrepared},
		{Kind: domain.ComponentMCPServer, Name: "docs", Support: domain.SupportPrepared},
	}
	delivery, err := windsurfFixtureStager(t).StageWithPluginData(context.Background(), envelope, plan, operation, domain.CompatibilityHints{}, filepath.Join(t.TempDir(), "data"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(delivery.ActivePath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(delivery.StagingPath, delivery.ActivePath); err != nil {
		t.Fatal(err)
	}
	delivery.StagingPath = ""
	return delivery
}

func windsurfActivationRequest(delivery domain.StagedDelivery, configRoot string, previous []domain.NativeObjectOwnership) domain.ActivationRequest {
	return domain.ActivationRequest{
		Client: domain.DetectedClient{ClientID: domain.ClientWindsurf, Status: domain.DetectionDetected, ConfigRoot: configRoot},
		Plan: domain.DeliveryPlan{ClientID: domain.ClientWindsurf, ActivePath: delivery.ActivePath, PhysicalArtifactID: "demo-0123456789ab", Authentication: domain.AuthenticationNotChecked,
			Components: []domain.ComponentDecision{{Kind: domain.ComponentMCPServer, Name: "local", Support: domain.SupportPrepared}, {Kind: domain.ComponentMCPServer, Name: "docs", Support: domain.SupportPrepared}}},
		DeclaredName: "demo", Delivery: delivery, PreviousNativeObjects: previous,
	}
}

func windsurfTestEnvelope(t *testing.T, revision string) domain.PackageEnvelope {
	t.Helper()
	envelope := stagingEnvelope(t)
	localRaw := json.RawMessage(`{"type":"stdio","command":"node","args":["${PLUGIN_ROOT}/runtime/server.js","${PLUGIN_DATA}/cache"]}`)
	remoteRaw := json.RawMessage(`{"type":"streamable-http","url":"https://docs.test/` + revision + `"}`)
	envelope.MCP.Servers = map[string]domain.MCPServer{
		"local": {Name: "local", Type: "stdio", Raw: localRaw, Decoded: map[string]any{"type": "stdio", "command": "node", "args": []any{"${PLUGIN_ROOT}/runtime/server.js", "${PLUGIN_DATA}/cache"}}},
		"docs":  {Name: "docs", Type: "streamable-http", Raw: remoteRaw, Decoded: map[string]any{"type": "streamable-http", "url": "https://docs.test/" + revision}},
	}
	return envelope
}

func assertWindsurfConfig(t *testing.T, path, revision string, foreign bool) {
	t.Helper()
	document := readObject(t, path)
	servers := document["mcpServers"].(map[string]any)
	if revision == "" {
		if _, exists := servers["docs"]; exists {
			t.Fatalf("owned Windsurf entries remain: %+v", servers)
		}
	} else if got := servers["docs"].(map[string]any)["serverUrl"]; got != "https://docs.test/"+revision {
		t.Fatalf("docs URL = %v", got)
	}
	if foreign {
		if got := servers["foreign"].(map[string]any)["url"]; got != "https://foreign.test" {
			t.Fatalf("foreign entry changed: %+v", servers)
		}
	}
}

func removeWindsurfEntryForTest(path, name string) error {
	body, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var document map[string]any
	if err := json.Unmarshal(body, &document); err != nil {
		return err
	}
	delete(document["mcpServers"].(map[string]any), name)
	next, err := json.Marshal(document)
	if err != nil {
		return err
	}
	return os.WriteFile(path, next, 0o600)
}

// These fixture bytes are explicitly injected and never executed. Native helper
// execution is covered by managedstdio subprocess tests and CLI integration.
func windsurfFixtureStager(t *testing.T) Stager {
	t.Helper()
	if !managedstdio.Supported() {
		t.Skip("managed stdio platform unsupported")
	}
	path := filepath.Join(t.TempDir(), "fixture-cli")
	if err := os.WriteFile(path, []byte("fixture-v1"), 0700); err != nil {
		t.Fatal(err)
	}
	source, err := managedstdio.NewSource(path, "fixture")
	if err != nil {
		t.Fatal(err)
	}
	return Stager{LauncherSource: source}
}

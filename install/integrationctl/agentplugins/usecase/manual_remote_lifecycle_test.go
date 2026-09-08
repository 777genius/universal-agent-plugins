package usecase

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers/nativeconfig"
)

func TestSignedChatGPTPreparationSupportsAddUpdateAndRepairWhileRemoteActivationIsPending(t *testing.T) {
	t.Parallel()
	service, store, _ := serviceFixture(t)
	service.NativeObserver = providers.NativeIdentityObserver{Stager: service.Stager}
	client := domain.DetectedClient{ClientID: domain.ClientChatGPT, DisplayName: "ChatGPT", Status: domain.DetectionNotDetected}

	add := signedChatGPTInput(t, client, "1.0.0", "sha256:chatgpt-v1", "sha256:chatgpt-manifest-v1")
	added, err := service.AddGroup(context.Background(), GroupInput{Targets: []AddInput{add}, OperationGroupID: "chatgpt-add", Confirmed: true})
	if err != nil {
		t.Fatal(err)
	}
	if added.Targets[0].Activation.Activation != domain.ActivationManual || added.Targets[0].Activation.Authentication != domain.AuthenticationPending || added.Targets[0].Activation.Verification == domain.VerificationInstalled {
		t.Fatalf("add claimed remote ChatGPT completion: %+v", added.Targets[0])
	}

	update := signedChatGPTInput(t, client, "2.0.0", "sha256:chatgpt-v2", "sha256:chatgpt-manifest-v2")
	updated, err := service.UpdateGroup(context.Background(), GroupInput{Targets: []AddInput{update}, CompatibilityChecks: []AddInput{update}, OperationGroupID: "chatgpt-update", Confirmed: true})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Targets[0].Activation.Activation != domain.ActivationManual || updated.Targets[0].Activation.Verification == domain.VerificationInstalled {
		t.Fatalf("update claimed remote ChatGPT completion: %+v", updated.Targets[0])
	}
	state, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	binding := onlyBinding(state.Installations[0])
	if err := os.RemoveAll(binding.TargetLocator); err != nil {
		t.Fatal(err)
	}
	update.InstallationID = added.InstallationID
	repaired, err := service.RepairGroup(context.Background(), GroupInput{Targets: []AddInput{update}, OperationGroupID: "chatgpt-repair", Confirmed: true, Repair: true})
	if err != nil {
		t.Fatal(err)
	}
	if repaired.Targets[0].Activation.Activation != domain.ActivationManual || repaired.Targets[0].Activation.Verification == domain.VerificationInstalled {
		t.Fatalf("repair claimed remote ChatGPT completion: %+v", repaired.Targets[0])
	}
}

func TestKiroSkillSupportsAutomaticAddUpdateAndRepair(t *testing.T) {
	t.Parallel()
	service, store, _ := serviceFixture(t)
	service.NativeObserver = providers.NativeIdentityObserver{Stager: service.Stager}
	client := domain.DetectedClient{ClientID: domain.ClientKiro, DisplayName: "Kiro", Status: domain.DetectionDetected, ConfigRoot: filepath.Join(t.TempDir(), ".kiro")}

	add := kiroPowerInput(t, client, "1.0.0", "sha256:kiro-v1", "sha256:kiro-manifest-v1")
	added, err := service.AddGroup(context.Background(), GroupInput{Targets: []AddInput{add}, OperationGroupID: "kiro-add", Confirmed: true})
	if err != nil {
		t.Fatal(err)
	}
	if added.Targets[0].Activation.Activation != domain.ActivationActive || added.Targets[0].Activation.Verification != domain.VerificationInstalled {
		t.Fatalf("Kiro add did not complete native installation: %+v", added.Targets[0])
	}
	if _, err := os.Stat(filepath.Join(client.ConfigRoot, "skills", "docs", "SKILL.md")); err != nil {
		t.Fatalf("Kiro skill was not installed: %v", err)
	}

	update := kiroPowerInput(t, client, "2.0.0", "sha256:kiro-v2", "sha256:kiro-manifest-v2")
	if _, err := service.UpdateGroup(context.Background(), GroupInput{Targets: []AddInput{update}, CompatibilityChecks: []AddInput{update}, OperationGroupID: "kiro-update", Confirmed: true}); err != nil {
		t.Fatal(err)
	}
	state, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	binding := onlyBinding(state.Installations[0])
	if err := os.RemoveAll(binding.TargetLocator); err != nil {
		t.Fatal(err)
	}
	update.InstallationID = added.InstallationID
	repaired, err := service.RepairGroup(context.Background(), GroupInput{Targets: []AddInput{update}, OperationGroupID: "kiro-repair", Confirmed: true, Repair: true})
	if err != nil {
		t.Fatal(err)
	}
	if repaired.Targets[0].Activation.Activation != domain.ActivationActive || repaired.Targets[0].Activation.Verification != domain.VerificationInstalled {
		t.Fatalf("Kiro repair did not restore native installation: %+v", repaired.Targets[0])
	}
	removed, err := service.RemoveGroup(context.Background(), RemoveGroupInput{
		Selector:         added.InstallationID,
		Targets:          []RemoveInput{{Client: client, Scope: domain.ScopeUser}},
		OperationGroupID: "kiro-remove", Confirmed: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !removed.Mutated {
		t.Fatalf("Kiro grouped remove did not mutate: %+v", removed)
	}
	if _, err := os.Stat(filepath.Join(client.ConfigRoot, "skills", "docs")); !os.IsNotExist(err) {
		t.Fatalf("Kiro grouped remove retained managed skill: %v", err)
	}
}

func TestOpenCodeSupportsAutomaticMCPAndSkillLifecycle(t *testing.T) {
	service, store, _ := serviceFixture(t)
	service.NativeObserver = providers.NativeIdentityObserver{Stager: service.Stager}
	client := domain.DetectedClient{ClientID: domain.ClientOpenCode, DisplayName: "OpenCode", Status: domain.DetectionDetected,
		ConfigRoot: filepath.Join(t.TempDir(), "xdg", "opencode"), ExecutablePath: "/test/bin/opencode"}
	runtimeRoot := t.TempDir()
	for _, runtimePath := range []string{filepath.Join(runtimeRoot, "node"), filepath.Join(runtimeRoot, "bun")} {
		if err := os.WriteFile(runtimePath, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
			t.Fatalf("write isolated runtime fixture: %v", err)
		}
	}
	t.Setenv("PATH", runtimeRoot)

	add := openCodePluginInput(t, client, "1.0.0", "sha256:opencode-v1", "sha256:opencode-manifest-v1", "node")
	added, err := service.AddGroup(context.Background(), GroupInput{Targets: []AddInput{add}, OperationGroupID: "opencode-add", Confirmed: true})
	if err != nil {
		t.Fatal(err)
	}
	if added.Targets[0].Activation.Activation != domain.ActivationActive || added.Targets[0].Activation.Verification != domain.VerificationInstalled {
		t.Fatalf("OpenCode add did not complete native installation: %+v", added.Targets[0])
	}
	config := readUsecaseObject(t, filepath.Join(client.ConfigRoot, "opencode.json"))
	server := config["mcp"].(map[string]any)["docs"].(map[string]any)
	if argv := server["command"].([]any); server["type"] != "local" || argv[0] != "node" {
		t.Fatalf("unexpected OpenCode MCP projection: %#v", server)
	}
	if _, err := os.Stat(filepath.Join(client.ConfigRoot, "skills", "docs", "SKILL.md")); err != nil {
		t.Fatalf("OpenCode skill was not installed: %v", err)
	}

	update := openCodePluginInput(t, client, "2.0.0", "sha256:opencode-v2", "sha256:opencode-manifest-v2", "bun")
	if _, err := service.UpdateGroup(context.Background(), GroupInput{Targets: []AddInput{update}, CompatibilityChecks: []AddInput{update}, OperationGroupID: "opencode-update", Confirmed: true}); err != nil {
		t.Fatal(err)
	}
	config = readUsecaseObject(t, filepath.Join(client.ConfigRoot, "opencode.json"))
	server = config["mcp"].(map[string]any)["docs"].(map[string]any)
	if argv := server["command"].([]any); argv[0] != "bun" {
		t.Fatalf("OpenCode MCP update did not converge: %#v", server)
	}
	state, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(onlyBinding(state.Installations[0]).NativeObjects) != 3 {
		t.Fatalf("OpenCode ownership receipts were not persisted: %+v", onlyBinding(state.Installations[0]).NativeObjects)
	}
	for key, binding := range state.Installations[0].Clients {
		binding.Authentication = domain.AuthenticationComplete
		binding.Materialization = domain.MaterializationDegraded
		state.Installations[0].Clients[key] = binding
	}
	if err := store.Save(state); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(client.ConfigRoot, "opencode.json")
	if err := os.WriteFile(configPath, []byte(`{"mcp":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	// No executable is needed to observe native ownership. A same-revision
	// update must not claim no-change while its exact entry is absent.
	update.InstallationID = added.InstallationID
	update.Confirmed = true
	update.OperationID = "opencode-no-change-probe"
	if result, err := service.Update(context.Background(), update); err == nil || result.NoChange {
		t.Fatalf("missing native entry was accepted as no-change: result=%+v err=%v", result, err)
	}
	update.OperationID = "opencode-native-repair"
	repaired, err := service.Repair(context.Background(), update)
	if err != nil || !repaired.Mutated || repaired.Activation.Verification != domain.VerificationInstalled {
		t.Fatalf("native-only repair failed: result=%+v err=%v", repaired, err)
	}
	state, err = store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if binding := onlyBinding(state.Installations[0]); binding.Materialization != domain.MaterializationMaterialized || binding.Authentication != domain.AuthenticationComplete {
		t.Fatalf("native repair did not converge degraded/attested state: %+v", binding)
	}
	config = readUsecaseObject(t, configPath)
	if _, exists := config["mcp"].(map[string]any)["docs"]; !exists {
		t.Fatalf("native-only repair did not recreate the exact entry: %#v", config)
	}

	foreign := `{"mcp":{"docs":{"type":"local","command":["foreign"]}}}`
	if err := os.WriteFile(configPath, []byte(foreign), 0o600); err != nil {
		t.Fatal(err)
	}
	update.OperationID = "opencode-tampered-repair"
	if _, err := service.Repair(context.Background(), update); err == nil {
		t.Fatal("repair adopted a tampered OpenCode entry")
	}
	if body, err := os.ReadFile(configPath); err != nil || string(body) != foreign {
		t.Fatalf("failed repair changed tampered entry: %s, %v", body, err)
	}
	// Removal remains idempotent when the exact owned entry is already absent.
	if err := os.WriteFile(configPath, []byte(`{"mcp":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	removed, err := service.RemoveGroup(context.Background(), RemoveGroupInput{Selector: added.InstallationID,
		Targets: []RemoveInput{{Client: client, Scope: domain.ScopeUser}}, OperationGroupID: "opencode-remove", Confirmed: true})
	if err != nil || !removed.Mutated {
		t.Fatalf("OpenCode remove failed: result=%+v err=%v", removed, err)
	}
	config = readUsecaseObject(t, configPath)
	if _, exists := config["mcp"].(map[string]any)["docs"]; exists {
		t.Fatalf("managed OpenCode MCP entry survived remove: %#v", config)
	}
	if _, err := os.Stat(filepath.Join(client.ConfigRoot, "skills", "docs")); !os.IsNotExist(err) {
		t.Fatalf("managed OpenCode skill survived remove: %v", err)
	}
}

func TestClinePackageSupportsAutomaticAddUpdateRepairAndRemoveInIsolatedHome(t *testing.T) {
	root := t.TempDir()
	settings := filepath.Join(root, "cline-data", "settings", "cline_mcp_settings.json")
	t.Setenv("CLINE_MCP_SETTINGS_PATH", settings)
	service, store, _ := serviceFixture(t)
	service.NativeObserver = providers.NativeIdentityObserver{Stager: service.Stager}
	var lockCycle int
	cleanupErr := fmt.Errorf("injected post-write Cline lock cleanup failure")
	kernel := nativeconfig.NewWithLockAcquirer(func(nativeconfig.Paths, nativeconfig.Codec) (func() error, error) {
		lockCycle++
		cycle := lockCycle
		return func() error {
			if cycle == 2 || cycle == 3 {
				return cleanupErr
			}
			return nil
		}, nil
	})
	service.Activator = providers.Activator{NativeConfig: &kernel}
	client := domain.DetectedClient{ClientID: domain.ClientCline, DisplayName: "Cline", Status: domain.DetectionDetected, ConfigRoot: filepath.Join(root, ".cline")}

	add := clinePackageInput(t, client, "1.0.0", "sha256:cline-v1", "sha256:cline-manifest-v1", "sh")
	added, err := service.AddGroup(context.Background(), GroupInput{Targets: []AddInput{add}, OperationGroupID: "cline-add", Confirmed: true})
	if err != nil {
		t.Fatal(err)
	}
	if added.Targets[0].Activation.Activation != domain.ActivationActive || added.Targets[0].Activation.Verification != domain.VerificationInstalled {
		t.Fatalf("Cline add did not complete: %+v", added.Targets[0])
	}
	if _, err := os.Stat(filepath.Join(client.ConfigRoot, "skills", "docs", "SKILL.md")); err != nil {
		t.Fatal(err)
	}
	if body, err := os.ReadFile(settings); err != nil || !strings.Contains(string(body), `"transport"`) || !strings.Contains(string(body), `"sh"`) {
		t.Fatalf("Cline MCP projection = %s, %v", body, err)
	}

	update := clinePackageInput(t, client, "2.0.0", "sha256:cline-v2", "sha256:cline-manifest-v2", "env")
	update.Confirmed = true
	updated, err := service.Update(context.Background(), update)
	if err != nil {
		t.Fatal(err)
	}
	if !containsAction(updated.Activation.UserActions, "lock cleanup degraded") {
		t.Fatalf("committed update cleanup degradation was not surfaced: %+v", updated.Activation)
	}
	state, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(onlyBinding(state.Installations[0]).TargetLocator); err != nil {
		t.Fatal(err)
	}
	update.InstallationID = added.InstallationID
	repaired, err := service.RepairGroup(context.Background(), GroupInput{Targets: []AddInput{update}, OperationGroupID: "cline-repair", Confirmed: true, Repair: true})
	if err != nil || repaired.Targets[0].Activation.Verification != domain.VerificationInstalled {
		t.Fatalf("Cline repair = %+v, %v", repaired, err)
	}
	if !containsAction(repaired.Targets[0].Activation.UserActions, "lock cleanup degraded") {
		t.Fatalf("committed repair cleanup degradation was not surfaced: %+v", repaired.Targets[0].Activation)
	}
	state, err = store.Load()
	if err != nil {
		t.Fatal(err)
	}
	binding := onlyBinding(state.Installations[0])
	var mcpDigest string
	for _, object := range binding.NativeObjects {
		if object.Kind == "cline_global_mcp_server" {
			mcpDigest = object.ManagedDigest
		}
	}
	if mcpDigest == "" {
		t.Fatalf("committed Cline receipt was reconciled away: %+v", binding.NativeObjects)
	}
	present, owned, inspectErr := nativeconfig.New().Inspect(nativeconfig.Paths{JSON: settings}, nativeconfig.CodecCline, "docs", &nativeconfig.Receipt{
		Version: "1", Path: settings, Codec: nativeconfig.CodecCline, Name: "docs", Digest: mcpDigest,
	})
	if inspectErr != nil || !present || !owned {
		t.Fatalf("Cline config/receipt diverged after degraded update+repair: present=%v owned=%v err=%v", present, owned, inspectErr)
	}
	removed, err := service.RemoveGroup(context.Background(), RemoveGroupInput{Selector: added.InstallationID, Targets: []RemoveInput{{Client: client, Scope: domain.ScopeUser}}, OperationGroupID: "cline-remove", Confirmed: true})
	if err != nil || !removed.Mutated {
		t.Fatalf("Cline remove = %+v, %v", removed, err)
	}
	if body, _ := os.ReadFile(settings); strings.Contains(string(body), `"docs"`) {
		t.Fatalf("Cline remove retained MCP entry: %s", body)
	}
}

func containsAction(actions []string, fragment string) bool {
	for _, action := range actions {
		if strings.Contains(action, fragment) {
			return true
		}
	}
	return false
}

func signedChatGPTInput(t *testing.T, client domain.DetectedClient, version, treeDigest, manifestDigest string) AddInput {
	t.Helper()
	input := addInput(t, client, "https://example.com/chatgpt")
	setEnvelopeVersion(t, &input.Envelope, version, treeDigest, manifestDigest)
	appBody := []byte(`{"apps":{"docs":{"id":"asdk_app_docs_123"}}}`)
	if err := os.WriteFile(filepath.Join(input.Envelope.SnapshotRoot, ".app.json"), appBody, 0o644); err != nil {
		t.Fatal(err)
	}
	mcpBody := []byte(`{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"docs":{"type":"streamable-http","url":"https://example.test/mcp"}}}`)
	if err := os.WriteFile(filepath.Join(input.Envelope.SnapshotRoot, "mcp.json"), mcpBody, 0o644); err != nil {
		t.Fatal(err)
	}
	input.Envelope.App = domain.AppComponent{Present: true, Declared: true, Enabled: true, Raw: appBody, Bindings: map[string]domain.AppBinding{
		"docs": {Alias: "docs", ID: "asdk_app_docs_123", Raw: []byte(`{"id":"asdk_app_docs_123"}`)},
	}}
	input.Envelope.MCP = domain.MCPComponent{Present: true, Enabled: true, SchemaURI: domain.MCPSchemaV1, Raw: mcpBody, Servers: map[string]domain.MCPServer{
		"docs": {Name: "docs", Type: "streamable-http", Raw: []byte(`{"type":"streamable-http","url":"https://example.test/mcp"}`), Decoded: map[string]any{"type": "streamable-http", "url": "https://example.test/mcp"}},
	}}
	input.Envelope.Inventory.AppPresent = true
	input.Envelope.Inventory.AppBindings = []string{"docs"}
	input.Envelope.Inventory.MCPPresent = true
	input.Envelope.Inventory.MCPEnabled = true
	input.Envelope.Inventory.MCPServers = []string{"docs"}
	input.Envelope.CatalogEvidence = &domain.CatalogEvidence{SchemaVersion: 1, CatalogVersion: "directory-snapshot-1", Digest: "sha256:" + strings.Repeat("a", 64), Compatibility: map[string]domain.CatalogCompatibility{
		"chatgpt": {Package: "projected", Verification: "tested", Authentication: domain.AuthenticationRequirementRequired,
			AppBinding: &domain.CatalogAppBinding{AppKey: "docs", ID: "asdk_app_docs_123", MCPServer: "docs", MCPURL: "https://example.test/mcp"}},
	}}
	input.Hints.Compatibility = input.Envelope.CatalogEvidence.Compatibility
	return input
}

func kiroPowerInput(t *testing.T, client domain.DetectedClient, version, treeDigest, manifestDigest string) AddInput {
	t.Helper()
	input := addInput(t, client, "https://example.com/kiro")
	setEnvelopeVersion(t, &input.Envelope, version, treeDigest, manifestDigest)
	skillRoot := filepath.Join(input.Envelope.SnapshotRoot, "skills", "docs")
	if err := os.MkdirAll(skillRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillRoot, "SKILL.md"), []byte("---\nname: docs\ndescription: docs\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	input.Envelope.Skills = map[string]domain.Skill{"docs": {Name: "docs", Description: "docs", RelativePath: "skills/docs/SKILL.md", Raw: []byte("---\nname: docs\ndescription: docs\n---\n")}}
	input.Envelope.Inventory.Skills = []string{"docs"}
	return input
}

func openCodePluginInput(t *testing.T, client domain.DetectedClient, version, treeDigest, manifestDigest, command string) AddInput {
	t.Helper()
	input := addInput(t, client, "https://example.com/opencode")
	setEnvelopeVersion(t, &input.Envelope, version, treeDigest, manifestDigest)
	skillBody := []byte("---\nname: docs\ndescription: docs " + version + "\n---\n")
	skillRoot := filepath.Join(input.Envelope.SnapshotRoot, "skills", "docs")
	if err := os.MkdirAll(skillRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillRoot, "SKILL.md"), skillBody, 0o644); err != nil {
		t.Fatal(err)
	}
	mcpBody := []byte(fmt.Sprintf(`{"mcpServers":{"docs":{"type":"stdio","command":%q,"args":["${PLUGIN_ROOT}/server.js"]}}}`, command))
	if err := os.WriteFile(filepath.Join(input.Envelope.SnapshotRoot, "mcp.json"), mcpBody, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(input.Envelope.SnapshotRoot, "server.js"), []byte("// test"), 0o644); err != nil {
		t.Fatal(err)
	}
	input.Envelope.Skills = map[string]domain.Skill{"docs": {Name: "docs", Description: "docs", RelativePath: "skills/docs/SKILL.md", Raw: skillBody}}
	input.Envelope.MCP = domain.MCPComponent{Present: true, Enabled: true, Raw: mcpBody, Servers: map[string]domain.MCPServer{
		"docs": {Name: "docs", Type: "stdio", Raw: mcpBody, Decoded: map[string]any{"type": "stdio", "command": command, "args": []any{"${PLUGIN_ROOT}/server.js"}}},
	}}
	input.Envelope.Inventory.Skills = []string{"docs"}
	input.Envelope.Inventory.MCPPresent = true
	input.Envelope.Inventory.MCPEnabled = true
	input.Envelope.Inventory.MCPServers = []string{"docs"}
	return input
}

func clinePackageInput(t *testing.T, client domain.DetectedClient, version, treeDigest, manifestDigest, command string) AddInput {
	t.Helper()
	input := addInput(t, client, "https://example.com/cline")
	setEnvelopeVersion(t, &input.Envelope, version, treeDigest, manifestDigest)
	skillRoot := filepath.Join(input.Envelope.SnapshotRoot, "skills", "docs")
	if err := os.MkdirAll(skillRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	skill := []byte("---\nname: docs\ndescription: docs\n---\n")
	if err := os.WriteFile(filepath.Join(skillRoot, "SKILL.md"), skill, 0o644); err != nil {
		t.Fatal(err)
	}
	mcp := []byte(fmt.Sprintf(`{"$schema":%q,"mcpServers":{"docs":{"type":"stdio","command":%q}}}`, domain.MCPSchemaV1, command))
	if err := os.WriteFile(filepath.Join(input.Envelope.SnapshotRoot, "mcp.json"), mcp, 0o644); err != nil {
		t.Fatal(err)
	}
	input.Envelope.Skills = map[string]domain.Skill{"docs": {Name: "docs", Description: "docs", RelativePath: "skills/docs/SKILL.md", Raw: skill}}
	input.Envelope.MCP = domain.MCPComponent{Present: true, Enabled: true, SchemaURI: domain.MCPSchemaV1, Raw: mcp, Servers: map[string]domain.MCPServer{
		"docs": {Name: "docs", Type: "stdio", Raw: []byte(fmt.Sprintf(`{"type":"stdio","command":%q}`, command)), Decoded: map[string]any{"type": "stdio", "command": command}, StdioRequirement: &domain.StdioRequirement{Command: command, Kind: domain.ExecutableBare}},
	}}
	input.Envelope.Inventory.Skills = []string{"docs"}
	input.Envelope.Inventory.MCPPresent = true
	input.Envelope.Inventory.MCPEnabled = true
	input.Envelope.Inventory.MCPServers = []string{"docs"}
	return input
}

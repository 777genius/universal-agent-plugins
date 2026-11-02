package pluginkitairepo_test

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

var (
	trailingCommaRE = regexp.MustCompile(`,\s*([}\]])`)
	operationLineRE = regexp.MustCompile(`(?m)^(?:🆔\s*)?Operation:\s*(\S+)\s*$`)
)

type integrationsLifecycleVersion struct {
	Version             string
	CursorAlias         string
	CodexPrompt         string
	OpenCodePluginRef   string
	OpenCodePermission  string
	OpenCodeExampleBody string
	IncludeDefaultAgent bool
	IncludeStalePlugin  bool
}

type fileSnapshot struct {
	Exists bool
	Body   string
}

func TestPluginKitAIIntegrationsLifecycleAcrossAgents(t *testing.T) {
	pluginKitAIBin := buildPluginKitAI(t)
	workspaceRoot := t.TempDir()
	homeRoot := t.TempDir()
	sourceRoot := filepath.Join(workspaceRoot, "sources", "lifecycle-demo")

	statePath := filepath.Join(homeRoot, ".plugin-kit-ai", "state.json")
	cursorConfigPath := filepath.Join(workspaceRoot, ".cursor", "mcp.json")
	opencodeConfigPath := filepath.Join(workspaceRoot, "opencode.json")
	codexCatalogPath := filepath.Join(workspaceRoot, ".agents", "plugins", "marketplace.json")
	codexPluginRoot := filepath.Join(workspaceRoot, ".agents", "plugins", "plugins", "lifecycle-demo")
	codexPluginManifestPath := filepath.Join(codexPluginRoot, ".codex-plugin", "plugin.json")
	codexMCPPath := filepath.Join(codexPluginRoot, ".mcp.json")
	opencodeExamplePath := filepath.Join(workspaceRoot, ".opencode", "plugins", "example.js")
	opencodeStalePath := filepath.Join(workspaceRoot, ".opencode", "plugins", "stale.js")
	opencodePackagePath := filepath.Join(workspaceRoot, ".opencode", "package.json")

	seedIntegrationLifecycleWorkspace(t, workspaceRoot)

	writeIntegrationLifecycleSource(t, sourceRoot, integrationsLifecycleVersion{
		Version:             "0.1.0",
		CursorAlias:         "release-checks",
		CodexPrompt:         "Run the original lifecycle checks",
		OpenCodePluginRef:   "@acme/lifecycle-demo-plugin",
		OpenCodePermission:  `{"bash":"ask"}`,
		OpenCodeExampleBody: "export const ExamplePlugin = async () => ({ version: 'v1' })\n",
		IncludeDefaultAgent: true,
		IncludeStalePlugin:  true,
	})

	initialCursorConfig := snapshotFile(t, cursorConfigPath)
	initialOpenCodeConfig := snapshotFile(t, opencodeConfigPath)
	initialCodexCatalog := snapshotFile(t, codexCatalogPath)

	addDryRunOutput := runIntegrationsLifecycleCommand(t, pluginKitAIBin, workspaceRoot, homeRoot,
		"integrations", "add", sourceRoot, "--scope", "project", "--dry-run=true")
	for _, want := range []string{
		`Dry-run plan for integration "lifecycle-demo" at version 0.1.0.`,
		"✅ Already present: cursor, opencode",
		"⬇️ New install: codex",
		"codex - will install Codex plugin",
		"✅ cursor - will adopt existing Cursor MCP setup",
		"✅ opencode - will adopt existing OpenCode plugin",
	} {
		if !strings.Contains(addDryRunOutput, want) {
			t.Fatalf("add dry-run output missing %q:\n%s", want, addDryRunOutput)
		}
	}

	assertNoOperationRecord(t, homeRoot, operationIDFromOutput(t, addDryRunOutput))
	assertPathAbsent(t, statePath)
	assertPathAbsent(t, codexPluginRoot)
	assertPathAbsent(t, opencodeExamplePath)
	assertPathAbsent(t, opencodeStalePath)
	assertPathAbsent(t, opencodePackagePath)
	assertFileSnapshot(t, "Cursor config after add dry-run", cursorConfigPath, initialCursorConfig)
	assertFileSnapshot(t, "OpenCode config after add dry-run", opencodeConfigPath, initialOpenCodeConfig)
	assertFileSnapshot(t, "Codex catalog after add dry-run", codexCatalogPath, initialCodexCatalog)

	addOutput := runIntegrationsLifecycleCommand(t, pluginKitAIBin, workspaceRoot, homeRoot,
		"integrations", "add", sourceRoot, "--scope", "project", "--dry-run=false")
	for _, want := range []string{
		`Installed integration "lifecycle-demo" at version 0.1.0.`,
		"🚀 Ready now: cursor",
		"🔄 Restart or reload: opencode",
		"🧩 Finish setup: codex",
		"codex - prepared Codex plugin for in-app install",
		"✅ cursor - installed Cursor MCP setup",
		"🟡 opencode - installed OpenCode plugin",
	} {
		if !strings.Contains(addOutput, want) {
			t.Fatalf("add output missing %q:\n%s", want, addOutput)
		}
	}

	addOperationID := operationIDFromOutput(t, addOutput)
	assertCommittedOperation(t, readOperationRecord(t, homeRoot, addOperationID), addOperationID, "add", "lifecycle-demo", []string{"cursor", "codex", "opencode"})

	state := readIntegrationLifecycleState(t, homeRoot)
	if state.SchemaVersion != 1 {
		t.Fatalf("schema version = %d, want 1", state.SchemaVersion)
	}
	record := requireManagedIntegration(t, state, "lifecycle-demo")
	assertDigestLike(t, "source digest after add", record.SourceDigest)
	assertDigestLike(t, "manifest digest after add", record.ManifestDigest)
	addSourceDigest := record.SourceDigest
	addManifestDigest := record.ManifestDigest
	if record.ResolvedVersion != "0.1.0" {
		t.Fatalf("resolved version = %q, want 0.1.0", record.ResolvedVersion)
	}
	if !sameExistingPath(t, record.WorkspaceRoot, workspaceRoot) {
		t.Fatalf("workspace root = %q, want path-equivalent to %q", record.WorkspaceRoot, workspaceRoot)
	}
	if len(record.Targets) != 3 {
		t.Fatalf("target count = %d, want 3", len(record.Targets))
	}

	cursorTarget := record.Targets[domain.TargetCursor]
	if cursorTarget.DeliveryKind != domain.DeliveryCursorMCP {
		t.Fatalf("cursor delivery kind = %s, want %s", cursorTarget.DeliveryKind, domain.DeliveryCursorMCP)
	}
	if cursorTarget.State != domain.InstallInstalled {
		t.Fatalf("cursor state = %s, want %s", cursorTarget.State, domain.InstallInstalled)
	}
	assertStringSet(t, "Cursor capability surface after add", cursorTarget.CapabilitySurface, []string{"mcp"})
	if got := metadataString(cursorTarget.AdapterMetadata, "config_path"); got == "" || !sameExistingPath(t, got, cursorConfigPath) {
		t.Fatalf("Cursor config_path = %q, want %q", got, cursorConfigPath)
	}
	assertStringSet(t, "Cursor owned aliases in state after add", ownedNamesForKind(cursorTarget, "cursor_mcp_server"), []string{"release-checks"})
	assertStringSet(t, "Cursor metadata owned aliases after add", metadataStringSlice(cursorTarget.AdapterMetadata, "owned_aliases"), []string{"release-checks"})

	codexTarget := record.Targets[domain.TargetCodex]
	if codexTarget.DeliveryKind != domain.DeliveryCodexMarketplace {
		t.Fatalf("codex delivery kind = %s, want %s", codexTarget.DeliveryKind, domain.DeliveryCodexMarketplace)
	}
	if codexTarget.State != domain.InstallActivationPending {
		t.Fatalf("codex state = %s, want %s", codexTarget.State, domain.InstallActivationPending)
	}
	assertStringSet(t, "Codex capability surface after add", codexTarget.CapabilitySurface, []string{"plugin_bundle", "skills", "mcp", "app"})
	if got := metadataString(codexTarget.AdapterMetadata, "catalog_path"); got == "" || !sameExistingPath(t, got, codexCatalogPath) {
		t.Fatalf("Codex catalog_path = %q, want %q", got, codexCatalogPath)
	}
	if got := metadataString(codexTarget.AdapterMetadata, "plugin_root"); got == "" || !sameExistingPath(t, got, codexPluginRoot) {
		t.Fatalf("Codex plugin_root = %q, want %q", got, codexPluginRoot)
	}
	if got := metadataString(codexTarget.AdapterMetadata, "catalog_name"); got != "local-repo" {
		t.Fatalf("Codex catalog_name = %q, want local-repo", got)
	}
	if got := metadataString(codexTarget.AdapterMetadata, "plugin_name"); got != "lifecycle-demo" {
		t.Fatalf("Codex plugin_name = %q, want lifecycle-demo", got)
	}
	if got := metadataString(codexTarget.AdapterMetadata, "activation_method"); got != "plugin_directory_install" {
		t.Fatalf("Codex activation_method = %q, want plugin_directory_install", got)
	}
	assertStringSet(t, "Codex owned object kinds after add", ownedKinds(codexTarget), []string{"marketplace_catalog", "marketplace_entry", "plugin_root"})

	opencodeTarget := record.Targets[domain.TargetOpenCode]
	if opencodeTarget.DeliveryKind != domain.DeliveryOpenCodePlugin {
		t.Fatalf("OpenCode delivery kind = %s, want %s", opencodeTarget.DeliveryKind, domain.DeliveryOpenCodePlugin)
	}
	if opencodeTarget.State != domain.InstallInstalled {
		t.Fatalf("OpenCode state = %s, want %s", opencodeTarget.State, domain.InstallInstalled)
	}
	assertStringSet(t, "OpenCode capability surface after add", opencodeTarget.CapabilitySurface, []string{"plugin", "mcp", "skills", "commands", "agents", "themes", "tools"})
	if got := metadataString(opencodeTarget.AdapterMetadata, "config_path"); got == "" || !sameExistingPath(t, got, opencodeConfigPath) {
		t.Fatalf("OpenCode config_path = %q, want %q", got, opencodeConfigPath)
	}
	assertStringSet(t, "OpenCode managed config keys after add", metadataStringSlice(opencodeTarget.AdapterMetadata, "managed_config_keys"), []string{"default_agent", "permission"})
	assertStringSet(t, "OpenCode plugin refs in metadata after add", metadataStringSlice(opencodeTarget.AdapterMetadata, "owned_plugin_refs"), []string{"@acme/lifecycle-demo-plugin"})
	assertStringSet(t, "OpenCode MCP aliases in metadata after add", metadataStringSlice(opencodeTarget.AdapterMetadata, "owned_mcp_aliases"), []string{"release-checks"})
	assertPathSetEquivalent(t, "OpenCode copied paths after add", metadataStringSlice(opencodeTarget.AdapterMetadata, "copied_paths"), []string{opencodeExamplePath, opencodePackagePath, opencodeStalePath})
	assertStringSet(t, "OpenCode owned plugin refs after add", ownedNamesForKind(opencodeTarget, "opencode_plugin_ref"), []string{"@acme/lifecycle-demo-plugin"})
	assertStringSet(t, "OpenCode owned MCP aliases after add", ownedNamesForKind(opencodeTarget, "opencode_mcp_server"), []string{"release-checks"})
	assertStringSet(t, "OpenCode owned config keys after add", ownedNamesForKind(opencodeTarget, "opencode_config_key"), []string{"default_agent", "permission"})
	assertPathSetEquivalent(t, "OpenCode owned file paths after add", ownedPathsForKind(opencodeTarget, "file"), []string{opencodeConfigPath, opencodeExamplePath, opencodePackagePath, opencodeStalePath})

	assertStringSet(t, "Cursor aliases after add", cursorAliases(t, cursorConfigPath), []string{"release-checks", "user-owned"})
	assertStringSet(t, "OpenCode plugins after add", openCodePluginRefs(t, opencodeConfigPath), []string{"@acme/lifecycle-demo-plugin", "@user/existing"})
	assertStringSet(t, "OpenCode MCP aliases after add", openCodeMCPAliases(t, opencodeConfigPath), []string{"release-checks", "user-owned"})
	opencodeDoc := readJSONDoc(t, opencodeConfigPath)
	if theme, _ := opencodeDoc["theme"].(string); theme != "midnight" {
		t.Fatalf("OpenCode theme = %q, want midnight", theme)
	}
	if _, ok := opencodeDoc["default_agent"]; !ok {
		t.Fatalf("expected default_agent in OpenCode config after add:\n%v", opencodeDoc)
	}
	if _, ok := opencodeDoc["permission"]; !ok {
		t.Fatalf("expected permission in OpenCode config after add:\n%v", opencodeDoc)
	}
	for _, path := range []string{opencodeExamplePath, opencodeStalePath, opencodePackagePath} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected projected OpenCode asset %s: %v", path, err)
		}
	}

	assertStringSet(t, "Codex marketplace entries after add", codexMarketplacePluginNames(t, codexCatalogPath), []string{"alpha", "lifecycle-demo"})
	codexPluginManifest := readFileText(t, codexPluginManifestPath)
	for _, want := range []string{`"version": "0.1.0"`, "Run the original lifecycle checks"} {
		if !strings.Contains(codexPluginManifest, want) {
			t.Fatalf("Codex plugin manifest missing %q:\n%s", want, codexPluginManifest)
		}
	}
	assertStringSet(t, "Codex MCP aliases after add", codexMCPAliases(t, codexMCPPath), []string{"release-checks"})

	writeIntegrationLifecycleSource(t, sourceRoot, integrationsLifecycleVersion{
		Version:             "0.2.0",
		CursorAlias:         "release-checks-v2",
		CodexPrompt:         "Run the updated lifecycle checks",
		OpenCodePluginRef:   "@acme/lifecycle-demo-next-plugin",
		OpenCodePermission:  `{"edit":"allow"}`,
		OpenCodeExampleBody: "export const ExamplePlugin = async () => ({ version: 'v2' })\n",
	})

	beforeUpdateState := snapshotFile(t, statePath)
	beforeUpdateCursorConfig := snapshotFile(t, cursorConfigPath)
	beforeUpdateOpenCodeConfig := snapshotFile(t, opencodeConfigPath)
	beforeUpdateCodexCatalog := snapshotFile(t, codexCatalogPath)
	beforeUpdateCodexPluginManifest := snapshotFile(t, codexPluginManifestPath)
	beforeUpdateOpenCodeExample := snapshotFile(t, opencodeExamplePath)
	beforeUpdateOpenCodeStale := snapshotFile(t, opencodeStalePath)
	beforeUpdateOpenCodePackage := snapshotFile(t, opencodePackagePath)

	updateDryRunOutput := runIntegrationsLifecycleCommand(t, pluginKitAIBin, workspaceRoot, homeRoot,
		"integrations", "update", "lifecycle-demo", "--dry-run=true")
	for _, want := range []string{
		`Dry-run update_version plan for "lifecycle-demo".`,
		"✅ Already present: cursor, opencode",
		"codex - will update Codex plugin",
		"✅ cursor - will update Cursor MCP setup",
		"✅ opencode - will update OpenCode plugin",
	} {
		if !strings.Contains(updateDryRunOutput, want) {
			t.Fatalf("update dry-run output missing %q:\n%s", want, updateDryRunOutput)
		}
	}

	assertNoOperationRecord(t, homeRoot, operationIDFromOutput(t, updateDryRunOutput))
	assertFileSnapshot(t, "state after update dry-run", statePath, beforeUpdateState)
	assertFileSnapshot(t, "Cursor config after update dry-run", cursorConfigPath, beforeUpdateCursorConfig)
	assertFileSnapshot(t, "OpenCode config after update dry-run", opencodeConfigPath, beforeUpdateOpenCodeConfig)
	assertFileSnapshot(t, "Codex catalog after update dry-run", codexCatalogPath, beforeUpdateCodexCatalog)
	assertFileSnapshot(t, "Codex plugin manifest after update dry-run", codexPluginManifestPath, beforeUpdateCodexPluginManifest)
	assertFileSnapshot(t, "OpenCode example.js after update dry-run", opencodeExamplePath, beforeUpdateOpenCodeExample)
	assertFileSnapshot(t, "OpenCode stale.js after update dry-run", opencodeStalePath, beforeUpdateOpenCodeStale)
	assertFileSnapshot(t, "OpenCode package.json after update dry-run", opencodePackagePath, beforeUpdateOpenCodePackage)

	updateOutput := runIntegrationsLifecycleCommand(t, pluginKitAIBin, workspaceRoot, homeRoot,
		"integrations", "update", "lifecycle-demo", "--dry-run=false")
	for _, want := range []string{
		`Updated integration "lifecycle-demo".`,
		"🚀 Ready now: cursor",
		"🔄 Restart or reload: opencode",
		"🧩 Finish setup: codex",
		"codex - prepared Codex plugin for in-app install",
		"✅ cursor - installed Cursor MCP setup",
		"🟡 opencode - installed OpenCode plugin",
	} {
		if !strings.Contains(updateOutput, want) {
			t.Fatalf("update output missing %q:\n%s", want, updateOutput)
		}
	}

	updateOperationID := operationIDFromOutput(t, updateOutput)
	assertCommittedOperation(t, readOperationRecord(t, homeRoot, updateOperationID), updateOperationID, "update", "lifecycle-demo", []string{"codex", "cursor", "opencode"})

	state = readIntegrationLifecycleState(t, homeRoot)
	record = requireManagedIntegration(t, state, "lifecycle-demo")
	assertDigestLike(t, "source digest after update", record.SourceDigest)
	assertDigestLike(t, "manifest digest after update", record.ManifestDigest)
	if record.SourceDigest == addSourceDigest {
		t.Fatalf("source digest did not change after update: %q", record.SourceDigest)
	}
	if record.ManifestDigest == addManifestDigest {
		t.Fatalf("manifest digest did not change after update: %q", record.ManifestDigest)
	}
	if record.ResolvedVersion != "0.2.0" {
		t.Fatalf("resolved version = %q, want 0.2.0", record.ResolvedVersion)
	}
	if len(record.Targets) != 3 {
		t.Fatalf("target count after update = %d, want 3", len(record.Targets))
	}

	cursorTarget = record.Targets[domain.TargetCursor]
	assertStringSet(t, "Cursor owned aliases in state after update", ownedNamesForKind(cursorTarget, "cursor_mcp_server"), []string{"release-checks-v2"})
	assertStringSet(t, "Cursor metadata owned aliases after update", metadataStringSlice(cursorTarget.AdapterMetadata, "owned_aliases"), []string{"release-checks-v2"})

	codexTarget = record.Targets[domain.TargetCodex]
	if got := metadataString(codexTarget.AdapterMetadata, "activation_method"); got != "plugin_directory_refresh" {
		t.Fatalf("Codex activation_method after update = %q, want plugin_directory_refresh", got)
	}

	opencodeTarget = record.Targets[domain.TargetOpenCode]
	assertStringSet(t, "OpenCode managed config keys after update", metadataStringSlice(opencodeTarget.AdapterMetadata, "managed_config_keys"), []string{"permission"})
	assertStringSet(t, "OpenCode plugin refs in metadata after update", metadataStringSlice(opencodeTarget.AdapterMetadata, "owned_plugin_refs"), []string{"@acme/lifecycle-demo-next-plugin"})
	assertStringSet(t, "OpenCode MCP aliases in metadata after update", metadataStringSlice(opencodeTarget.AdapterMetadata, "owned_mcp_aliases"), []string{"release-checks-v2"})
	assertPathSetEquivalent(t, "OpenCode copied paths after update", metadataStringSlice(opencodeTarget.AdapterMetadata, "copied_paths"), []string{opencodeExamplePath, opencodePackagePath})
	assertStringSet(t, "OpenCode owned plugin refs after update", ownedNamesForKind(opencodeTarget, "opencode_plugin_ref"), []string{"@acme/lifecycle-demo-next-plugin"})
	assertStringSet(t, "OpenCode owned MCP aliases after update", ownedNamesForKind(opencodeTarget, "opencode_mcp_server"), []string{"release-checks-v2"})
	assertStringSet(t, "OpenCode owned config keys after update", ownedNamesForKind(opencodeTarget, "opencode_config_key"), []string{"permission"})
	assertPathSetEquivalent(t, "OpenCode owned file paths after update", ownedPathsForKind(opencodeTarget, "file"), []string{opencodeConfigPath, opencodeExamplePath, opencodePackagePath})

	assertStringSet(t, "Cursor aliases after update", cursorAliases(t, cursorConfigPath), []string{"release-checks-v2", "user-owned"})
	opencodeDoc = readJSONDoc(t, opencodeConfigPath)
	assertStringSet(t, "OpenCode plugins after update", openCodePluginRefs(t, opencodeConfigPath), []string{"@acme/lifecycle-demo-next-plugin", "@user/existing"})
	assertStringSet(t, "OpenCode MCP aliases after update", openCodeMCPAliases(t, opencodeConfigPath), []string{"release-checks-v2", "user-owned"})
	if _, ok := opencodeDoc["default_agent"]; ok {
		t.Fatalf("default_agent should be removed after update:\n%v", opencodeDoc)
	}
	permissionDoc, ok := opencodeDoc["permission"].(map[string]any)
	if !ok {
		t.Fatalf("permission doc = %T, want object", opencodeDoc["permission"])
	}
	if edit, _ := permissionDoc["edit"].(string); edit != "allow" {
		t.Fatalf("OpenCode permission.edit = %q, want allow", edit)
	}
	if body := readFileText(t, opencodeExamplePath); !strings.Contains(body, "v2") {
		t.Fatalf("updated OpenCode example.js missing v2 marker:\n%s", body)
	}
	if _, err := os.Stat(opencodeStalePath); !os.IsNotExist(err) {
		t.Fatalf("stale OpenCode plugin file should be removed after update: %v", err)
	}

	codexPluginManifest = readFileText(t, codexPluginManifestPath)
	for _, want := range []string{`"version": "0.2.0"`, "Run the updated lifecycle checks"} {
		if !strings.Contains(codexPluginManifest, want) {
			t.Fatalf("updated Codex plugin manifest missing %q:\n%s", want, codexPluginManifest)
		}
	}
	assertStringSet(t, "Codex MCP aliases after update", codexMCPAliases(t, codexMCPPath), []string{"release-checks-v2"})
	assertStringSet(t, "Codex marketplace entries after update", codexMarketplacePluginNames(t, codexCatalogPath), []string{"alpha", "lifecycle-demo"})

	beforeRemoveState := snapshotFile(t, statePath)
	beforeRemoveCursorConfig := snapshotFile(t, cursorConfigPath)
	beforeRemoveOpenCodeConfig := snapshotFile(t, opencodeConfigPath)
	beforeRemoveCodexCatalog := snapshotFile(t, codexCatalogPath)
	beforeRemoveCodexPluginManifest := snapshotFile(t, codexPluginManifestPath)
	beforeRemoveOpenCodeExample := snapshotFile(t, opencodeExamplePath)
	beforeRemoveOpenCodePackage := snapshotFile(t, opencodePackagePath)

	removeDryRunOutput := runIntegrationsLifecycleCommand(t, pluginKitAIBin, workspaceRoot, homeRoot,
		"integrations", "remove", "lifecycle-demo", "--dry-run=true")
	for _, want := range []string{
		`Dry-run remove_orphaned_target plan for "lifecycle-demo".`,
		"✅ Already present: cursor, opencode",
		"codex - will remove Codex plugin",
		"✅ cursor - will remove Cursor MCP setup",
		"✅ opencode - will remove OpenCode plugin",
	} {
		if !strings.Contains(removeDryRunOutput, want) {
			t.Fatalf("remove dry-run output missing %q:\n%s", want, removeDryRunOutput)
		}
	}

	assertNoOperationRecord(t, homeRoot, operationIDFromOutput(t, removeDryRunOutput))
	assertFileSnapshot(t, "state after remove dry-run", statePath, beforeRemoveState)
	assertFileSnapshot(t, "Cursor config after remove dry-run", cursorConfigPath, beforeRemoveCursorConfig)
	assertFileSnapshot(t, "OpenCode config after remove dry-run", opencodeConfigPath, beforeRemoveOpenCodeConfig)
	assertFileSnapshot(t, "Codex catalog after remove dry-run", codexCatalogPath, beforeRemoveCodexCatalog)
	assertFileSnapshot(t, "Codex plugin manifest after remove dry-run", codexPluginManifestPath, beforeRemoveCodexPluginManifest)
	assertFileSnapshot(t, "OpenCode example.js after remove dry-run", opencodeExamplePath, beforeRemoveOpenCodeExample)
	assertFileSnapshot(t, "OpenCode package.json after remove dry-run", opencodePackagePath, beforeRemoveOpenCodePackage)

	// Exact removal accounting across agents (Codex/Cursor/OpenCode): every
	// deleted path in the workspace must be plugin-owned, proven by a full
	// file-inventory diff around the real (non-dry-run) remove.
	removeInventoryBefore := nativeFileInventory(t, workspaceRoot)
	removeOutput := runIntegrationsLifecycleCommand(t, pluginKitAIBin, workspaceRoot, homeRoot,
		"integrations", "remove", "lifecycle-demo", "--dry-run=false")
	for _, want := range []string{
		`Removed managed targets from integration "lifecycle-demo".`,
		"🚀 Ready now: cursor",
		"🔄 Restart or reload: opencode",
		"codex - removed Codex plugin",
		"✅ cursor - removed Cursor MCP setup",
		"🟡 opencode - removed OpenCode plugin",
	} {
		if !strings.Contains(removeOutput, want) {
			t.Fatalf("remove output missing %q:\n%s", want, removeOutput)
		}
	}

	removeOperationID := operationIDFromOutput(t, removeOutput)
	assertCommittedOperation(t, readOperationRecord(t, homeRoot, removeOperationID), removeOperationID, "remove", "lifecycle-demo", []string{"codex", "cursor", "opencode"})

	state = readIntegrationLifecycleState(t, homeRoot)
	if len(state.Installations) != 0 {
		t.Fatalf("installations = %+v, want none after remove", state.Installations)
	}
	assertStringSet(t, "Cursor aliases after remove", cursorAliases(t, cursorConfigPath), []string{"user-owned"})
	assertStringSet(t, "OpenCode plugins after remove", openCodePluginRefs(t, opencodeConfigPath), []string{"@user/existing"})
	assertStringSet(t, "OpenCode MCP aliases after remove", openCodeMCPAliases(t, opencodeConfigPath), []string{"user-owned"})
	opencodeDoc = readJSONDoc(t, opencodeConfigPath)
	if _, ok := opencodeDoc["permission"]; ok {
		t.Fatalf("permission should be removed after remove:\n%v", opencodeDoc)
	}
	if _, ok := opencodeDoc["default_agent"]; ok {
		t.Fatalf("default_agent should be removed after remove:\n%v", opencodeDoc)
	}
	if _, err := os.Stat(opencodeExamplePath); !os.IsNotExist(err) {
		t.Fatalf("OpenCode example.js should be removed after remove: %v", err)
	}
	if _, err := os.Stat(opencodeStalePath); !os.IsNotExist(err) {
		t.Fatalf("OpenCode stale.js should be removed after remove: %v", err)
	}
	if _, err := os.Stat(opencodePackagePath); !os.IsNotExist(err) {
		t.Fatalf("OpenCode package.json should be removed after remove: %v", err)
	}
	if _, err := os.Stat(codexPluginRoot); !os.IsNotExist(err) {
		t.Fatalf("Codex plugin root should be removed after remove: %v", err)
	}
	assertStringSet(t, "Codex marketplace entries after remove", codexMarketplacePluginNames(t, codexCatalogPath), []string{"alpha"})

	removedPaths, addedPaths, changedPaths := nativeInventoryDiff(removeInventoryBefore, nativeFileInventory(t, workspaceRoot))
	for _, path := range removedPaths {
		owned := strings.HasPrefix(path, codexPluginRoot+string(os.PathSeparator)) ||
			path == opencodeExamplePath || path == opencodeStalePath || path == opencodePackagePath ||
			strings.Contains(path, "lifecycle-demo")
		if !owned {
			t.Fatalf("remove deleted a file outside the plugin's owned scope: %s (removed=%v)", path, removedPaths)
		}
	}
	t.Logf("exact removal inventory across agents: removed=%d all plugin-owned, added=%d, changed=%d", len(removedPaths), len(addedPaths), len(changedPaths))
}

func seedIntegrationLifecycleWorkspace(t *testing.T, workspaceRoot string) {
	t.Helper()
	mustWriteIntegrationFile(t, filepath.Join(workspaceRoot, ".git", "keep"), "")
	mustWriteIntegrationFile(t, filepath.Join(workspaceRoot, "docs", "generated", "integrationctl_evidence_registry.json"), `{
  "schema_version": 1,
  "entries": [
    {"key":"target.cursor.native_surface","claim":"Cursor native surface","evidence_class":"confirmed_vendor_fact","urls":["https://example.com/cursor"]},
    {"key":"target.codex.native_surface","claim":"Codex native surface","evidence_class":"confirmed_vendor_fact","urls":["https://example.com/codex"]},
    {"key":"target.opencode.native_surface","claim":"OpenCode native surface","evidence_class":"confirmed_vendor_fact","urls":["https://example.com/opencode"]}
  ]
}
`)
	mustWriteIntegrationFile(t, filepath.Join(workspaceRoot, ".cursor", "mcp.json"), `{
  "mcpServers": {
    "user-owned": {
      "command": "node",
      "args": ["user.mjs"]
    }
  }
}
`)
	mustWriteIntegrationFile(t, filepath.Join(workspaceRoot, "opencode.json"), `{
  "theme": "midnight",
  "plugin": ["@user/existing"],
  "mcp": {
    "user-owned": {
      "type": "local",
      "command": ["node", "user.js"]
    }
  }
}
`)
	mustWriteIntegrationFile(t, filepath.Join(workspaceRoot, ".agents", "plugins", "marketplace.json"), `{
  "name": "local-repo",
  "plugins": [
    {
      "name": "alpha",
      "source": {"source": "local", "path": "./plugins/alpha"},
      "policy": {"installation": "AVAILABLE", "authentication": "ON_INSTALL"},
      "category": "Productivity"
    }
  ]
}
`)
}

func writeIntegrationLifecycleSource(t *testing.T, sourceRoot string, version integrationsLifecycleVersion) {
	t.Helper()
	if err := os.RemoveAll(sourceRoot); err != nil {
		t.Fatalf("remove source root: %v", err)
	}
	pluginYAML := fmt.Sprintf(`api_version: v1
name: lifecycle-demo
version: %s
description: Lifecycle integration e2e fixture
targets:
  - cursor
  - codex-package
  - opencode
`, version.Version)
	mcpYAML := fmt.Sprintf(`api_version: v1
servers:
  %s:
    type: stdio
    targets:
      - cursor
      - codex-package
      - opencode
    stdio:
      command: node
      args:
        - ${package.root}/bin/%s.mjs
`, version.CursorAlias, version.CursorAlias)
	codexPackageYAML := "homepage: https://example.com/lifecycle-demo\nauthor:\n  name: Example\nkeywords:\n  - lifecycle\n"
	codexInterfaceJSON := fmt.Sprintf("{\n  \"defaultPrompt\": [\n    %q\n  ]\n}\n", version.CodexPrompt)
	opencodePackageYAML := fmt.Sprintf("plugins:\n  - %q\n", version.OpenCodePluginRef)
	opencodePackageJSON := fmt.Sprintf("{\n  \"name\": \"lifecycle-demo-opencode\",\n  \"version\": %q\n}\n", version.Version)

	mustWriteIntegrationFile(t, filepath.Join(sourceRoot, "plugin", "plugin.yaml"), pluginYAML)
	mustWriteIntegrationFile(t, filepath.Join(sourceRoot, "plugin", "mcp", "servers.yaml"), mcpYAML)
	mustWriteIntegrationFile(t, filepath.Join(sourceRoot, "plugin", "targets", "codex-package", "package.yaml"), codexPackageYAML)
	mustWriteIntegrationFile(t, filepath.Join(sourceRoot, "plugin", "targets", "codex-package", "interface.json"), codexInterfaceJSON)
	mustWriteIntegrationFile(t, filepath.Join(sourceRoot, "plugin", "targets", "opencode", "package.yaml"), opencodePackageYAML)
	mustWriteIntegrationFile(t, filepath.Join(sourceRoot, "plugin", "targets", "opencode", "permission.json"), version.OpenCodePermission+"\n")
	mustWriteIntegrationFile(t, filepath.Join(sourceRoot, "plugin", "targets", "opencode", "package.json"), opencodePackageJSON)
	mustWriteIntegrationFile(t, filepath.Join(sourceRoot, "plugin", "targets", "opencode", "plugins", "example.js"), version.OpenCodeExampleBody)
	if version.IncludeDefaultAgent {
		mustWriteIntegrationFile(t, filepath.Join(sourceRoot, "plugin", "targets", "opencode", "default_agent.txt"), "reviewer\n")
	}
	if version.IncludeStalePlugin {
		mustWriteIntegrationFile(t, filepath.Join(sourceRoot, "plugin", "targets", "opencode", "plugins", "stale.js"), "export const StalePlugin = true\n")
	}
}

func runIntegrationsLifecycleCommand(t *testing.T, pluginKitAIBin, workspaceRoot, homeRoot string, args ...string) string {
	t.Helper()
	cmd := exec.Command(pluginKitAIBin, args...)
	cmd.Dir = workspaceRoot
	cmd.Env = append(os.Environ(),
		"HOME="+homeRoot,
		"USERPROFILE="+homeRoot,
		"XDG_CONFIG_HOME="+filepath.Join(homeRoot, ".config"),
		"XDG_DATA_HOME="+filepath.Join(homeRoot, ".local", "share"),
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

func readIntegrationLifecycleState(t *testing.T, homeRoot string) ports.StateFile {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(homeRoot, ".plugin-kit-ai", "state.json"))
	if err != nil {
		t.Fatalf("read state file: %v", err)
	}
	var state ports.StateFile
	if err := json.Unmarshal(body, &state); err != nil {
		t.Fatalf("unmarshal state file: %v", err)
	}
	return state
}

func requireManagedIntegration(t *testing.T, state ports.StateFile, integrationID string) domain.InstallationRecord {
	t.Helper()
	for _, item := range state.Installations {
		if item.IntegrationID == integrationID {
			return item
		}
	}
	t.Fatalf("managed integration %q not found in state: %+v", integrationID, state.Installations)
	return domain.InstallationRecord{}
}

func mustWriteIntegrationFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func readFileText(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(body)
}

func snapshotFile(t *testing.T, path string) fileSnapshot {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fileSnapshot{}
		}
		t.Fatalf("read snapshot %s: %v", path, err)
	}
	return fileSnapshot{Exists: true, Body: string(body)}
}

func assertFileSnapshot(t *testing.T, label, path string, want fileSnapshot) {
	t.Helper()
	got := snapshotFile(t, path)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s changed:\ngot:  %#v\nwant: %#v", label, got, want)
	}
}

func assertPathAbsent(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected path %s to be absent, err=%v", path, err)
	}
}

func readJSONDoc(t *testing.T, path string) map[string]any {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read json %s: %v", path, err)
	}
	var doc map[string]any
	if err := json.Unmarshal(body, &doc); err != nil {
		normalized := trailingCommaRE.ReplaceAll(body, []byte("$1"))
		if retryErr := json.Unmarshal(normalized, &doc); retryErr != nil {
			t.Fatalf("parse json %s: %v\n%s", path, retryErr, body)
		}
	}
	return doc
}

func cursorAliases(t *testing.T, path string) []string {
	t.Helper()
	doc := readJSONDoc(t, path)
	servers, ok := doc["mcpServers"].(map[string]any)
	if !ok {
		servers = doc
	}
	out := make([]string, 0, len(servers))
	for alias := range servers {
		out = append(out, alias)
	}
	sort.Strings(out)
	return out
}

func openCodePluginRefs(t *testing.T, path string) []string {
	t.Helper()
	doc := readJSONDoc(t, path)
	items, ok := doc["plugin"].([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		switch typed := item.(type) {
		case string:
			out = append(out, typed)
		case []any:
			if len(typed) > 0 {
				if name, ok := typed[0].(string); ok && strings.TrimSpace(name) != "" {
					out = append(out, name)
				}
			}
		}
	}
	sort.Strings(out)
	return out
}

func openCodeMCPAliases(t *testing.T, path string) []string {
	t.Helper()
	doc := readJSONDoc(t, path)
	mcp, ok := doc["mcp"].(map[string]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(mcp))
	for alias := range mcp {
		out = append(out, alias)
	}
	sort.Strings(out)
	return out
}

func codexMarketplacePluginNames(t *testing.T, path string) []string {
	t.Helper()
	doc := readJSONDoc(t, path)
	plugins, ok := doc["plugins"].([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(plugins))
	for _, item := range plugins {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if name, ok := entry["name"].(string); ok && strings.TrimSpace(name) != "" {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

func codexMCPAliases(t *testing.T, path string) []string {
	t.Helper()
	doc := readJSONDoc(t, path)
	out := make([]string, 0, len(doc))
	for alias := range doc {
		out = append(out, alias)
	}
	sort.Strings(out)
	return out
}

func operationIDFromOutput(t *testing.T, output string) string {
	t.Helper()
	matches := operationLineRE.FindStringSubmatch(output)
	if len(matches) != 2 {
		t.Fatalf("missing operation id in output:\n%s", output)
	}
	return matches[1]
}

func readOperationRecord(t *testing.T, homeRoot, operationID string) domain.OperationRecord {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(homeRoot, ".plugin-kit-ai", "operations", operationID+".json"))
	if err != nil {
		t.Fatalf("read operation %s: %v", operationID, err)
	}
	var record domain.OperationRecord
	if err := json.Unmarshal(body, &record); err != nil {
		t.Fatalf("unmarshal operation %s: %v", operationID, err)
	}
	return record
}

func assertNoOperationRecord(t *testing.T, homeRoot, operationID string) {
	t.Helper()
	assertPathAbsent(t, filepath.Join(homeRoot, ".plugin-kit-ai", "operations", operationID+".json"))
}

func assertCommittedOperation(t *testing.T, record domain.OperationRecord, wantOperationID, wantType, wantIntegrationID string, orderedTargets []string) {
	t.Helper()
	if record.OperationID != wantOperationID {
		t.Fatalf("operation id = %q, want %q", record.OperationID, wantOperationID)
	}
	if record.Type != wantType {
		t.Fatalf("operation type = %q, want %q", record.Type, wantType)
	}
	if record.IntegrationID != wantIntegrationID {
		t.Fatalf("operation integration = %q, want %q", record.IntegrationID, wantIntegrationID)
	}
	if record.Status != "committed" {
		t.Fatalf("operation status = %q, want committed", record.Status)
	}
	if strings.TrimSpace(record.StartedAt) == "" {
		t.Fatalf("expected StartedAt in operation record: %+v", record)
	}

	wantSteps := make([]domain.JournalStep, 0, len(orderedTargets)*4+1)
	for _, target := range orderedTargets {
		for _, action := range []string{"inspect", "plan", "apply", "verify"} {
			wantSteps = append(wantSteps, domain.JournalStep{Target: target, Action: action, Status: "ok"})
		}
	}
	wantSteps = append(wantSteps, domain.JournalStep{Target: "state", Action: "persist_state", Status: "ok"})
	if !reflect.DeepEqual(record.Steps, wantSteps) {
		t.Fatalf("operation steps = %#v, want %#v", record.Steps, wantSteps)
	}
}

func assertDigestLike(t *testing.T, label, value string) {
	t.Helper()
	if !strings.HasPrefix(value, "sha256:") || len(value) <= len("sha256:") {
		t.Fatalf("%s = %q, want sha256 digest", label, value)
	}
}

func metadataString(meta map[string]any, key string) string {
	if meta == nil {
		return ""
	}
	value, _ := meta[key].(string)
	return strings.TrimSpace(value)
}

func metadataStringSlice(meta map[string]any, key string) []string {
	if meta == nil {
		return nil
	}
	raw, ok := meta[key]
	if !ok {
		return nil
	}
	var out []string
	switch typed := raw.(type) {
	case []string:
		out = append(out, typed...)
	case []any:
		for _, item := range typed {
			if value, ok := item.(string); ok && strings.TrimSpace(value) != "" {
				out = append(out, value)
			}
		}
	}
	sort.Strings(out)
	return out
}

func ownedKinds(target domain.TargetInstallation) []string {
	out := make([]string, 0, len(target.OwnedNativeObjects))
	seen := map[string]struct{}{}
	for _, item := range target.OwnedNativeObjects {
		if _, ok := seen[item.Kind]; ok {
			continue
		}
		seen[item.Kind] = struct{}{}
		out = append(out, item.Kind)
	}
	sort.Strings(out)
	return out
}

func ownedNamesForKind(target domain.TargetInstallation, kind string) []string {
	var out []string
	for _, item := range target.OwnedNativeObjects {
		if item.Kind == kind && strings.TrimSpace(item.Name) != "" {
			out = append(out, item.Name)
		}
	}
	sort.Strings(out)
	return out
}

func ownedPathsForKind(target domain.TargetInstallation, kind string) []string {
	var out []string
	for _, item := range target.OwnedNativeObjects {
		if item.Kind == kind && strings.TrimSpace(item.Path) != "" {
			out = append(out, filepath.Clean(item.Path))
		}
	}
	sort.Strings(out)
	return out
}

func assertPathSetEquivalent(t *testing.T, label string, got, want []string) {
	t.Helper()
	got = normalizeExistingPaths(t, got)
	want = normalizeExistingPaths(t, want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s = %#v, want %#v", label, got, want)
	}
}

func normalizeExistingPaths(t *testing.T, items []string) []string {
	t.Helper()
	out := make([]string, 0, len(items))
	for _, item := range items {
		resolved, err := filepath.EvalSymlinks(item)
		if err != nil {
			if os.IsNotExist(err) {
				out = append(out, filepath.Clean(item))
				continue
			}
			t.Fatalf("eval symlinks %s: %v", item, err)
		}
		out = append(out, filepath.Clean(resolved))
	}
	sort.Strings(out)
	return out
}

func assertStringSet(t *testing.T, label string, got, want []string) {
	t.Helper()
	got = append([]string(nil), got...)
	want = append([]string(nil), want...)
	sort.Strings(got)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s = %#v, want %#v", label, got, want)
	}
}

func sameExistingPath(t *testing.T, a, b string) bool {
	t.Helper()
	aResolved, err := filepath.EvalSymlinks(a)
	if err != nil {
		t.Fatalf("eval symlinks %s: %v", a, err)
	}
	bResolved, err := filepath.EvalSymlinks(b)
	if err != nil {
		t.Fatalf("eval symlinks %s: %v", b, err)
	}
	return filepath.Clean(aResolved) == filepath.Clean(bResolved)
}

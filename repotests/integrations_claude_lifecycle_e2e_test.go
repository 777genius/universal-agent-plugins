package pluginkitairepo_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
)

func TestPluginKitAIIntegrationsLifecycleClaudeProjectScope(t *testing.T) {
	pluginKitAIBin := buildPluginKitAI(t)
	workspaceRoot := t.TempDir()
	homeRoot := t.TempDir()
	sourceRoot := filepath.Join(workspaceRoot, "sources", "claude-demo")
	stateFile := filepath.Join(t.TempDir(), "claude-state.tsv")
	logFile := filepath.Join(t.TempDir(), "claude.log")
	marketplacesFile := filepath.Join(t.TempDir(), "claude-marketplaces.tsv")
	fakeBinDir := filepath.Join(t.TempDir(), "bin")
	settingsPath := filepath.Join(workspaceRoot, ".claude", "settings.json")
	statePath := filepath.Join(homeRoot, ".plugin-kit-ai", "state.json")
	materializedRoot := filepath.Join(homeRoot, ".plugin-kit-ai", "materialized", "claude", "claude-demo")
	marketplaceManifestPath := filepath.Join(materializedRoot, ".claude-plugin", "marketplace.json")
	pluginManifestPath := filepath.Join(materializedRoot, "plugins", "claude-demo", ".claude-plugin", "plugin.json")
	skillPath := filepath.Join(materializedRoot, "plugins", "claude-demo", "skills", "demo", "SKILL.md")

	seedClaudeLifecycleWorkspace(t, workspaceRoot)
	writeFakeClaudeCLI(t, fakeBinDir, stateFile, logFile, marketplacesFile, "user-review@user-marketplace")
	t.Setenv("PATH", fakeBinDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	// These ambient test-only values must never reach the read-only Claude probe.
	// The fake CLI uses the fixture paths embedded below so this remains a real
	// regression for the production environment allowlist.
	t.Setenv("PLUGIN_KIT_AI_FAKE_CLAUDE_STATE", "/attacker/state")
	t.Setenv("PLUGIN_KIT_AI_FAKE_CLAUDE_LOG", "/attacker/log")
	t.Setenv("PLUGIN_KIT_AI_FAKE_CLAUDE_MARKETPLACES", "/attacker/marketplaces")
	t.Setenv("PLUGIN_KIT_AI_FAKE_CLAUDE_PRESERVE_REFS", "attacker@marketplace")

	writeClaudeLifecycleSource(t, sourceRoot, "0.1.0", "Original review", "# Original\n")

	initialSettings := snapshotFile(t, settingsPath)
	addDryRunOutput := runIntegrationsLifecycleCommand(t, pluginKitAIBin, workspaceRoot, homeRoot,
		"integrations", "add", sourceRoot, "--scope", "project", "--dry-run=true")
	for _, want := range []string{
		`Dry-run plan for integration "claude-demo" at version 0.1.0.`,
		"⬇️ New install: claude",
		"⬇️ claude - will install Claude plugin",
	} {
		if !strings.Contains(addDryRunOutput, want) {
			t.Fatalf("add dry-run output missing %q:\n%s", want, addDryRunOutput)
		}
	}

	assertNoOperationRecord(t, homeRoot, operationIDFromOutput(t, addDryRunOutput))
	assertPathAbsent(t, statePath)
	assertPathAbsent(t, materializedRoot)
	assertFileSnapshot(t, "Claude settings after add dry-run", settingsPath, initialSettings)
	if refs := fakeClaudeInstalledRefs(t, stateFile); len(refs) != 0 {
		t.Fatalf("fake Claude installed refs after add dry-run = %#v, want empty", refs)
	}
	if marketplaces := fakeClaudeMarketplaces(t, marketplacesFile); len(marketplaces) != 0 {
		t.Fatalf("fake Claude marketplaces after add dry-run = %#v, want empty", marketplaces)
	}
	addDryRunLog := readFakeClaudeLog(t, logFile)
	assertExactLines(t, "fake Claude commands during add dry-run", addDryRunLog, []string{"plugin list --json"})

	addOutput := runIntegrationsLifecycleCommand(t, pluginKitAIBin, workspaceRoot, homeRoot,
		"integrations", "add", sourceRoot, "--scope", "project", "--dry-run=false")
	for _, want := range []string{
		`Installed integration "claude-demo" at version 0.1.0.`,
		"🚀 Ready now: claude",
		"✅ claude - installed Claude plugin",
	} {
		if !strings.Contains(addOutput, want) {
			t.Fatalf("add output missing %q:\n%s", want, addOutput)
		}
	}

	addOperationID := operationIDFromOutput(t, addOutput)
	assertCommittedOperation(t, readOperationRecord(t, homeRoot, addOperationID), addOperationID, "add", "claude-demo", []string{"claude"})

	state := readIntegrationLifecycleState(t, homeRoot)
	record := requireManagedIntegration(t, state, "claude-demo")
	assertDigestLike(t, "Claude source digest after add", record.SourceDigest)
	assertDigestLike(t, "Claude manifest digest after add", record.ManifestDigest)
	addSourceDigest := record.SourceDigest
	addManifestDigest := record.ManifestDigest
	if record.ResolvedVersion != "0.1.0" {
		t.Fatalf("resolved version = %q, want 0.1.0", record.ResolvedVersion)
	}
	target := record.Targets[domain.TargetClaude]
	if target.DeliveryKind != domain.DeliveryClaudeMarketplace {
		t.Fatalf("Claude delivery kind = %s, want %s", target.DeliveryKind, domain.DeliveryClaudeMarketplace)
	}
	if target.State != domain.InstallInstalled {
		t.Fatalf("Claude state after add = %s, want %s", target.State, domain.InstallInstalled)
	}
	assertStringSet(t, "Claude capability surface after add", target.CapabilitySurface, []string{"skills", "commands", "agents", "hooks", "mcp"})
	if got := metadataString(target.AdapterMetadata, "marketplace_name"); got != "integrationctl-claude-demo" {
		t.Fatalf("Claude marketplace_name = %q, want integrationctl-claude-demo", got)
	}
	if got := metadataString(target.AdapterMetadata, "plugin_ref"); got != "claude-demo@integrationctl-claude-demo" {
		t.Fatalf("Claude plugin_ref = %q, want claude-demo@integrationctl-claude-demo", got)
	}
	if got := metadataString(target.AdapterMetadata, "materialized_source_root"); got == "" || !sameExistingPath(t, got, materializedRoot) {
		t.Fatalf("Claude materialized_source_root = %q, want %q", got, materializedRoot)
	}
	assertStringSet(t, "Claude owned object kinds after add", ownedKinds(target), []string{"managed_marketplace_root", "settings_file"})
	assertStringSet(t, "Claude marketplace_add_argv after add", metadataStringSlice(target.AdapterMetadata, "marketplace_add_argv"), []string{"claude", "plugin", "marketplace", "add", materializedRoot})
	assertStringSet(t, "Claude plugin_install_argv after add", metadataStringSlice(target.AdapterMetadata, "plugin_install_argv"), []string{"claude", "plugin", "install", "claude-demo@integrationctl-claude-demo", "--scope", "project"})

	assertEnabledClaudePlugins(t, settingsPath, []string{"claude-demo@integrationctl-claude-demo", "user-review@user-marketplace"})

	pluginManifest := readFileText(t, pluginManifestPath)
	for _, want := range []string{`"version": "0.1.0"`, `"skills": "./skills/"`, `"agents": "./agents/"`, `"mcpServers": "./.mcp.json"`} {
		if !strings.Contains(pluginManifest, want) {
			t.Fatalf("Claude plugin manifest missing %q:\n%s", want, pluginManifest)
		}
	}
	marketplaceManifest := readFileText(t, marketplaceManifestPath)
	for _, want := range []string{`"name": "integrationctl-claude-demo"`, `"source": "./plugins/claude-demo"`, `"version": "0.1.0"`} {
		if !strings.Contains(marketplaceManifest, want) {
			t.Fatalf("Claude marketplace manifest missing %q:\n%s", want, marketplaceManifest)
		}
	}
	skillBody := readFileText(t, skillPath)
	if !strings.Contains(skillBody, "Original") {
		t.Fatalf("Claude skill body after add = %q", skillBody)
	}
	assertStringSet(t, "fake Claude installed refs after add", fakeClaudeInstalledRefs(t, stateFile), []string{"claude-demo@integrationctl-claude-demo|project"})
	assertStringSet(t, "fake Claude marketplaces after add", fakeClaudeMarketplaces(t, marketplacesFile), []string{"integrationctl-claude-demo|" + materializedRoot})

	addLog := readFakeClaudeLog(t, logFile)
	assertExactLines(t, "fake Claude commands after add", addLog[len(addDryRunLog):], []string{
		"plugin list --json",
		"plugin marketplace add " + materializedRoot,
		"plugin install claude-demo@integrationctl-claude-demo --scope project",
		"plugin list --json",
	})

	writeClaudeLifecycleSource(t, sourceRoot, "0.2.0", "Updated review", "# Updated\n")

	beforeUpdateState := snapshotFile(t, statePath)
	beforeUpdateSettings := snapshotFile(t, settingsPath)
	beforeUpdatePluginManifest := snapshotFile(t, pluginManifestPath)
	beforeUpdateMarketplaceManifest := snapshotFile(t, marketplaceManifestPath)
	beforeUpdateSkill := snapshotFile(t, skillPath)
	beforeUpdateMarketplaces := snapshotFile(t, marketplacesFile)

	updateDryRunOutput := runIntegrationsLifecycleCommand(t, pluginKitAIBin, workspaceRoot, homeRoot,
		"integrations", "update", "claude-demo", "--dry-run=true")
	for _, want := range []string{
		`Dry-run update_version plan for "claude-demo".`,
		"✅ Already present: claude",
		"✅ claude - will update Claude plugin",
	} {
		if !strings.Contains(updateDryRunOutput, want) {
			t.Fatalf("update dry-run output missing %q:\n%s", want, updateDryRunOutput)
		}
	}

	assertNoOperationRecord(t, homeRoot, operationIDFromOutput(t, updateDryRunOutput))
	assertFileSnapshot(t, "state after Claude update dry-run", statePath, beforeUpdateState)
	assertFileSnapshot(t, "Claude settings after update dry-run", settingsPath, beforeUpdateSettings)
	assertFileSnapshot(t, "Claude plugin manifest after update dry-run", pluginManifestPath, beforeUpdatePluginManifest)
	assertFileSnapshot(t, "Claude marketplace manifest after update dry-run", marketplaceManifestPath, beforeUpdateMarketplaceManifest)
	assertFileSnapshot(t, "Claude skill after update dry-run", skillPath, beforeUpdateSkill)
	assertFileSnapshot(t, "fake Claude marketplaces after update dry-run", marketplacesFile, beforeUpdateMarketplaces)
	assertStringSet(t, "fake Claude installed refs after update dry-run", fakeClaudeInstalledRefs(t, stateFile), []string{"claude-demo@integrationctl-claude-demo|project"})
	assertStringSet(t, "fake Claude marketplaces after update dry-run", fakeClaudeMarketplaces(t, marketplacesFile), []string{"integrationctl-claude-demo|" + materializedRoot})
	updateDryRunLog := readFakeClaudeLog(t, logFile)
	assertExactLines(t, "fake Claude commands during update dry-run", updateDryRunLog[len(addLog):], []string{"plugin list --json"})

	updateOutput := runIntegrationsLifecycleCommand(t, pluginKitAIBin, workspaceRoot, homeRoot,
		"integrations", "update", "claude-demo", "--dry-run=false")
	for _, want := range []string{
		`Updated integration "claude-demo".`,
		"🚀 Ready now: claude",
		"✅ claude - installed Claude plugin",
	} {
		if !strings.Contains(updateOutput, want) {
			t.Fatalf("update output missing %q:\n%s", want, updateOutput)
		}
	}

	updateOperationID := operationIDFromOutput(t, updateOutput)
	assertCommittedOperation(t, readOperationRecord(t, homeRoot, updateOperationID), updateOperationID, "update", "claude-demo", []string{"claude"})

	state = readIntegrationLifecycleState(t, homeRoot)
	record = requireManagedIntegration(t, state, "claude-demo")
	assertDigestLike(t, "Claude source digest after update", record.SourceDigest)
	assertDigestLike(t, "Claude manifest digest after update", record.ManifestDigest)
	if record.SourceDigest == addSourceDigest {
		t.Fatalf("Claude source digest did not change after update: %q", record.SourceDigest)
	}
	if record.ManifestDigest == addManifestDigest {
		t.Fatalf("Claude manifest digest did not change after update: %q", record.ManifestDigest)
	}
	if record.ResolvedVersion != "0.2.0" {
		t.Fatalf("resolved version = %q, want 0.2.0", record.ResolvedVersion)
	}
	target = record.Targets[domain.TargetClaude]
	if target.State != domain.InstallInstalled {
		t.Fatalf("Claude state after update = %s, want %s", target.State, domain.InstallInstalled)
	}
	if got := metadataString(target.AdapterMetadata, "marketplace_name"); got != "integrationctl-claude-demo" {
		t.Fatalf("Claude marketplace_name after update = %q, want integrationctl-claude-demo", got)
	}
	if got := metadataString(target.AdapterMetadata, "plugin_ref"); got != "claude-demo@integrationctl-claude-demo" {
		t.Fatalf("Claude plugin_ref after update = %q, want claude-demo@integrationctl-claude-demo", got)
	}
	if got := metadataString(target.AdapterMetadata, "materialized_source_root"); got == "" || !sameExistingPath(t, got, materializedRoot) {
		t.Fatalf("Claude materialized_source_root after update = %q, want %q", got, materializedRoot)
	}
	assertStringSet(t, "Claude marketplace_update_argv after update", metadataStringSlice(target.AdapterMetadata, "marketplace_update_argv"), []string{"claude", "plugin", "marketplace", "update", "integrationctl-claude-demo"})
	assertStringSet(t, "Claude plugin_uninstall_argv after update", metadataStringSlice(target.AdapterMetadata, "plugin_uninstall_argv"), []string{"claude", "plugin", "uninstall", "claude-demo@integrationctl-claude-demo", "--scope", "project"})
	assertStringSet(t, "Claude plugin_install_argv after update", metadataStringSlice(target.AdapterMetadata, "plugin_install_argv"), []string{"claude", "plugin", "install", "claude-demo@integrationctl-claude-demo", "--scope", "project"})

	pluginManifest = readFileText(t, pluginManifestPath)
	if !strings.Contains(pluginManifest, `"version": "0.2.0"`) {
		t.Fatalf("Claude plugin manifest after update:\n%s", pluginManifest)
	}
	marketplaceManifest = readFileText(t, marketplaceManifestPath)
	if !strings.Contains(marketplaceManifest, `"version": "0.2.0"`) {
		t.Fatalf("Claude marketplace manifest after update:\n%s", marketplaceManifest)
	}
	skillBody = readFileText(t, skillPath)
	if !strings.Contains(skillBody, "Updated") {
		t.Fatalf("Claude skill body after update = %q", skillBody)
	}
	assertEnabledClaudePlugins(t, settingsPath, []string{"claude-demo@integrationctl-claude-demo", "user-review@user-marketplace"})
	assertStringSet(t, "fake Claude installed refs after update", fakeClaudeInstalledRefs(t, stateFile), []string{"claude-demo@integrationctl-claude-demo|project"})
	assertStringSet(t, "fake Claude marketplaces after update", fakeClaudeMarketplaces(t, marketplacesFile), []string{"integrationctl-claude-demo|" + materializedRoot})

	updateLog := readFakeClaudeLog(t, logFile)
	assertExactLines(t, "fake Claude commands during update", updateLog[len(updateDryRunLog):], []string{
		"plugin list --json",
		"plugin marketplace update integrationctl-claude-demo",
		"plugin uninstall claude-demo@integrationctl-claude-demo --scope project",
		"plugin install claude-demo@integrationctl-claude-demo --scope project",
		"plugin list --json",
	})

	beforeRemoveState := snapshotFile(t, statePath)
	beforeRemoveSettings := snapshotFile(t, settingsPath)
	beforeRemovePluginManifest := snapshotFile(t, pluginManifestPath)
	beforeRemoveMarketplaceManifest := snapshotFile(t, marketplaceManifestPath)
	beforeRemoveSkill := snapshotFile(t, skillPath)
	beforeRemoveMarketplaces := snapshotFile(t, marketplacesFile)

	removeDryRunOutput := runIntegrationsLifecycleCommand(t, pluginKitAIBin, workspaceRoot, homeRoot,
		"integrations", "remove", "claude-demo", "--dry-run=true")
	for _, want := range []string{
		`Dry-run remove_orphaned_target plan for "claude-demo".`,
		"✅ Already present: claude",
		"✅ claude - will remove Claude plugin",
	} {
		if !strings.Contains(removeDryRunOutput, want) {
			t.Fatalf("remove dry-run output missing %q:\n%s", want, removeDryRunOutput)
		}
	}

	assertNoOperationRecord(t, homeRoot, operationIDFromOutput(t, removeDryRunOutput))
	assertFileSnapshot(t, "state after Claude remove dry-run", statePath, beforeRemoveState)
	assertFileSnapshot(t, "Claude settings after remove dry-run", settingsPath, beforeRemoveSettings)
	assertFileSnapshot(t, "Claude plugin manifest after remove dry-run", pluginManifestPath, beforeRemovePluginManifest)
	assertFileSnapshot(t, "Claude marketplace manifest after remove dry-run", marketplaceManifestPath, beforeRemoveMarketplaceManifest)
	assertFileSnapshot(t, "Claude skill after remove dry-run", skillPath, beforeRemoveSkill)
	assertFileSnapshot(t, "fake Claude marketplaces after remove dry-run", marketplacesFile, beforeRemoveMarketplaces)
	assertStringSet(t, "fake Claude installed refs after remove dry-run", fakeClaudeInstalledRefs(t, stateFile), []string{"claude-demo@integrationctl-claude-demo|project"})
	assertStringSet(t, "fake Claude marketplaces after remove dry-run", fakeClaudeMarketplaces(t, marketplacesFile), []string{"integrationctl-claude-demo|" + materializedRoot})
	removeDryRunLog := readFakeClaudeLog(t, logFile)
	assertExactLines(t, "fake Claude commands during remove dry-run", removeDryRunLog[len(updateLog):], []string{"plugin list --json"})

	removeOutput := runIntegrationsLifecycleCommand(t, pluginKitAIBin, workspaceRoot, homeRoot,
		"integrations", "remove", "claude-demo", "--dry-run=false")
	for _, want := range []string{
		`Removed managed targets from integration "claude-demo".`,
		"🚀 Ready now: claude",
		"✅ claude - removed Claude plugin",
	} {
		if !strings.Contains(removeOutput, want) {
			t.Fatalf("remove output missing %q:\n%s", want, removeOutput)
		}
	}

	removeOperationID := operationIDFromOutput(t, removeOutput)
	assertCommittedOperation(t, readOperationRecord(t, homeRoot, removeOperationID), removeOperationID, "remove", "claude-demo", []string{"claude"})

	state = readIntegrationLifecycleState(t, homeRoot)
	if len(state.Installations) != 0 {
		t.Fatalf("installations = %+v, want none after Claude remove", state.Installations)
	}
	if _, err := os.Stat(materializedRoot); !os.IsNotExist(err) {
		t.Fatalf("Claude materialized root should be removed: %v", err)
	}
	assertEnabledClaudePlugins(t, settingsPath, []string{"user-review@user-marketplace"})
	if refs := fakeClaudeInstalledRefs(t, stateFile); len(refs) != 0 {
		t.Fatalf("fake Claude installed refs after remove = %#v, want empty", refs)
	}
	if marketplaces := fakeClaudeMarketplaces(t, marketplacesFile); len(marketplaces) != 0 {
		t.Fatalf("fake Claude marketplaces after remove = %#v, want empty", marketplaces)
	}

	removeLog := readFakeClaudeLog(t, logFile)
	assertExactLines(t, "fake Claude commands during remove", removeLog[len(removeDryRunLog):], []string{
		"plugin list --json",
		"plugin uninstall claude-demo@integrationctl-claude-demo --scope project",
		"plugin marketplace remove integrationctl-claude-demo",
		"plugin list --json",
	})
}

func seedClaudeLifecycleWorkspace(t *testing.T, workspaceRoot string) {
	t.Helper()
	mustWriteIntegrationFile(t, filepath.Join(workspaceRoot, ".git", "keep"), "")
	mustWriteIntegrationFile(t, filepath.Join(workspaceRoot, "docs", "generated", "integrationctl_evidence_registry.json"), `{
  "schema_version": 1,
  "entries": [
    {"key":"target.claude.native_surface","claim":"Claude native surface","evidence_class":"confirmed_vendor_fact","urls":["https://example.com/claude"]}
  ]
}
`)
	mustWriteIntegrationFile(t, filepath.Join(workspaceRoot, ".claude", "settings.json"), `{
  "enabledPlugins": {
    "user-review@user-marketplace": true
  }
}
`)
}

func writeClaudeLifecycleSource(t *testing.T, sourceRoot, version, prompt, skillBody string) {
	t.Helper()
	if err := os.RemoveAll(sourceRoot); err != nil {
		t.Fatalf("remove Claude source root: %v", err)
	}
	mustWriteIntegrationFile(t, filepath.Join(sourceRoot, "plugin", "plugin.yaml"), `api_version: v1
name: claude-demo
version: `+version+`
description: Claude lifecycle integration e2e fixture
targets:
  - claude
`)
	mustWriteIntegrationFile(t, filepath.Join(sourceRoot, "plugin", "skills", "demo", "SKILL.md"), skillBody)
	mustWriteIntegrationFile(t, filepath.Join(sourceRoot, "plugin", "targets", "claude", "settings.json"), `{"agent":"reviewer"}`+"\n")
	mustWriteIntegrationFile(t, filepath.Join(sourceRoot, "plugin", "targets", "claude", "user-config.json"), `{"mode":"strict"}`+"\n")
	mustWriteIntegrationFile(t, filepath.Join(sourceRoot, "plugin", "targets", "claude", "commands", "review.md"), "# review\n")
	mustWriteIntegrationFile(t, filepath.Join(sourceRoot, "plugin", "targets", "claude", "agents", "reviewer.md"), "# reviewer\n")
	mustWriteIntegrationFile(t, filepath.Join(sourceRoot, "plugin", "targets", "claude", "hooks", "hooks.json"), "{\n  \"hooks\": {}\n}\n")
	mustWriteIntegrationFile(t, filepath.Join(sourceRoot, "plugin", "mcp", "servers.yaml"), `api_version: v1
servers:
  docs:
    type: remote
    targets:
      - claude
    remote:
      protocol: streamable_http
      url: "https://example.com/mcp"
`)
	mustWriteIntegrationFile(t, filepath.Join(sourceRoot, "plugin", "targets", "claude", "manifest.extra.json"), "{\n  \"interface\": {\"defaultPrompt\": ["+quoteJSON(prompt)+"]}\n}\n")
}

func writeFakeClaudeCLI(t *testing.T, binDir, stateFile, logFile, marketplacesFile, preserveRefs string) {
	t.Helper()
	for _, path := range []string{stateFile, logFile, marketplacesFile} {
		mustWriteIntegrationFile(t, path, "")
	}
	script := `#!/bin/sh
set -eu

state_file=` + quoteClaudeFakeShell(stateFile) + `
log_file=` + quoteClaudeFakeShell(logFile) + `
marketplaces_file=` + quoteClaudeFakeShell(marketplacesFile) + `
preserve_refs=` + quoteClaudeFakeShell(preserveRefs) + `

printf '%s\n' "$*" >> "$log_file"

fail() {
  echo "$*" >&2
  exit 1
}

settings_path_for_scope() {
  scope="$1"
  if [ "$scope" = "project" ]; then
    printf '%s/.claude/settings.json' "$PWD"
  else
    printf '%s/.claude/settings.json' "$HOME"
  fi
}

write_settings() {
  scope="$1"
  settings_path="$(settings_path_for_scope "$scope")"
  tmp="${settings_path}.tmp"
  mkdir -p "$(dirname "$settings_path")"
  {
    printf '{\n  "enabledPlugins": {'
    first=1

    old_ifs=$IFS
    IFS=','
    set -- $preserve_refs
    IFS=$old_ifs
    for ref in "$@"; do
      [ -n "$ref" ] || continue
      if [ "$first" -eq 0 ]; then
        printf ','
      fi
      printf '\n    "%s": true' "$ref"
      first=0
    done

    if [ -s "$state_file" ]; then
      while IFS='	' read -r id item_scope enabled; do
        [ -n "$id" ] || continue
        [ "$item_scope" = "$scope" ] || continue
        if [ "$first" -eq 0 ]; then
          printf ','
        fi
        printf '\n    "%s": true' "$id"
        first=0
      done < "$state_file"
    fi

    if [ "$first" -eq 0 ]; then
      printf '\n  }\n}\n'
    else
      printf '}\n}\n'
    fi
  } > "$tmp"
  mv "$tmp" "$settings_path"
}

add_state() {
  plugin_ref="$1"
  scope="$2"
  tmp="${state_file}.tmp"
  : > "$tmp"
  found=0
  if [ -s "$state_file" ]; then
    while IFS='	' read -r id item_scope enabled; do
      [ -n "$id" ] || continue
      if [ "$id" = "$plugin_ref" ] && [ "$item_scope" = "$scope" ]; then
        printf '%s\t%s\t1\n' "$plugin_ref" "$scope" >> "$tmp"
        found=1
      else
        printf '%s\t%s\t%s\n' "$id" "$item_scope" "$enabled" >> "$tmp"
      fi
    done < "$state_file"
  fi
  if [ "$found" -eq 0 ]; then
    printf '%s\t%s\t1\n' "$plugin_ref" "$scope" >> "$tmp"
  fi
  mv "$tmp" "$state_file"
  return 0
}

has_state() {
  plugin_ref="$1"
  scope="$2"
  if [ ! -s "$state_file" ]; then
    return 1
  fi
  while IFS='	' read -r id item_scope enabled; do
    [ -n "$id" ] || continue
    if [ "$id" = "$plugin_ref" ] && [ "$item_scope" = "$scope" ]; then
      return 0
    fi
  done < "$state_file"
  return 1
}

remove_state() {
  plugin_ref="$1"
  scope="$2"
  tmp="${state_file}.tmp"
  : > "$tmp"
  if [ -s "$state_file" ]; then
    while IFS='	' read -r id item_scope enabled; do
      [ -n "$id" ] || continue
      if [ "$id" = "$plugin_ref" ] && [ "$item_scope" = "$scope" ]; then
        continue
      fi
      printf '%s\t%s\t%s\n' "$id" "$item_scope" "$enabled" >> "$tmp"
    done < "$state_file"
  fi
  mv "$tmp" "$state_file"
}

print_list() {
  if [ ! -s "$state_file" ]; then
    printf '[]\n'
    return
  fi
  first=1
  printf '['
  while IFS='	' read -r id item_scope enabled; do
    [ -n "$id" ] || continue
    if [ "$first" -eq 0 ]; then
      printf ','
    fi
    printf '\n  {"id":"%s","scope":"%s","enabled":true}' "$id" "$item_scope"
    first=0
  done < "$state_file"
  if [ "$first" -eq 0 ]; then
    printf '\n'
  fi
  printf ']\n'
}

record_marketplace() {
  marketplace_name="$1"
  root="$2"
  tmp="${marketplaces_file}.tmp"
  : > "$tmp"
  found=0
  if [ -s "$marketplaces_file" ]; then
    while IFS='	' read -r name existing_root; do
      [ -n "$name" ] || continue
      if [ "$name" = "$marketplace_name" ]; then
        printf '%s\t%s\n' "$marketplace_name" "$root" >> "$tmp"
        found=1
      else
        printf '%s\t%s\n' "$name" "$existing_root" >> "$tmp"
      fi
    done < "$marketplaces_file"
  fi
  if [ "$found" -eq 0 ]; then
    printf '%s\t%s\n' "$marketplace_name" "$root" >> "$tmp"
  fi
  mv "$tmp" "$marketplaces_file"
  return 0
}

marketplace_root() {
  marketplace_name="$1"
  if [ ! -s "$marketplaces_file" ]; then
    return 1
  fi
  while IFS='	' read -r name root; do
    [ -n "$name" ] || continue
    if [ "$name" = "$marketplace_name" ]; then
      printf '%s' "$root"
      return 0
    fi
  done < "$marketplaces_file"
  return 1
}

drop_marketplace() {
  marketplace_name="$1"
  tmp="${marketplaces_file}.tmp"
  : > "$tmp"
  if [ -s "$marketplaces_file" ]; then
    while IFS='	' read -r name root; do
      [ -n "$name" ] || continue
      if [ "$name" = "$marketplace_name" ]; then
        continue
      fi
      printf '%s\t%s\n' "$name" "$root" >> "$tmp"
    done < "$marketplaces_file"
  fi
  mv "$tmp" "$marketplaces_file"
}

validate_marketplace_root() {
  root="$1"
  marketplace_json="$root/.claude-plugin/marketplace.json"
  plugin_json="$root/plugins/claude-demo/.claude-plugin/plugin.json"
  [ -f "$marketplace_json" ] || fail "missing marketplace manifest: $marketplace_json"
  [ -f "$plugin_json" ] || fail "missing plugin manifest: $plugin_json"
  grep -Eq '"name"[[:space:]]*:[[:space:]]*"integrationctl-claude-demo"' "$marketplace_json" || fail "invalid marketplace name in $marketplace_json"
  grep -Eq '"source"[[:space:]]*:[[:space:]]*"./plugins/claude-demo"' "$marketplace_json" || fail "invalid marketplace source in $marketplace_json"
  grep -Eq '"version"[[:space:]]*:[[:space:]]*"[^"]+"' "$marketplace_json" || fail "missing marketplace version in $marketplace_json"
  grep -Eq '"name"[[:space:]]*:[[:space:]]*"claude-demo"' "$plugin_json" || fail "invalid plugin name in $plugin_json"
  grep -Eq '"version"[[:space:]]*:[[:space:]]*"[^"]+"' "$plugin_json" || fail "missing plugin version in $plugin_json"
}

require_marketplace_root() {
  marketplace_name="$1"
  root="$(marketplace_root "$marketplace_name" || true)"
  [ -n "$root" ] || fail "missing marketplace state for $marketplace_name"
  validate_marketplace_root "$root"
  printf '%s' "$root"
}

validate_plugin_ref() {
  plugin_ref="$1"
  case "$plugin_ref" in
    claude-demo@integrationctl-claude-demo)
      root="$(require_marketplace_root integrationctl-claude-demo)"
      [ -f "$root/plugins/claude-demo/.claude-plugin/plugin.json" ] || fail "missing plugin manifest for $plugin_ref"
      ;;
    *)
      fail "unsupported fake claude plugin ref: $plugin_ref"
      ;;
  esac
}

plugin_scope() {
  scope="user"
  while [ "$#" -gt 0 ]; do
    case "$1" in
      --scope)
        scope="$2"
        shift 2
        ;;
      *)
        shift
        ;;
    esac
  done
  printf '%s' "$scope"
}

if [ "$#" -ge 4 ] && [ "$1" = "plugin" ] && [ "$2" = "marketplace" ] && [ "$3" = "add" ]; then
  root="$4"
  validate_marketplace_root "$root"
  record_marketplace "integrationctl-claude-demo" "$root"
  exit 0
fi
if [ "$#" -ge 4 ] && [ "$1" = "plugin" ] && [ "$2" = "marketplace" ] && [ "$3" = "update" ]; then
  marketplace_name="$4"
  root="$(require_marketplace_root "$marketplace_name")"
  record_marketplace "$marketplace_name" "$root"
  exit 0
fi
if [ "$#" -ge 4 ] && [ "$1" = "plugin" ] && [ "$2" = "marketplace" ] && [ "$3" = "remove" ]; then
  marketplace_name="$4"
  root="$(require_marketplace_root "$marketplace_name")"
  [ -n "$root" ] || fail "missing marketplace root for $marketplace_name"
  drop_marketplace "$marketplace_name"
  exit 0
fi
if [ "$#" -ge 3 ] && [ "$1" = "plugin" ] && [ "$2" = "install" ]; then
  plugin_ref="$3"
  scope="$(plugin_scope "$@")"
  validate_plugin_ref "$plugin_ref"
  add_state "$plugin_ref" "$scope"
  write_settings "$scope"
  exit 0
fi
if [ "$#" -ge 3 ] && [ "$1" = "plugin" ] && [ "$2" = "uninstall" ]; then
  plugin_ref="$3"
  scope="$(plugin_scope "$@")"
  has_state "$plugin_ref" "$scope" || fail "plugin is not installed: $plugin_ref ($scope)"
  remove_state "$plugin_ref" "$scope"
  write_settings "$scope"
  exit 0
fi
if [ "$#" -eq 3 ] && [ "$1" = "plugin" ] && [ "$2" = "list" ] && [ "$3" = "--json" ]; then
  print_list
  exit 0
fi

echo "unsupported fake claude command: $*" >&2
exit 1
`
	mustWriteIntegrationFile(t, filepath.Join(binDir, "claude"), script)
	if err := os.Chmod(filepath.Join(binDir, "claude"), 0o755); err != nil {
		t.Fatalf("chmod fake claude: %v", err)
	}
}

func quoteClaudeFakeShell(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func fakeClaudeInstalledRefs(t *testing.T, stateFile string) []string {
	t.Helper()
	body, err := os.ReadFile(stateFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("read fake Claude state: %v", err)
	}
	var out []string
	for _, line := range strings.Split(strings.TrimSpace(string(body)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 2 {
			t.Fatalf("invalid fake Claude state line %q", line)
		}
		out = append(out, parts[0]+"|"+parts[1])
	}
	sort.Strings(out)
	return out
}

func fakeClaudeMarketplaces(t *testing.T, marketplacesFile string) []string {
	t.Helper()
	body, err := os.ReadFile(marketplacesFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("read fake Claude marketplaces: %v", err)
	}
	var out []string
	for _, line := range strings.Split(strings.TrimSpace(string(body)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) != 2 {
			t.Fatalf("invalid fake Claude marketplace line %q", line)
		}
		out = append(out, parts[0]+"|"+filepath.Clean(parts[1]))
	}
	sort.Strings(out)
	return out
}

func readFakeClaudeLog(t *testing.T, logFile string) []string {
	t.Helper()
	body, err := os.ReadFile(logFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("read fake Claude log: %v", err)
	}
	var out []string
	for _, line := range strings.Split(strings.TrimSpace(string(body)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		out = append(out, line)
	}
	return out
}

func assertEnabledClaudePlugins(t *testing.T, settingsPath string, want []string) {
	t.Helper()
	settingsDoc := readJSONDoc(t, settingsPath)
	enabledPlugins, ok := settingsDoc["enabledPlugins"].(map[string]any)
	if !ok {
		t.Fatalf("settings enabledPlugins = %#v", settingsDoc["enabledPlugins"])
	}
	got := make([]string, 0, len(enabledPlugins))
	for ref := range enabledPlugins {
		got = append(got, ref)
	}
	assertStringSet(t, "Claude enabledPlugins", got, want)
}

func quoteJSON(value string) string {
	body, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return string(body)
}

func assertExactLines(t *testing.T, label string, got, want []string) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s = %#v, want %#v", label, got, want)
	}
}

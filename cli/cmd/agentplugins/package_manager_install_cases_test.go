//go:build uappm_e2e

package main

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/dependencymanifest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func pmInstallNoSaveLifecycle(t *testing.T) {
	o := newPMInstallOrigin(t)
	f := newPMInstallJourney(t, o)
	foreign := pmInstallStableTree(t, f.native("", "gemini"))
	delete(foreign, "settings.json")
	f.run(t, "", "add", o.url(), "--no-save", "--target", "gemini")
	installation, binding := f.assertInstalled(t, "", "gemini", "v1", false)
	if installation.Source.ResolvedRevision != "sha256:"+o.digest() {
		t.Fatalf("imperative installation recorded a different archive revision: %+v", installation.Source)
	}
	if len(f.state(t, "").EnvironmentReceipts) != 0 {
		t.Fatal("imperative add gained saved ownership")
	}
	projectJourneyAbsent(t, filepath.Join(f.cache, "uap.toml"), filepath.Join(f.cache, "uap.lock"))
	before := f.activeSnapshot(t, "")
	f.run(t, "", "add", o.url(), "--no-save", "--target", "gemini")
	pmInstallEqual(t, "imperative repeat", before, f.activeSnapshot(t, ""))
	f.assertInstalled(t, "", "gemini", "v1", false)
	for _, receipt := range installation.DataReceipts {
		projectJourneyWrite(t, filepath.Join(receipt.Locator, "keep.txt"), "imperative retained data\n")
	}
	f.run(t, "", "remove", installation.InstallationID, "--target", "gemini")
	projectJourneyAbsent(t, binding.TargetLocator)
	for _, retained := range f.state(t, "").Installations {
		if len(retained.Clients) != 0 {
			t.Fatalf("imperative removal retained client bindings: %+v", retained.Clients)
		}
	}
	remaining := pmInstallStableTree(t, f.native("", "gemini"))
	delete(remaining, "settings.json")
	pmInstallEqual(t, "foreign native paths after imperative removal", foreign, remaining)
	var config map[string]any
	if err := json.Unmarshal([]byte(projectJourneyRead(t, filepath.Join(f.native("", "gemini"), "settings.json"))), &config); err != nil {
		t.Fatal(err)
	}
	pmInstallEqual(t, "foreign native configuration members", map[string]any{"mcpServers": map[string]any{"foreign": map[string]any{"httpUrl": "https://foreign.test/mcp"}}, "foreignPreference": "keep"}, config)
	for _, receipt := range installation.DataReceipts {
		if projectJourneyRead(t, filepath.Join(receipt.Locator, "keep.txt")) != "imperative retained data\n" {
			t.Fatal("imperative removal lost data")
		}
	}
	projectJourneyAbsent(t, filepath.Join(f.cache, "uap.toml"), filepath.Join(f.cache, "uap.lock"))
	if o.count() == 0 || f.security.calls == 0 {
		t.Fatal("imperative journey did not acquire and assess its package")
	}
}

func pmInstallSelectedTargetOwnership(t *testing.T) {
	o := newPMInstallOrigin(t)
	f := newPMInstallJourney(t, o)
	f.run(t, f.a, "add", o.url(), "--alias", "rich", "--target", "cursor,gemini")
	pin := f.assertPin(t, f.a, "cursor", "gemini")
	f.assertInstalled(t, f.a, "cursor", "v1", false)
	installation, gemini := f.assertInstalled(t, f.a, "gemini", "v1", false)
	_, cursor := f.binding(t, f.a, "cursor")
	if len(f.state(t, f.a).EnvironmentReceipts) != 2 {
		t.Fatal("two target add lost environment ownership")
	}
	geminiNative := securityFixtureTree(t, f.native(f.a, "gemini"))
	geminiManaged := securityFixtureTree(t, gemini.TargetLocator)
	for _, receipt := range installation.DataReceipts {
		projectJourneyWrite(t, filepath.Join(receipt.Locator, "keep.txt"), "selected target retained data\n")
	}
	f.run(t, f.a, "remove", "rich", "--target", "cursor")
	projectJourneyAbsent(t, cursor.TargetLocator, filepath.Join(f.native(f.a, "cursor"), "skills", "docs"), filepath.Join(f.native(f.a, "cursor"), "skills", "audit"))
	pmInstallEqual(t, "remaining Gemini native objects", geminiNative, securityFixtureTree(t, f.native(f.a, "gemini")))
	pmInstallEqual(t, "remaining Gemini managed package", geminiManaged, securityFixtureTree(t, gemini.TargetLocator))
	after := f.assertPin(t, f.a, "gemini")
	if after.TreeDigest != pin.TreeDigest {
		t.Fatal("selected removal changed the remaining pin")
	}
	pair := f.pair(t, f.a)
	document, err := dependencymanifest.Decode([]byte(pair[0]), "uap.toml")
	if err != nil {
		t.Fatal(err)
	}
	requirements, err := document.Root()
	if err != nil {
		t.Fatal(err)
	}
	pmInstallEqual(t, "remaining target intent", map[string][]string{"gemini": {"rich"}}, requirements.Targets)
	if len(requirements.Dependencies) != 1 || requirements.Dependencies["rich"].Archive == nil || requirements.Dependencies["rich"].Archive.URL != o.url() {
		t.Fatalf("remaining dependency intent=%+v", requirements)
	}
	state := f.state(t, f.a)
	if len(state.EnvironmentReceipts) != 1 || state.EnvironmentReceipts[0].ClientID != "gemini" {
		t.Fatalf("remaining ownership=%+v", state.EnvironmentReceipts)
	}
	for _, retained := range state.Installations {
		for _, client := range retained.Clients {
			if client.ClientID == "cursor" {
				t.Fatalf("selected removal retained Cursor binding: %+v", client)
			}
		}
	}
	var cursorConfig map[string]any
	if err := json.Unmarshal([]byte(projectJourneyRead(t, filepath.Join(f.native(f.a, "cursor"), "mcp.json"))), &cursorConfig); err != nil {
		t.Fatal(err)
	}
	pmInstallEqual(t, "removed Cursor native MCP configuration", map[string]any{"mcpServers": map[string]any{"foreign": map[string]any{"url": "https://foreign.test/mcp", "disabled": true}}, "foreignPreference": "keep"}, cursorConfig)
	f.assertInstalled(t, f.a, "gemini", "v1", false)
	f.assertPin(t, f.a, "gemini")
	for _, receipt := range installation.DataReceipts {
		if projectJourneyRead(t, filepath.Join(receipt.Locator, "keep.txt")) != "selected target retained data\n" {
			t.Fatal("selected removal lost shared data")
		}
	}
}

func pmInstallComponentReplacement(t *testing.T) {
	o := newPMInstallOrigin(t)
	f := newPMInstallJourney(t, o)
	f.run(t, f.a, "add", o.url(), "--alias", "rich", "--target", "gemini")
	old, binding := f.assertInstalled(t, f.a, "gemini", "v1", false)
	oldPin := f.assertPin(t, f.a, "gemini")
	for _, receipt := range old.DataReceipts {
		projectJourneyWrite(t, filepath.Join(receipt.Locator, "keep.txt"), "component update retained data\n")
	}
	o.set(t, pmInstallFiles("v2", true))
	f.run(t, f.a, "update", "rich", "--target", "gemini")
	updated, selected := f.assertInstalled(t, f.a, "gemini", "v2", true)
	pin := f.assertPin(t, f.a, "gemini")
	if pin.TreeDigest == oldPin.TreeDigest || updated.Source.SourceBindingID != old.Source.SourceBindingID || selected.ClientBindingID != binding.ClientBindingID {
		t.Fatal("component update lost identity or did not advance")
	}
	for _, root := range []string{binding.TargetLocator, f.native(f.a, "gemini")} {
		projectJourneyAbsent(t, filepath.Join(root, "skills", "audit"))
	}
	for key, receipt := range old.DataReceipts {
		if updated.DataReceipts[key].Locator != receipt.Locator || projectJourneyRead(t, filepath.Join(receipt.Locator, "keep.txt")) != "component update retained data\n" {
			t.Fatal("component replacement changed persistent data")
		}
	}
}

func pmInstallForeignSkillCollision(t *testing.T) {
	o := newPMInstallOrigin(t)
	f := newPMInstallJourney(t, o)
	path := filepath.Join(f.native(f.a, "gemini"), "skills", "docs", "SKILL.md")
	projectJourneyWrite(t, path, "foreign same-name instructions\n")
	native := securityFixtureTree(t, f.native(f.a, "gemini"))
	out, err := f.execute(t, f.a, "", false, "add", o.url(), "--alias", "rich", "--target", "gemini", "--format", "json")
	if err == nil || !strings.Contains(err.Error(), "native identity is unmanaged") {
		t.Fatalf("foreign skill was not refused at native boundary: %v\n%s", err, out)
	}
	if o.count() == 0 || f.security.calls == 0 {
		t.Fatal("collision did not reach package acquisition/assessment")
	}
	pmInstallEqual(t, "foreign skill collision", native, securityFixtureTree(t, f.native(f.a, "gemini")))
	f.noActive(t, f.a)
	projectJourneyAbsent(t, filepath.Join(f.a, "uap.toml"), filepath.Join(f.a, "uap.lock"))
}

func pmInstallForeignDisabledMCPCollision(t *testing.T) {
	o := newPMInstallOrigin(t)
	f := newPMInstallJourney(t, o)
	config := filepath.Join(f.native(f.a, "cursor"), "mcp.json")
	projectJourneyWrite(t, config, `{"mcpServers":{"docs":{"url":"https://foreign.test/disabled","disabled":true},"foreign":{"url":"https://foreign.test/mcp","disabled":true}},"foreignPreference":"keep"}`+"\n")
	native := securityFixtureTree(t, f.native(f.a, "cursor"))
	out, err := f.execute(t, f.a, "", false, "add", o.url(), "--alias", "rich", "--target", "cursor", "--format", "json")
	if err == nil || !strings.Contains(err.Error(), "native identity is unmanaged") {
		t.Fatalf("disabled foreign MCP was not refused at its named boundary: %v\n%s", err, out)
	}
	if o.count() == 0 || f.security.calls == 0 {
		t.Fatal("MCP collision did not reach package acquisition/assessment")
	}
	pmInstallEqual(t, "foreign disabled MCP collision", native, securityFixtureTree(t, f.native(f.a, "cursor")))
	f.noActive(t, f.a)
	projectJourneyAbsent(t, filepath.Join(f.a, "uap.toml"), filepath.Join(f.a, "uap.lock"))
}

func pmInstallMalformedNativeConfig(t *testing.T) {
	o := newPMInstallOrigin(t)
	f := newPMInstallJourney(t, o)
	f.run(t, f.a, "add", o.url(), "--alias", "rich", "--target", "gemini")
	f.assertInstalled(t, f.a, "gemini", "v1", false)
	projectJourneyWrite(t, filepath.Join(f.native(f.a, "gemini"), "settings.json"), "{ invalid preserved user settings\n")
	before := f.activeSnapshot(t, f.a)
	o.set(t, pmInstallFiles("v2", false))
	requests, assessments := o.count(), f.security.calls
	out, err := f.execute(t, f.a, "", false, "update", "rich", "--target", "gemini", "--format", "json")
	if err == nil || (!strings.Contains(strings.ToLower(err.Error()), "json") && !strings.Contains(strings.ToLower(err.Error()), "invalid")) {
		t.Fatalf("malformed config did not refuse: %v\n%s", err, out)
	}
	if o.count() <= requests || f.security.calls <= assessments {
		t.Fatal("malformed-config update did not load and assess advanced source")
	}
	pmInstallEqual(t, "malformed native update refusal", before, f.activeSnapshot(t, f.a))
}

func pmInstallDriftRecordedPinRepair(t *testing.T) {
	o := newPMInstallOrigin(t)
	f := newPMInstallJourney(t, o)
	f.run(t, f.a, "add", o.url(), "--alias", "rich", "--target", "gemini")
	installation, binding := f.assertInstalled(t, f.a, "gemini", "v1", false)
	f.assertPin(t, f.a, "gemini")
	pair := f.pair(t, f.a)
	for _, receipt := range installation.DataReceipts {
		projectJourneyWrite(t, filepath.Join(receipt.Locator, "keep.txt"), "repair retained data\n")
	}
	projectJourneyWrite(t, filepath.Join(binding.TargetLocator, "skills", "docs", "SKILL.md"), "unreviewed managed drift\n")
	before := f.activeSnapshot(t, f.a)
	o.set(t, pmInstallFiles("v2", false))
	requests := o.count()
	out, err := f.execute(t, f.a, "", false, "deps", "sync", "--locked", "--yes", "--format", "json")
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "drift") {
		t.Fatalf("managed drift was not refused: %v\n%s", err, out)
	}
	pmInstallEqual(t, "managed drift refusal", before, f.activeSnapshot(t, f.a))
	f.run(t, f.a, "repair", installation.InstallationID, "--target", "gemini")
	repaired, _ := f.assertInstalled(t, f.a, "gemini", "v1", false)
	if repaired.Source.TreeDigest != installation.Source.TreeDigest || pair != f.pair(t, f.a) || o.count() != requests {
		t.Fatal("recorded-pin repair changed agreement or acquired advanced source")
	}
	for _, receipt := range installation.DataReceipts {
		if projectJourneyRead(t, filepath.Join(receipt.Locator, "keep.txt")) != "repair retained data\n" {
			t.Fatal("repair lost persistent data")
		}
	}
}

func pmInstallPreviewAndDecline(t *testing.T) {
	for _, mode := range []string{"preview", "decline"} {
		t.Run(mode, func(t *testing.T) {
			o := newPMInstallOrigin(t)
			f := newPMInstallJourney(t, o)
			beforeGemini, beforeCursor := securityFixtureTree(t, f.native(f.a, "gemini")), securityFixtureTree(t, f.native(f.a, "cursor"))
			args := []string{"add", o.url(), "--alias", "rich", "--target", "cursor,gemini"}
			input, terminal := "", false
			var pair [2]string
			if mode == "preview" {
				args = append(args, "--dry-run", "--format", "json")
			} else {
				projectJourneyWrite(t, filepath.Join(f.a, "uap.toml"), "schema_version = 1\n[dependencies.rich]\narchive = \""+o.url()+"\"\n[targets]\ncursor = [\"rich\"]\ngemini = [\"rich\"]\n")
				f.run(t, f.a, "deps", "lock")
				pair = f.pair(t, f.a)
				args = []string{"deps", "sync", "--locked"}
				input, terminal = "n\n", true
			}
			out, err := f.execute(t, f.a, input, terminal, args...)
			if mode == "preview" && err != nil {
				t.Fatalf("preview: %v\n%s", err, out)
			}
			if mode == "decline" && (err != nil || !strings.Contains(out, "Apply the preflighted dependency sync?") || !strings.Contains(out, "No changes made.")) {
				t.Fatalf("decline was not observed: %v\n%s", err, out)
			}
			for _, target := range []string{"cursor", "gemini"} {
				if !strings.Contains(out, target) {
					t.Fatalf("complete target plan missing %s: %s", target, out)
				}
			}
			if o.count() == 0 || f.security.calls == 0 {
				t.Fatal("preview/decline did not acquire and assess its source")
			}
			pmInstallEqual(t, "preview/decline Gemini", beforeGemini, securityFixtureTree(t, f.native(f.a, "gemini")))
			pmInstallEqual(t, "preview/decline Cursor", beforeCursor, securityFixtureTree(t, f.native(f.a, "cursor")))
			f.noActive(t, f.a)
			if mode == "preview" {
				projectJourneyAbsent(t, filepath.Join(f.a, "uap.toml"), filepath.Join(f.a, "uap.lock"))
			} else {
				pmInstallEqual(t, "declined agreement", pair, f.pair(t, f.a))
			}
		})
	}
}

func pmInstallRichColdRestoreIsolation(t *testing.T) {
	o := newPMInstallOrigin(t)
	f := newPMInstallJourney(t, o)
	for _, root := range []string{"", f.a, f.b} {
		f.run(t, root, "add", o.url(), "--alias", "rich", "--target", "gemini")
		f.assertInstalled(t, root, "gemini", "v1", false)
	}
	original, _ := f.binding(t, f.a, "gemini")
	pin := f.assertPin(t, f.a, "gemini")
	userBefore, bBefore := f.activeSnapshot(t, ""), f.activeSnapshot(t, f.b)
	exchange := projectJourneyTemp(t)
	f.run(t, f.a, "deps", "export", exchange, "--yes")
	cold := newPMInstallJourney(t, o)
	projectJourneyAbsent(t, cold.cache, filepath.Join(cold.a, ".uap"))
	requests := o.count()
	cold.run(t, cold.a, "deps", "import", exchange, "--install", "--yes")
	restored, binding := cold.assertInstalled(t, cold.a, "gemini", "v1", false)
	restoredPin := cold.assertPin(t, cold.a, "gemini")
	if o.count() <= requests || restored.Source.SourceBindingID != original.Source.SourceBindingID || restored.Source.TreeDigest != original.Source.TreeDigest || !reflect.DeepEqual(pin, restoredPin) || f.pair(t, f.a) != cold.pair(t, cold.a) {
		t.Fatal("cold import did not acquire and reproduce the exact portable package pin")
	}
	for _, receipt := range restored.DataReceipts {
		if strings.HasPrefix(receipt.Locator, f.home) || !strings.HasPrefix(receipt.Locator, filepath.Join(cold.a, ".uap", "plugin-data")) {
			t.Fatalf("cold native data reused source environment authority: %+v", receipt)
		}
	}
	if binding.Scope != string(domain.ScopeProject) {
		t.Fatal("cold import lost project scope")
	}
	pmInstallEqual(t, "source user environment", userBefore, f.activeSnapshot(t, ""))
	pmInstallEqual(t, "source project B", bBefore, f.activeSnapshot(t, f.b))
}

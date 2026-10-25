//go:build linux

package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestClineGateNativeMCPRequiresExactEnabledSourceRows(t *testing.T) {
	const path = "/tmp/project/.uap/clients/cline/mcp.json"
	row := func() map[string]any {
		return map[string]any{"name": "marker", "transportType": "streamableHttp", "disabled": false, "path": path}
	}
	for _, kind := range []string{"exact", "empty", "disabled", "wrong-source", "extra", "duplicate", "malformed", "trailing", "duplicate-key"} {
		t.Run(kind, func(t *testing.T) {
			r := row()
			rows := []map[string]any{r}
			switch kind {
			case "empty":
				rows = make([]map[string]any, 0, 4)
			case "disabled":
				r["disabled"] = true
			case "wrong-source":
				r["path"] = "/tmp/user/mcp.json"
			case "extra":
				r["unknown"] = true
			case "duplicate":
				rows = append(rows, row())
			case "malformed":
				r["disabled"] = "false"
			}
			body, err := json.Marshal(rows)
			if err != nil {
				t.Fatal(err)
			}
			if kind == "trailing" {
				body = append(body, []byte(" {}")...)
			}
			if kind == "duplicate-key" {
				body = []byte("[{\"name\":\"marker\",\"name\":\"marker\"}]")
			}
			err = clineGateMCP(body, path, []string{"marker"})
			if (err == nil) != (kind == "exact") {
				t.Fatalf("kind=%s err=%v", kind, err)
			}
		})
	}
}
func TestClineGateSkillPathsKeepDuplicatesAndRejectSiblingMarkers(t *testing.T) {
	const path = "/tmp/A/.cline/skills/marker/SKILL.md"
	row := func() map[string]any {
		return map[string]any{"name": "marker", "path": path, "description": "A", "instructions": "Inert A", "frontmatter": map[string]any{"disable-model-invocation": true}}
	}
	want := []clineGateSkill{{path: path, name: "marker", marker: "A"}}
	for _, kind := range []string{"exact", "empty", "wrong-path", "duplicate", "contradictory-duplicate", "sibling", "sibling-foreign"} {
		t.Run(kind, func(t *testing.T) {
			r := row()
			rows := []map[string]any{r}
			switch kind {
			case "empty":
				rows = make([]map[string]any, 0, 4)
			case "wrong-path":
				r["path"] = "/tmp/user/marker/SKILL.md"
			case "duplicate":
				rows = append(rows, row())
			case "contradictory-duplicate":
				extra := row()
				extra["name"] = "other"
				rows = append(rows, extra)
			case "sibling":
				extra := row()
				extra["path"] = "/tmp/B/.cline/skills/marker/SKILL.md"
				rows = append(rows, extra)
			case "sibling-foreign":
				extra := row()
				extra["name"] = "foreign-b"
				extra["path"] = "/tmp/B/.cline/skills/foreign-b/SKILL.md"
				rows = append(rows, extra)
			}
			body, err := json.Marshal(rows)
			if err != nil {
				t.Fatal(err)
			}
			err = clineGateSkills(body, want, []string{"/tmp/B/.cline/skills"})
			if (err == nil) != (kind == "exact") {
				t.Fatalf("kind=%s err=%v", kind, err)
			}
		})
	}
}
func TestClineGateChildArgumentsAndEnvironmentAreClosed(t *testing.T) {
	for _, args := range [][]string{{"--version"}, {"config", "skills", "--json"}, {"config", "mcp", "--json"}} {
		if !reflectClineArgs(args) {
			t.Fatal("reviewed command refused")
		}
	}
	for _, args := range [][]string{{"config"}, {"config", "tools", "--json"}, {"mcp", "list"}, {"--help"}, {"--version", "--extra"}, {"config", "mcp", "--json", "--extra"}} {
		if reflectClineArgs(args) {
			t.Fatal("unreviewed command accepted", args)
		}
	}
	f := &clineGate{t: t, root: "/tmp/fixture", home: "/tmp/fixture/home", A: "/tmp/fixture/A", B: "/tmp/fixture/B"}
	t.Setenv("CLINE_MCP_SETTINGS_PATH", "/foreign")
	t.Setenv("GITHUB_TOKEN", "inert")
	base := strings.Join(f.env(""), "\n")
	if strings.Contains(base, "GITHUB_TOKEN") || strings.Contains(base, "CLINE_MCP_SETTINGS_PATH") || !strings.Contains(base, "CLINE_NO_AUTO_UPDATE=1") {
		t.Fatal("ambient authority leaked")
	}
	if !strings.Contains(strings.Join(f.env(f.A), "\n"), "CLINE_MCP_SETTINGS_PATH="+f.projectMCP(f.A)) {
		t.Fatal("exact override missing")
	}
	for label := range clineGateExpectedUsed() {
		if clineGateClass(label) == "" {
			t.Fatal("finite table inconsistent", label)
		}
	}
	if clineGateClass("unreviewed") != "" || len(clineGateExpectedUsed()) != 38 {
		t.Fatal("finite table grew unexpectedly")
	}
}
func TestClineGateLiveBoundRetainsLimitAndCancelsBeforeOverflow(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	b := &clineGateBuffer{cancel: cancel}
	n, err := b.Write([]byte(strings.Repeat("x", clineGateStream+1)))
	body, overflow := b.snapshot()
	if err == nil || n != clineGateStream || len(body) != clineGateStream || !overflow || ctx.Err() == nil {
		t.Fatal("live limit not enforced")
	}
}
func TestClineGatePublicProgressIsExactAndNeverAnArbitraryWarning(t *testing.T) {
	cases := []struct{ label, expected string }{
		{"add-A", "Resolving and validating one Agent Plugin package for every selected target...\nInstalling uap-cline-a-package for 1 selected clients...\n"},
		{"repeat-add-A", "Resolving and validating one Agent Plugin package for every selected target...\nApplying transactional client package...\n"},
		{"update-A", "Resolving and validating one updated Agent Plugin package for every selected target...\n"},
		{"missing-repair-A", "Resolving and validating each unique exact installed package revision once...\n"}, {"remove-A", ""}}
	for _, item := range cases {
		if !clineGateAcceptProgress(item.label, []byte(item.expected)) {
			t.Fatal("source-established progress refused", item.label)
		}
		for _, bad := range []string{item.expected + "warning\n", item.expected + item.expected + "extra\n", "prefix" + item.expected, strings.ReplaceAll(item.expected, "1 selected", "2 selected")} {
			if bad != item.expected && clineGateAcceptProgress(item.label, []byte(bad)) {
				t.Fatal("arbitrary/changed progress accepted", item.label)
			}
		}
	}
	if clineGateAcceptProgress("unknown", nil) {
		t.Fatal("unknown progress admitted")
	}
}
func clineGateSourceCleanup(t *testing.T, f *clineGate) {
	t.Helper()
	t.Cleanup(func() {
		f.cancel()
		result := f.cleanup()
		if result.err != nil || result.exit != 0 || result.overflow {
			t.Errorf("owned source cleanup:%v", result.err)
		}
	})
}
func TestClineGateSourceUsesPublicCobraAndUneditedEnabledFiles(t *testing.T) {
	f := newClineGate(t)
	clineGateSourceCleanup(t, f)
	source := f.prepareUnique()
	before := f.snapshot(f.ownedRoots()...)
	nativeRoots := []string{filepath.Join(f.A, ".cline", "skills"), f.projectMCP(f.A)}
	nativeStable := f.snapshot(nativeRoots...)
	for _, step := range []struct{ label, verb, selector string }{{"repeat-add-A", "add", source}, {"unchanged-update-A", "update", "uap-cline-a-package"}, {"intact-repair-A", "repair", "uap-cline-a-package"}} {
		args := []string{step.verb, step.selector}
		if step.verb == "add" {
			args = append(args, "--no-save")
		}
		clineGateMutation(t, f.uap(step.label, f.A, args...), step.verb == "repair")
		if step.verb != "repair" {
			f.requireSnapshot(before, f.ownedRoots()...)
		} else {
			f.requireSnapshot(nativeStable, nativeRoots...)
		}
	}
	if f.runner.calls != 0 {
		t.Fatal("source stage called native/MCP")
	}
	for _, project := range []string{f.A, f.B} {
		f.assertPrepared(project, "")
	}
	if !strings.Contains(projectJourneyRead(t, f.projectMCP(f.A)), "http://127.0.0.1:9/A-ONE") {
		t.Fatal("emitted enabled marker changed")
	}
	if _, err := os.Stat(f.skill(f.A, "uap-cline-a")); err != nil {
		t.Fatal(err)
	}
}
func TestClineGatePublicSourceUpdateMissingRepairAndRemove(t *testing.T) {
	f := newClineGate(t)
	clineGateSourceCleanup(t, f)
	source := f.prepareUnique()
	roots := []string{filepath.Join(f.B, ".uap"), filepath.Join(f.B, ".cline"), filepath.Join(f.home, ".cline"), filepath.Join(f.cache, "state-v2.json")}
	other := f.snapshot(roots...)
	f.source("A", "uap-cline-a-package", "uap-cline-a", "A-TWO")
	clineGateMutation(t, f.uap("update-A", f.A, "update", source), true)
	if !strings.Contains(projectJourneyRead(t, f.skill(f.A, "uap-cline-a")), "A-TWO") {
		t.Fatal("public update missing")
	}
	if err := os.RemoveAll(filepath.Dir(f.skill(f.A, "uap-cline-a"))); err != nil {
		t.Fatal(err)
	}
	clineGateMutation(t, f.uap("missing-repair-A", f.A, "repair", "uap-cline-a-package"), true)
	if !strings.Contains(projectJourneyRead(t, f.skill(f.A, "uap-cline-a")), "A-TWO") {
		t.Fatal("public repair did not restore selected local bytes")
	}
	f.inspect("repaired-A-inspection.json", f.A)
	clineGateMutation(t, f.uap("remove-A", f.A, "remove", "uap-cline-a-package"), true)
	if _, err := os.Lstat(f.skill(f.A, "uap-cline-a")); !os.IsNotExist(err) {
		t.Fatal("owned skill retained after removal")
	}
	object := clineGateObject(t, []byte(projectJourneyRead(t, f.projectMCP(f.A))))
	servers, ok := object["mcpServers"].(map[string]any)
	if !ok {
		t.Fatal("remaining MCP object missing")
	}
	if _, ok := servers["uap-cline-a"]; ok {
		t.Fatal("owned MCP member retained")
	}
	f.foreign(f.A)
	f.requireSnapshot(other, roots...)
	if f.runner.calls != 0 {
		t.Fatal("source-only lifecycle launched native")
	}
}

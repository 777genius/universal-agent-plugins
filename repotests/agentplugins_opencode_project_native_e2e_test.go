package pluginkitairepo_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Current-source only. Debug observations never change the persisted Prepared
// lifecycle and never claim MCP initialization, model use or trust behavior.
func TestAgentpluginsOpenCodeNativeProjectComponentsLifecycle(t *testing.T) {
	if os.Getenv("AGENTPLUGINS_OPENCODE_PROJECT_NATIVE_E2E") != "1" {
		t.Skip("opt-in current-source project native discovery")
	}
	nativeRequireDisposable(t)
	f := newOpenCodeProjectFixture(t)
	defer f.finish(t)
	f.setup(t)
	f.run(t, "add-user", "", "add", f.sourceControl, "--no-save")
	added := f.run(t, "add-A", f.A, "add", f.sourceA, "--no-save")
	if err := commandsPlanSelection(added, "echo", "wizard"); err != nil {
		t.Fatal(err)
	}
	f.run(t, "add-B", f.B, "add", f.sourceB, "--no-save")
	nativeAttempt(f.stages, "positive_project_discovery", "opencode-project-evidence.json")
	f.observe(t, "initial-A", f.A, f.sourceA, "A-one", "echo", false)
	f.observe(t, "initial-B", f.B, f.sourceB, "B-one", "echo", false)
	f.binding(t, f.A)
	f.binding(t, f.B)
	f.stages["positive_project_discovery"] = nativeStage{Status: "passed", Reason: "fresh native config/skill JSON selects exact A/B managed marker and skill paths/bodies and retains distinct user control"}
	// Snapshot only original user ownership/config/state and B's installed state,
	// native files and package receipts; vendor caches are separate bootstrap effects.
	userRoots := []string{filepath.Join(f.work, "xdg-config", "opencode", "opencode.jsonc"), filepath.Join(f.work, "xdg-config", "opencode", "skills", "uap-user-control"), filepath.Join(f.state, "state-v2.json")}
	otherRoots := []string{filepath.Join(f.B, ".uap"), filepath.Join(f.B, ".opencode", "skills"), filepath.Join(f.B, "opencode.jsonc")}
	userBefore, otherBefore := f.snapshot(t, userRoots...), f.snapshot(t, otherRoots...)
	nativeAttempt(f.stages, "unchanged_lifecycle", "opencode-project-evidence.json")
	stable := f.snapshot(t, filepath.Join(f.A, ".uap", "state-v2.json"), filepath.Join(f.A, ".opencode", "skills"), filepath.Join(f.A, "opencode.jsonc"))
	for _, step := range []struct{ label, verb, selector string }{{"repeat-add-A", "add", f.sourceA}, {"unchanged-update-A", "update", commandsPluginName}, {"intact-repair-A", "repair", commandsPluginName}} {
		args := []string{step.verb, step.selector}
		if step.verb == "add" {
			args = append(args, "--no-save")
		}
		result := f.run(t, step.label, f.A, args...)
		commandsRequireMutated(t, step.label, result, false)
		if !reflect.DeepEqual(stable, f.snapshot(t, filepath.Join(f.A, ".uap", "state-v2.json"), filepath.Join(f.A, ".opencode", "skills"), filepath.Join(f.A, "opencode.jsonc"))) {
			t.Fatal("unchanged Prepared operation changed owned files/state")
		}
	}
	f.observe(t, "stable-A", f.A, f.sourceA, "A-one", "echo", false)
	f.stages["unchanged_lifecycle"] = nativeStage{Status: "passed", Reason: "repeat add, unchanged update and intact repair report mutated=false and preserve exact state/config/skills"}
	nativeAttempt(f.stages, "update", "opencode-project-evidence.json")
	commandsSwapInvocation(t, f.sourceA)
	path := filepath.Join(f.sourceA, "skills", commandsGuideSkill, "SKILL.md")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	nativeWrite(t, path, []byte(strings.ReplaceAll(string(body), "A-one", "A-two")), 0600)
	f.writeMCP(t, f.sourceA, "scope-marker", "A-two")
	updated := f.run(t, "update-A", f.A, "update", f.sourceA)
	commandsRequireMutated(t, "update", updated, true)
	if err := commandsPlanSelection(updated, "wizard", "echo"); err != nil {
		t.Fatal(err)
	}
	f.observe(t, "updated-A", f.A, f.sourceA, "A-two", "wizard", false)
	f.binding(t, f.A)
	f.stages["update"] = nativeStage{Status: "passed", Reason: "same-version update advances exact skill body, ordinary command selection and MCP marker; receipt stays Prepared"}
	nativeAttempt(f.stages, "missing_repair", "opencode-project-evidence.json")
	missing := filepath.Join(f.A, ".opencode", "skills", commandsGuideSkill)
	nativeRequireContained(t, f.A, missing)
	if err := os.RemoveAll(missing); err != nil {
		t.Fatal(err)
	}
	// Advance the mutable source after the installed pin. Repair must restore the
	// installed A-two bytes, not silently import this later uninstalled text.
	body, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	nativeWrite(t, path, append(body, []byte("UNINSTALLED-SOURCE-ADVANCE\n")...), 0600)
	repaired := f.run(t, "missing-repair-A", f.A, "repair", commandsPluginName)
	commandsRequireMutated(t, "missing repair", repaired, true)
	// Expected source bytes come from the last owned receipt's managed package.
	binding := f.binding(t, f.A)
	f.observe(t, "repaired-A", f.A, binding.TargetLocator, "A-two", "wizard", false)
	f.stages["missing_repair"] = nativeStage{Status: "passed", Reason: "missing owned skill restored from exact installed package despite later source advance; fresh native JSON proves pinned body/path"}
	nativeAttempt(f.stages, "remove", "opencode-project-evidence.json")
	f.run(t, "remove-A", f.A, "remove", commandsPluginName)
	for _, name := range []string{commandsGuideSkill, commandsSkill("echo"), commandsSkill("wizard")} {
		if _, err := os.Lstat(filepath.Join(f.A, ".opencode", "skills", name)); !os.IsNotExist(err) {
			t.Fatal("removed skill remains on disk")
		}
	}
	f.observe(t, "removed-A", f.A, "", "", "", true)
	afterRemove := f.snapshot(t, filepath.Join(f.A, ".uap"), filepath.Join(f.A, "opencode.jsonc"), filepath.Join(f.A, ".opencode", "skills"))
	f.run(t, "repeat-remove-A", f.A, "remove", commandsPluginName)
	if !reflect.DeepEqual(afterRemove, f.snapshot(t, filepath.Join(f.A, ".uap"), filepath.Join(f.A, "opencode.jsonc"), filepath.Join(f.A, ".opencode", "skills"))) {
		t.Fatal("repeat removal refusal changed owned state/files")
	}
	f.stages["remove"] = nativeStage{Status: "passed", Reason: "owned native keys/skills absent by fresh native JSON and filesystem; repeated removal gives recognized absent-installation refusal with exact retained bytes"}
	if !reflect.DeepEqual(userBefore, f.snapshot(t, userRoots...)) || !reflect.DeepEqual(otherBefore, f.snapshot(t, otherRoots...)) {
		t.Fatal("A lifecycle changed original user or B ownership/state")
	}
	for _, root := range []string{f.A, f.B} {
		body, err := os.ReadFile(filepath.Join(root, "opencode.jsonc"))
		if err != nil || !strings.Contains(string(body), openCodeJSONCComment) || !strings.Contains(string(body), "uap-inert-project") {
			t.Fatal("foreign JSONC preference/comment changed")
		}
		if string(nativeReadProject(t, filepath.Join(root, ".uap", "data", "caller-owned", "keep.txt"))) != "caller-owned persistent data\n" {
			t.Fatal("caller-owned data changed")
		}
	}
	f.keepJSON(t, "foreign-snapshots.json", map[string]any{"original_user": userBefore, "project_B": otherBefore})
	f.stages["foreign_preservation"] = nativeStage{Status: "passed", Reason: "exact original-user state/native control and B state/native/package bytes unchanged; foreign JSONC comments/settings and caller-owned data retained"}
	openCodeProjectSameName(t, f)
	openCodeProjectJSONCase(t, f)
	if len(f.used) != len(f.table) {
		t.Fatalf("finite command table incomplete: used%d want%d", len(f.used), len(f.table))
	}
	f.evidence["native_discovery_qualified"] = true
}
func nativeReadProject(t *testing.T, path string) []byte {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return body
}
func (f *openCodeProjectFixture) keepJSON(t *testing.T, name string, value any) {
	t.Helper()
	body, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	f.keep(t, name, append(body, '\n'))
}

func openCodeProjectSameName(t *testing.T, f *openCodeProjectFixture) {
	t.Helper()
	nativeAttempt(f.stages, "same_name_precedence", "opencode-project-evidence.json")
	f.run(t, "add-same-user", "", "add", f.sourceUser, "--no-save")
	f.run(t, "add-same-A", f.A, "add", f.sourceA, "--no-save")
	// The mutable source intentionally carries the later body in this secondary
	// phase. User/A/B receipts and files all survive regardless of native winner.
	snapshots := f.snapshot(t, filepath.Join(f.state, "state-v2.json"), filepath.Join(f.A, ".uap"), filepath.Join(f.B, ".uap"), filepath.Join(f.work, "xdg-config", "opencode", "skills"), filepath.Join(f.A, ".opencode", "skills"), filepath.Join(f.B, ".opencode", "skills"))
	winners := map[string]any{}
	known := map[string]map[string]openCodeProjectSkill{}
	known["user"] = f.expected(t, f.user, f.sourceUser, "echo")
	known["A"] = f.expected(t, f.A, f.sourceA, "wizard")
	known["B"] = f.expected(t, f.B, f.sourceB, "echo")
	for _, item := range []struct{ label, root string }{{"user", f.user}, {"A", f.A}, {"B", f.B}} {
		config := f.config(t, "same-"+item.label, item.root, nil)
		skills := f.skills(t, "same-"+item.label, item.root, true)
		guide, ok := skills[commandsGuideSkill]
		if !ok {
			t.Fatal("same-name phase has no valid winner")
		}
		origin := ""
		for label, expected := range known {
			want := expected[commandsGuideSkill]
			if openCodeProjectSamePath(guide.Location, want.Location) && strings.TrimSpace(guide.Content) == strings.TrimSpace(want.Content) {
				origin = label
			}
		}
		if origin == "" {
			t.Fatal("same-name winner is outside exact fixture paths/bodies")
		}
		mcp, ok := config["mcp"].(map[string]any)
		if !ok {
			t.Fatal("same-name config lacks MCP object")
		}
		server, ok := mcp["scope-marker"].(map[string]any)
		if !ok {
			t.Fatal("same-name config lacks marker")
		}
		url, ok := server["url"].(string)
		if !ok {
			t.Fatal("same-name marker is malformed")
		}
		allowed := false
		for _, marker := range []string{"USER-same", "A-two", "B-one"} {
			allowed = allowed || url == f.recorder.url(marker)
		}
		if !allowed {
			t.Fatal("same-name MCP winner outside fixture values")
		}
		winners[item.label] = map[string]any{"skill_origin": origin, "skill": guide, "mcp_url": url}
	}
	if f.stages["same_name_precedence"].Status != "blocked" {
		f.stages["same_name_precedence"] = nativeStage{Status: "blocked", Reason: "valid actual same-name winners retained; simultaneous user/A/B activation and general precedence remain unqualified"}
	}
	f.keepJSON(t, "same-name-winners.json", winners)
	nativeAttempt(f.stages, "inline_precedence", "opencode-project-evidence.json")
	inline, err := json.Marshal(map[string]any{"mcp": map[string]any{"scope-marker": map[string]any{"type": "remote", "url": f.recorder.url("INLINE"), "enabled": true}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := openCodeProjectCheckConfig(f.config(t, "inline-A", f.A, []string{"OPENCODE_CONFIG_CONTENT=" + string(inline)}), map[string]string{"scope-marker": f.recorder.url("INLINE")}, nil); err != nil {
		t.Fatal(err)
	}
	restarted := f.config(t, "restart-A", f.A, nil)
	if err := openCodeProjectCheckConfig(restarted, map[string]string{"scope-marker": f.recorder.url("A-two")}, nil); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(snapshots, f.snapshot(t, filepath.Join(f.state, "state-v2.json"), filepath.Join(f.A, ".uap"), filepath.Join(f.B, ".uap"), filepath.Join(f.work, "xdg-config", "opencode", "skills"), filepath.Join(f.A, ".opencode", "skills"), filepath.Join(f.B, ".opencode", "skills"))) {
		t.Fatal("secondary observation changed receipts/owned skill files")
	}
	f.stages["inline_precedence"] = nativeStage{Status: "passed", Reason: "fixed process-only MCP marker wins inline; fresh ordinary restart returns to selected project file, without owned receipt/skill mutation"}
}
func openCodeProjectJSONCase(t *testing.T, f *openCodeProjectFixture) {
	t.Helper()
	nativeAttempt(f.stages, "root_json", "opencode-project-evidence.json")
	f.run(t, "add-json", f.short, "add", f.sourceJSON, "--no-save")
	// Same-name user skills are present in this secondary case: config route and
	// exact UAP file ownership are mandatory; native winner limits are retained.
	if err := openCodeProjectCheckConfig(f.config(t, "json-installed", f.short, nil), map[string]string{"scope-marker": f.recorder.url("JSON-one")}, nil); err != nil {
		t.Fatal(err)
	}
	f.skills(t, "json-installed", f.short, true)
	f.binding(t, f.short)
	if _, err := os.Lstat(filepath.Join(f.short, "opencode.jsonc")); !os.IsNotExist(err) {
		t.Fatal("JSON route invented a JSONC sibling")
	}
	f.run(t, "remove-json", f.short, "remove", commandsPluginName)
	// User same-name components remain, so native lists must not be treated as a
	// filesystem deletion oracle. Exact project paths are separately absent.
	config := f.config(t, "json-removed", f.short, nil)
	if err := openCodeProjectCheckConfig(config, map[string]string{"scope-marker": f.recorder.url("USER-same")}, nil); err != nil {
		t.Fatal(err)
	}
	rows := f.skills(t, "json-removed", f.short, true)
	for _, row := range rows {
		if strings.HasPrefix(row.Location, f.short+string(os.PathSeparator)) {
			t.Fatal("removed JSON project skill remains natively listed")
		}
	}
	for _, name := range []string{commandsGuideSkill, commandsSkill("echo")} {
		if _, err := os.Lstat(filepath.Join(f.short, ".opencode", "skills", name)); !os.IsNotExist(err) {
			t.Fatal("JSON project skill survives removal")
		}
	}
	f.stages["root_json"] = nativeStage{Status: "passed", Reason: "actual UAP root JSON install/config observation/removal; exact project files absent, same-name user runtime winner not treated as project ownership"}
}

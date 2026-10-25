//go:build linux

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func (f *clineGate) source(id, plugin, skill, marker string) string {
	root := filepath.Join(f.root, "sources", id)
	manifest, _ := json.Marshal(map[string]any{"$schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json", "name": plugin, "version": "1.0.0"})
	mcp, _ := json.Marshal(map[string]any{"$schema": "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json", "mcpServers": map[string]any{skill: map[string]any{"type": "streamable-http", "url": "http://127.0.0.1:9/" + marker}}})
	projectJourneyWrite(f.t, filepath.Join(root, "plugin.json"), string(manifest))
	projectJourneyWrite(f.t, filepath.Join(root, "mcp.json"), string(mcp))
	projectJourneyWrite(f.t, filepath.Join(root, "skills", skill, "SKILL.md"), fmt.Sprintf("---\nname: %s\ndescription: %s\ndisable-model-invocation: true\n---\n%s\nInert text only. No command, model, hook or server execution.\n", skill, marker, marker))
	return root
}
func (f *clineGate) seedForeign() {
	for _, item := range []struct{ root, path, label string }{{f.home, f.userMCP(), "user"}, {f.A, f.projectMCP(f.A), "a"}, {f.B, f.projectMCP(f.B), "b"}} {
		body, _ := json.Marshal(map[string]any{"mcpServers": map[string]any{"foreign-" + item.label: map[string]any{"transport": map[string]any{"type": "streamableHttp", "url": "http://127.0.0.1:9/FOREIGN-" + item.label}}}, "foreignPreference": "retain-" + item.label})
		projectJourneyWrite(f.t, item.path, string(body))
		projectJourneyWrite(f.t, f.skill(item.root, "foreign-"+item.label), fmt.Sprintf("---\nname: foreign-%s\ndescription: Foreign fixture\ndisable-model-invocation: true\n---\nRetain exact unrelated inert bytes.\n", item.label))
	}
	for _, project := range []string{f.A, f.B} {
		projectJourneyWrite(f.t, filepath.Join(project, ".uap", "data", "caller-owned", "keep.txt"), "caller-owned fixture data\n")
	}
}
func (f *clineGate) foreign(project string) {
	label, path, skillRoot := "a", f.projectMCP(project), project
	if project == "" {
		label, path, skillRoot = "user", f.userMCP(), f.home
	} else if project == f.B {
		label = "b"
	} else if project != f.A {
		f.t.Fatal("foreign preservation outside selected scope")
	}
	body := clineGateObject(f.t, []byte(projectJourneyRead(f.t, path)))
	servers, ok := body["mcpServers"].(map[string]any)
	entry, eok := servers["foreign-"+label].(map[string]any)
	transport, tok := entry["transport"].(map[string]any)
	if !ok || !eok || !tok || body["foreignPreference"] != "retain-"+label || transport["url"] != "http://127.0.0.1:9/FOREIGN-"+label || transport["type"] != "streamableHttp" || len(entry) != 1 || len(transport) != 2 {
		f.t.Fatal("unrelated MCP preference/member changed")
	}
	expected := fmt.Sprintf("---\nname: foreign-%s\ndescription: Foreign fixture\ndisable-model-invocation: true\n---\nRetain exact unrelated inert bytes.\n", label)
	if projectJourneyRead(f.t, f.skill(skillRoot, "foreign-"+label)) != expected {
		f.t.Fatal("unrelated skill bytes changed")
	}
	if project != "" && projectJourneyRead(f.t, filepath.Join(project, ".uap", "data", "caller-owned", "keep.txt")) != "caller-owned fixture data\n" {
		f.t.Fatal("caller data changed")
	}
}
func (f *clineGate) emitted(name, path string) {
	body := []byte(projectJourneyRead(f.t, path))
	object := clineGateObject(f.t, body)
	servers, ok := object["mcpServers"].(map[string]any)
	if !ok || len(servers) == 0 {
		f.t.Fatal("actual UAP enabled config missing")
	}
	for _, value := range servers {
		entry, a := value.(map[string]any)
		transport, b := entry["transport"].(map[string]any)
		url, c := transport["url"].(string)
		_, disabled := entry["disabled"]
		if !a || !b || !c || disabled || transport["type"] != "streamableHttp" || !strings.HasPrefix(url, "http://127.0.0.1:9/") {
			f.t.Fatal("emitted file is not actual enabled remote-only codec contract")
		}
	}
	f.keep(name, body)
}
func clineGateProgress(label string) (string, bool) {
	names := map[string]string{"add-user": "uap-cline-user-package", "add-A": "uap-cline-a-package", "add-B": "uap-cline-b-package", "add-same-user": "uap-cline-same", "add-same-A": "uap-cline-same", "add-same-B": "uap-cline-same"}
	if name, ok := names[label]; ok {
		return "Resolving and validating one Agent Plugin package for every selected target...\nInstalling " + name + " for 1 selected clients...\n", true
	}
	switch label {
	case "repeat-add-A":
		return "Resolving and validating one Agent Plugin package for every selected target...\nApplying transactional client package...\n", true
	case "unchanged-update-A", "update-A":
		return "Resolving and validating one updated Agent Plugin package for every selected target...\n", true
	case "intact-repair-A", "missing-repair-A":
		return "Resolving and validating each unique exact installed package revision once...\n", true
	case "remove-A":
		return "", true
	}
	return "", false
}
func clineGateAcceptProgress(label string, body []byte) bool {
	expected, ok := clineGateProgress(label)
	return ok && string(body) == expected
}
func clineGateMutation(t *testing.T, result map[string]any, want bool) {
	t.Helper()
	count := 0
	var visit func(any)
	visit = func(value any) {
		switch v := value.(type) {
		case map[string]any:
			for key, item := range v {
				if key == "no_change" && item == true && want {
					t.Fatal("actual mutation classified no_change")
				}
				if key == "mutated" {
					count++
					if item != want {
						t.Fatalf("mutated=%v,want%t", item, want)
					}
				}
				visit(item)
			}
		case []any:
			for _, item := range v {
				visit(item)
			}
		}
	}
	visit(result)
	if count == 0 {
		t.Fatal("actual public mutation field absent")
	}
}
func (f *clineGate) pass(stage, reason string) {
	f.stages[stage] = map[string]string{"status": "passed", "reason": reason}
}
func (f *clineGate) prepareUnique() string {
	f.begin("source_install")
	f.seedForeign()
	user := f.source("user", "uap-cline-user-package", "uap-cline-user", "USER")
	a := f.source("A", "uap-cline-a-package", "uap-cline-a", "A-ONE")
	b := f.source("B", "uap-cline-b-package", "uap-cline-b", "B-ONE")
	clineGateMutation(f.t, f.uap("add-user", "", "add", user, "--no-save"), true)
	clineGateMutation(f.t, f.uap("add-A", f.A, "add", a, "--no-save"), true)
	clineGateMutation(f.t, f.uap("add-B", f.B, "add", b, "--no-save"), true)
	f.emitted("prepared-user-config.json", f.userMCP())
	f.emitted("prepared-A-config.json", f.projectMCP(f.A))
	f.emitted("prepared-B-config.json", f.projectMCP(f.B))
	f.inspect("prepared-A-inspection.json", f.A)
	f.inspect("prepared-B-inspection.json", f.B)
	f.pass("source_install", "public UAP created exact enabled remote config and scoped receipts; explicit inert security admission")
	return a
}
func (f *clineGate) observeUnique() {
	f.begin("unique_discovery")
	raw := string(f.native("version", f.user, "", "--version"))
	if raw != "3.0.73\n" {
		f.t.Fatalf("native exact version differs:%q", raw)
	}
	f.evidence["client_version_measured"] = "3.0.73"
	f.checkSkills("initial-A-skills", f.A, "A-ONE", false)
	f.checkSkills("initial-B-skills", f.B, "B-ONE", false)
	f.checkMCP("default-A", f.A, "", f.userMCP(), "foreign-user", "uap-cline-user")
	f.checkMCP("default-B", f.B, "", f.userMCP(), "foreign-user", "uap-cline-user")
	f.checkMCP("override-A", f.A, f.A, f.projectMCP(f.A), "foreign-a", "uap-cline-a")
	f.checkMCP("override-B", f.B, f.B, f.projectMCP(f.B), "foreign-b", "uap-cline-b")
	f.checkMCP("cross-A-from-B", f.B, f.A, f.projectMCP(f.A), "foreign-a", "uap-cline-a")
	f.checkMCP("restart-default-A", f.A, "", f.userMCP(), "foreign-user", "uap-cline-user")
	f.pass("unique_discovery", "populated exact A/B/user skill paths and default/explicit/cross-cwd/restart unedited enabled source files observed")
}
func (f *clineGate) uniqueLifecycle(source string) {
	userRoots := []string{filepath.Join(f.home, ".cline", "skills"), f.userMCP(), filepath.Join(f.cache, "state-v2.json")}
	otherRoots := []string{filepath.Join(f.B, ".cline", "skills"), f.projectMCP(f.B), filepath.Join(f.B, ".uap")}
	userBefore, otherBefore := f.snapshot(userRoots...), f.snapshot(otherRoots...)
	stableRoots := []string{filepath.Join(f.A, ".cline", "skills"), f.projectMCP(f.A), filepath.Join(f.A, ".uap", "state-v2.json")}
	stable := f.snapshot(stableRoots...)
	nativeRoots := []string{filepath.Join(f.A, ".cline", "skills"), f.projectMCP(f.A)}
	nativeStable := f.snapshot(nativeRoots...)
	f.begin("stable_lifecycle")
	for _, step := range []struct{ label, verb, selector string }{{"repeat-add-A", "add", source}, {"unchanged-update-A", "update", "uap-cline-a-package"}, {"intact-repair-A", "repair", "uap-cline-a-package"}} {
		args := []string{step.verb, step.selector}
		if step.verb == "add" {
			args = append(args, "--no-save")
		}
		clineGateMutation(f.t, f.uap(step.label, f.A, args...), step.verb == "repair")
		if step.verb != "repair" {
			f.requireSnapshot(stable, stableRoots...)
		} else {
			f.requireSnapshot(nativeStable, nativeRoots...)
		}
	}
	f.checkSkills("stable-A-skills", f.A, "A-ONE", false)
	f.checkMCP("stable-A-mcp", f.A, f.A, f.projectMCP(f.A), "foreign-a", "uap-cline-a")
	f.pass("stable_lifecycle", "replay/update unchanged; intact repair truthfully restages with mutation and exact native bytes")
	f.begin("update")
	f.source("A", "uap-cline-a-package", "uap-cline-a", "A-TWO")
	clineGateMutation(f.t, f.uap("update-A", f.A, "update", source), true)
	f.checkSkills("updated-A-skills", f.A, "A-TWO", false)
	f.checkMCP("updated-A-mcp", f.A, f.A, f.projectMCP(f.A), "foreign-a", "uap-cline-a")
	f.emitted("updated-A-config.json", f.projectMCP(f.A))
	f.pass("update", "same-version update changed owned bytes and receipt; listing read-only")
	f.begin("missing_repair")
	if err := os.RemoveAll(filepath.Dir(f.skill(f.A, "uap-cline-a"))); err != nil {
		f.t.Fatal(err)
	}
	clineGateMutation(f.t, f.uap("missing-repair-A", f.A, "repair", "uap-cline-a-package"), true)
	f.checkSkills("repaired-A-skills", f.A, "A-TWO", false)
	f.checkMCP("repaired-A-mcp", f.A, f.A, f.projectMCP(f.A), "foreign-a", "uap-cline-a")
	f.emitted("repaired-A-config.json", f.projectMCP(f.A))
	f.assertPrepared(f.A, "uap-cline-a-package")
	f.inspect("repaired-A-inspection.json", f.A)
	f.pass("missing_repair", "restored installed A-TWO from unchanged selected local source; remote immutable-pin proof remains separate TLS Git fixture")
	f.begin("remove")
	clineGateMutation(f.t, f.uap("remove-A", f.A, "remove", "uap-cline-a-package"), true)
	f.checkSkills("removed-A-skills", f.A, "", true)
	f.checkMCP("removed-A-mcp", f.A, f.A, f.projectMCP(f.A), "foreign-a")
	f.emitted("removed-A-config.json", f.projectMCP(f.A))
	f.foreign(f.A)
	f.foreign(f.B)
	f.requireSnapshot(userBefore, userRoots...)
	f.requireSnapshot(otherBefore, otherRoots...)
	snapshots, _ := json.MarshalIndent(map[string]any{"user": userBefore, "B": otherBefore}, "", "  ")
	f.keep("foreign-snapshots.json", snapshots)
	f.pass("remove", "owned skill/member absent; foreign A/user/B bytes retained")
	f.pass("foreign_preservation", "original user and B state/native bytes exact across A lifecycle")
}
func (f *clineGate) sameNameForbidden(project string) []string {
	roots := make([]string, 0, 2)
	for _, candidate := range []string{f.A, f.B} {
		if candidate != project {
			roots = append(roots, filepath.Join(candidate, ".cline", "skills"))
		}
	}
	return roots
}
func (f *clineGate) sameName() {
	f.begin("same_name_precedence")
	const plugin = "uap-cline-same"
	for _, item := range []struct{ id, project, marker string }{{"same-user", "", "SAME-USER"}, {"same-A", f.A, "SAME-A"}, {"same-B", f.B, "SAME-B"}} {
		source := f.source(item.id, plugin, "uap-cline-shared", item.marker)
		before := f.snapshot(f.ownedRoots()...)
		clineGateMutation(f.t, f.uap("add-"+item.id, item.project, "add", source, "--no-save"), true)
		for path, want := range before {
			own := item.project
			if own == "" {
				own = f.home
			}
			if !strings.HasPrefix(path, own+string(filepath.Separator)) && (item.project != "" || !strings.HasPrefix(path, f.cache+string(filepath.Separator))) {
				if f.snapshot(path)[path] != want {
					f.t.Fatal("same-name add changed another scope")
				}
			}
		}
		if item.project != "" {
			f.assertPrepared(item.project, plugin)
		}
		f.foreign(item.project)
	}
	f.evidence["same_name_owned_bindings"] = map[string]any{"user": f.binding("", plugin), "A": f.binding(f.A, plugin), "B": f.binding(f.B, plugin)}
	winners := map[string]any{}
	for _, item := range []struct{ label, project, cwd string }{{"same-user", "", f.user}, {"same-A", f.A, f.A}, {"same-B", f.B, f.B}} {
		body := f.native(item.label+"-skills", item.cwd, "", "config", "skills", "--json")
		rows, err := clineGateRows(body)
		if err != nil {
			f.t.Fatal(err)
		}
		selected := []map[string]any{}
		wants := []clineGateSkill{}
		markers := map[string]string{f.skill("", "uap-cline-shared"): "SAME-USER", f.skill(f.A, "uap-cline-shared"): "SAME-A", f.skill(f.B, "uap-cline-shared"): "SAME-B"}
		for _, row := range rows {
			if row["name"] != "uap-cline-shared" {
				continue
			}
			path, ok := clineGateText(row["path"])
			if !ok {
				f.t.Fatal("same-name path missing")
			}
			if path != f.skill("", "uap-cline-shared") && (item.project == "" || path != f.skill(item.project, "uap-cline-shared")) {
				f.t.Fatal("same-name row leaked sibling/foreign path")
			}
			selected = append(selected, row)
			wants = append(wants, clineGateSkill{path: path, name: "uap-cline-shared", marker: markers[path]})
		}
		if len(selected) == 0 {
			f.t.Fatal("same-name inventory empty")
		}
		if err := clineGateSkills(body, wants, f.sameNameForbidden(item.project)); err != nil {
			f.t.Fatal(err)
		}
		winners[item.label] = selected
		names := []string{"foreign-user", "uap-cline-user", "uap-cline-shared"}
		path, override := f.userMCP(), ""
		if item.project != "" {
			label := "a"
			if item.project == f.B {
				label = "b"
			}
			names = []string{"foreign-" + label, "uap-cline-shared"}
			if item.project == f.B {
				names = append(names, "uap-cline-b")
			}
			path, override = f.projectMCP(item.project), item.project
		}
		f.checkMCP(item.label+"-mcp", item.cwd, override, path, names...)
		f.emitted(item.label+"-config.json", path)
	}
	body, _ := json.MarshalIndent(winners, "", "  ")
	f.keep("same-name-winners.json", body)
	f.stages["same_name_precedence"] = map[string]string{"status": "blocked", "reason": "actual rows separate from three UAP receipts; independent active versions/session policy unqualified"}
	f.evidence["same_name_observation_complete"] = true
}
func (f *clineGate) inspect(name, project string) {
	before := f.snapshot(f.ownedRoots()...)
	app, err := f.journey.app.ForScope("project", project)
	if err != nil {
		f.t.Fatal(err)
	}
	view, err := app.Installer.Inspect(f.ctx)
	if err != nil || view.Recovery.Required {
		f.t.Fatalf("fresh read-only inspection:%v %+v", err, view.Recovery)
	}
	count := 0
	for _, installation := range view.Installations {
		for _, binding := range installation.Bindings {
			if binding.ClientID != "cline" || binding.Materialization == "absent" {
				continue
			}
			count++
			if binding.Scope != "project" || binding.Activation != "prepared" || binding.Verification != "installation_verified" || binding.ProjectionStatus == "projection_missing" || binding.ProjectionStatus == "projection_mismatch" || binding.TreeDigest == "" {
				f.t.Fatalf("fresh SDK Prepared projection differs:%+v", binding)
			}
		}
	}
	if count == 0 || f.runner.calls != 0 {
		f.t.Fatal("fresh SDK omitted binding or invoked native")
	}
	f.requireSnapshot(before, f.ownedRoots()...)
	body, err := json.MarshalIndent(view, "", "  ")
	if err != nil {
		f.t.Fatal(err)
	}
	f.keep(name, append(body, '\n'))
}
func TestClineCurrentSourceNativeProjectConfigLifecycle(t *testing.T) {
	requireClineGate(t)
	f := newClineGate(t)
	defer f.finish()
	f.gitRoots()
	source := f.prepareUnique()
	f.observeUnique()
	f.uniqueLifecycle(source)
	f.sameName()
	if f.runner.calls != 0 {
		t.Fatal("public UAP invoked native/MCP command")
	}
	if !reflect.DeepEqual(f.used, clineGateExpectedUsed()) {
		t.Fatal("finite command table not completed")
	}
	f.evidence["read_only_unique_marker_discovery_qualified"] = true
}

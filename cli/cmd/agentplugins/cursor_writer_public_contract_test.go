package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli"
)

// Exercise the same native writer and no-change lifecycle with a tiny remote
// recipe. No endpoint is contacted and no managed stdio launcher is copied.
func TestProjectCursorNativeWriterLockPublicContracts(t *testing.T) {
	f := newCursorProjectJourney(t, newProjectJourneyTransport(t, "zip"))
	profile := filepath.Join(f.a, ".cursor")
	// These are this fixture's known leaves, removed before the new baseline.
	for _, name := range []string{"hooks.json", "skills/foreign/SKILL.md", "skills/foreign", "skills", "mcp.json"} {
		if err := os.Remove(filepath.Join(profile, filepath.FromSlash(name))); err != nil {
			t.Fatal(err)
		}
	}
	projectJourneyWrite(t, filepath.Join(profile, "foreign-owned.txt"), "caller-owned a\n")
	var writerLock os.FileInfo
	source := projectJourneyTemp(t)
	write := func(version string) {
		t.Helper()
		projectJourneyWrite(t, filepath.Join(source, "plugin.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"native-proof","version":"`+version+`"}`+"\n")
		projectJourneyWrite(t, filepath.Join(source, "mcp.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"uapcursora":{"type":"streamable-http","url":"http://127.0.0.1:9/mcp"}}}`+"\n")
	}
	write("1.0.0")
	var iid, oldState string
	run := func(stage, verb, selector string, changed bool) {
		t.Helper()
		var stdout, stderr bytes.Buffer
		app := f.app
		app.Input, app.Output, app.ErrorOutput, app.Terminal = strings.NewReader(""), &stdout, &stderr, false
		root := agentpluginscli.NewRoot(app)
		args := []string{verb, selector, "--scope", "project", "--project-root", f.a, "--target", "cursor", "--format", "json"}
		if verb == "add" {
			args = append(args, "--no-save")
		}
		root.SetArgs(args)
		if err := root.Execute(); err != nil {
			t.Fatalf("%s: %v\n%s\n%s", stage, err, stdout.String(), stderr.String())
		}
		var row map[string]any
		if err := json.Unmarshal(stdout.Bytes(), &row); err != nil {
			t.Fatal(err)
		}
		data := row["data"].(map[string]any)
		output := data
		if stage == "repeat-add-a" {
			if _, present := data["targets"]; present || data["operation_id"] != nil {
				t.Fatal("repeat add is not the exact singleton")
			}
		} else {
			if data["batch"] != true || data["succeeded"] != float64(1) || data["failed"] != float64(0) {
				t.Fatal("public batch differs", row)
			}
			output = data["targets"].([]any)[0].(map[string]any)["output"].(map[string]any)
		}
		outcome := output["result"].(map[string]any)
		keys := map[string]bool{"installation_id": true, "requires_confirmation": true, "mutated": true}
		if verb == "remove" {
			for _, key := range []string{"plugin", "client_id", "affected_surfaces", "deactivation", "group_phase"} {
				keys[key] = true
			}
		} else {
			keys["plan"], keys["activation"] = true, true
			if stage != "repeat-add-a" {
				keys["group_phase"] = true
			}
			if stage == "repeat-add-a" || stage == "unchanged-update-a" {
				keys["no_change"] = true
				if outcome["no_change"] != true {
					t.Fatal("observed read-only result lost no_change", stage)
				}
			}
		}
		if len(outcome) != len(keys) {
			t.Fatal("exact measured public result keys differ", stage, outcome)
		}
		for key := range outcome {
			if !keys[key] {
				t.Fatal("unknown public result key", stage, key)
			}
		}
		if stage != "repeat-add-a" && outcome["group_phase"] != "external_completed" {
			t.Fatal("measured public group phase differs", stage)
		}
		if iid == "" {
			iid = outcome["installation_id"].(string)
		}
		if outcome["installation_id"] != iid || outcome["mutated"] != changed || outcome["requires_confirmation"] != false {
			t.Fatal("held public outcome differs", row)
		}
		label := "direct local source"
		if verb == "repair" || verb == "remove" {
			label = "local"
		}
		if output["source"] != label {
			t.Fatal("source label differs", row)
		}
		if verb != "remove" {
			plan, activation := outcome["plan"].(map[string]any), outcome["activation"].(map[string]any)
			if plan["verification"] != "package_validated" || activation["verification"] != "installation_verified" || !reflect.DeepEqual(plan["components"], []any{map[string]any{"kind": "mcp_server", "name": "uapcursora", "support": "native"}}) {
				t.Fatal("Prepared MCP-only DTO differs", row)
			}
		}
		if stage == "unchanged-update-a" && outcome["no_change"] != true {
			t.Fatal("unchanged update flag differs", row)
		}
		state := projectJourneyRead(t, filepath.Join(f.a, ".uap", "state-v2.json"))
		if !changed && state != oldState {
			t.Fatal("public no-op rewrote exact state")
		}
		oldState = state
		writerLock = cursorNativeWriterProfileContract(t, profile, true, writerLock)
	}
	run("add-a", "add", source, true)
	run("repeat-add-a", "add", source, false)
	run("unchanged-update-a", "update", iid, false)
	run("intact-repair-a", "repair", iid, true)
	write("1.1.0")
	run("update-a", "update", iid, true)
	// Remove only the sole owned native document, leaving the package intact.
	config := filepath.Join(f.a, ".cursor", "mcp.json")
	if err := os.Remove(config); err != nil {
		t.Fatal(err)
	}
	writerLock = cursorNativeWriterProfileContract(t, profile, false, writerLock)
	run("missing-repair-a", "repair", iid, true)
	run("remove-a", "remove", iid, true)
	if _, err := f.execute(f.a, "remove", iid, "--target", "cursor"); err == nil || err.Error() != `plugin is not installed for target "cursor" in project scope; no target was changed` {
		t.Fatal("repeated removal refusal differs", err)
	}
	if projectJourneyRead(t, filepath.Join(f.a, ".uap", "state-v2.json")) != oldState {
		t.Fatal("repeated removal changed state")
	}
	cursorNativeWriterProfileContract(t, profile, true, writerLock)
}

func cursorNativeWriterProfileContract(t *testing.T, root string, configured bool, held os.FileInfo) os.FileInfo {
	t.Helper()
	want := map[string]bool{"foreign-owned.txt": true, "mcp.json.agentplugins.lock": true}
	if configured {
		want["mcp.json"] = true
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != len(want) {
		t.Fatalf("exact public Cursor profile membership: %v, %v", entries, err)
	}
	for _, entry := range entries {
		if !want[entry.Name()] || !entry.Type().IsRegular() {
			t.Fatal("unknown or non-ordinary public profile member", entry.Name())
		}
	}
	path := filepath.Join(root, "mcp.json.agentplugins.lock")
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() != 0 {
		t.Fatal("source-defined empty writer lock differs", err)
	}
	if body, err := os.ReadFile(path); err != nil || len(body) != 0 {
		t.Fatal("writer lock body differs", err)
	}
	// The Linux-tagged frame contract checks exact 0600 and numeric ownership.
	// This portable route also preserves the actual lock identity and mode.
	if held != nil && (!os.SameFile(held, info) || held.Mode() != info.Mode()) {
		t.Fatal("public lifecycle replaced writer lock authority")
	}
	if projectJourneyRead(t, filepath.Join(root, "foreign-owned.txt")) != "caller-owned a\n" {
		t.Fatal("public lifecycle changed the foreign caller file")
	}
	return info
}

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

// Real public Cobra/installer/filesystem composition with inert host discovery
// and assessment. No Cursor process or authored MCP process is executed.
func TestProjectCursorDirectMCPCyclePublicContracts(t *testing.T) {
	f := newCursorProjectJourney(t, newProjectJourneyTransport(t, "zip"))
	projectJourneyWrite(t, filepath.Join(f.a, ".cursor", "mcp.json"), "{\"mcpServers\":{}}\n")
	source := projectJourneyTemp(t)
	write := func(version string) {
		t.Helper()
		projectJourneyWrite(t, filepath.Join(source, "plugin.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"native-proof","version":"`+version+`"}`+"\n")
		projectJourneyWrite(t, filepath.Join(source, "mcp.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"uapcursora":{"type":"stdio","command":"./bin/probe","args":["cursor-component"],"cwd":"${PLUGIN_ROOT}"}}}`+"\n")
		projectJourneyWrite(t, filepath.Join(source, "bin", "probe"), "inert fixture bytes; never executed\n")
		if err := os.Chmod(filepath.Join(source, "bin", "probe"), 0700); err != nil {
			t.Fatal(err)
		}
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
	run("missing-repair-a", "repair", iid, true)
	run("remove-a", "remove", iid, true)
	if _, err := f.execute(f.a, "remove", iid, "--target", "cursor"); err == nil || err.Error() != `plugin is not installed for target "cursor" in project scope; no target was changed` {
		t.Fatal("repeated removal refusal differs", err)
	}
	if projectJourneyRead(t, filepath.Join(f.a, ".uap", "state-v2.json")) != oldState {
		t.Fatal("repeated removal changed state")
	}
}

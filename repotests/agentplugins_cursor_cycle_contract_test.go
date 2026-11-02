//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestCursorCycleLiteralCommands(t *testing.T) {
	f := &cursorProjectAttempt{base: &nativeFixture{Project: "/owned/A", Package: "/owned/sourceA"}, other: "/owned/B", outside: "/owned/outside", installer: "/held/uap", client: "/held/cursor-agent", git: "/held/git"}
	c := &cursorMCPProjectCycle{f: f, recipes: map[string]*cursorCycleRecipe{"a": {Scope: "a", Project: "/owned/A", Source: "/owned/sourceA", Installation: domain.Installation{InstallationID: "held-a"}}, "b": {Scope: "b", Project: "/owned/B", Source: "/owned/sourceB", Installation: domain.Installation{InstallationID: "held-b"}}}}
	// Independent approved table: no production table is used to author expectations.
	want := strings.Fields("git-a:a:git git-b:b:git version:outside:version empty-a:a:list empty-b:b:list empty-outside:outside:list add-a:a:add approve-a-one:a:enable installed-a:a:list-tools add-b:b:add approve-b-one:b:enable installed-b:b:list-tools installed-outside:outside:list repeat-add-a:a:add unchanged-update-a:a:update intact-repair-a:a:repair stable-a:a:list-tools stable-b:b:list-tools update-a:a:update approve-a-two:a:enable updated-a:a:list-tools updated-b:b:list-tools updated-outside:outside:list damaged-a:a:list missing-repair-a:a:repair repaired-a:a:list-tools remove-a:a:remove removed-a:a:list remaining-b:b:list-tools removed-outside:outside:list repeat-remove-a:a:remove remove-b:b:remove final-a:a:list final-b:b:list final-outside:outside:list")
	if len(cursorCycleSteps) != len(want) {
		t.Fatal("approved stage closure changed")
	}
	for index, literal := range want {
		parts := strings.Split(literal, ":")
		step := cursorCycleStep{parts[0], parts[1], parts[2]}
		if cursorCycleSteps[index] != step {
			t.Fatal("approved stage changed", index)
		}
		cwd := "/owned/outside"
		if step.scope != "outside" {
			cwd = "/owned/" + strings.ToUpper(step.scope)
		}
		var argv []string
		switch step.verb {
		case "git":
			argv = []string{"/held/git", "init", "--quiet", "--initial-branch=main"}
		case "version":
			argv = []string{"/held/cursor-agent", "--version"}
		case "list":
			argv = []string{"/held/cursor-agent", "mcp", "list"}
		case "enable", "list-tools":
			argv = []string{"/held/cursor-agent", "mcp", step.verb, "uapcursora"}
		default:
			selector := "held-" + step.scope
			if step.verb == "add" {
				selector = "/owned/source" + strings.ToUpper(step.scope)
			}
			argv = []string{"/held/uap", step.verb, selector}
			if step.verb == "add" {
				argv = append(argv, "--no-save")
			}
			argv = append(argv, "--scope", "project", "--project-root", cwd, "--target", "cursor", "--format", "json")
		}
		command := cursorProjectCommand{step.label, cwd, argv}
		if !reflect.DeepEqual(c.commandAt(index), command) {
			t.Fatal("approved argv/cwd changed", step.label)
		}
		c.completed = append(c.completed, step.label)
	}
}

func cursorCycleProbeContract(t *testing.T, methods []string, pid int) []byte {
	t.Helper()
	var raw []byte
	for index, method := range methods {
		body, err := json.Marshal(map[string]any{"method": method, "pid": pid, "session": strconv.Itoa(pid) + "-100", "timestamp": time.Unix(100, int64(index+1)).UTC().Format(time.RFC3339Nano), "nonce": "held-nonce", "facts": nil})
		if err != nil {
			t.Fatal(err)
		}
		raw = append(raw, append(body, '\n')...)
	}
	return raw
}

func TestCursorCycleProbeSessions(t *testing.T) {
	start, end := time.Unix(100, 0), time.Unix(101, 0)
	for _, methods := range [][]string{{"initialize", "tools/list"}, {"initialize", "tools/list", "tools/list", "tools/list"}} {
		raw := cursorCycleProbeContract(t, methods, 7)
		if ids, err := cursorCycleProbeDelta(raw, "held-nonce", start, end, map[string]bool{}); err != nil || !reflect.DeepEqual(ids, []string{"7-100"}) {
			t.Fatal("valid fresh discovery refused", err)
		}
	}
	for _, methods := range [][]string{{}, {"initialize"}, {"tools/list"}, {"initialize", "initialize", "tools/list"}, {"initialize", "tools/call"}, {"initialize", "tools/list", "tools/call"}} {
		if _, err := cursorCycleProbeDelta(cursorCycleProbeContract(t, methods, 7), "held-nonce", start, end, nil); err == nil {
			t.Fatal("invalid MCP sequence admitted", methods)
		}
	}
	raw := cursorCycleProbeContract(t, []string{"initialize", "tools/list"}, 7)
	for _, hostile := range [][]byte{bytes.ReplaceAll(raw, []byte("held-nonce"), []byte("foreign")), bytes.ReplaceAll(raw, []byte("7-100"), []byte("7-0")), bytes.ReplaceAll(raw, []byte("\"facts\":null"), []byte("\"facts\":{}")), bytes.ReplaceAll(raw, []byte("\"pid\":7"), []byte("\"pid\":8")), bytes.ReplaceAll(raw, []byte("\"method\":"), []byte("\"unknown\":0,\"method\":")), raw[:len(raw)-1]} {
		if _, err := cursorCycleProbeDelta(hostile, "held-nonce", start, end, nil); err == nil {
			t.Fatal("hostile probe admitted")
		}
	}
	if _, err := cursorCycleProbeDelta(raw, "held-nonce", start, end, map[string]bool{"7-100": true}); err == nil {
		t.Fatal("old session admitted")
	}
}

func TestCursorCycleHeldFileCannotAdoptMutation(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "held")
	projectNativeRuntimeWrite(t, path, []byte("original"), 0600)
	f := &cursorProjectAttempt{ctx: context.Background(), held: map[string]cursorProjectHeld{}}
	c := &cursorMCPProjectCycle{f: f, dirs: map[string]os.FileInfo{}, trees: map[string]map[string][]byte{}}
	if err := c.acceptFile(path, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("replaced"), 0600); err != nil {
		t.Fatal(err)
	}
	if c.acceptFile(path, []byte("replaced"), 0600) == nil || c.checkError() == nil {
		t.Fatal("new bytes renewed original authority")
	}
}

func TestCursorCycleEmptyStreamHasCanonicalBase64Body(t *testing.T) {
	for _, input := range [][]byte{nil, {}} {
		value, err := cursorCycleStream(input)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		object, err := projectNativeRuntimeJSON(raw)
		if err != nil || !reflect.DeepEqual(object, map[string]any{"bytes": json.Number("0"), "sha256": clineAttemptSHA(nil), "body": ""}) {
			t.Fatal("empty process channel is not canonical base64", string(raw), err)
		}
	}
}

func TestCursorCycleRemovalRequiresExactDeactivation(t *testing.T) {
	r := &cursorCycleRecipe{Installation: domain.Installation{InstallationID: "held-installation"}}
	step := cursorCycleStep{"remove-a", "a", "remove"}
	deactivation := map[string]any{"activation": "not_required", "artifact_removal_allowed": true, "external_removal_complete": true}
	outcome := map[string]any{"installation_id": "held-installation", "plugin": "native-proof", "client_id": "cursor", "requires_confirmation": false, "mutated": true, "deactivation": deactivation, "affected_surfaces": []any{"cursor"}, "group_phase": "external_completed"}
	output := map[string]any{"result": outcome}
	if err := cursorCyclePublicOutcome(output, step, r); err != nil {
		t.Fatal(err)
	}
	for key, bad := range map[string]any{"activation": "active", "external_removal_complete": false, "artifact_removal_allowed": false} {
		original := deactivation[key]
		deactivation[key] = bad
		if cursorCyclePublicOutcome(output, step, r) == nil {
			t.Fatal("invalid native removal was accepted", key)
		}
		deactivation[key] = original
	}
}

func TestCursorCyclePublicOutcomeRequiresMeasuredStageShape(t *testing.T) {
	r := &cursorCycleRecipe{Installation: domain.Installation{InstallationID: "held-installation"}, Data: domain.DataReceipt{PhysicalBackend: "held-physical"}}
	for _, stage := range []string{"add-a", "repeat-add-a", "unchanged-update-a", "intact-repair-a", "update-a", "missing-repair-a"} {
		t.Run(stage, func(t *testing.T) {
			verb := "add"
			if strings.Contains(stage, "update") {
				verb = "update"
			} else if strings.Contains(stage, "repair") {
				verb = "repair"
			}
			step := cursorCycleStep{stage, "a", verb}
			plan := map[string]any{"client_id": "cursor", "scope": "project", "status": "ready", "package_mode": "native", "physical_artifact_id": "held-physical", "activation": "prepared", "authentication": "not_checked", "policy": "allowed", "verification": "package_validated", "components": []any{map[string]any{"kind": "mcp_server", "name": "uapcursora", "support": "native"}}}
			activation := map[string]any{"activation": "prepared", "authentication": "not_checked", "policy": "allowed", "verification": "installation_verified"}
			outcome := map[string]any{"installation_id": "held-installation", "plan": plan, "activation": activation, "requires_confirmation": false, "mutated": stage != "repeat-add-a" && stage != "unchanged-update-a"}
			if stage != "repeat-add-a" {
				outcome["group_phase"] = "external_completed"
			}
			if stage == "repeat-add-a" || stage == "unchanged-update-a" {
				outcome["no_change"] = true
			}
			output := map[string]any{"result": outcome}
			if err := cursorCyclePublicOutcome(output, step, r); err != nil {
				t.Fatal("measured stage shape refused", err)
			}
			for _, field := range []string{"no_change", "group_phase"} {
				before, present := outcome[field]
				if present {
					delete(outcome, field)
				} else {
					outcome[field] = true
				}
				if cursorCyclePublicOutcome(output, step, r) == nil {
					t.Fatal("cross-stage or missing field admitted", field)
				}
				if present {
					outcome[field] = false
					if cursorCyclePublicOutcome(output, step, r) == nil {
						t.Fatal("incorrect measured field value admitted", field)
					}
					outcome[field] = before
				} else {
					delete(outcome, field)
				}
			}
		})
	}
}

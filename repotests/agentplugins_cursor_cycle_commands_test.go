//go:build linux

package pluginkitairepo_test

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type cursorCycleStep struct{ label, scope, verb string }

var cursorCycleSteps = []cursorCycleStep{
	{"git-a", "a", "git"}, {"git-b", "b", "git"}, {"version", "outside", "version"},
	{"empty-a", "a", "list"}, {"empty-b", "b", "list"}, {"empty-outside", "outside", "list"},
	{"add-a", "a", "add"}, {"approve-a-one", "a", "enable"}, {"installed-a", "a", "list-tools"},
	{"add-b", "b", "add"}, {"approve-b-one", "b", "enable"}, {"installed-b", "b", "list-tools"}, {"installed-outside", "outside", "list"},
	{"repeat-add-a", "a", "add"}, {"unchanged-update-a", "a", "update"}, {"intact-repair-a", "a", "repair"},
	{"stable-a", "a", "list-tools"}, {"stable-b", "b", "list-tools"},
	{"update-a", "a", "update"}, {"approve-a-two", "a", "enable"}, {"updated-a", "a", "list-tools"}, {"updated-b", "b", "list-tools"}, {"updated-outside", "outside", "list"},
	{"damaged-a", "a", "list"}, {"missing-repair-a", "a", "repair"}, {"repaired-a", "a", "list-tools"},
	{"remove-a", "a", "remove"}, {"removed-a", "a", "list"}, {"remaining-b", "b", "list-tools"}, {"removed-outside", "outside", "list"},
	{"repeat-remove-a", "a", "remove"}, {"remove-b", "b", "remove"},
	{"final-a", "a", "list"}, {"final-b", "b", "list"}, {"final-outside", "outside", "list"},
}

func (c *cursorMCPProjectCycle) commandAt(index int) cursorProjectCommand {
	step := cursorCycleSteps[index]
	f, cwd := c.f, c.f.outside
	r := c.recipes[step.scope]
	if r != nil {
		cwd = r.Project
	}
	var argv []string
	switch step.verb {
	case "git":
		argv = []string{f.git, "init", "--quiet", "--initial-branch=main"}
	case "version":
		argv = []string{f.client, "--version"}
	case "list":
		argv = []string{f.client, "mcp", "list"}
	case "enable", "list-tools":
		argv = []string{f.client, "mcp", step.verb, "uapcursora"}
	default:
		selector := r.Installation.InstallationID
		if step.verb == "add" {
			selector = r.Source
		}
		argv = []string{f.installer, step.verb, selector}
		if step.verb == "add" {
			argv = append(argv, "--no-save")
		}
		argv = append(argv, "--scope", "project", "--project-root", r.Project, "--target", "cursor", "--format", "json")
	}
	return cursorProjectCommand{step.label, cwd, argv}
}

func (c *cursorMCPProjectCycle) admit(command cursorProjectCommand) error {
	if len(c.completed) >= len(cursorCycleSteps) || !reflect.DeepEqual(command, c.commandAt(len(c.completed))) {
		return fmt.Errorf("Cursor exact cycle command/order differs")
	}
	return nil
}

func (c *cursorMCPProjectCycle) capture(t *testing.T, command cursorProjectCommand, result ports.CommandResult, receipt map[string]any, start, end time.Time) {
	t.Helper()
	value := map[string]any{}
	for key, item := range receipt {
		value[key] = item
	}
	value["started_at"], value["ended_at"] = start.UTC().Format(time.RFC3339Nano), end.UTC().Format(time.RFC3339Nano)
	value["stdout"], value["stderr"] = c.stream(t, result.Stdout), c.stream(t, result.Stderr)
	c.append(t, cursorCycleMembers[0], command.label, value)
	c.completed = append(c.completed, command.label)
	step := cursorCycleSteps[len(c.completed)-1]
	c.observeProbe(t, step, start, end)
	if step.verb == "enable" && result.ExitCode == 0 && receipt["error"] == false && receipt["deadline"] == false {
		c.acceptApproval(t, c.recipes[step.scope], command.label)
	}
}

func (c *cursorMCPProjectCycle) allowedExit(command cursorProjectCommand, result ports.CommandResult) bool {
	return command.label == "repeat-remove-a" && result.ExitCode == 1 && len(result.Stdout) == 0 && string(result.Stderr) == "agentplugins: plugin is not installed for target \"cursor\" in project scope; no target was changed\n"
}

func cursorCycleMutates(label string) bool {
	switch label {
	case "add-a", "add-b", "intact-repair-a", "update-a", "missing-repair-a", "remove-a", "remove-b":
		return true
	default:
		return false
	}
}

func (c *cursorMCPProjectCycle) run(t *testing.T, index int) {
	t.Helper()
	step := cursorCycleSteps[index]
	if step.label == "update-a" {
		c.revisedA(t)
	}
	if step.label == "damaged-a" {
		c.missingA(t)
	}
	c.check(t)
	r := c.recipes[step.scope]
	if cursorCycleMutates(step.label) {
		c.openSelected(t, r)
	}
	if step.verb == "enable" {
		c.openApproval(t, r)
	}
	c.stage = step.label
	c.f.evidence["cycle_stage"] = step.label
	command := c.commandAt(index)
	result := c.f.command(t, command)
	if step.verb == "add" || step.verb == "update" || step.verb == "repair" || step.verb == "remove" {
		c.publicProof(t, r, step, result)
	} else {
		if len(result.Stderr) != 0 {
			t.Fatal("Cursor cycle native stderr differs")
		}
		switch step.verb {
		case "version":
			if string(result.Stdout) != cursorProjectVersion+"\n" {
				t.Fatal("Cursor cycle exact version differs")
			}
		case "list":
			if string(result.Stdout) != "No MCP servers configured (expected in .cursor/mcp.json or ~/.cursor/mcp.json)\n" {
				t.Fatal("Cursor cycle exact empty native catalog differs")
			}
		case "list-tools":
			if string(result.Stdout) != "Tools for uapcursora (1):\n- inspect_runtime (marker, nonce, operation)\n" {
				t.Fatal("Cursor cycle exact owned tools differ")
			}
		case "enable":
			if string(result.Stdout) != "✓ Enabled and approved MCP server: uapcursora\n" {
				t.Fatal("Cursor cycle exact native approval differs")
			}
		case "git":
			if len(result.Stdout) != 0 {
				t.Fatal("Cursor cycle Git setup output differs")
			}
		}
	}
	c.check(t)
}

func cursorCyclePublicDocument(raw []byte) (map[string]any, error) {
	value, err := projectNativeRuntimeJSON(raw)
	object, ok := value.(map[string]any)
	if err != nil || !ok || nativeValidateInstallerJSON(raw) != nil || projectNativeRuntimeKeys(object, "schema_version command result data", "") != nil || object["schema_version"] != json.Number("1") || object["result"] != "success" {
		return nil, fmt.Errorf("Cursor exact public result envelope differs")
	}
	return object, nil
}

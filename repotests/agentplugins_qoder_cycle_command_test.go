//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/process"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type qoderCycleCommand struct {
	Argv     []string `json:"argv"`
	CWD      string   `json:"cwd"`
	Exit     int      `json:"exit"`
	Error    bool     `json:"error"`
	Deadline bool     `json:"deadline"`
	Elapsed  int64    `json:"elapsed_ms"`
	Cleanup  string   `json:"cleanup"`
}

type qoderCycleCommandRow struct {
	Command qoderCycleCommand       `json:"command"`
	Started string                  `json:"started_at"`
	Ended   string                  `json:"ended_at"`
	Stdout  qoderCycleBytes         `json:"stdout"`
	Stderr  qoderCycleBytes         `json:"stderr"`
	Log     qoderCycleLogTransition `json:"log"`
}

func (c *qoderSkillsCycle) run(t *testing.T, member, label, cwd string, argv []string, native bool) ports.CommandResult {
	t.Helper()
	c.check(t)
	ctx, cancel := context.WithTimeout(c.f.ctx, 30*time.Second)
	defer cancel()
	kind := "public"
	if member == qoderCycleAggregates[1] {
		kind = "catalog"
	}
	c.f.evidence["cycle_stage"] = kind + ":" + label
	start := time.Now().UTC()
	result, err := (process.OS{}).RunWithDescendantExitGrace(ctx, ports.Command{Argv: argv, Env: c.f.env(), Dir: cwd, StdoutLimitBytes: 1 << 20}, 5*time.Second)
	end := time.Now().UTC()
	var log qoderCycleLogTransition
	var logErr error
	if native {
		log, logErr = c.logs.native(cwd, argv, start, end)
	} else {
		log, logErr = c.logs.unchanged()
	}
	command := qoderCycleCommand{Argv: append([]string{}, argv...), CWD: cwd, Exit: result.ExitCode, Error: err != nil, Deadline: ctx.Err() != nil, Elapsed: end.Sub(start).Milliseconds(), Cleanup: "full descendant supervision; forced cleanup fails"}
	c.append(t, member, label, qoderCycleCommandRow{Command: command, Started: start.Format(time.RFC3339Nano), Ended: end.Format(time.RFC3339Nano), Stdout: qoderCycleStream(t, result.Stdout), Stderr: qoderCycleStream(t, result.Stderr), Log: log})
	if member == qoderCycleAggregates[0] {
		c.public = append(c.public, label)
	} else {
		c.catalogs = append(c.catalogs, label)
	}
	if err != nil || ctx.Err() != nil || end.Sub(start) > 35*time.Second || len(result.Stdout) > 1<<20 || len(result.Stderr) > 1<<20 || bytes.Contains(result.Stderr, []byte("stderr bytes omitted")) {
		t.Fatal("Qoder cycle command/capture/ordinary cleanup failed")
	}
	if logErr != nil {
		t.Fatal("Qoder strict original/log transition refused", logErr)
	}
	c.check(t)
	return result
}

func qoderCycleRole(label string) string {
	if strings.HasSuffix(label, "-a") {
		return "a"
	}
	if strings.HasSuffix(label, "-b") {
		return "b"
	}
	return "outside"
}

func (c *qoderSkillsCycle) publicArgs(label string) (string, []string) {
	key := qoderCycleRole(label)
	cwd := c.f.outside
	if key != "outside" {
		cwd = c.recipes[key].Project
	}
	if strings.HasPrefix(label, "git-") {
		return cwd, []string{c.f.git, "-c", "core.hooksPath=/dev/null", "init", "--quiet"}
	}
	if label == "version" {
		return cwd, []string{c.f.client, "--version"}
	}
	r := c.recipes[key]
	verb := strings.Split(label, "-")[0]
	switch label {
	case "repeat-add-a":
		verb = "add"
	case "unchanged-update-a":
		verb = "update"
	case "intact-repair-a", "missing-repair-a":
		verb = "repair"
	case "repeat-remove-a":
		verb = "remove"
	}
	selector := qoderCycleName
	if verb == "add" || label == "update-a" {
		selector = r.Source
	}
	if verb == "repair" {
		selector = r.Installation.InstallationID
	}
	args := []string{c.f.installer, verb, selector, "--scope", "project", "--project-root", cwd, "--target", "qoder", "--format", "json"}
	if verb == "add" {
		args = append(args, "--no-save")
	}
	return cwd, args
}

func (c *qoderSkillsCycle) publicCommand(t *testing.T, label string) ports.CommandResult {
	t.Helper()
	if len(c.public) >= len(qoderCyclePublic) || qoderCyclePublic[len(c.public)] != label {
		t.Fatal("Qoder public one-shot order")
	}
	cwd, args := c.publicArgs(label)
	if label == "version" && len(c.logs.runs) != 0 {
		t.Fatal("Qoder initial version is not the first native logger")
	}
	result := c.run(t, qoderCycleAggregates[0], label, cwd, args, label == "version")
	if label == "repeat-remove-a" {
		if result.ExitCode != 1 || len(result.Stdout) != 0 || string(result.Stderr) != "agentplugins: plugin is not installed for target \"qoder\" in project scope; no target was changed\n" {
			t.Fatal("Qoder exact repeated removal refusal differs")
		}
	} else if result.ExitCode != 0 {
		t.Fatal("Qoder public child failed")
	}
	return result
}

func qoderCycleHasFlag(stage string) bool {
	switch stage {
	case "empty-outside", "empty-a", "empty-b", "installed-outside", "updated-outside", "removed-outside", "final-outside":
		return false
	default:
		return true
	}
}

func (c *qoderSkillsCycle) catalog(t *testing.T, stage string) {
	t.Helper()
	if len(c.catalogs) >= len(qoderCycleCatalog) || qoderCycleCatalog[len(c.catalogs)] != stage || len(c.logs.runs) != len(c.catalogs)+1 {
		t.Fatal("Qoder fresh catalog/log order")
	}
	key := qoderCycleRole(stage)
	cwd := c.f.outside
	if key != "outside" {
		cwd = c.recipes[key].Project
	}
	args := []string{c.f.client, "plugins", "list", "--json"}
	var selected *qoderCycleRecipe
	if qoderCycleHasFlag(stage) {
		selected = c.recipes[key]
		if selected == nil || selected.View == "" {
			t.Fatal("Qoder exact current/former receipt view required")
		}
		args = append(args, "--plugin-dir", selected.View)
	}
	result := c.run(t, qoderCycleAggregates[1], stage, cwd, args, true)
	if result.ExitCode != 0 || len(result.Stderr) != 0 {
		t.Fatal("Qoder catalog native process/diagnostics")
	}
	if err := c.catalogJSON(result.Stdout, selected); err != nil {
		t.Fatal(err)
	}
}

func (c *qoderSkillsCycle) catalogJSON(raw []byte, selected *qoderCycleRecipe) error {
	value, err := projectNativeRuntimeJSON(raw)
	rows, ok := value.([]any)
	if err != nil || !ok {
		return fmt.Errorf("Qoder whole catalog array required")
	}
	expected := []any{}
	if selected != nil && !selected.Removed && !selected.Missing {
		expected = append(expected, map[string]any{"id": qoderCycleName + "@flag", "name": qoderCycleName, "source": qoderCycleName + "@inline", "version": "1.0.0", "scope": "flag", "enabled": true, "canDisable": false, "installPath": selected.View, "description": qoderCycleDescription, "resources": qoderCycleResources(qoderCycleName, fmt.Sprintf("Disposable Qoder %s %s %s", selected.Key, selected.Revision, selected.Nonce))})
	}
	for _, key := range []string{"a", "b", "user"} {
		name := qoderCycleControlName(key)
		parent := c.f.base.Home
		switch key {
		case "a":
			parent = c.f.base.Project
		case "b":
			parent = c.f.other
		}
		description := "Original Qoder " + key + " " + c.f.base.Nonce
		row := map[string]any{"id": name + "@local", "name": name, "source": name + "@local", "version": "0.9.0", "scope": "user", "enabled": false, "canDisable": true, "installPath": parent + "/.qoder/plugins/" + name, "description": description, "resources": qoderCycleResources(name, description)}
		if key != "user" {
			row["scope"], row["projectPath"] = "project", parent
		}
		expected = append(expected, row)
	}
	if !reflect.DeepEqual(rows, expected) {
		return fmt.Errorf("Qoder exact original/flag catalog resource identity differs")
	}
	return nil
}

func qoderCycleResources(name, description string) map[string]any {
	return map[string]any{"skills": []any{map[string]any{"name": name, "description": description}}, "agents": []any{}, "mcpServers": []any{}, "commands": []any{}, "hooks": []any{}}
}

func qoderCyclePublicJSON(raw []byte, verb string, mutated bool) error {
	if err := clineCyclePublicJSON(raw, mutated); err != nil {
		return err
	}
	value, err := projectNativeRuntimeJSON(raw)
	row, ok := value.(map[string]any)
	if err != nil || !ok || projectNativeRuntimeKeys(row, "schema_version command result data", "") != nil || row["schema_version"] != json.Number("1") || row["command"] != verb || row["result"] != "success" {
		return fmt.Errorf("Qoder exact public success envelope")
	}
	return nil
}

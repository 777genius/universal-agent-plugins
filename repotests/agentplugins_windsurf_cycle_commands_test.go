//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/process"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func windCyclePublic(label string) bool {
	return strings.HasPrefix(label, "add-") || strings.HasPrefix(label, "repeat-") || strings.Contains(label, "update-a") || strings.Contains(label, "repair-a") || label == "remove-a" || label == "remove-b"
}

func windCycleMutated(label string) bool {
	return windCyclePublic(label) && label != "repeat-add-a" && label != "unchanged-update-a" && label != "repeat-remove-a"
}

func (c *windProjectCycle) selectCommand(label string) (windComponentCommand, error) {
	for _, old := range windComponentLabels {
		if old == label {
			return c.w.selectCommand(label)
		}
	}
	known := false
	for _, name := range windCycleLabels {
		known = known || label == name
	}
	if !known {
		return windComponentCommand{}, fmt.Errorf("Windsurf cycle unregistered command")
	}
	scope := "a"
	if strings.Contains(label, "-b-") || strings.HasSuffix(label, "-b") {
		scope = "b"
	}
	r := c.w.recipes[scope]
	command := windComponentCommand{CWD: r.Project, Native: !windCyclePublic(label), Scope: scope, Timeout: 10 * time.Second}
	if command.Native {
		args := []string{"skills", "list", "--json"}
		if strings.HasSuffix(label, "-get") {
			args = []string{"mcp", "get", windComponentName}
		}
		if strings.HasSuffix(label, "-mcp") {
			args = []string{"mcp", "list"}
		}
		command.Argv = append([]string{c.w.f.client}, args...)
	} else {
		verb := "remove"
		switch label {
		case "repeat-add-a":
			verb = "add"
		case "unchanged-update-a", "update-a":
			verb = "update"
		case "intact-repair-a", "missing-repair-a":
			verb = "repair"
		}
		selector := r.Installation.InstallationID
		if verb == "add" {
			selector = r.Source
		}
		command.Timeout = 30 * time.Second
		command.Argv = []string{c.w.f.installer, verb, selector}
		if verb == "add" {
			command.Argv = append(command.Argv, "--no-save")
		}
		command.Argv = append(command.Argv, "--scope", "project", "--target", "windsurf", "--format", "json", "--project-root", r.Project)
	}
	return command, nil
}

func (c *windProjectCycle) command(t *testing.T, label string) {
	t.Helper()
	w, f := c.w, c.w.f
	if len(c.completed) >= len(windCycleLabels) || windCycleLabels[len(c.completed)] != label || f.used[label] {
		t.Fatal("Windsurf closed lifecycle prefix")
	}
	if label == "update-a" {
		c.revise(t, time.Now())
	}
	if label == "damaged-a-skills" {
		c.damage(t)
	}
	command, err := c.selectCommand(label)
	if err != nil {
		t.Fatal(err)
	}
	w.stage = label
	f.evidence["attempt_stage"] = label
	w.check(t)
	before := w.profile(t)
	f.used[label] = true
	ctx, cancel := context.WithTimeout(f.ctx, command.Timeout)
	defer cancel()
	start := time.Now()
	result, runErr := (process.OS{}).RunWithDescendantExitGrace(ctx, ports.Command{Argv: command.Argv, Dir: command.CWD, Env: f.env(), StdoutLimitBytes: 1 << 20}, 5*time.Second)
	end := time.Now()
	c.append(t, windCycleMembers[0], label, map[string]any{"argv": command.Argv, "cwd": command.CWD, "exit": result.ExitCode, "error": runErr != nil, "deadline": ctx.Err() != nil, "elapsed_ms": end.Sub(start).Milliseconds(), "started_at": start.UTC().Format(time.RFC3339Nano), "ended_at": end.UTC().Format(time.RFC3339Nano), "cleanup": "full descendant supervision; forced cleanup fails", "stdout": windCycleStream(result.Stdout), "stderr": windCycleStream(result.Stderr)})
	c.completed = append(c.completed, label)
	if runErr != nil || ctx.Err() != nil || len(result.Stdout) > 1<<20 || len(result.Stderr) > 1<<20 || bytes.Contains(result.Stderr, []byte("stderr bytes omitted")) {
		t.Fatal("Windsurf lifecycle process/capture supervision failed")
	}
	if label == "repeat-remove-a" {
		if result.ExitCode != 1 || len(result.Stdout) != 0 || string(result.Stderr) != "agentplugins: plugin is not installed for target \"windsurf\" in project scope; no target was changed\n" {
			t.Fatal("Windsurf repeated removal boundary")
		}
	} else if result.ExitCode != 0 {
		t.Fatal("Windsurf lifecycle child failed")
	}
	after := w.profile(t)
	if command.Native {
		valid := reflect.DeepEqual(before, after)
		if label != "version" {
			valid = w.runtimeClosure(before, after, start, end) == nil
		}
		if !valid {
			t.Fatal("Windsurf lifecycle native profile closure")
		}
		if len(result.Stderr) != 0 {
			t.Fatal("Windsurf lifecycle native diagnostics")
		}
		if label != "version" {
			w.freezeRuntime(after)
		}
		if err := c.native(label, result.Stdout); err != nil {
			t.Fatal(err)
		}
	} else if windCyclePublic(label) {
		if err := windPublicProfileClosure(before, after, command.Scope, start, end); err != nil {
			t.Fatal(err)
		}
		c.publicProof(t, label, w.recipes[command.Scope], result, start, end)
	} else {
		if len(result.Stdout) != 0 || len(result.Stderr) != 0 || !reflect.DeepEqual(before, after) {
			t.Fatal("Windsurf lifecycle Git diagnostics/profile")
		}
		if err := w.gitFreeze(label); err != nil {
			t.Fatal(err)
		}
	}
	probe, err := projectNativeRuntimeRead(f.base.Events, 1<<20)
	if err != nil || len(probe) != 0 || windCycleProbe(t, c) != c.probeRecord {
		t.Fatal("Windsurf metadata commands unexpectedly invoked MCP")
	}
	c.append(t, windCycleMembers[2], label, map[string]any{"events": windCycleStream(probe)})
	w.check(t)
}

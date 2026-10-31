//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/process"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type copilotCycleCommand struct {
	Argv         []string `json:"argv"`
	CWD          string   `json:"cwd"`
	Exit         int      `json:"exit"`
	Elapsed      int64    `json:"elapsed_ms"`
	Error        bool     `json:"error"`
	ContextError bool     `json:"context_error"`
	Cleanup      string   `json:"cleanup"`
}

func (c *copilotProjectCycle) run(t *testing.T, cwd string, env, argv []string, timeout time.Duration) (ports.CommandResult, copilotCycleCommand, time.Time, time.Time) {
	t.Helper()
	c.check(t)
	ctx, cancel := context.WithTimeout(c.f.ctx, timeout)
	defer cancel()
	start := time.Now().UTC()
	result, err := (process.OS{}).RunWithDescendantExitGrace(ctx, ports.Command{Argv: argv, Env: env, Dir: cwd, StdoutLimitBytes: 1 << 20}, 5*time.Second)
	end := time.Now().UTC()
	record := copilotCycleCommand{Argv: slices.Clone(argv), CWD: cwd, Exit: result.ExitCode, Elapsed: end.Sub(start).Milliseconds(), Error: err != nil, ContextError: ctx.Err() != nil,
		Cleanup: "ordinary full-descendant supervision; forced cleanup is a failure"}
	return result, record, start, end
}

func copilotCycleCommandClean(result ports.CommandResult, record copilotCycleCommand) bool {
	return !record.Error && !record.ContextError && len(result.Stdout) <= 1<<20 && len(result.Stderr) <= 1<<20 && !bytes.Contains(result.Stderr, []byte("stderr bytes omitted"))
}

func (c *copilotProjectCycle) publicCommand(t *testing.T, label, key string) ports.CommandResult {
	t.Helper()
	r := c.recipes[key]
	expectedKey := "user"
	if strings.HasSuffix(label, "-a") {
		expectedKey = "a"
	} else if strings.HasSuffix(label, "-b") {
		expectedKey = "b"
	}
	if r == nil || key != expectedKey || len(c.public) >= len(copilotCyclePublic) || copilotCyclePublic[len(c.public)] != label {
		t.Fatal("Copilot cycle public command owner/order differs")
	}
	cwd := r.Project
	var argv []string
	switch label {
	case "git-a", "git-b":
		argv = []string{c.f.git, "-c", "core.hooksPath=/dev/null", "init", "--quiet"}
	case "version":
		cwd, argv = c.f.outside, []string{c.f.client, "--version"}
	default:
		verb := strings.Split(label, "-")[0]
		switch label {
		case "repeat-add-a":
			verb = "add"
		case "unchanged-update-a":
			verb = "update"
		case "intact-repair-a", "missing-repair-a":
			verb = "repair"
		case "repeat-remove-a", "repeat-remove-b", "repeat-remove-user":
			verb = "remove"
		}
		selector := "native-proof"
		if verb == "add" || label == "update-a" {
			selector = r.Source
		}
		argv = []string{c.f.installer, verb, selector, "--scope", copilotCycleScope(r), "--target", "copilot", "--format", "json"}
		if r.Key != "user" {
			argv = append(argv, "--project-root", r.Project)
		}
		if verb == "add" {
			argv = append(argv, "--no-save")
		}
	}
	c.f.evidence["cycle_stage"] = "public:" + label
	result, record, _, _ := c.run(t, cwd, c.f.env(), argv, 30*time.Second)
	copilotCycleSafe(t, result.Stdout)
	copilotCycleSafe(t, result.Stderr)
	raw, _ := json.Marshal(record)
	c.f.keep(t, label+".command.json", raw)
	c.f.keep(t, label+".stdout.log", result.Stdout)
	c.f.keep(t, label+".stderr.log", result.Stderr)
	c.public = append(c.public, label)
	c.check(t)
	if !copilotCycleCommandClean(result, record) {
		t.Fatal("Copilot cycle public capture/context/ordinary cleanup failed")
	}
	if strings.HasPrefix(label, "repeat-remove-") {
		want := fmt.Sprintf("agentplugins: plugin is not installed for target %q in %s scope; no target was changed\n", "copilot", copilotCycleScope(r))
		if result.ExitCode != 1 || len(result.Stdout) != 0 || string(result.Stderr) != want {
			t.Fatal("Copilot cycle repeated removal exact supported refusal differs")
		}
	} else if result.ExitCode != 0 {
		t.Fatal("Copilot cycle public command failed")
	}
	return result
}

func (c *copilotProjectCycle) registryList(t *testing.T, stage string) {
	t.Helper()
	if len(c.registry) >= len(copilotCycleRegistry) || copilotCycleRegistry[len(c.registry)] != stage {
		t.Fatal("Copilot cycle native registry stage/order differs")
	}
	c.f.evidence["cycle_stage"] = "registry:" + stage
	result, record, start, end := c.run(t, c.f.outside, c.f.env(), []string{c.f.client, "plugin", "list"}, 30*time.Second)
	c.append(t, "cycle-registry-listings.jsonl", stage, map[string]any{"command": record, "started_at": start.Format(time.RFC3339Nano), "ended_at": end.Format(time.RFC3339Nano),
		"stdout": copilotCycleStream(t, result.Stdout), "stderr": copilotCycleStream(t, result.Stderr)})
	c.registry = append(c.registry, stage)
	c.check(t)
	if !copilotCycleCommandClean(result, record) || result.ExitCode != 0 || len(result.Stderr) != 0 {
		t.Fatal("Copilot cycle native registry process/capture failed")
	}
	want := "No plugins installed.\n\nUse 'copilot plugin install <source>' to install a plugin.\nBuilt-in plugins:\n  • browser-use (v0.0.4) [disabled]\n"
	u := c.recipes["user"]
	if u.Installation.InstallationID != "" && !u.Removed {
		market := "agentplugins-" + copilotProjectConfigHash([]byte(u.Binding.PhysicalArtifact))[:12]
		want = "Built-in plugins:\n  • browser-use (v0.0.4) [disabled]\nLive Plugins (loaded from a local marketplace directory, never copied):\n  • native-proof@" + market + " (v1.0.0) (enabled)\n      from " + u.Active + "\n"
	}
	if string(result.Stdout) != want {
		t.Fatal("Copilot cycle original user registry authority differs")
	}
}

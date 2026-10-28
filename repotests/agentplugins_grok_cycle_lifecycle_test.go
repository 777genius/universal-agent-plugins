//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/process"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func (c *grokProjectCycle) execute(t *testing.T, member, label, cwd string, argv []string) ports.CommandResult {
	t.Helper()
	c.check(t)
	ctx, cancel := context.WithTimeout(c.f.ctx, 30*time.Second)
	defer cancel()
	started := time.Now()
	result, err := (process.OS{}).RunWithDescendantExitGrace(ctx, ports.Command{Argv: argv, Env: c.f.env(), Dir: cwd, StdoutLimitBytes: 1 << 20}, 5*time.Second)
	meta := map[string]any{"argv": argv, "cwd": cwd, "exit": result.ExitCode, "error": err != nil, "deadline": ctx.Err() != nil, "elapsed_ms": time.Since(started).Milliseconds(), "cleanup": "full descendant supervision; forced cleanup fails", "stdout": c.stream(t, result.Stdout), "stderr": c.stream(t, result.Stderr)}
	c.append(t, member, label, meta)
	if member == "grok-cycle-public.jsonl" {
		c.public = append(c.public, label)
	} else {
		c.inspections = append(c.inspections, label)
	}
	c.check(t)
	if err != nil || ctx.Err() != nil || len(result.Stdout) > 1<<20 || bytes.Contains(result.Stderr, []byte("stderr bytes omitted")) || result.ExitCode < 0 {
		t.Fatal("Grok cycle child/capture/ordinary cleanup failed")
	}
	if label == "repeat-remove-a" {
		if result.ExitCode != 1 || len(result.Stdout) != 0 || string(result.Stderr) != "agentplugins: plugin is not installed for target \"grok\" in project scope; no target was changed\n" {
			t.Fatal("Grok cycle exact repeated-removal refusal differs")
		}
	} else if result.ExitCode != 0 {
		t.Fatal("Grok cycle command exited nonzero")
	}
	return result
}

func (c *grokProjectCycle) publicCommand(t *testing.T, label string) ports.CommandResult {
	t.Helper()
	if len(c.public) >= len(grokCyclePublic) || grokCyclePublic[len(c.public)] != label {
		t.Fatal("Grok fixed public command order differs")
	}
	f := c.f
	scope := "a"
	if strings.HasSuffix(label, "-b") {
		scope = "b"
	}
	r, cwd := c.recipes[scope], c.recipes[scope].Project
	var argv []string
	switch label {
	case "git-a", "git-b", "git-outside":
		if label == "git-outside" {
			cwd = f.outside
		}
		argv = []string{f.git, "-c", "core.hooksPath=/dev/null", "init", "--quiet"}
	case "version":
		cwd, argv = f.outside, []string{f.client, "--version"}
	default:
		verb, selector := strings.Split(label, "-")[0], r.Name
		switch label {
		case "repeat-add-a":
			verb = "add"
		case "unchanged-update-a":
			verb = "update"
		case "intact-repair-a", "missing-repair-a":
			verb, selector = "repair", r.Installation.InstallationID
		case "repeat-remove-a":
			verb = "remove"
		}
		if verb == "add" || label == "update-a" {
			selector = r.Source
		}
		argv = []string{f.installer, verb, selector}
		if verb == "add" {
			argv = append(argv, "--no-save")
		}
		argv = append(argv, "--scope", "project", "--project-root", r.Project, "--target", "grok", "--format", "json")
	}
	return c.execute(t, "grok-cycle-public.jsonl", label, cwd, argv)
}

func (c *grokProjectCycle) inspect(t *testing.T, stage, scope string) {
	t.Helper()
	if len(c.inspections) >= len(grokCycleInspections) || grokCycleInspections[len(c.inspections)] != stage {
		t.Fatal("Grok fixed fresh inspection order differs")
	}
	cwd := c.f.outside
	if scope != "outside" {
		cwd = c.recipes[scope].Project
	}
	result := c.execute(t, "grok-cycle-inspect.jsonl", stage, cwd, []string{c.f.client, "inspect", "--json"})
	if len(result.Stderr) != 0 || grokCycleInspect(result.Stdout, cwd, scope != "outside", c.expectedInspection(scope, stage == "damaged-a")) != nil {
		t.Fatal("Grok exact fresh skill/source inspection differs")
	}
}

func grokCycleProgress(stage, name string) string {
	switch stage {
	case "add-a", "add-b":
		return "Resolving and validating one Agent Plugin package for every selected target...\nInstalling " + name + " for 1 selected clients...\n"
	case "repeat-add-a":
		return "Resolving and validating one Agent Plugin package for every selected target...\nApplying transactional client package...\n"
	case "update-a", "unchanged-update-a":
		return "Resolving and validating one updated Agent Plugin package for every selected target...\n"
	case "intact-repair-a", "missing-repair-a":
		return "Resolving and validating each unique exact installed package revision once...\n"
	default:
		return ""
	}
}

func (c *grokProjectCycle) apply(t *testing.T, stage, scope string, mutated bool) {
	t.Helper()
	r := c.recipes[scope]
	c.check(t)
	statePath := filepath.Join(r.Project, ".uap", "state-v2.json")
	stableState, stableActive := c.guards[statePath], c.guards[r.Active]
	delete(c.guards, statePath)
	delete(c.absent, statePath)
	if r.Active != "" {
		delete(c.guards, r.Active)
	}
	if stage == "missing-repair-a" {
		delete(c.absent, r.Active)
	}
	if strings.HasPrefix(stage, "remove-") {
		delete(c.absent, filepath.Join(r.Project, ".uap", "plugin-data", r.Binding.PhysicalArtifact))
	}
	result := c.publicCommand(t, stage)
	if stage != "repeat-remove-a" {
		value, err := projectNativeRuntimeJSON(result.Stdout)
		document, ok := value.(map[string]any)
		if err != nil || !ok || nativeValidateInstallerJSON(result.Stdout) != nil || string(result.Stderr) != grokCycleProgress(stage, r.Name) {
			t.Fatal("Grok exact public success/progress differs")
		}
		commandsRequireMutated(t, stage, document, mutated)
	}
	if strings.HasPrefix(stage, "remove-") {
		r.Removed = true
	}
	c.proof(t, r, stage)
	if !mutated {
		if !reflect.DeepEqual(c.guards[statePath], stableState) || !reflect.DeepEqual(c.guards[r.Active], stableActive) {
			t.Fatal("Grok no-op/repeated removal changed exact held state/package identity")
		}
	}
}

func (c *grokProjectCycle) reviseA(t *testing.T) {
	t.Helper()
	r := c.recipes["a"]
	c.proof(t, r, "before-source-revision-a")
	c.check(t)
	delete(c.guards, r.Source)
	r.Revision = "a-two"
	c.author(t, r, true)
	c.check(t)
}

func (c *grokProjectCycle) missingA(t *testing.T) {
	t.Helper()
	r := c.recipes["a"]
	c.proof(t, r, "before-missing-a")
	c.check(t)
	before, err := grokCycleSnapshot(r.Active)
	if err != nil || !reflect.DeepEqual(before, c.guards[r.Active]) || grokCyclePayload(before, grokCycleProjected(r)) != nil {
		t.Fatal("Grok sole owned package fault authority differs", err)
	}
	delete(c.guards, r.Active)
	for _, relative := range []string{"plugin.json", "skills/" + r.Name + "/SKILL.md", ".grok-plugin/plugin.json", "skills/" + r.Name, "skills", ".grok-plugin", "."} {
		path := filepath.Join(r.Active, filepath.FromSlash(relative))
		entry := before[relative]
		info, err := os.Lstat(path)
		if err != nil {
			t.Fatal(err)
		}
		stat := info.Sys().(*syscall.Stat_t)
		if info.Mode()&os.ModeSymlink != 0 || stat.Ino != entry.Inode || stat.Dev != entry.Device || stat.Uid != entry.UID || stat.Gid != entry.GID || uint32(info.Mode().Perm()) != entry.Mode {
			t.Fatal("Grok held owned fault entry identity changed")
		}
		if entry.Content != nil {
			body, err := projectNativeRuntimeRead(path, 1<<20)
			if err != nil || !bytes.Equal(body, entry.Content.Body) {
				t.Fatal("Grok held owned fault file bytes changed", err)
			}
		}
		// Every directory removal is nonrecursive and refuses an extra entry.
		if err := os.Remove(path); err != nil {
			t.Fatal("Grok exact owned package absence fault", err)
		}
	}
	c.absent[r.Active] = true
	c.append(t, "grok-cycle-proof.jsonl", "damaged-a", map[string]any{"scope": "a", "removed_path": r.Active, "before": before, "absent": true, "preserved": c.preserved(t)})
	c.proofs = append(c.proofs, "damaged-a")
	c.check(t)
}

func (c *grokProjectCycle) finish(t *testing.T) {
	t.Helper()
	defer c.f.finish(t)
	defer func() {
		for _, member := range grokCycleMembers {
			c.f.keep(t, member, c.aggregates[member])
		}
		c.f.evidence["cycle_public_stages"], c.f.evidence["cycle_inspect_stages"], c.f.evidence["cycle_proof_stages"] = c.public, c.inspections, c.proofs
		c.f.evidence["cycle_observed_complete"] = c.complete && reflect.DeepEqual(c.public, grokCyclePublic) && reflect.DeepEqual(c.inspections, grokCycleInspections) && reflect.DeepEqual(c.proofs, grokCycleProofs)
	}()
	c.check(t)
	if c.f.ctx.Err() != nil {
		t.Error("Grok original work deadline expired")
	}
}

func TestAgentpluginsGrokNativeProjectCompleteCycle(t *testing.T) {
	if os.Getenv("AGENTPLUGINS_GROK_PROJECT_RUNTIME_ATTEMPT") != "1" {
		t.Skip("opt-in genuine hosted complete Grok skills cycle")
	}
	spec, err := projectNativeCLISpecification("grok")
	if err != nil {
		t.Fatal(err)
	}
	f := newProjectNativeCLIAttempt(t, spec)
	c := &grokProjectCycle{f: f, recipes: map[string]*grokCycleRecipe{}, aggregates: map[string][]byte{}, guards: map[string]map[string]grokCycleFile{}, fixed: map[string]map[string]grokCycleFile{}, absent: map[string]bool{}, public: []string{}, inspections: []string{}, proofs: []string{}}
	f.evidence["capture_mode"], f.evidence["attempt_stage"] = "grok-complete-skills-cycle", "complete-cycle"
	defer c.finish(t)
	c.prepare(t)
	for _, stage := range []string{"git-a", "git-b", "git-outside", "version"} {
		result := c.publicCommand(t, stage)
		if len(result.Stderr) != 0 || (stage != "version" && len(result.Stdout) != 0) || (stage == "version" && string(result.Stdout) != spec.versionOutput) {
			t.Fatal("Grok exact Git/version setup differs")
		}
	}
	for _, scope := range []string{"outside", "a", "b"} {
		c.inspect(t, "empty-"+scope, scope)
	}
	c.apply(t, "add-a", "a", true)
	c.apply(t, "add-b", "b", true)
	for _, scope := range []string{"a", "b", "outside"} {
		c.inspect(t, "installed-"+scope, scope)
	}
	for _, stage := range []string{"repeat-add-a", "unchanged-update-a"} {
		c.apply(t, stage, "a", false)
	}
	c.apply(t, "intact-repair-a", "a", true)
	c.inspect(t, "stable-a", "a")
	c.reviseA(t)
	c.apply(t, "update-a", "a", true)
	for _, scope := range []string{"a", "b", "outside"} {
		c.inspect(t, "updated-"+scope, scope)
	}
	c.missingA(t)
	c.inspect(t, "damaged-a", "a")
	c.apply(t, "missing-repair-a", "a", true)
	c.inspect(t, "repaired-a", "a")
	c.apply(t, "remove-a", "a", true)
	for _, pair := range [][2]string{{"removed-a", "a"}, {"remaining-b", "b"}, {"removed-outside", "outside"}} {
		c.inspect(t, pair[0], pair[1])
	}
	c.apply(t, "repeat-remove-a", "a", false)
	c.inspect(t, "repeat-removed-a", "a")
	c.apply(t, "remove-b", "b", true)
	for _, pair := range [][2]string{{"removed-b", "b"}, {"final-a", "a"}, {"final-b", "b"}, {"final-outside", "outside"}} {
		c.inspect(t, pair[0], pair[1])
	}
	c.append(t, "grok-cycle-proof.jsonl", "final", map[string]any{"preserved": c.preserved(t), "retained_a": c.guards[c.recipes["a"].Data], "retained_b": c.guards[c.recipes["b"].Data], "absent_a": c.recipes["a"].Active, "absent_b": c.recipes["b"].Active})
	c.proofs = append(c.proofs, "final")
	c.complete = true
}

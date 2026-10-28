//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/process"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func geminiCycleScope(label string) string {
	if label == "version" || strings.HasSuffix(label, "-user") {
		return "user"
	}
	if strings.HasSuffix(label, "-a") {
		return "a"
	}
	if strings.HasSuffix(label, "-b") {
		return "b"
	}
	return ""
}
func (c *geminiProjectCycle) publicArguments(label string) (string, []string, error) {
	r := c.recipes[geminiCycleScope(label)]
	if r == nil {
		return "", nil, fmt.Errorf("Gemini registered public scope")
	}
	if strings.HasPrefix(label, "git-") {
		return r.Project, []string{c.f.git, "-c", "core.hooksPath=/dev/null", "init", "--quiet"}, nil
	}
	if label == "version" {
		return c.f.outside, []string{c.f.node, c.f.client, "--version"}, nil
	}
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
	selector := geminiCycleName
	if verb == "add" || label == "update-a" {
		selector = r.Source
	}
	argv := []string{c.f.installer, verb, selector, "--scope", r.scopeName(), "--target", "gemini", "--format", "json"}
	if r.Scope != "user" {
		argv = append(argv, "--project-root", r.Project)
	}
	if verb == "add" {
		argv = append(argv, "--no-save")
	}
	return r.Project, argv, nil
}
func (c *geminiProjectCycle) command(t *testing.T, label string) ports.CommandResult {
	t.Helper()
	if len(c.public) >= len(geminiCyclePublic) || geminiCyclePublic[len(c.public)] != label {
		t.Fatal("Gemini exact public command prefix")
	}
	cwd, argv, err := c.publicArguments(label)
	if err != nil {
		t.Fatal(err)
	}
	c.stage = label
	c.f.evidence["attempt_stage"] = label
	c.check(t)
	ctx, cancel := context.WithTimeout(c.f.ctx, 30*time.Second)
	defer cancel()
	started := time.Now()
	result, runErr := (process.OS{}).RunWithDescendantExitGrace(ctx, ports.Command{Argv: argv, Env: c.f.env(), Dir: cwd, StdoutLimitBytes: 1 << 20}, 5*time.Second)
	record, _ := json.Marshal(map[string]any{"argv": argv, "cwd": cwd, "exit": result.ExitCode, "error": runErr != nil, "context_error": ctx.Err() != nil, "elapsed_ms": time.Since(started).Milliseconds(), "cleanup": "ordinary full-descendant supervision; forced cleanup is a failure"})
	for name, body := range map[string][]byte{label + ".command.json": record, label + ".stdout.log": result.Stdout, label + ".stderr.log": result.Stderr} {
		if geminiCyclePrivateCredential(body) {
			t.Fatal("Gemini cycle public privacy refusal")
		}
		c.f.keep(t, name, body)
	}
	c.public = append(c.public, label)
	if ctx.Err() != nil || len(result.Stdout) > 1<<20 || bytes.Contains(result.Stderr, []byte("stderr bytes omitted")) {
		t.Fatal("Gemini public capture/deadline/cleanup")
	}
	if label == "repeat-remove-a" {
		if !geminiCycleAbsentRemoval(result, runErr) {
			t.Fatal("Gemini exact repeated removal refusal")
		}
		return result
	}
	if runErr != nil || result.ExitCode != 0 {
		t.Fatal("Gemini public command failed")
	}
	return result
}
func geminiCycleAbsentRemoval(result ports.CommandResult, err error) bool {
	return err == nil && result.ExitCode == 1 && len(result.Stdout) == 0 && string(result.Stderr) == "agentplugins: plugin is not installed for target \"gemini\" in project scope; no target was changed\n"
}
func geminiCycleProgress(label string) string {
	switch label {
	case "add-user", "add-a", "add-b":
		return "Resolving and validating one Agent Plugin package for every selected target...\nInstalling " + geminiCycleName + " for 1 selected clients...\n"
	case "repeat-add-a":
		return "Resolving and validating one Agent Plugin package for every selected target...\nApplying transactional client package...\n"
	case "unchanged-update-a", "update-a":
		return "Resolving and validating one updated Agent Plugin package for every selected target...\n"
	case "intact-repair-a", "missing-repair-a":
		return "Resolving and validating each unique exact installed package revision once...\n"
	default:
		return ""
	}
}
func geminiCyclePublicJSON(raw []byte, mutated bool) error {
	value, err := projectNativeRuntimeJSON(raw)
	if err != nil {
		return err
	}
	if err = nativeValidateInstallerJSON(raw); err != nil {
		return err
	}
	count := 0
	var visit func(any) error
	visit = func(value any) error {
		switch node := value.(type) {
		case map[string]any:
			for key, item := range node {
				if key == "mutated" {
					count++
					if item != mutated {
						return fmt.Errorf("Gemini public mutation truth differs")
					}
				}
				if key == "no_change" && item == true && mutated {
					return fmt.Errorf("Gemini public false no-change")
				}
				if err := visit(item); err != nil {
					return err
				}
			}
		case []any:
			for _, item := range node {
				if err := visit(item); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err = visit(value); err != nil {
		return err
	}
	if count == 0 {
		return fmt.Errorf("Gemini public mutation receipt absent")
	}
	return nil
}
func (c *geminiProjectCycle) apply(t *testing.T, label, scope string, mutated bool) {
	t.Helper()
	r := c.recipes[scope]
	if r == nil || geminiCycleScope(label) != scope {
		t.Fatal("Gemini exact public selected scope")
	}
	c.check(t)
	stable := map[string]string{}
	if !mutated {
		for _, path := range []string{filepath.Join(r.StateRoot, "state-v2.json"), r.Config} {
			body, err := projectNativeRuntimeRead(path, 1<<20)
			if err != nil {
				t.Fatal(err)
			}
			stable[path] = geminiCycleSHA(body)
		}
	}
	profile, err := geminiCycleTree(c.f.ctx, r.Profile)
	if err != nil {
		t.Fatal(err)
	}
	c.profileBefore[scope] = profile
	for _, path := range []string{r.StateRoot, r.Project, filepath.Join(r.Profile, "skills")} {
		c.open[path] = true
	}
	c.open[r.Config] = true
	for _, path := range []string{filepath.Join(r.StateRoot, "state-v2.json"), r.Config} {
		delete(c.f.protected, path)
	}
	if label == "missing-repair-a" {
		delete(c.absent, filepath.Dir(r.SkillPath))
		delete(c.absent, r.SkillPath)
	}
	before := r.Installation
	result := c.command(t, label)
	for path, want := range stable {
		raw, err := projectNativeRuntimeRead(path, 1<<20)
		if err != nil || geminiCycleSHA(raw) != want {
			t.Fatal("Gemini no-change public bytes drifted")
		}
	}
	if label == "repeat-remove-a" {
		raw, err := projectNativeRuntimeRead(filepath.Join(r.StateRoot, "state-v2.json"), 1<<20)
		state, e := geminiCycleState(raw)
		if err != nil || e != nil || len(state.Installations) != 1 || !reflect.DeepEqual(before, state.Installations[0]) {
			t.Fatal("Gemini repeated removal changed retained installation")
		}
	} else if geminiCyclePublicJSON(result.Stdout, mutated) != nil || string(result.Stderr) != geminiCycleProgress(label) {
		t.Fatal("Gemini exact public result/progress")
	}
	if strings.HasPrefix(label, "remove-") {
		r.Removed = true
	}
	var assessment *geminiCycleAssessment
	if geminiCycleSourceOperation(label) {
		expected, err := geminiCycleAssessmentFor(r)
		if err != nil {
			t.Fatal(err)
		}
		assessment = &expected
	}
	c.proofWithAssessment(t, r, label, assessment)
}

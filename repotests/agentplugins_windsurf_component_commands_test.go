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

const windComponentMode = "windsurf-component-listing-first-capture"
const windComponentName = "uap-windsurf-component"

var windComponentLabels = []string{"git-a", "git-b", "version", "empty-user-skills", "empty-user-mcp", "empty-a-skills", "empty-a-mcp", "empty-b-skills", "empty-b-mcp", "add-user", "user-skills", "user-mcp", "user-get", "add-a", "a-skills", "a-mcp", "a-get", "b-before-skills", "b-before-mcp", "add-b", "b-skills", "b-mcp", "b-get"}

type windComponentCommand struct {
	CWD     string
	Argv    []string
	Timeout time.Duration
	Native  bool
	Scope   string
}

func (w *windComponentFixture) selectCommand(label string) (windComponentCommand, error) {
	f := w.f
	scope := "user"
	switch {
	case label == "git-a" || label == "add-a" || strings.HasPrefix(label, "a-") || strings.HasPrefix(label, "empty-a-"):
		scope = "a"
	case label == "git-b" || label == "add-b" || strings.HasPrefix(label, "b-") || strings.HasPrefix(label, "empty-b-"):
		scope = "b"
	}
	r := w.recipes[scope]
	if r == nil {
		return windComponentCommand{}, fmt.Errorf("closed component scope")
	}
	c := windComponentCommand{CWD: r.Project, Timeout: 10 * time.Second, Native: true, Scope: scope}
	switch label {
	case "git-a", "git-b":
		c.Native = false
		c.Timeout = 30 * time.Second
		c.Argv = []string{f.git, "-c", "core.hooksPath=/dev/null", "init", "--quiet"}
	case "version":
		c.Timeout = 30 * time.Second
		c.Argv = []string{f.client, "--version"}
	case "add-user", "add-a", "add-b":
		c.Native = false
		c.Timeout = 30 * time.Second
		c.Argv = []string{f.installer, "add", r.Source, "--no-save", "--scope", r.scopeName(), "--target", "windsurf", "--format", "json"}
		if scope != "user" {
			c.Argv = append(c.Argv, "--project-root", r.Project)
		}
	default:
		known := false
		for _, name := range windComponentLabels {
			known = known || name == label
		}
		if !known {
			return windComponentCommand{}, fmt.Errorf("unregistered component command")
		}
		switch {
		case strings.HasSuffix(label, "-skills"):
			c.Argv = []string{f.client, "skills", "list", "--json"}
		case strings.HasSuffix(label, "-mcp"):
			c.Argv = []string{f.client, "mcp", "list"}
		case strings.HasSuffix(label, "-get"):
			c.Argv = []string{f.client, "mcp", "get", windComponentName}
		default:
			return windComponentCommand{}, fmt.Errorf("unregistered component arguments")
		}
	}
	return c, nil
}
func (w *windComponentFixture) admitCommand(label string, c windComponentCommand) error {
	if len(w.completed) >= len(windComponentLabels) || windComponentLabels[len(w.completed)] != label || w.f.used[label] {
		return fmt.Errorf("component prefix/repetition")
	}
	want, err := w.selectCommand(label)
	if err != nil || !reflect.DeepEqual(c, want) {
		return fmt.Errorf("component argv/cwd/deadline differs")
	}
	return nil
}
func (w *windComponentFixture) command(t *testing.T, label string) {
	t.Helper()
	f := w.f
	c, err := w.selectCommand(label)
	if err != nil || w.admitCommand(label, c) != nil {
		t.Fatal("closed component command selection")
	}
	w.stage = label
	f.evidence["attempt_stage"] = label
	w.check(t)
	var before map[string]windProfileRecord
	public := strings.HasPrefix(label, "add-")
	if c.Native || public {
		before = w.profile(t)
	}
	f.used[label] = true
	ctx, cancel := context.WithTimeout(f.ctx, c.Timeout)
	defer cancel()
	started := time.Now()
	result, runErr := (process.OS{}).RunWithDescendantExitGrace(ctx, ports.Command{Argv: c.Argv, Dir: c.CWD, Env: f.env(), StdoutLimitBytes: 1 << 20}, 5*time.Second)
	ended := time.Now()
	f.keep(t, label+".stdout.log", result.Stdout)
	f.keep(t, label+".stderr.log", result.Stderr)
	receipt, _ := json.Marshal(map[string]any{"argv": c.Argv, "cwd": c.CWD, "exit": result.ExitCode, "error": runErr != nil, "deadline": ctx.Err() != nil, "elapsed_ms": ended.Sub(started).Milliseconds(), "cleanup": "full descendant supervision; forced cleanup fails"})
	f.keep(t, label+".command.json", receipt)
	w.completed = append(w.completed, label)
	if c.Native {
		after := w.profile(t)
		valid := reflect.DeepEqual(before, after)
		if label != "version" {
			valid = w.runtimeClosure(before, after, started, ended) == nil
		}
		if !valid {
			f.evidence["profile_delta"] = windProfileDifference(before, after)
			f.evidence["profile_delta_pages"] = windProfileDifferencePages(before, after)
			w.configDiagnostic(after)
			// Preserve the original immutable/protected fatal as well as the
			// stronger complete-frame refusal; diagnostics authorize nothing.
			f.check(t)
			t.Fatal("Windsurf component changed the complete original profile frame")
		}
		if label != "version" {
			w.freezeRuntime(after)
		}
	}
	// Selected public closure is opened only after all non-selected paths were
	// checked before the child. Complete independent proofs precede re-freezing.
	if runErr != nil || ctx.Err() != nil || result.ExitCode != 0 || len(result.Stdout) > 1<<20 || bytes.Contains(result.Stderr, []byte("stderr bytes omitted")) {
		t.Fatal("Windsurf component child/capture/cleanup failed")
	}
	if public {
		if err := windPublicProfileClosure(before, w.profile(t), c.Scope, started, ended); err != nil {
			t.Fatal("Windsurf public profile closure", err)
		}
		w.publicProof(t, w.recipes[c.Scope], result.Stdout, started, ended)
	}
	if label == "git-a" || label == "git-b" {
		if err := w.gitFreeze(label); err != nil {
			t.Fatal(err)
		}
	}
	w.check(t)
	if c.Native && len(result.Stderr) != 0 {
		t.Fatal("Windsurf component stderr requires observed raw review")
	}
	if label == "version" && string(result.Stdout) != f.spec.versionOutput {
		t.Fatal("exact frozen Windsurf version differs")
	}
	if (label == "git-a" || label == "git-b") && (len(result.Stdout) != 0 || len(result.Stderr) != 0) {
		t.Fatal("Windsurf Git setup diagnostics")
	}
}
func (w *windComponentFixture) profile(t *testing.T) map[string]windProfileRecord {
	t.Helper()
	frame, err := windProfileSnapshot(w.f.ctx, w.f.base.Root)
	if err != nil {
		w.f.evidence["profile_refusal"] = err
		t.Fatal("Windsurf complete profile inventory refused")
	}
	return frame
}
func (w *windComponentFixture) configDiagnostic(frame map[string]windProfileRecord) {
	f := w.f
	if _, exists := f.evidence["windsurf_protected_config_change"]; exists {
		return
	}
	path := filepath.Join(f.base.Root, "xdg-config", "devin", "config.json")
	record, ok := frame["xdg-config\x00devin/config.json"]
	if !ok || record.Kind != "file" || record.SHA256 == f.protected[path] {
		return
	}
	change := windsurfConfigChange{Status: "deadline"}
	if f.ctx.Err() == nil {
		change = windsurfConfigDiagnostic(f.protected[path], record.SHA256, nil, func() ([]byte, error) { return projectNativeRuntimeRead(path, windsurfConfigDiagnosticLimit) })
	}
	if raw, err := json.Marshal(change); err != nil || len(raw) > 2<<10 {
		change = windsurfConfigChange{Status: "diagnostic-bound"}
	}
	f.evidence["windsurf_protected_config_change"] = change
}

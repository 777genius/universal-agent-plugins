//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"context"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/process"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func (c *geminiProjectCycle) execute(t *testing.T, kind, stage, scope string, argv, env []string, timeout time.Duration) (ports.CommandResult, time.Time, time.Time) {
	t.Helper()
	r := c.recipes[scope]
	c.stage = kind + ":" + stage
	c.f.evidence["attempt_stage"] = c.stage
	c.check(t)
	eventBefore, eventErr := c.eventLog()
	if eventErr != nil {
		t.Fatal(eventErr)
	}
	c.openEvents = true
	ctx, cancel := context.WithTimeout(c.f.ctx, timeout)
	defer cancel()
	start := time.Now()
	result, err := (process.OS{}).RunWithDescendantExitGrace(ctx, ports.Command{Argv: argv, Env: env, Dir: r.Project, StdoutLimitBytes: 1 << 20}, 5*time.Second)
	end := time.Now()
	c.append(t, "cycle-native-commands.jsonl", c.stage, map[string]any{"kind": kind, "scope": scope, "argv": argv, "cwd": r.Project, "exit": result.ExitCode, "error": err != nil, "context_error": ctx.Err() != nil, "started_at": start.UTC().Format(time.RFC3339Nano), "finished_at": end.UTC().Format(time.RFC3339Nano), "elapsed_ms": end.Sub(start).Milliseconds(), "cleanup": "ordinary full-descendant supervision; forced cleanup is a failure", "stdout": c.stream(t, result.Stdout), "stderr": c.stream(t, result.Stderr)})
	eventAfter, eventErr := c.eventLog()
	if eventErr != nil || !bytes.HasPrefix(eventAfter, eventBefore) {
		t.Fatal("Gemini captured append-only probe stream")
	}
	probeStage := stage
	if kind == "listing" {
		probeStage = "listing:" + stage
	}
	c.append(t, "cycle-probe-events.jsonl", probeStage, map[string]any{"scope": scope, "stream": c.stream(t, eventAfter[len(eventBefore):])})
	if err != nil || ctx.Err() != nil || result.ExitCode != 0 || len(result.Stdout) > 1<<20 || bytes.Contains(result.Stderr, []byte("stderr bytes omitted")) {
		t.Fatal("Gemini cycle native capture/process/cleanup")
	}
	return result, start, end
}
func (c *geminiProjectCycle) native(t *testing.T, stage, scope string) {
	t.Helper()
	if len(c.natives) >= len(geminiCycleNative) || geminiCycleNative[len(c.natives)] != stage {
		t.Fatal("Gemini native stage prefix")
	}
	c.check(t)
	q, err := c.owner.arm(stage, scope)
	if err != nil {
		t.Fatal(err)
	}
	before, err := c.eventLog()
	if err != nil {
		t.Fatal(err)
	}
	r := c.recipes[scope]
	stateBefore := c.frozen[r.StateRoot]
	cwdBefore := c.frozen[r.Project]
	if q.operation != "none" {
		c.open[r.StateRoot] = true
		c.open[r.Project] = true
	}
	env := append(c.f.env(), "GEMINI_API_KEY="+c.f.secret, "GOOGLE_GEMINI_BASE_URL=http://"+c.owner.listener.Addr().String(), "GOOGLE_GENAI_API_VERSION=v1beta")
	argv := []string{c.f.node, c.f.client, "--prompt", q.prompt, "--approval-mode", "default", "--output-format", "stream-json", "--model", geminiProjectModel}
	result, start, end := c.execute(t, "native", stage, scope, argv, env, 60*time.Second)
	if !geminiProjectNativeStderrAllowed(result.Stderr) {
		t.Fatal("Gemini cycle unknown native stderr")
	}
	if err := c.owner.waitIdle(c.f.ctx); err != nil {
		t.Fatal(err)
	}
	q.mu.Lock()
	session, streamErr := geminiCycleStream(result.Stdout, q, c.f.base.Home, start, end)
	q.mu.Unlock()
	if streamErr != nil || c.sessions[session] {
		t.Fatal("Gemini exact fresh native stream/session", streamErr)
	}
	c.sessions[session] = true
	c.append(t, "cycle-native-events.jsonl", stage, map[string]any{"scope": scope, "session": session, "stream": c.stream(t, result.Stdout)})
	c.events(t, scope, before, start, end, q)
	if q.operation != "none" {
		stateAfter, e := geminiCycleTree(c.f.ctx, r.StateRoot)
		if e != nil {
			t.Fatal(e)
		}
		rel, _ := filepath.Rel(r.StateRoot, r.Data)
		if err := geminiCycleMarkerDelta(stateBefore, stateAfter, rel, r, q.operation == "write"); err != nil {
			t.Fatal(err)
		}
		cwdAfter, e := geminiCycleTree(c.f.ctx, r.Project)
		if e != nil {
			t.Fatal(e)
		}
		prefix, _ := filepath.Rel(r.Project, r.Data)
		if err := geminiCycleMarkerDelta(cwdBefore, cwdAfter, prefix, r, q.operation == "write" && r.Scope != "user"); err != nil {
			t.Fatal(err)
		}
		if _, e := c.dataProof(r, r.Installation, r.Binding.DataReceiptID, r.Binding.PhysicalArtifact); e != nil {
			t.Fatal(e)
		}
		// Freeze the exact proven maps, after confirming they still equal held
		// snapshots. An unchecked reread never becomes a replacement baseline.
		for path, want := range map[string]map[string]geminiCycleRecord{r.StateRoot: stateAfter, r.Project: cwdAfter} {
			again, e := geminiCycleTree(c.f.ctx, path)
			if e != nil || !reflect.DeepEqual(want, again) {
				t.Fatal("Gemini data changed during native proof")
			}
			c.frozen[path] = want
			delete(c.open, path)
		}
	}
	c.owner.complete(t, stage)
	c.natives = append(c.natives, stage)
	c.check(t)
}
func (c *geminiProjectCycle) listing(t *testing.T, stage, scope string) {
	t.Helper()
	if len(c.listings) >= len(geminiCycleListings) || geminiCycleListings[len(c.listings)] != stage {
		t.Fatal("Gemini exact discovery prefix")
	}
	before, err := c.eventLog()
	if err != nil {
		t.Fatal(err)
	}
	result, start, end := c.execute(t, "listing", stage, scope, []string{c.f.node, c.f.client, "skills", "list", "--all"}, c.f.env(), 10*time.Second)
	if !geminiCycleListingStderr(result.Stderr, scope, c.advertised(scope)) {
		t.Fatal("Gemini cycle unknown skill catalog diagnostic")
	}
	if err := c.skills(result.Stdout, scope); err != nil {
		t.Fatal(err)
	}
	c.events(t, scope, before, start, end, nil)
	c.append(t, "cycle-discovery.jsonl", stage, map[string]any{"scope": scope, "stdout": c.stream(t, result.Stdout), "stderr": c.stream(t, result.Stderr)})
	c.listings = append(c.listings, stage)
	c.check(t)
}
func geminiCycleListingStderr(body []byte, scope string, advertised []*geminiCycleRecipe) bool {
	names := map[string]string{"user": "uapgeminiu", "a": "uapgeminia", "b": "uapgeminib"}
	if len(body) > 1024 || len(advertised) > 2 || names[scope] == "" {
		return false
	}
	seen := map[string]bool{}
	pairs := []string{}
	for _, recipe := range advertised {
		if recipe == nil || recipe.Name == "" || recipe.Name != names[recipe.Scope] || recipe.Scope != "user" && recipe.Scope != scope || seen[recipe.Name] || recipe.Removed || recipe.Binding.ClientBindingID == "" {
			return false
		}
		seen[recipe.Name] = true
		pairs = append(pairs, "Registering notification handlers for server '"+recipe.Name+"'. Capabilities: { tools: {} }\n"+
			"Server '"+recipe.Name+"' has tools but did not declare 'listChanged' capability. Listening anyway for robustness...\n")
	}
	const ripgrep = "Ripgrep is not available. Falling back to GrepTool.\n"
	const refresh = "Scheduling MCP context refresh...\nExecuting MCP context refresh...\nMCP context refresh complete.\n"
	switch string(body) {
	case "", ripgrep, ripgrep + refresh:
		return true
	}
	if len(pairs) == 1 {
		return string(body) == ripgrep+pairs[0]+refresh
	}
	if len(pairs) == 2 {
		return string(body) == ripgrep+pairs[0]+pairs[1]+refresh || string(body) == ripgrep+pairs[1]+pairs[0]+refresh
	}
	return false
}
func (c *geminiProjectCycle) git(t *testing.T, scope string) {
	t.Helper()
	r := c.recipes[scope]
	before := c.frozen[r.Project]
	c.open[r.Project] = true
	result := c.command(t, "git-"+scope)
	if len(result.Stdout) != 0 || len(result.Stderr) != 0 {
		t.Fatal("Gemini exact Git initialization")
	}
	after, err := geminiCycleTree(c.f.ctx, r.Project)
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := before[".git"]; exists || after[".git"].Mode&0170000 != 0040000 {
		t.Fatal("Gemini fresh Git fixture authority")
	}
	for key := range geminiCycleKeys(before, after) {
		a, had := before[key]
		b, has := after[key]
		if had && has && a == b {
			continue
		}
		if key == "." && had && has && geminiCycleSameDirectory(a, b) {
			continue
		}
		if key == ".git" || len(key) > 5 && key[:5] == ".git/" {
			continue
		}
		t.Fatal("Gemini Git setup escaped exact .git closure")
	}
	c.frozen[r.Project] = after
	delete(c.open, r.Project)
	c.check(t)
}

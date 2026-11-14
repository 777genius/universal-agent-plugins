//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/process"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func (c *clineProjectCycle) listing(t *testing.T, stage, scope string, skills []string) {
	t.Helper()
	if len(c.listings) >= len(clineCycleListings) || clineCycleListings[len(c.listings)] != stage {
		t.Fatal("Cline listing stage order")
	}
	r := c.recipes[scope]
	if r == nil || clineCycleScope(stage) != scope {
		t.Fatal("Cline listing stage scope differs from fixed table")
	}
	for _, kind := range []string{"skills", "mcp"} {
		c.f.check(t)
		argv := []string{c.f.client, "--auto-approve", "false", "--cwd", r.Project, "config", kind, "--json"}
		env := c.f.env(false)
		if scope != "user" {
			env = append(env, "CLINE_MCP_SETTINGS_PATH="+r.Config)
		}
		ctx, cancel := context.WithTimeout(c.f.ctx, 30*time.Second)
		result, err := (process.OS{}).RunWithDescendantExitGrace(ctx, ports.Command{Argv: argv, Env: env, Dir: r.Project, StdoutLimitBytes: 1 << 20}, 5*time.Second)
		deadline := ctx.Err() != nil
		cancel()
		c.append(t, "cycle-discovery.jsonl", stage, map[string]any{"kind": kind, "argv": argv, "cwd": r.Project, "mcp_settings_override": scope != "user", "stdout": c.stream(t, result.Stdout), "stderr": c.stream(t, result.Stderr), "exit": result.ExitCode, "error": err != nil, "deadline": deadline})
		c.f.check(t)
		if err != nil || deadline || result.ExitCode != 0 || len(result.Stderr) != 0 {
			t.Fatal("Cline exact native config process/capture/cleanup")
		}
		if kind == "skills" {
			err = c.skills(result.Stdout, r, skills)
		} else {
			err = c.mcp(result.Stdout, r)
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	c.listings = append(c.listings, stage)
}
func clineCycleRows(raw []byte) ([]any, error) {
	v, err := projectNativeRuntimeJSON(raw)
	rows, ok := v.([]any)
	if err != nil || !ok || len(rows) > 128 {
		return nil, fmt.Errorf("Cline bounded native listing array required")
	}
	return rows, nil
}
func (c *clineProjectCycle) skills(raw []byte, r *clineCycleRecipe, selected []string) error {
	rows, err := clineCycleRows(raw)
	if err != nil {
		return err
	}
	want := map[string]string{}
	for _, scope := range []string{"user", r.Scope} {
		q := c.recipes[scope]
		root := q.Project
		if scope == "user" {
			root = c.f.base.Home
		}
		want[filepath.Join(root, ".cline", "skills", "foreign-"+scope, "SKILL.md")] = string([]byte("---\nname: foreign-" + scope + "\ndescription: Foreign inert marker\n---\nFOREIGN_" + scope + "\n"))
	}
	for _, scope := range selected {
		q := c.recipes[scope]
		want[q.SkillPath] = string(q.Authored["skills/"+q.Skill+"/SKILL.md"])
	}
	if len(rows) != len(want) {
		return fmt.Errorf("Cline exact native skill inventory count")
	}
	seen := map[string]bool{}
	for _, v := range rows {
		row, ok := v.(map[string]any)
		if !ok || projectNativeRuntimeKeys(row, "name description instructions path frontmatter", "") != nil {
			return fmt.Errorf("Cline native skill row grammar")
		}
		path, ok := row["path"].(string)
		source, exists := want[path]
		if !ok || !exists || seen[path] || filepath.Clean(path) != path || !filepath.IsAbs(path) {
			return fmt.Errorf("Cline native skill path/sibling/absence differs")
		}
		name, nameOK := row["name"].(string)
		description, descriptionOK := row["description"].(string)
		instructions, instructionsOK := row["instructions"].(string)
		front, ok := row["frontmatter"].(map[string]any)
		if !nameOK || !descriptionOK || !instructionsOK || !ok || !reflect.DeepEqual(front, map[string]any{"name": name, "description": description}) || !projectNativeRuntimeText(instructions, 1<<20) {
			return fmt.Errorf("Cline native skill parsed marker grammar")
		}
		parts := strings.SplitN(source, "---\n", 3)
		if len(parts) != 3 || strings.TrimSpace(instructions) != strings.TrimSpace(parts[2]) || !strings.Contains(parts[1], "name: "+name+"\n") || !strings.Contains(parts[1], "description: "+description+"\n") {
			return fmt.Errorf("Cline native skill content differs from independent recipe")
		}
		seen[path] = true
	}
	return nil
}
func (c *clineProjectCycle) mcp(raw []byte, r *clineCycleRecipe) error {
	rows, err := clineCycleRows(raw)
	if err != nil {
		return err
	}
	expected := map[string]any{"foreign-" + r.Scope: map[string]any{"name": "foreign-" + r.Scope, "path": r.Config, "disabled": true, "transportType": "streamableHttp"}}
	if r.Active != "" && !r.Removed && r.Scope != "user" {
		expected["uapclinea"] = map[string]any{"name": "uapclinea", "path": r.Config, "disabled": false, "transportType": "stdio"}
	}
	if len(rows) != len(expected) {
		return fmt.Errorf("Cline native MCP exact selected source count")
	}
	seen := map[string]bool{}
	for _, v := range rows {
		row, ok := v.(map[string]any)
		if !ok {
			return fmt.Errorf("Cline native MCP row object")
		}
		name, ok := row["name"].(string)
		if !ok || seen[name] || !reflect.DeepEqual(row, expected[name]) {
			return fmt.Errorf("Cline native MCP exact selected path/transport/absence")
		}
		seen[name] = true
	}
	return nil
}
func clineCycleEventBytes(path string) ([]byte, error) {
	raw, err := projectNativeRuntimeRead(path, 1<<20)
	if os.IsNotExist(err) {
		return []byte{}, nil
	}
	return raw, err
}
func (c *clineProjectCycle) native(t *testing.T, stage, scope string) {
	t.Helper()
	if len(c.sessions) >= len(clineCycleACP) || clineCycleACP[len(c.sessions)] != stage {
		t.Fatal("Cline native cycle stage order")
	}
	r := c.recipes[scope]
	if r == nil || clineCycleScope(stage) != scope {
		t.Fatal("Cline ACP stage scope differs from fixed table")
	}
	c.f.check(t)
	prior, err := clineCycleEventBytes(c.f.base.Events)
	if err != nil {
		t.Fatal(err)
	}
	q, err := c.owner.arm(stage, r)
	if err != nil {
		t.Fatal(err)
	}
	dataBefore := c.nativeDataBefore(t, r, q)
	base := *c.f.base
	base.Project = r.Project
	base.Nonce = r.Nonce
	f := &clineProjectAttempt{base: &base, provider: q, evidence: map[string]any{}, active: r.Active, data: r.Data}
	argv := []string{c.f.client, "--auto-approve", "false", "--cwd", r.Project, "--acp"}
	env := append(c.f.env(false), "CLINE_MCP_SETTINGS_PATH="+r.Config, "CLINE_PROVIDER="+clineAttemptProviderID, "CLINE_MODEL="+clineAttemptModel, "CLINE_API_KEY="+c.f.key)
	var transcript bytes.Buffer
	f.nativeStart = time.Now()
	err = (process.OS{}).RunDuplexWithPlannedShutdown(c.f.ctx, ports.Command{Argv: argv, Env: env, Dir: r.Project, StdoutLimitBytes: 1 << 20}, func(w io.Writer, reader io.Reader) error { return f.acp(w, reader, &transcript) })
	f.nativeEnd = time.Now()
	c.append(t, "cycle-native-acp.jsonl", stage, c.stream(t, transcript.Bytes()))
	c.append(t, "cycle-native-commands.jsonl", stage, map[string]any{"argv": argv, "cwd": r.Project, "mcp_settings_override": r.Config, "auto_approve": false, "injected_mcp_servers": []any{}, "error": err != nil, "deadline": c.f.ctx.Err() != nil, "error_stream": c.stream(t, clineCycleError(err)), "started_at": f.nativeStart.UTC().Format(time.RFC3339Nano), "ended_at": f.nativeEnd.UTC().Format(time.RFC3339Nano), "session": f.evidence["native_session_id"], "cleanup": "existing OS strict planned duplex shutdown; entire delegated descendant cgroup"})
	c.f.check(t)
	if err != nil || c.f.ctx.Err() != nil {
		t.Fatal("Cline cycle native ACP process/grammar/cleanup")
	}
	c.nativeDataAfter(t, r, q, dataBefore)
	after, err := clineCycleEventBytes(c.f.base.Events)
	if err != nil || !bytes.HasPrefix(after, prior) {
		t.Fatal("Cline cycle prior probe evidence changed")
	}
	events := after[len(prior):]
	c.append(t, "cycle-probe-events.jsonl", stage, c.stream(t, events))
	session, ok := f.evidence["native_session_id"].(string)
	if !ok || c.sessionIDs[session] {
		t.Fatal("Cline cycle native session reused")
	}
	c.sessionIDs[session] = true
	c.verifyNative(t, r, f, events)
	c.owner.complete(t, c, stage)
	c.sessions = append(c.sessions, stage)
}
func clineCycleError(err error) []byte {
	if err == nil {
		return nil
	}
	return []byte(err.Error())
}
func (c *clineProjectCycle) verifyNative(t *testing.T, r *clineCycleRecipe, f *clineProjectAttempt, events []byte) {
	t.Helper()
	q := f.provider
	q.mu.Lock()
	facts := q.facts
	q.mu.Unlock()
	if q.empty {
		if len(events) != 0 || f.nativeFacts != nil || facts != nil || q.issued || q.approved {
			t.Fatal("Cline empty stage invoked owned tool/permission/probe")
		}
		return
	}
	if !reflect.DeepEqual(f.nativeFacts, facts) || facts == nil || facts["nonce"] != r.Nonce || facts["revision"] != r.Revision || facts["cwd"] != r.Active || facts["plugin_root"] != r.Active || facts["plugin_data"] != r.Data || facts["marker"] != q.marker || facts["literal"] != "" || facts["once"] != "" || !reflect.DeepEqual(facts["argv"], []any{"Owned Cline " + r.Scope, r.Active + "/argument"}) {
		t.Fatal("Cline cycle provider/native facts differ from selected recipe/receipt")
	}
	for _, identity := range []string{"pid:" + fmt.Sprint(facts["pid"]), "session:" + fmt.Sprint(facts["session"])} {
		if c.probeIDs[identity] {
			t.Fatal("Cline cycle probe process/session reused")
		}
		c.probeIDs[identity] = true
	}
	if err := clineAttemptEvents(events, r.Nonce, facts, f.nativeStart, f.nativeEnd); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(r.Data, "native-marker.txt")
	marker, err := projectNativeRuntimeRead(path, 256)
	if err != nil || string(marker) != q.marker {
		t.Fatal("Cline cycle retained marker differs")
	}
	c.f.protected[path] = clineAttemptSHA(marker)
}

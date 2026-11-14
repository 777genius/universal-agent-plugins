//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
)

func (c *copilotProjectCycle) native(t *testing.T, stage, key, operation string) {
	t.Helper()
	if len(c.sessions) >= len(copilotCycleNative) || copilotCycleNative[len(c.sessions)] != stage {
		t.Fatal("Copilot cycle native phase/order")
	}
	project := c.recipes[key].Project
	var r *copilotCycleRecipe
	tool, nonce := "", copilotProjectConfigHash([]byte(c.f.base.Nonce + stage))[:32]
	if operation != "" {
		r = c.recipes[key]
		if r.Key == "user" || r.Removed || r.Active == "" || operation != "write" && operation != "read" {
			t.Fatal("Copilot native phase has no installed project authority")
		}
		tool, nonce = r.Skill+"-inspect_runtime", r.Nonce
	}
	before, err := c.events()
	if err != nil {
		t.Fatal(err)
	}
	var dataBefore map[string]string
	if operation == "write" {
		dataBefore = c.dataBeforeWrite(t, r)
	}
	var facts nativeFacts
	p := newCopilotCycleProvider(nonce, stage, tool, operation, func(body string) error { var err error; facts, err = c.facts(r, body); return err })
	if err = p.start(c.f.ctx); err != nil {
		t.Fatal(err)
	}
	defer p.shutdown(t, c, stage)
	env := append(c.f.env(), "COPILOT_PROVIDER_TYPE=openai", "COPILOT_PROVIDER_BASE_URL="+p.url+"/v1", "COPILOT_PROVIDER_WIRE_API=completions", "COPILOT_MODEL=uap-fixture-model")
	args := c.nativeArgs(stage, r)
	c.f.evidence["cycle_stage"] = "native:" + stage
	result, record, start, end := c.run(t, project, env, args, 60*time.Second)
	c.append(t, "cycle-native-commands.jsonl", stage, map[string]any{"command": record, "started_at": start.Format(time.RFC3339Nano), "ended_at": end.Format(time.RFC3339Nano), "stdout": copilotCycleStream(t, result.Stdout), "stderr": copilotCycleStream(t, result.Stderr)})
	c.append(t, "cycle-native-events.jsonl", stage, copilotCycleStream(t, result.Stdout))
	p.shutdown(t, c, stage)
	after, err := c.events()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(after, before) {
		t.Fatal("Copilot append-only probe prefix changed")
	}
	delta := after[len(before):]
	c.append(t, "cycle-probe-events.jsonl", stage, map[string]any{"before_bytes": len(before), "before_sha256": copilotProjectConfigHash(before), "after_bytes": len(after), "after_sha256": copilotProjectConfigHash(after), "delta": copilotCycleStream(t, delta)})
	c.sessions = append(c.sessions, stage)
	if !copilotCycleCommandClean(result, record) || result.ExitCode != 0 || len(result.Stderr) != 0 || t.Failed() {
		t.Fatal("Copilot native process/provider did not complete ordinarily")
	}
	s, err := c.nativeEvents(result.Stdout, start, end, stage, project, r, p)
	if err != nil {
		t.Fatal(err)
	}
	if c.nativeIDs[s.session] || c.probeIDs[s.session] || s.session == p.p.id {
		t.Fatal("Copilot fresh native identity reused")
	}
	if err = c.probe(delta, r, facts, start, end, s.session, p.p.id); err != nil {
		t.Fatal(err)
	}
	c.nativeIDs[s.session] = true
	if operation == "write" {
		c.dataAfterWrite(t, r, dataBefore)
	}
	if r != nil {
		c.proof(t, r, "after-native-"+stage)
	}
	c.check(t)
}

func (c *copilotProjectCycle) nativeArgs(stage string, r *copilotCycleRecipe) []string {
	names := strings.ReplaceAll(strings.TrimPrefix(copilotReadinessDisabled, "Disabled tools: "), ", ", ",")
	args := []string{c.f.client, "-p", copilotCyclePrompt(stage), "--output-format", "json", "--no-auto-update", "--disable-client-telemetry", "--disable-builtin-mcps", "--excluded-tools=" + names}
	if r != nil {
		args = append(args, "--allow-tool="+r.Skill+"(inspect_runtime)")
	}
	return append(args, "--deny-tool=shell", "--deny-tool=write", "--deny-tool=read", "--deny-tool=url", "--deny-tool=memory")
}

func (c *copilotProjectCycle) nativeEvents(body []byte, start, end time.Time, stage, project string, r *copilotCycleRecipe, p *copilotCycleProvider) (*copilotReadinessStream, error) {
	var s *copilotReadinessStream
	var err error
	if r == nil {
		s, err = copilotCycleNoCallDecode(body, start, end)
	} else {
		s, err = copilotReadinessDecodeOrder(body, start, end, true)
	}
	if err != nil {
		return nil, err
	}
	if err = c.metadata(s, project, r); err != nil {
		return nil, err
	}
	if r == nil {
		for _, check := range []func() error{func() error { return s.noCallMessages(copilotCyclePrompt(stage), p.p, start, end) }, func() error { return s.noCallHistory(p.p) }} {
			if err = check(); err != nil {
				return nil, err
			}
		}
		return s, nil
	}
	for _, check := range []func() error{s.telemetry, func() error { return s.messagesFor(copilotCyclePrompt(stage), p.p, start, end) }} {
		if err = check(); err != nil {
			return nil, err
		}
	}
	facts, err := s.toolFor(r.Skill, p.arguments(), p.p, func(body string) error { _, err := c.facts(r, body); return err })
	if err != nil {
		return nil, err
	}
	if err = s.historyFor(p.operation, p.p, facts); err != nil {
		return nil, err
	}
	last, err := copilotCycleRequest(p.p.requests[1], p.p.tool)
	if err != nil {
		return nil, err
	}
	messages := last["messages"].([]any)
	if !reflect.DeepEqual(messages[len(messages)-1], map[string]any{"role": "tool", "tool_call_id": p.p.id, "content": facts}) {
		return nil, fmt.Errorf("Copilot native/provider return differs")
	}
	return s, nil
}

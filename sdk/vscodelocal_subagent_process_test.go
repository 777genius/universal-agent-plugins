package pluginkitai_test

import (
	"strings"
	"testing"

	pluginkitai "github.com/777genius/plugin-kit-ai/sdk"
)

// Red: SubagentStop uses a different decoder that loses optional bool/session
// presence, accepts bad agent scalars, bypasses the shared size bound, or mutates
// native Stop into SubagentStop. The subagent consumer must always be suppressed.
func TestVSCodeLocalSubagentBoundaries(t *testing.T) {
	c := buildLocalConsumer(t)
	base := `{"timestamp":"2026-10-01T10:41:04.061Z","hook_event_name":"SubagentStop","agent_id":"TEST-agent","agent_type":"TEST-custom"`
	for _, flag := range []string{`,"stop_hook_active":false`, `,"stop_hook_active":true`, `,"stop_hook_active":null`, ""} {
		got := c.run(t, "VSCodeLocalSubagentStop", "empty", base+flag+"}")
		known := strings.HasSuffix(flag, "false") || strings.HasSuffix(flag, "true")
		if got.Code != 0 || got.Stdout != "{}" || got.Stderr != "" || got.Calls != 1 || got.Eligible ||
			got.Callback != "SubagentStop" || got.Known != known || got.Active != strings.HasSuffix(flag, "true") || got.SessionID != "" {
			t.Fatalf("subagent presence contract changed: %+v", got)
		}
	}
	for _, field := range []string{`"agent_id":123`, `"agent_type":false`, `"stop_hook_active":"false"`} {
		input := `{"hook_event_name":"SubagentStop",` + field + "}"
		got := c.run(t, "VSCodeLocalSubagentStop", "empty", input)
		if got.Code != 1 || got.Calls != 0 || got.Stdout != "" || got.Stderr == "" {
			t.Fatalf("wrong subagent scalar reached callback: %+v", got)
		}
	}
	future := c.run(t, "VSCodeLocalSubagentStop", "nil", base+`,"stop_hook_active":false,"future":{"deep":[1,null]}}`)
	if future.Code != 0 || future.Stdout != "{}" || future.Stderr != "" || future.Eligible {
		t.Fatalf("unknown subagent field rejected: %+v", future)
	}
	mismatch := c.run(t, "VSCodeLocalSubagentStop", "nil", `{"hook_event_name":"Stop","stop_hook_active":false}`)
	if mismatch.Code != 0 || mismatch.Callback != "SubagentStop" || mismatch.NativeName != "Stop" ||
		mismatch.Eligible || !strings.Contains(mismatch.Stderr, "does not match argv hook") || mismatch.Stdout != "{}" {
		t.Fatalf("mismatched subagent selector rewritten: %+v", mismatch)
	}
	oversize := base + "}" + strings.Repeat(" ", pluginkitai.MaxPayloadBytes)
	got := c.run(t, "VSCodeLocalSubagentStop", "nil", oversize)
	if got.Code != 1 || got.Calls != 0 || got.Stdout != "" || !strings.Contains(got.Stderr, "exceeds max payload size") {
		t.Fatalf("subagent decoder bypassed size bound: %+v", got)
	}
}

// Red: adding strict timestamp parsing/normalization or making nullable optional
// common fields required narrows the existing decoder contract. Semantic event
// and timestamp admission stays with the consumer; the SDK retains native data.
func TestVSCodeLocalCommonScalarSemantics(t *testing.T) {
	c := buildLocalConsumer(t)
	for _, session := range []string{`null`, `""`} {
		input := `{"timestamp":"TEST-unparsed-timestamp","hook_event_name":"Stop","stop_hook_active":false,"session_id":` + session + `,"cwd":null,"transcript_path":null}`
		got := c.run(t, "VSCodeLocalStop", "nil", input)
		if got.Code != 0 || got.Stdout != "{}" || got.Stderr != "" || got.Calls != 1 ||
			got.SessionID != "" || got.CWD != "" || got.TranscriptPath != "" || got.Timestamp != "TEST-unparsed-timestamp" {
			t.Fatalf("common scalar semantics changed: %+v", got)
		}
	}
}

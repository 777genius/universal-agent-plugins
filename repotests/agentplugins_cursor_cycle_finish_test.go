//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"os"
	"reflect"
	"testing"
)

var cursorCycleProofStages = []string{"source-a-1.0.0", "source-b-1.0.0", "original-controls", "add-a", "approve-a-one", "add-b", "approve-b-one", "repeat-add-a", "unchanged-update-a", "intact-repair-a", "source-a-1.1.0", "update-a", "approve-a-two", "missing-native-a", "missing-repair-a", "remove-a", "repeat-remove-a", "remove-b"}

func (c *cursorMCPProjectCycle) finish(t *testing.T, events []byte) {
	t.Helper()
	if err := c.checkError(); err != nil {
		t.Error(err)
	}
	if !bytes.Equal(events, c.probe) {
		t.Error("Cursor final probe prefix differs")
	}
	want := make([]string, len(cursorCycleSteps))
	for i, step := range cursorCycleSteps {
		want[i] = step.label
	}
	complete := c.complete && !t.Failed() && reflect.DeepEqual(c.completed, want) && reflect.DeepEqual(c.proofs, cursorCycleProofStages) && len(c.sessions) != 0 && c.recipes["a"].Removed && c.recipes["b"].Removed
	c.f.evidence["cycle_command_stages"], c.f.evidence["cycle_proof_stages"] = c.completed, c.proofs
	c.f.evidence["cycle_observed_complete"], c.f.evidence["runtime_attempt_complete"] = complete, complete
	c.f.evidence["cycle_probe"] = map[string]any{"bytes": len(events), "sha256": clineAttemptSHA(events), "sessions": len(c.sessions)}
	c.f.evidence["qualification_scope"] = "MCP-only: two project installations, fresh initialize/tools-list sessions; no tools-call, skills, runtime cwd/argv facts, vendor authentication or model"
	flags := map[string]bool{"project-lifecycle": complete, "native-restart-removal": complete, "mcp-initialize": complete, "mcp-tools-list": complete, "mcp-tools-call": false}
	c.f.evidence["qualification"] = flags
	for _, member := range cursorCycleMembers {
		c.f.keep(t, member, c.aggregates[member])
	}
}

func TestAgentpluginsCursorNativeProjectCompleteMCPCycle(t *testing.T) {
	if os.Getenv("AGENTPLUGINS_CURSOR_PROJECT_RUNTIME_ATTEMPT") != "1" {
		t.Skip("opt-in genuine hosted Cursor complete MCP project cycle")
	}
	f := newCursorProjectAttempt(t)
	defer f.finish(t)
	c := newCursorMCPProjectCycle(t, f)
	c.prepare(t)
	for index := range cursorCycleSteps {
		c.run(t, index)
	}
	c.check(t)
	c.complete = true
}

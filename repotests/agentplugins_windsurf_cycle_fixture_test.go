//go:build linux

package pluginkitairepo_test

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
)

const windCycleMode = "windsurf-complete-project-skills-cycle"

var windCycleMembers = []string{"windsurf-cycle-commands.jsonl", "windsurf-cycle-proofs.jsonl", "windsurf-cycle-probe.jsonl"}
var windCycleLabels = append(append([]string{}, windComponentLabels...), "repeat-add-a", "unchanged-update-a", "intact-repair-a", "stable-a-skills", "stable-b-skills", "update-a", "updated-a-skills", "updated-a-get", "updated-b-skills", "damaged-a-skills", "missing-repair-a", "repaired-a-skills", "repaired-a-get", "remove-a", "removed-a-skills", "removed-a-mcp", "remaining-b-skills", "remaining-b-mcp", "repeat-remove-a", "remove-b", "final-a-skills", "final-a-mcp", "final-b-skills", "final-b-mcp")

type windProjectCycle struct {
	w                 *windComponentFixture
	aggregates        map[string][]byte
	removed           map[string]bool
	markers           map[string][]byte
	cacheKeys         map[string]bool
	completed, proofs []string
	probeRecord       windProfileRecord
	complete          bool
}

func newWindProjectCycle(f *projectNativeCLIAttempt) *windProjectCycle {
	w := newWindComponentFixture(f)
	f.evidence["capture_mode"] = windCycleMode
	f.evidence["capture_review_required"] = false
	f.evidence["cycle_nonce"] = f.base.Nonce
	f.evidence["cycle_tool_pins"] = map[string]string{"installer": f.immutable[f.installer], "probe": f.immutable[f.base.Probe]}
	f.evidence["qualification_scope"] = "Project skills metadata lifecycle and restart/removal; MCP configuration visibility only, no MCP initialize/list/call, authentication or model"
	return &windProjectCycle{w: w, aggregates: map[string][]byte{}, removed: map[string]bool{}, markers: map[string][]byte{}, cacheKeys: map[string]bool{}, completed: []string{}, proofs: []string{}}
}

func windCycleStream(raw []byte) clineCycleBytes {
	return clineCycleBytes{Bytes: len(raw), SHA256: clineAttemptSHA(raw), Body: append([]byte{}, raw...)}
}

func (c *windProjectCycle) append(t *testing.T, member, stage string, value any) {
	t.Helper()
	known := false
	for _, name := range windCycleMembers {
		known = known || name == member
	}
	if !known || stage == "" {
		t.Fatal("Windsurf closed cycle aggregate")
	}
	raw, err := json.Marshal(map[string]any{"stage": stage, "value": value})
	if err != nil {
		t.Fatal(err)
	}
	raw = append(raw, '\n')
	total := len(raw)
	for _, body := range c.aggregates {
		total += len(body)
	}
	if len(c.aggregates[member])+len(raw) > 1<<20 || total > 16<<20 {
		t.Fatal("Windsurf encoded aggregate bound")
	}
	c.aggregates[member] = append(c.aggregates[member], raw...)
}

func (c *windProjectCycle) proof(t *testing.T, stage string, value any) {
	c.append(t, windCycleMembers[1], stage, value)
	c.proofs = append(c.proofs, stage)
}

func (c *windProjectCycle) finish(t *testing.T) {
	t.Helper()
	defer c.w.f.finish(t)
	f := c.w.f
	if err := windFrozenTrees(f.ctx, c.w.trees); err != nil {
		t.Error(err)
	}
	if err := c.w.events(); err != nil {
		t.Error(err)
	}
	if record, err := windCycleProbeRecord(c); err != nil || record != c.probeRecord {
		t.Error("Windsurf final held empty probe changed")
	}
	complete := c.complete && !t.Failed() && f.ctx.Err() == nil && reflect.DeepEqual(c.completed, windCycleLabels) && c.removed["a"] && c.removed["b"] && len(c.cacheKeys) == 4
	f.evidence["cycle_command_stages"], f.evidence["cycle_proof_stages"] = c.completed, c.proofs
	f.evidence["cycle_observed_complete"], f.evidence["runtime_attempt_complete"] = complete, complete
	f.evidence["attempt_stage"] = c.w.stage
	f.evidence["qualification"] = map[string]bool{"project-lifecycle": complete, "native-restart-removal": complete, "mcp-initialize": false, "mcp-tools-list": false, "mcp-tools-call": false}
	for _, member := range windCycleMembers {
		f.keep(t, member, c.aggregates[member])
	}
}

func TestAgentpluginsWindsurfNativeProjectCompleteSkillsCycle(t *testing.T) {
	spec, err := projectNativeCLISpecification("windsurf")
	if err != nil {
		t.Fatal(err)
	}
	if os.Getenv(spec.optin) != "1" {
		t.Skip("opt-in genuine hosted Windsurf project skills lifecycle")
	}
	f := newProjectNativeCLIAttempt(t, spec)
	c := newWindProjectCycle(f)
	defer c.finish(t)
	c.w.prepare(t)
	c.w.f.evidence["cycle_nonce"] = f.base.Nonce
	c.probeRecord = windCycleProbe(t, c)
	for _, label := range windCycleLabels {
		c.command(t, label)
	}
	c.w.check(t)
	c.complete = true
}

func windCycleVersion(r *windComponentRecipe) string {
	if r.Revision == "a-two" {
		return "1.1.0"
	}
	return "1.0.0"
}

func windCycleRequired(ok bool, message string) error {
	if !ok {
		return fmt.Errorf("Windsurf cycle %s", message)
	}
	return nil
}

func windCycleFinalizeEvidence(evidence map[string]any) {
	if evidence["capture_mode"] != windCycleMode || evidence["test_failed"] != true {
		return
	}
	evidence["runtime_attempt_complete"] = false
	evidence["qualification"] = map[string]bool{"project-lifecycle": false, "native-restart-removal": false, "mcp-initialize": false, "mcp-tools-list": false, "mcp-tools-call": false}
}

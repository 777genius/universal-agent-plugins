//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"fmt"
	"reflect"
	"testing"
	"time"
)

var qoderCycleProofStages = []string{"fixture", "source-a-a-one", "caller-a", "source-b-b-one", "caller-b", "original-user", "original-a", "original-b", "original-registry", "original-profile", "add-a", "add-b", "repeat-add-a", "unchanged-update-a", "intact-repair-a", "before-source-revision-a", "source-a-a-two", "update-a", "before-missing-a", "missing-a", "missing-repair-a", "remove-a", "repeat-remove-a", "remove-b", "log-ledger"}

// Run at the existing serializer boundary, after its own late deadline guard.
// Other capture modes retain their existing finalization behavior.
func qoderCycleFinalizeEvidence(evidence map[string]any) {
	if evidence["capture_mode"] != qoderCycleMode || evidence["test_failed"] != true {
		return
	}
	evidence["runtime_attempt_complete"] = false
	evidence["qualification"] = map[string]bool{"project-lifecycle": false, "native-restart-removal": false, "mcp-initialize": false, "mcp-tools-list": false, "mcp-tools-call": false}
}

func (c *qoderSkillsCycle) closure() error {
	for index, stages := range [][]string{qoderCyclePublic, qoderCycleCatalog, qoderCycleProofStages} {
		raw, ok := c.aggregates[qoderCycleAggregates[index]]
		if !ok || len(raw) > 1<<20 || !bytes.HasSuffix(raw, []byte{'\n'}) {
			return fmt.Errorf("Qoder full aggregate closure")
		}
		lines := bytes.Split(bytes.TrimSuffix(raw, []byte{'\n'}), []byte{'\n'})
		if len(lines) != len(stages) {
			return fmt.Errorf("Qoder full aggregate count")
		}
		for i, line := range lines {
			value, err := projectNativeRuntimeJSON(line)
			row, ok := value.(map[string]any)
			if err != nil || !ok || projectNativeRuntimeKeys(row, "stage value", "") != nil || row["stage"] != stages[i] {
				return fmt.Errorf("Qoder full aggregate ordered stage differs")
			}
		}
	}
	return nil
}

func (c *qoderSkillsCycle) finish(t *testing.T) {
	t.Helper()
	f := c.f
	f.evidence["runtime_attempt_complete"] = false
	flags := map[string]bool{"project-lifecycle": false, "native-restart-removal": false, "mcp-initialize": false, "mcp-tools-list": false, "mcp-tools-call": false}
	f.evidence["qualification"] = flags
	f.evidence["cycle_observed_complete"] = c.complete
	f.evidence["cycle_public_stages"], f.evidence["cycle_catalog_stages"] = append([]string{}, c.public...), append([]string{}, c.catalogs...)
	runs := 0
	if c.logs != nil {
		runs = len(c.logs.runs)
	}
	f.evidence["cycle_log_runs"] = runs
	if err := c.authority(); err != nil {
		t.Error(err)
	}
	for _, name := range qoderCycleAggregates {
		if raw, ok := c.aggregates[name]; ok {
			if qoderCyclePrivate(raw) {
				t.Error("Qoder private/unbounded aggregate refused")
				continue
			}
			f.keep(t, name, raw)
		}
	}
	closed := c.closure() == nil
	correct := f.evidence["capture_mode"] == qoderCycleMode && f.evidence["cycle_stage"] == "complete-cycle" && f.evidence["vendor_authentication_qualified"] == false && f.evidence["real_model_qualified"] == false && reflect.DeepEqual(c.public, qoderCyclePublic) && reflect.DeepEqual(c.catalogs, qoderCycleCatalog) && runs == 21
	complete := c.complete && !t.Failed() && f.ctx.Err() == nil && correct && closed && len(f.files) == 3 && time.Since(f.started) < 180*time.Second
	if c.complete && !complete {
		t.Error("Qoder full final deadline/ordinary cleanup/source/capture gate failed")
	}
	if complete {
		f.evidence["runtime_attempt_complete"] = true
		flags["project-lifecycle"], flags["native-restart-removal"] = true, true
	}
	f.finish(t)
}

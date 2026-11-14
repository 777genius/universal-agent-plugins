//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"fmt"
	"reflect"
	"slices"
	"testing"
	"time"
)

func (c *copilotProjectCycle) finish(t *testing.T) {
	t.Helper()
	c.f.cycle = c
	c.f.finish(t)
}

func (c *copilotProjectCycle) finalize(t *testing.T) {
	t.Helper()
	f := c.f
	f.evidence["runtime_attempt_complete"] = false
	q := map[string]bool{}
	for _, id := range clineCycleQualificationIDs {
		q[id] = false
	}
	f.evidence["qualification"] = q
	f.evidence["cycle_observed_complete"], f.evidence["provider_closed"] = c.complete, false
	f.evidence["cycle_public_stages"], f.evidence["cycle_registry_stages"], f.evidence["cycle_native_stages"] = slices.Clone(c.public), slices.Clone(c.registry), slices.Clone(c.sessions)
	f.evidence["cycle_native_sessions"], f.evidence["cycle_probe_sessions"] = len(c.nativeIDs), len(c.probeIDs)
	if err := c.authority(); err != nil {
		t.Error(err)
	}
	for _, name := range copilotCycleAggregates {
		if raw, exists := c.aggregates[name]; exists {
			if copilotCyclePrivate(raw) {
				t.Error("Copilot cycle aggregate privacy/bound refusal")
				continue
			}
			f.keep(t, name, raw)
		}
	}
	closed := c.aggregateClosure() == nil
	f.evidence["provider_closed"] = closed
	sourceCorrect := f.evidence["mode"] == copilotCycleMode && f.evidence["cycle_stage"] == "complete-cycle" && f.evidence["vendor_authentication_qualified"] == false && f.evidence["real_model_qualified"] == false && reflect.DeepEqual(c.public, copilotCyclePublic) && reflect.DeepEqual(c.registry, copilotCycleRegistry) && reflect.DeepEqual(c.sessions, copilotCycleNative) && len(c.nativeIDs) == 14 && len(c.probeIDs) == 6
	complete := copilotCycleComplete(c.complete, t.Failed(), f.ctx.Err() == nil, sourceCorrect, closed, copilotCycleCapture(f.files), time.Since(f.started))
	if c.complete && !complete {
		t.Error("Copilot lifecycle final ordinary closure/authority/capture/deadline gate failed")
	}
	if complete {
		f.evidence["runtime_attempt_complete"] = true
		for _, id := range clineCycleQualificationIDs {
			q[id] = true
		}
	}
}

func copilotCycleCapture(files map[string][]byte) bool {
	if len(files) != 59 {
		return false
	}
	for _, label := range copilotCyclePublic {
		for _, suffix := range []string{".command.json", ".stdout.log", ".stderr.log"} {
			if _, exists := files[label+suffix]; !exists {
				return false
			}
		}
	}
	for _, name := range copilotCycleAggregates {
		if _, exists := files[name]; !exists {
			return false
		}
	}
	return true
}

func (c *copilotProjectCycle) aggregateClosure() error {
	counts := map[string]int{"cycle-native-events.jsonl": 14, "cycle-native-commands.jsonl": 14, "cycle-registry-listings.jsonl": 15, "cycle-provider-requests.jsonl": 20, "cycle-provider-responses.jsonl": 20, "cycle-provider-receipts.jsonl": 14, "cycle-probe-events.jsonl": 14, "cycle-proof.jsonl": 33}
	if len(c.aggregates) != len(counts) {
		return fmt.Errorf("Copilot lifecycle aggregate set incomplete")
	}
	for name, count := range counts {
		raw := c.aggregates[name]
		if len(raw) > 1<<20 || !bytes.HasSuffix(raw, []byte{'\n'}) || len(bytes.Split(bytes.TrimSuffix(raw, []byte{'\n'}), []byte{'\n'})) != count {
			return fmt.Errorf("Copilot lifecycle aggregate phase/count closure")
		}
	}
	for _, line := range bytes.Split(bytes.TrimSuffix(c.aggregates["cycle-provider-receipts.jsonl"], []byte{'\n'}), []byte{'\n'}) {
		value, err := projectNativeRuntimeJSON(line)
		row, ok := value.(map[string]any)
		if err != nil || !ok || projectNativeRuntimeKeys(row, "stage value", "") != nil {
			return fmt.Errorf("Copilot lifecycle provider receipt shape")
		}
		receipt, ok := row["value"].(map[string]any)
		if !ok || projectNativeRuntimeKeys(receipt, "requests responses error returned shutdown serve_closed connections_closed forced", "") != nil || receipt["error"] != false || receipt["returned"] != true || receipt["shutdown"] != true || receipt["serve_closed"] != true || receipt["connections_closed"] != true || receipt["forced"] != false {
			return fmt.Errorf("Copilot lifecycle provider did not close ordinarily")
		}
	}
	return nil
}

func copilotCycleComplete(observed, failed, workAlive, sourceCorrect, closed, captured bool, elapsed time.Duration) bool {
	return observed && !failed && workAlive && sourceCorrect && closed && captured && elapsed >= 0 && elapsed < 180*time.Second
}

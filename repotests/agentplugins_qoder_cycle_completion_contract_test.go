//go:build linux

package pluginkitairepo_test

import (
	"encoding/json"
	"testing"
)

func TestQoderCycleLateSharedFinalizationFailureCannotSerializeQualification(t *testing.T) {
	for _, mode := range []string{qoderCycleMode, "legacy-attempt"} {
		evidence := map[string]any{"capture_mode": mode, "runtime_attempt_complete": true, "test_failed": false,
			"qualification": map[string]bool{"project-lifecycle": true, "native-restart-removal": true, "mcp-initialize": false, "mcp-tools-list": false, "mcp-tools-call": false}}
		qoderCycleFinalizeEvidence(evidence)
		if evidence["runtime_attempt_complete"] != true {
			t.Fatal("successful candidate changed before late guard")
		}
		// The existing shared 180-second guard records failure after c.finish
		// may have provisionally qualified. Exercise its exact serialization hook.
		evidence["test_failed"], evidence["elapsed_ms"] = true, int64(180001)
		qoderCycleFinalizeEvidence(evidence)
		raw, err := json.Marshal(evidence)
		if err != nil {
			t.Fatal(err)
		}
		value, err := projectNativeRuntimeJSON(raw)
		if err != nil {
			t.Fatal(err)
		}
		row := value.(map[string]any)
		if row["runtime_attempt_complete"] != (mode != qoderCycleMode) {
			t.Fatal("late failure completion or legacy behavior changed")
		}
		if mode == qoderCycleMode {
			for _, flag := range row["qualification"].(map[string]any) {
				if flag != false {
					t.Fatal("late failed summary retained a qualification flag")
				}
			}
		}
	}
}

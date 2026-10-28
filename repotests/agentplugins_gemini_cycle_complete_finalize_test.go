//go:build linux

package pluginkitairepo_test

import (
	"encoding/json"
	"testing"
	"time"
)

func geminiCycleClearCompletion(evidence map[string]any) {
	evidence["runtime_attempt_complete"] = false
	evidence["vendor_authentication_qualified"], evidence["real_model_qualified"] = false, false
	evidence["qualification"] = map[string]bool{"project-lifecycle": false, "native-restart-removal": false, "mcp-initialize": false, "mcp-tools-list": false, "mcp-tools-call": false}
	evidence["capture_mode"], evidence["cycle_capture_review_required"] = geminiCycleMode, true
	if evidence["attempt_stage"] == "complete-cycle" {
		evidence["attempt_stage"] = "complete-cycle-capture-review"
	}
}

// This detached candidate is never published until complete raw admission.
func (c *geminiProjectCycle) completionCandidate(t *testing.T) map[string]any {
	t.Helper()
	raw, err := json.Marshal(c.f.evidence)
	if err != nil {
		t.Fatal("Gemini current summary encoding")
	}
	value, err := projectNativeRuntimeJSON(raw)
	result, ok := value.(map[string]any)
	if err != nil || !ok {
		t.Fatal("Gemini current summary decoding")
	}
	result["cycle_observed_complete"] = c.observed
	result["cycle_public_stages"], result["cycle_listing_stages"], result["cycle_native_stages"] = c.public, c.listings, c.natives
	result["attempt_stage"], result["test_failed"], result["elapsed_ms"] = c.stage, t.Failed(), time.Since(c.f.started).Milliseconds()
	result["capture_mode"], result["cycle_capture_review_required"] = geminiCycleCompleteMode, false
	result["runtime_attempt_complete"], result["test_failed"], result["attempt_stage"] = true, false, "complete-cycle"
	result["qualification"] = map[string]bool{"project-lifecycle": true, "native-restart-removal": true, "mcp-initialize": true, "mcp-tools-list": true, "mcp-tools-call": true}
	return result
}

func geminiCycleCompletionSource(t *testing.T, source any) any {
	t.Helper()
	body, err := json.Marshal(source)
	if err != nil {
		t.Fatal("Gemini original source identity encoding")
	}
	value, err := projectNativeRuntimeJSON(body)
	if err != nil {
		t.Fatal("Gemini original source identity snapshot")
	}
	return value
}

func (c *geminiProjectCycle) admitComplete(t *testing.T, heldSource any) error {
	t.Helper()
	c.f.evidence["cycle_completion"] = map[string]any{"schema_version": 1, "started_at": c.f.started.UTC().Format(time.RFC3339Nano), "work_deadline_at": c.f.started.Add(150 * time.Second).UTC().Format(time.RFC3339Nano), "finished_at": time.Now().UTC().Format(time.RFC3339Nano), "work_context_error": c.f.ctx.Err() != nil, "original_checks_error": false, "open_authorities": len(c.open), "open_events": c.openEvents, "provider": c.owner.closure}
	for pass := range 2 {
		result := c.completionCandidate(t)
		raw, err := json.Marshal(result)
		if err != nil {
			return err
		}
		files := map[string][]byte{}
		for name, body := range c.f.files {
			files[name] = body
		}
		files["runtime-attempt.json"] = raw
		if geminiCycleCompleteFiles(files, heldSource) != nil {
			return geminiCompleteRefusal()
		}
		if pass == 0 {
			c.f.evidence["cycle_completion"].(map[string]any)["finished_at"] = time.Now().UTC().Format(time.RFC3339Nano)
		}
	}
	if t.Failed() || c.f.ctx.Err() != nil || time.Since(c.f.started) > 180*time.Second {
		return geminiCompleteRefusal()
	}
	c.stage = "complete-cycle"
	c.f.evidence = c.completionCandidate(t)
	return nil
}

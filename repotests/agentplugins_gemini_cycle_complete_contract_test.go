//go:build linux

package pluginkitairepo_test

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestGeminiCycleCompleteActualKernelStateMatchesIndependentPureProofReader(t *testing.T) {
	c := geminiCycleInert(t)
	for _, scope := range []string{"user", "a", "b"} {
		geminiCycleStageActual(t, c, scope)
	}
	paths := c.f.evidence["cycle_layout"].(map[string]string)
	rows, err := geminiCompleteRows(c.aggregates["cycle-proof.jsonl"], geminiCompleteProofOrder[:6], false)
	if err != nil || len(rows) != 6 {
		t.Fatal("actual inert source/kernel proof rows", err)
	}
	recipes := map[string]*geminiCycleRecipe{}
	raw := map[string][]byte{}
	for _, row := range rows {
		if strings.HasPrefix(row.Stage, "source-") {
			r, err := geminiCompleteRecipe(row.Value, paths)
			if err != nil {
				t.Fatal("actual independent source recipe refused", err)
			}
			recipes[r.Scope] = r
			continue
		}
		if err := geminiCompleteStateRow(row, recipes, paths, raw); err != nil {
			t.Fatal("actual loader/planner/stager/kernel/adapter state refused", row.Stage, err)
		}
	}
	for scope, recipe := range recipes {
		if !reflect.DeepEqual(recipe.Installation, c.recipes[scope].Installation) || !reflect.DeepEqual(recipe.Binding, c.recipes[scope].Binding) {
			t.Fatal("pure reader changed actual inert state identity", scope)
		}
	}
}

func geminiCompleteInertClosure() map[string]any {
	return map[string]any{"schema_version": 1, "started_at": "2026-10-27T00:00:00Z", "work_deadline_at": "2026-10-27T00:02:30Z", "finished_at": "2026-10-27T00:01:00Z", "work_context_error": false, "original_checks_error": false, "open_authorities": 0, "open_events": false,
		"provider": map[string]any{"shutdown_complete": true, "forced_close": false, "serve_returned": true, "error": false, "busy": 0, "current_stage": "", "requests": 16, "completed_stages": append([]string{}, geminiCycleNative...)}}
}

func TestGeminiCycleCompleteClosureIsFiniteActualOwnerStateNotFlags(t *testing.T) {
	for _, kind := range []string{"valid", "missing", "forced", "shutdown", "serve", "error", "busy", "current", "requests", "stages", "work", "checks", "open", "events", "deadline", "origin", "late", "elapsed", "type", "extra"} {
		t.Run(kind, func(t *testing.T) {
			value := geminiCompleteInertClosure()
			provider := value["provider"].(map[string]any)
			elapsed := 61000
			switch kind {
			case "missing":
				delete(value, "provider")
			case "forced":
				provider["forced_close"] = true
			case "shutdown":
				provider["shutdown_complete"] = false
			case "serve":
				provider["serve_returned"] = false
			case "error":
				provider["error"] = true
			case "busy":
				provider["busy"] = 1
			case "current":
				provider["current_stage"] = "removed-user"
			case "requests":
				provider["requests"] = 15
			case "stages":
				provider["completed_stages"] = geminiCycleNative[:9]
			case "work":
				value["work_context_error"] = true
			case "checks":
				value["original_checks_error"] = true
			case "open":
				value["open_authorities"] = 1
			case "events":
				value["open_events"] = true
			case "deadline":
				value["work_deadline_at"] = "2026-10-27T00:03:00Z"
			case "origin":
				value["started_at"] = "2026-10-27T00:01:00Z"
			case "late":
				value["finished_at"] = "2026-10-27T00:02:30.001Z"
			case "elapsed":
				elapsed = 59999
			case "type":
				provider["requests"] = true
			case "extra":
				provider["ignored"] = true
			}
			raw, err := json.Marshal(map[string]any{"cycle_completion": value, "elapsed_ms": elapsed})
			if err != nil {
				t.Fatal(err)
			}
			result, err := geminiCompleteJSON(raw)
			if err != nil {
				t.Fatal(err)
			}
			_, _, err = geminiCompleteClosure(result)
			if (err == nil) != (kind == "valid") {
				t.Fatal("closure predicate mismatch", err)
			}
		})
	}
}

func TestGeminiCycleCompleteStagedFlagsClearOnLateFailure(t *testing.T) {
	evidence := map[string]any{"runtime_attempt_complete": true, "capture_mode": geminiCycleCompleteMode, "cycle_capture_review_required": false, "attempt_stage": "complete-cycle", "qualification": map[string]bool{"mcp-tools-call": true}, "vendor_authentication_qualified": true, "real_model_qualified": true}
	geminiCycleClearCompletion(evidence)
	if evidence["runtime_attempt_complete"] != false || evidence["capture_mode"] != geminiCycleMode || evidence["cycle_capture_review_required"] != true || evidence["attempt_stage"] != "complete-cycle-capture-review" || evidence["vendor_authentication_qualified"] != false || evidence["real_model_qualified"] != false || len(evidence["qualification"].(map[string]bool)) != 5 {
		t.Fatal("late failure retained a staged success")
	}
	for _, value := range evidence["qualification"].(map[string]bool) {
		if value {
			t.Fatal("late failure retained a qualification")
		}
	}
}

func TestGeminiCycleCompleteActualProviderCloseRecordsOrdinaryAndIncompleteState(t *testing.T) {
	c := geminiCycleInert(t)
	c.f.started = time.Now()
	ctx, cancel := context.WithDeadline(context.Background(), c.f.started.Add(150*time.Second))
	defer cancel()
	c.f.ctx = ctx
	c.startProvider(t)
	// An unarmed, never-called inert HTTP server closes ordinarily but has not
	// completed any native matrix. Its actual receipt cannot qualify that state.
	if c.owner.close(t) {
		t.Fatal("empty actual provider close became a full cycle")
	}
	value := c.owner.closure
	if value["shutdown_complete"] != true || value["forced_close"] != false || value["serve_returned"] != true || value["error"] != true || value["requests"] != 0 || value["busy"] != 0 || value["current_stage"] != "" {
		t.Fatal("actual ordinary incomplete close receipt differs", value)
	}
}

func TestGeminiCycleCompleteCommandDurationRetainsNanosecondPrecision(t *testing.T) {
	start := time.Date(2026, 10, 27, 0, 0, 0, 123456789, time.UTC)
	end := start.Add(500*time.Millisecond - time.Nanosecond)
	c := geminiCompleteCapture{Paths: map[string]string{"node": "/inert/node", "client": "/inert/gemini.js", "a": "/inert/project"}, Started: start.Add(-time.Second), Finished: end.Add(time.Second)}
	for _, elapsed := range []int{499, 500} {
		value := map[string]any{"kind": "listing", "scope": "a", "argv": []string{"/inert/node", "/inert/gemini.js", "skills", "list", "--all"}, "cwd": "/inert/project", "exit": 0, "error": false, "context_error": false, "started_at": start.Format(time.RFC3339Nano), "finished_at": end.Format(time.RFC3339Nano), "elapsed_ms": elapsed, "cleanup": "ordinary full-descendant supervision; forced cleanup is a failure", "stdout": geminiCompleteContractReceipt([]byte{}), "stderr": geminiCompleteContractReceipt([]byte{})}
		row := geminiCompleteContractDecode(t, geminiCompleteContractEncode(t, value))
		_, _, err := c.observation(geminiCompleteRow{Stage: "listing:baseline-a", Value: row})
		if (err == nil) != (elapsed == 499) {
			t.Fatal("nanosecond duration floor was normalized or widened", elapsed, err)
		}
	}
}

func TestGeminiCycleCompleteStreamRequiresCanonicalBase64(t *testing.T) {
	for _, item := range []struct {
		text  string
		body  string
		valid bool
	}{{"Zg==", "f", true}, {"", "", true}, {"Zh==", "f", false}, {"Zg==\n", "f", false}, {"Zg==\r\n", "f", false}, {"\n", "", false}, {"Zg", "f", false}, {"Zg====", "f", false}} {
		raw := geminiCompleteContractEncode(t, map[string]any{"bytes": len(item.body), "sha256": geminiCycleSHA([]byte(item.body)), "body": item.text})
		row := geminiCompleteContractDecode(t, raw)
		body, err := geminiCompleteStream(row)
		if (err == nil) != item.valid || item.valid && string(body) != item.body {
			t.Fatal("base64 canonical/pad-bit boundary differs", item.text, err)
		}
	}
}

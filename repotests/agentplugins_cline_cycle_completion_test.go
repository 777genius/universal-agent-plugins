//go:build linux

package pluginkitairepo_test

import (
	"context"
	"fmt"
	"reflect"
	"testing"
)

var clineCycleQualificationIDs = []string{"project-lifecycle", "native-restart-removal", "mcp-initialize", "mcp-tools-list", "mcp-tools-call"}

// clineCycleFinalize runs after the actual test and deferred provider/live/
// privacy checks. It never qualifies a failed capture or a partial cycle.
func clineCycleFinalize(evidence map[string]any, failed bool, workErr error) error {
	evidence["runtime_attempt_complete"] = false
	qualification := map[string]bool{}
	for _, id := range clineCycleQualificationIDs {
		qualification[id] = false
	}
	evidence["qualification"] = qualification
	if failed {
		return nil
	}
	cleanup := map[string]any{"shutdown_complete": true, "forced_close": false, "error": false, "completed_stages": clineCycleACP}
	if workErr != nil || evidence["capture_mode"] != "cline-complete-cycle" || evidence["cycle_capture_review_required"] != false || evidence["cycle_observed_complete"] != true || evidence["attempt_stage"] != "complete-cycle" || evidence["vendor_authentication_qualified"] != false || evidence["real_model_qualified"] != false || !reflect.DeepEqual(evidence["cycle_public_stages"], clineCyclePublic) || !reflect.DeepEqual(evidence["cycle_listing_stages"], clineCycleListings) || !reflect.DeepEqual(evidence["cycle_acp_stages"], clineCycleACP) || !reflect.DeepEqual(evidence["cycle_provider_cleanup"], cleanup) {
		return fmt.Errorf("Cline complete cycle cannot qualify without full ordinary closure in its original work budget")
	}
	for _, id := range clineCycleQualificationIDs {
		qualification[id] = true
	}
	evidence["runtime_attempt_complete"] = true
	return nil
}

func clineCycleCompletionFixture() map[string]any {
	return map[string]any{"capture_mode": "cline-complete-cycle", "cycle_capture_review_required": false, "cycle_observed_complete": true, "attempt_stage": "complete-cycle", "vendor_authentication_qualified": false, "real_model_qualified": false, "cycle_public_stages": append([]string{}, clineCyclePublic...), "cycle_listing_stages": append([]string{}, clineCycleListings...), "cycle_acp_stages": append([]string{}, clineCycleACP...), "cycle_provider_cleanup": map[string]any{"shutdown_complete": true, "forced_close": false, "error": false, "completed_stages": append([]string{}, clineCycleACP...)}}
}

func TestClineCycleCompletionRequiresExactFinalClosureAndResetsLateFailures(t *testing.T) {
	evidence := clineCycleCompletionFixture()
	if err := clineCycleFinalize(evidence, false, nil); err != nil || evidence["runtime_attempt_complete"] != true {
		t.Fatal("complete inert finalization fixture refused", err)
	}
	for _, id := range clineCycleQualificationIDs {
		if !evidence["qualification"].(map[string]bool)[id] {
			t.Fatal("complete closure omitted qualification", id)
		}
	}
	for _, kind := range []string{"failed", "deadline", "mode", "review", "unobserved", "stage", "public", "listing", "native", "cleanup", "forced", "provider-error", "handler-stage", "auth", "model", "typed-bool"} {
		evidence := clineCycleCompletionFixture()
		evidence["runtime_attempt_complete"] = true
		evidence["qualification"] = map[string]bool{"project-lifecycle": true}
		failed, workErr := false, error(nil)
		switch kind {
		case "failed":
			failed = true
		case "deadline":
			workErr = context.DeadlineExceeded
		case "mode":
			evidence["capture_mode"] = "cline-complete-cycle-first-capture"
		case "review":
			evidence["cycle_capture_review_required"] = true
		case "unobserved":
			evidence["cycle_observed_complete"] = false
		case "typed-bool":
			evidence["cycle_observed_complete"] = 1
		case "stage":
			evidence["attempt_stage"] = "complete-cycle-capture-review"
		case "public":
			evidence["cycle_public_stages"] = clineCyclePublic[:len(clineCyclePublic)-1]
		case "listing":
			evidence["cycle_listing_stages"] = clineCycleListings[:1]
		case "native":
			evidence["cycle_acp_stages"] = []string{}
		case "cleanup":
			evidence["cycle_provider_cleanup"] = nil
		case "forced":
			evidence["cycle_provider_cleanup"].(map[string]any)["forced_close"] = true
		case "provider-error":
			evidence["cycle_provider_cleanup"].(map[string]any)["error"] = true
		case "handler-stage":
			evidence["cycle_provider_cleanup"].(map[string]any)["completed_stages"] = clineCycleACP[:1]
		case "auth":
			evidence["vendor_authentication_qualified"] = true
		case "model":
			evidence["real_model_qualified"] = true
		}
		err := clineCycleFinalize(evidence, failed, workErr)
		if (!failed && err == nil) || evidence["runtime_attempt_complete"] != false {
			t.Fatal("incomplete finalization accepted", kind)
		}
		for _, value := range evidence["qualification"].(map[string]bool) {
			if value {
				t.Fatal("late failure retained any qualification", kind)
			}
		}
	}
}

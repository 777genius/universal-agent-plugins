package pluginkitairepo_test

import (
	"fmt"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func openCodeProjectIntactRepair(t *testing.T, f *openCodeProjectFixture) {
	t.Helper()
	before := f.binding(t, f.A)
	nativeRequireContained(t, f.A, before.TargetLocator)
	statePath := filepath.Join(f.A, ".uap", "state-v2.json")
	nativePaths := []string{before.TargetLocator, filepath.Join(f.A, ".opencode", "skills"), filepath.Join(f.A, "opencode.jsonc")}
	stateBefore, nativeBefore := f.snapshot(t, statePath), f.snapshot(t, nativePaths...)
	facts := map[string]any{"state_before": stateBefore, "native_before": nativeBefore, "receipt_count_before": len(before.Receipts)}
	f.evidence["intact_repair"] = facts
	result := f.run(t, "intact-repair-A", f.A, "repair", commandsPluginName)
	if err := openCodeProjectRepairResult(result); err != nil {
		t.Fatal(err)
	}
	after := f.binding(t, f.A)
	stateAfter, nativeAfter := f.snapshot(t, statePath), f.snapshot(t, nativePaths...)
	facts["state_after"], facts["native_after"], facts["receipt_count_after"] = stateAfter, nativeAfter, len(after.Receipts)
	if reflect.DeepEqual(stateBefore, stateAfter) || !reflect.DeepEqual(nativeBefore, nativeAfter) {
		t.Fatal("intact group repair did not record restaging with identical package/native files")
	}
	if err := openCodeProjectRepairBinding(before, after); err != nil {
		t.Fatal(err)
	}
	facts["binding_invariants_preserved"] = true
}

func openCodeProjectRepairResult(result map[string]any) error {
	view, err := openCodeProjectAssertionEnvelope(result)
	if err != nil {
		return err
	}
	data, _ := view["data"].(map[string]any)
	outcome, err := commandsTargetOutcome(view)
	if err != nil || data["status"] != "completed" || outcome["mutated"] != true {
		return fmt.Errorf("intact group repair did not report an actual completed mutation")
	}
	if value, exists := outcome["no_change"]; exists && value != false {
		return fmt.Errorf("intact group repair carries contradictory unchanged status")
	}
	return nil
}

func openCodeProjectRepairBinding(before, after domain.ClientBinding) error {
	if before.NativeActivationAttempt != "" || after.NativeActivationAttempt != "" || before.PendingNativeIntent != nil || after.PendingNativeIntent != nil {
		return fmt.Errorf("intact repair has unresolved native authority")
	}
	count := len(before.Receipts)
	if count == 0 || len(after.Receipts) != count+1 || !reflect.DeepEqual(before.Receipts, after.Receipts[:count]) {
		return fmt.Errorf("intact repair did not preserve and append exact receipt history")
	}
	previous, receipt := before.Receipts[count-1], after.Receipts[count]
	if receipt.Phase != "committed" || receipt.MutationType != "directory_swap" || receipt.ClientBindingID != before.ClientBindingID || receipt.ActivePath != before.TargetLocator ||
		previous.AfterDigest == "" || receipt.BeforeDigest != previous.AfterDigest || receipt.AfterDigest != previous.AfterDigest {
		return fmt.Errorf("intact repair receipt lost prior owned package identity")
	}
	// The kernel appends a new physical restaging receipt and updates the
	// timestamp. Every other binding field, including native ownership, is stable.
	after.Receipts, after.UpdatedAt = before.Receipts, before.UpdatedAt
	if !reflect.DeepEqual(before, after) {
		return fmt.Errorf("intact repair changed binding authority, projection or lifecycle")
	}
	return nil
}

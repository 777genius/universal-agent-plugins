package pluginkitairepo_test

import (
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestOpenCodeProjectIntactRepairRequiresActualMutation(t *testing.T) {
	for _, change := range []string{"absent no_change", "false no_change", "true no_change", "malformed no_change", "null no_change", "false mutated", "malformed mutated", "missing mutated", "single", "mixed", "failed batch"} {
		t.Run(change, func(t *testing.T) {
			raw := openCodeProjectResultFixture(true)
			outcome, _ := commandsTargetOutcome(raw)
			outcome["mutated"] = true
			delete(outcome, "no_change")
			switch change {
			case "false no_change":
				outcome["no_change"] = false
			case "true no_change":
				outcome["no_change"] = true
			case "malformed no_change":
				outcome["no_change"] = "false"
			case "null no_change":
				outcome["no_change"] = nil
			case "false mutated":
				outcome["mutated"] = false
			case "malformed mutated":
				outcome["mutated"] = "true"
			case "missing mutated":
				delete(outcome, "mutated")
			case "single":
				raw = openCodeProjectResultFixture(false)
			case "mixed":
				raw["data"].(map[string]any)["result"] = outcome
			case "failed batch":
				raw["data"].(map[string]any)["status"] = "failed"
			}
			want := change == "absent no_change" || change == "false no_change"
			if err := openCodeProjectRepairResult(raw); (err == nil) != want {
				t.Fatalf("repair mutation admission=%v want=%v", err, want)
			}
		})
	}
}

func TestOpenCodeProjectIntactRepairPreservesOwnedBinding(t *testing.T) {
	for _, change := range []string{"valid", "missing history", "altered history", "extra history", "wrong binding", "wrong path", "uncommitted", "removal", "empty digest", "wrong before digest", "wrong after digest", "projection", "native ownership", "lifecycle", "authentication", "attempt", "intent"} {
		t.Run(change, func(t *testing.T) {
			before := domain.ClientBinding{ClientBindingID: "binding", TargetLocator: "managed", Activation: domain.ActivationPrepared, Authentication: domain.AuthenticationNotChecked,
				Receipts: []domain.MutationReceipt{{AfterDigest: "owned"}}, NativeObjects: []domain.NativeObjectOwnership{{Kind: "managed_package_directory", ManagedDigest: "owned"}}}
			after := before
			after.Receipts = append([]domain.MutationReceipt{}, before.Receipts...)
			after.Receipts = append(after.Receipts, domain.MutationReceipt{Phase: "committed", MutationType: "directory_swap", ClientBindingID: "binding", ActivePath: "managed", BeforeDigest: "owned", AfterDigest: "owned"})
			after.UpdatedAt = "new timestamp"
			switch change {
			case "missing history":
				before.Receipts = nil
			case "altered history":
				after.Receipts[0].AfterDigest = "foreign"
			case "extra history":
				after.Receipts = append(after.Receipts, after.Receipts[1])
			case "wrong binding":
				after.Receipts[1].ClientBindingID = "foreign"
			case "wrong path":
				after.Receipts[1].ActivePath = "foreign"
			case "uncommitted":
				after.Receipts[1].Phase = "state_committed"
			case "removal":
				after.Receipts[1].MutationType = "directory_remove"
			case "empty digest":
				before.Receipts[0].AfterDigest, after.Receipts[0].AfterDigest = "", ""
			case "wrong before digest":
				after.Receipts[1].BeforeDigest = "foreign"
			case "wrong after digest":
				after.Receipts[1].AfterDigest = "foreign"
			case "projection":
				after.PhysicalArtifact = "foreign"
			case "native ownership":
				after.NativeObjects = nil
			case "lifecycle":
				after.Activation = domain.ActivationActive
			case "authentication":
				after.Authentication = domain.AuthenticationComplete
			case "attempt":
				before.NativeActivationAttempt, after.NativeActivationAttempt = "pending", "pending"
			case "intent":
				before.PendingNativeIntent = &domain.PendingNativeIntent{}
				after.PendingNativeIntent = before.PendingNativeIntent
			}
			if err := openCodeProjectRepairBinding(before, after); (err == nil) != (change == "valid") {
				t.Fatalf("repair binding admission=%v", err)
			}
		})
	}
}

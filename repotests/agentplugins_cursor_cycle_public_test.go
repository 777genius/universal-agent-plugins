//go:build linux

package pluginkitairepo_test

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func cursorCyclePublicOutput(document map[string]any, step cursorCycleStep, r *cursorCycleRecipe) (map[string]any, error) {
	refused := fmt.Errorf("Cursor exact public cycle DTO differs")
	if r == nil || document["command"] != step.verb {
		return nil, refused
	}
	data, ok := document["data"].(map[string]any)
	if !ok {
		return nil, refused
	}
	output := data
	if step.label != "repeat-add-a" {
		if projectNativeRuntimeKeys(data, "operation_id batch status succeeded failed plugin dry_run targets", "version source tree_digest manifest_digest security action_required environment plugin_data_preserved data_retained retained_data retained_data_action") != nil || data["batch"] != true || data["succeeded"] != json.Number("1") || data["failed"] != json.Number("0") || data["plugin"] != cursorCycleName || data["dry_run"] != false {
			return nil, refused
		}
		status := "completed"
		if step.verb == "remove" {
			status = "data_retained"
		}
		if data["status"] != status {
			return nil, refused
		}
		targets, ok := data["targets"].([]any)
		if !ok || len(targets) != 1 {
			return nil, refused
		}
		target, ok := targets[0].(map[string]any)
		if !ok || projectNativeRuntimeKeys(target, "target status output", "selected next_action") != nil || target["target"] != "cursor" || target["status"] != "external_completed" {
			return nil, refused
		}
		if step.verb == "update" && target["selected"] != true {
			return nil, refused
		}
		output, ok = target["output"].(map[string]any)
		if !ok || output["operation_id"] != data["operation_id"] {
			return nil, refused
		}
		if cursorCycleMutates(step.label) && data["operation_id"] != r.Installation.OperationGroupID {
			return nil, refused
		}
		if step.verb == "add" && !reflect.DeepEqual(data["environment"], map[string]any{"reason": "no_save_requested", "saved": false}) {
			return nil, refused
		}
		if step.verb == "remove" {
			retained := []any{map[string]any{"data_receipt_id": r.Data.DataReceiptID, "physical_backend_id": r.Data.PhysicalBackend, "scope": "project", "state": "owned"}}
			if data["data_retained"] != true || data["plugin_data_preserved"] != true || !reflect.DeepEqual(data["retained_data"], retained) {
				return nil, refused
			}
		}
	}
	optional := "operation_id next_action security"
	if step.label == "repeat-add-a" {
		optional = "next_action security"
	}
	if projectNativeRuntimeKeys(output, "plugin version source tree_digest manifest_digest dry_run result", optional) != nil {
		return nil, refused
	}
	source := "direct local source"
	if step.verb == "repair" || step.verb == "remove" {
		source = "local"
	}
	for key, want := range map[string]any{"plugin": cursorCycleName, "version": r.Version, "source": source, "tree_digest": r.Digest, "manifest_digest": r.Installation.Package.ManifestDigest, "dry_run": false} {
		if output[key] != want {
			return nil, refused
		}
	}
	return output, nil
}

func cursorCyclePublicOutcome(output map[string]any, step cursorCycleStep, r *cursorCycleRecipe) error {
	refused := fmt.Errorf("Cursor public outcome differs from held Prepared binding")
	outcome, ok := output["result"].(map[string]any)
	if !ok || outcome["installation_id"] != r.Installation.InstallationID || outcome["requires_confirmation"] != false || outcome["mutated"] != cursorCycleMutates(step.label) {
		return refused
	}
	if step.verb == "remove" {
		if projectNativeRuntimeKeys(outcome, "installation_id plugin client_id requires_confirmation mutated deactivation affected_surfaces group_phase", "") != nil || outcome["plugin"] != cursorCycleName || outcome["client_id"] != "cursor" || outcome["group_phase"] != "external_completed" || !reflect.DeepEqual(outcome["affected_surfaces"], []any{"cursor"}) {
			return refused
		}
		deactivation, ok := outcome["deactivation"].(map[string]any)
		if !ok || !reflect.DeepEqual(deactivation, map[string]any{"activation": "not_required", "artifact_removal_allowed": true, "external_removal_complete": true}) {
			return refused
		}
		return nil
	}
	optional := ""
	if step.label != "repeat-add-a" {
		optional = "group_phase no_change"
	}
	if projectNativeRuntimeKeys(outcome, "installation_id plan activation requires_confirmation mutated", optional) != nil {
		return refused
	}
	if step.label == "unchanged-update-a" && outcome["no_change"] != true {
		return refused
	}
	plan, p := outcome["plan"].(map[string]any)
	activation, a := outcome["activation"].(map[string]any)
	if !p || !a || projectNativeRuntimeKeys(plan, "client_id scope status package_mode activation authentication policy verification physical_artifact_id components", "user_actions warnings") != nil || projectNativeRuntimeKeys(activation, "activation authentication policy verification", "user_actions") != nil {
		return refused
	}
	for key, value := range map[string]any{"client_id": "cursor", "scope": "project", "status": "ready", "package_mode": "native", "physical_artifact_id": r.Data.PhysicalBackend, "activation": "prepared", "authentication": "not_checked", "policy": "allowed", "verification": "package_validated"} {
		if plan[key] != value {
			return refused
		}
	}
	for key, value := range map[string]any{"activation": "prepared", "authentication": "not_checked", "policy": "allowed", "verification": "installation_verified"} {
		if activation[key] != value {
			return refused
		}
	}
	if !reflect.DeepEqual(plan["components"], []any{map[string]any{"kind": "mcp_server", "name": "uapcursora", "support": "native"}}) {
		return refused
	}
	return nil
}

func (c *cursorMCPProjectCycle) publicProof(t *testing.T, r *cursorCycleRecipe, step cursorCycleStep, result ports.CommandResult) {
	t.Helper()
	if step.label == "repeat-remove-a" {
		if !c.allowedExit(c.commandAt(len(c.completed)-1), result) {
			t.Fatal("Cursor repeated removal differs")
		}
		c.removedProof(t, r, step.label)
		return
	}
	document, err := cursorCyclePublicDocument(result.Stdout)
	if err != nil || string(result.Stderr) != grokCycleProgress(step.label, cursorCycleName) {
		t.Fatal("Cursor exact public JSON/progress differs", err)
	}
	if step.verb == "remove" {
		c.removedProof(t, r, step.label)
	} else {
		c.installedProof(t, r, step.label, cursorCycleMutates(step.label))
	}
	output, err := cursorCyclePublicOutput(document, step, r)
	if err != nil {
		t.Fatal(err)
	}
	if err := cursorCyclePublicOutcome(output, step, r); err != nil {
		t.Fatal(err)
	}
}

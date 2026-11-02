//go:build linux

package pluginkitairepo_test

import (
	"encoding/json"
	"fmt"
	"reflect"
)

func qoderCyclePublicProof(raw []byte, stage, verb string, changed bool, r *qoderCycleRecipe) (string, error) {
	value, err := projectNativeRuntimeJSON(raw)
	row, ok := value.(map[string]any)
	if err != nil || !ok || projectNativeRuntimeKeys(row, "schema_version command result data", "") != nil || row["schema_version"] != json.Number("1") || row["command"] != verb || row["result"] != "success" {
		return "", fmt.Errorf("Qoder public proof envelope differs")
	}
	data, ok := row["data"].(map[string]any)
	if ok && stage == "repeat-add-a" {
		if changed || projectNativeRuntimeKeys(data, "plugin version source tree_digest manifest_digest dry_run result", "security next_action") != nil {
			return "", fmt.Errorf("Qoder exact singleton repeat-add fields differ")
		}
		return "", qoderCyclePublicOutput(data, r, false, "direct local source")
	}
	if !ok || projectNativeRuntimeKeys(data, "batch status succeeded failed plugin dry_run targets", "operation_id action_required version source revision tree_digest manifest_digest security environment plugin_data_preserved data_retained retained_data retained_data_action") != nil {
		return "", fmt.Errorf("Qoder public group fields differ")
	}
	status := "completed"
	if verb == "remove" {
		status = "data_retained"
	}
	for field, expected := range map[string]any{"batch": true, "status": status, "succeeded": json.Number("1"), "failed": json.Number("0"), "plugin": qoderCycleName, "dry_run": false} {
		if !reflect.DeepEqual(data[field], expected) {
			return "", fmt.Errorf("Qoder public group outcome differs")
		}
	}
	targets, ok := data["targets"].([]any)
	if !ok || len(targets) != 1 {
		return "", fmt.Errorf("Qoder public single target required")
	}
	target, ok := targets[0].(map[string]any)
	required := "target status output"
	if verb == "update" {
		required += " selected"
	}
	if !ok || projectNativeRuntimeKeys(target, required, "next_action") != nil || target["target"] != "qoder" || target["status"] != "external_completed" || verb == "update" && target["selected"] != true {
		return "", fmt.Errorf("Qoder public selected target differs")
	}
	output, ok := target["output"].(map[string]any)
	if !ok || projectNativeRuntimeKeys(output, "plugin version source tree_digest manifest_digest dry_run result", "operation_id revision security next_action") != nil {
		return "", fmt.Errorf("Qoder public target output fields differ")
	}
	i := r.Installation
	source := "direct local source"
	if verb == "repair" || verb == "remove" {
		source = "local"
	}
	if err := qoderCyclePublicOutput(output, r, changed, source); err != nil {
		return "", err
	}
	group, ok := data["operation_id"].(string)
	if !ok || !qoderCycleOperation.MatchString(group) || output["operation_id"] != group || changed && group != i.OperationGroupID {
		return "", fmt.Errorf("Qoder public operation/group differs from current proof")
	}
	if err := qoderCyclePublicIdentities(data, qoderCyclePublicIdentityFields(r, changed), changed); err != nil {
		return "", err
	}
	if verb == "remove" {
		var retained any
		for _, receipt := range i.DataReceipts {
			retained = []any{map[string]any{"data_receipt_id": receipt.DataReceiptID, "physical_backend_id": receipt.PhysicalBackend, "scope": receipt.Scope, "state": string(receipt.State)}}
		}
		if data["plugin_data_preserved"] != true || data["data_retained"] != true || !reflect.DeepEqual(data["retained_data"], retained) {
			return "", fmt.Errorf("Qoder public retained data differs from actual owner allocation")
		}
	}
	return group, nil
}

func qoderCyclePublicOutput(output map[string]any, r *qoderCycleRecipe, changed bool, source string) error {
	for field, expected := range map[string]any{"plugin": qoderCycleName, "version": "1.0.0", "source": source, "tree_digest": r.Digest, "manifest_digest": r.Installation.Package.ManifestDigest, "dry_run": false} {
		if !reflect.DeepEqual(output[field], expected) {
			return fmt.Errorf("Qoder public source/revision differs from current proof")
		}
	}
	result, ok := output["result"].(map[string]any)
	if !ok || result["installation_id"] != r.Installation.InstallationID || result["requires_confirmation"] != false || result["mutated"] != changed {
		return fmt.Errorf("Qoder public mutation differs from current proof")
	}
	return qoderCyclePublicIdentities(output, qoderCyclePublicIdentityFields(r, changed), changed)
}

func qoderCyclePublicIdentityFields(r *qoderCycleRecipe, changed bool) map[string]any {
	i, b := r.Installation, r.Binding
	return map[string]any{"tree_digest": r.Digest, "manifest_digest": i.Package.ManifestDigest, "installation_id": i.InstallationID, "client_binding_id": b.ClientBindingID, "physical_artifact_id": b.PhysicalArtifact, "target_locator": b.TargetLocator, "client_id": "qoder", "target": "qoder", "scope": "project", "mutated": changed}
}

func qoderCyclePublicIdentities(value any, checks map[string]any, changed bool) error {
	switch item := value.(type) {
	case map[string]any:
		for field, child := range item {
			if expected, exists := checks[field]; exists && !reflect.DeepEqual(child, expected) {
				return fmt.Errorf("Qoder recursive public identity differs from current proof")
			}
			if field == "no_change" {
				if flag, ok := child.(bool); !ok || changed && flag {
					return fmt.Errorf("Qoder public no-change claim contradicts mutation")
				}
			}
			if err := qoderCyclePublicIdentities(child, checks, changed); err != nil {
				return err
			}
		}
	case []any:
		for _, child := range item {
			if err := qoderCyclePublicIdentities(child, checks, changed); err != nil {
				return err
			}
		}
	}
	return nil
}

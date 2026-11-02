//go:build linux

package pluginkitairepo_test

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

func (c *windProjectCycle) publicDTO(label string, r *windComponentRecipe, raw []byte) error {
	refused := fmt.Errorf("Windsurf exact stage public DTO refused")
	value, err := projectNativeRuntimeJSON(raw)
	doc, ok := value.(map[string]any)
	verb := "add"
	switch {
	case strings.Contains(label, "update"):
		verb = "update"
	case strings.Contains(label, "repair"):
		verb = "repair"
	case strings.HasPrefix(label, "remove"):
		verb = "remove"
	}
	if err != nil || !ok || projectNativeRuntimeKeys(doc, "schema_version command result data", "") != nil || doc["schema_version"] != json.Number("1") || doc["command"] != verb || doc["result"] != "success" {
		return refused
	}
	data, ok := doc["data"].(map[string]any)
	if !ok {
		return refused
	}
	output := data
	if label != "repeat-add-a" {
		if projectNativeRuntimeKeys(data, "operation_id batch status succeeded failed plugin dry_run targets", "version source tree_digest manifest_digest security action_required environment plugin_data_preserved data_retained retained_data retained_data_action") != nil || data["batch"] != true || data["succeeded"] != json.Number("1") || data["failed"] != json.Number("0") || data["plugin"] != windComponentName || data["dry_run"] != false {
			return refused
		}
		status := "completed"
		if verb == "remove" {
			status = "data_retained"
		}
		if data["status"] != status {
			return refused
		}
		targets, ok := data["targets"].([]any)
		if !ok || len(targets) != 1 {
			return refused
		}
		target, ok := targets[0].(map[string]any)
		if !ok || projectNativeRuntimeKeys(target, "target status output", "selected next_action") != nil || target["target"] != "windsurf" || target["status"] != "external_completed" {
			return refused
		}
		if verb == "update" && target["selected"] != true {
			return refused
		}
		output, ok = target["output"].(map[string]any)
		if !ok || output["operation_id"] != data["operation_id"] {
			return refused
		}
		if windCycleMutated(label) && data["operation_id"] != r.Installation.OperationGroupID {
			return refused
		}
		if verb == "add" && !reflect.DeepEqual(data["environment"], map[string]any{"reason": "no_save_requested", "saved": false}) {
			return refused
		}
		if verb == "remove" && (data["data_retained"] != true || data["plugin_data_preserved"] != true) {
			return refused
		}
	}
	optional := "operation_id next_action"
	if label == "repeat-add-a" {
		optional = "next_action security"
	}
	if projectNativeRuntimeKeys(output, "plugin version source tree_digest manifest_digest dry_run result", optional) != nil {
		return refused
	}
	source := "direct local source"
	if verb == "repair" || verb == "remove" {
		source = "local"
	}
	for key, want := range map[string]any{"plugin": windComponentName, "version": windCycleVersion(r), "source": source, "tree_digest": r.Digest, "manifest_digest": r.Installation.Package.ManifestDigest, "dry_run": false} {
		if output[key] != want {
			return refused
		}
	}
	row, ok := output["result"].(map[string]any)
	if !ok || row["installation_id"] != r.Installation.InstallationID || row["requires_confirmation"] != false || row["mutated"] != windCycleMutated(label) {
		return refused
	}
	if verb == "remove" {
		if projectNativeRuntimeKeys(row, "installation_id plugin client_id requires_confirmation mutated deactivation affected_surfaces group_phase", "") != nil || row["plugin"] != windComponentName || row["client_id"] != "windsurf" || row["group_phase"] != "external_completed" || !reflect.DeepEqual(row["affected_surfaces"], []any{"windsurf"}) || !reflect.DeepEqual(row["deactivation"], map[string]any{"activation": "not_required", "artifact_removal_allowed": true, "external_removal_complete": true}) {
			return refused
		}
		return nil
	}
	keys := "installation_id plan activation requires_confirmation mutated"
	if label != "repeat-add-a" {
		keys += " group_phase"
		if row["group_phase"] != "external_completed" {
			return refused
		}
	}
	if label == "repeat-add-a" || label == "unchanged-update-a" {
		keys += " no_change"
		if row["no_change"] != true {
			return refused
		}
	}
	if projectNativeRuntimeKeys(row, keys, "") != nil {
		return refused
	}
	plan, p := row["plan"].(map[string]any)
	activation, a := row["activation"].(map[string]any)
	if !p || !a || projectNativeRuntimeKeys(plan, "client_id scope status package_mode activation authentication policy verification physical_artifact_id components", "user_actions warnings") != nil || projectNativeRuntimeKeys(activation, "activation authentication policy verification", "user_actions") != nil {
		return refused
	}
	for key, want := range map[string]any{"client_id": "windsurf", "scope": r.scopeName(), "status": "ready", "package_mode": "prepared_package", "activation": "prepared", "authentication": "not_checked", "policy": "allowed", "verification": "package_validated", "physical_artifact_id": r.Binding.PhysicalArtifact} {
		if plan[key] != want {
			return refused
		}
	}
	wantActivation := "prepared"
	if r.Scope == "user" {
		wantActivation = "active"
	}
	for key, want := range map[string]any{"activation": wantActivation, "authentication": "not_checked", "policy": "allowed", "verification": "installation_verified"} {
		if activation[key] != want {
			return refused
		}
	}
	components := []any{map[string]any{"kind": "skill", "name": windComponentName, "support": "prepared"}, map[string]any{"kind": "mcp_server", "name": windComponentName, "support": "prepared"}}
	return windCycleRequired(reflect.DeepEqual(plan["components"], components), "exact prepared components")
}

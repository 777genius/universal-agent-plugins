//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"encoding/json"
	"reflect"
	"regexp"
	"strings"
)

const geminiCompleteReload = "in a running Gemini CLI session use `/mcp reload` and `/skills reload`, or restart Gemini CLI"
const geminiCompleteNext = geminiCompleteReload + "; verify this plugin's authentication requirements before using it"

var geminiCompleteOperationID = regexp.MustCompile(`^op-[0-9a-f]{32}$`)

func geminiCompleteEqual(got, want any) bool {
	body, err := json.Marshal(want)
	if err != nil {
		return false
	}
	value, err := projectNativeRuntimeJSON(body)
	return err == nil && reflect.DeepEqual(geminiCompleteNormalizeIntegers(got), geminiCompleteNormalizeIntegers(value))
}

func (c *geminiCompleteCapture) public() error {
	fixture := &geminiProjectCycle{f: &geminiProjectAttempt{installer: c.Paths["installer"], node: c.Paths["node"], client: c.Paths["client"], git: c.Paths["git"]}, recipes: c.Proof.Views[""]}
	operations := map[string]bool{}
	for _, label := range geminiCyclePublic {
		fixture.recipes = c.Proof.Views[geminiCompleteProofStage(label)]
		if fixture.recipes == nil {
			fixture.recipes = c.Proof.Views[""]
		}
		cwd, argv, err := fixture.publicArguments(label)
		if label == "version" {
			cwd = c.Paths["outside"]
		}
		row, readErr := geminiCompleteJSON(c.Files[label+".command.json"])
		exit := 0
		if label == "repeat-remove-a" {
			exit = 1
		}
		if err != nil || readErr != nil || geminiCompleteReceipt(row, argv, cwd, 30000, exit) != nil {
			return geminiCompleteRefusal()
		}
		stdout, stderr := c.Files[label+".stdout.log"], c.Files[label+".stderr.log"]
		if strings.HasPrefix(label, "git-") {
			if len(stdout) != 0 || len(stderr) != 0 {
				return geminiCompleteRefusal()
			}
			continue
		}
		if label == "version" {
			if !bytes.Equal(stdout, []byte("0.65.0\n")) || len(stderr) != 0 {
				return geminiCompleteRefusal()
			}
			continue
		}
		if label == "repeat-remove-a" {
			if len(stdout) != 0 || string(stderr) != "agentplugins: plugin is not installed for target \"gemini\" in project scope; no target was changed\n" {
				return geminiCompleteRefusal()
			}
			continue
		}
		if string(stderr) != geminiCycleProgress(label) || c.publicResult(label, operations) != nil {
			return geminiCompleteRefusal()
		}
	}
	return nil
}

func (c *geminiCompleteCapture) publicResult(label string, operations map[string]bool) error {
	row, err := geminiCompleteJSON(c.Files[label+".stdout.log"])
	if err != nil {
		return err
	}
	r := c.Proof.Views[label][geminiCycleScope(label)]
	data, ok := row["data"].(map[string]any)
	if !ok || r == nil {
		return geminiCompleteRefusal()
	}
	op, _ := data["operation_id"].(string)
	if label != "repeat-add-a" && (!geminiCompleteOperationID.MatchString(op) || operations[op]) {
		return geminiCompleteRefusal()
	}
	if label != "repeat-add-a" {
		operations[op] = true
	}
	verb := strings.Split(label, "-")[0]
	switch label {
	case "repeat-add-a":
		verb = "add"
	case "unchanged-update-a":
		verb = "update"
	case "intact-repair-a", "missing-repair-a":
		verb = "repair"
	}
	if label != "repeat-add-a" && label != "unchanged-update-a" {
		want := r.Binding.Receipts[len(r.Binding.Receipts)-1].OperationGroupID
		if verb == "remove" {
			state, err := geminiCycleState(c.Proof.Raw[label])
			if err != nil {
				return err
			}
			want = state.TransactionReceipts[0].OperationGroupID
		}
		if op != want {
			return geminiCompleteRefusal()
		}
	}
	expected := map[string]any{"schema_version": 1, "command": verb, "result": "success", "data": geminiCompletePublicData(r, label, op)}
	if !geminiCompleteEqual(row, expected) {
		return geminiCompleteRefusal()
	}
	return nil
}

func geminiCompletePublicPlan(r *geminiCycleRecipe) map[string]any {
	action := "agentplugins will install and verify Gemini CLI skills and MCP servers automatically; reload them in a running session or restart Gemini CLI"
	if r.Scope != "user" {
		action = "project skills and MCP settings can be configured automatically; native discovery, workspace trust and system policy require observation in Gemini CLI"
	}
	return map[string]any{"client_id": "gemini", "scope": r.scopeName(), "status": "ready", "package_mode": "native", "activation": "prepared", "authentication": "not_checked", "policy": "allowed", "verification": "package_validated", "physical_artifact_id": r.Binding.PhysicalArtifact,
		"components": []any{map[string]any{"kind": "skill", "name": r.Name, "support": "native"}, map[string]any{"kind": "mcp_server", "name": r.Name, "support": "native"}}, "user_actions": []string{"verify the plugin's authentication requirements before using it", action}, "warnings": []string{"authentication_not_catalog_verified"}}
}

func geminiCompletePublicResult(r *geminiCycleRecipe, label string) map[string]any {
	result := map[string]any{"installation_id": r.Installation.InstallationID, "requires_confirmation": false, "mutated": label != "repeat-add-a" && label != "unchanged-update-a"}
	if strings.HasPrefix(label, "remove-") {
		result["plugin"], result["client_id"], result["group_phase"] = geminiCycleName, "gemini", "external_completed"
		result["affected_surfaces"] = []string{"gemini"}
		result["deactivation"] = map[string]any{"activation": "not_required", "artifact_removal_allowed": true, "external_removal_complete": true}
		return result
	}
	activation, actions := "active", []string{geminiCompleteReload}
	if r.Scope != "user" {
		activation = "prepared"
		actions = append(actions, "Gemini project configuration is installed; native discovery, workspace trust, system policy and MCP initialization remain unobserved; start Gemini CLI in the selected project and inspect skills list and mcp list")
	}
	result["plan"] = geminiCompletePublicPlan(r)
	result["activation"] = map[string]any{"activation": activation, "authentication": "not_checked", "policy": "allowed", "verification": "installation_verified", "user_actions": actions}
	if label != "repeat-add-a" {
		result["group_phase"] = "external_completed"
	}
	if label == "repeat-add-a" || label == "unchanged-update-a" {
		result["no_change"] = true
	}
	return result
}

func geminiCompleteSecurity(r *geminiCycleRecipe, cache bool) map[string]any {
	evidence := "local_scan"
	if cache {
		evidence = "cache"
	}
	return map[string]any{"schema_version": 1, "subject": map[string]any{"tree_digest": r.Digest, "manifest_digest": "sha256:" + geminiCycleSHA(r.Authored["plugin.json"])}, "scanner": map[string]any{"id": "lintai", "version": "0.1.3"}, "policy": map[string]any{"id": "agent-plugin-install", "version": 2, "digest": "sha256:9cf869e299d847d7078aeca01f5a182fcdb0144bbf513c83290459991c79e037"}, "outcome": "no_blocking_findings", "counts": map[string]any{"blocking": 0, "warnings": 0, "total": 0}, "scanned_files": 3, "report_digest": "sha256:90f5acb6ff3ac64f669be4f4ba6deea7ae5aa3a9954662441c0e5498b8d56598", "evidence_source": evidence}
}

func geminiCompletePublicData(r *geminiCycleRecipe, label, operation string) map[string]any {
	source := "direct local source"
	if strings.Contains(label, "repair-") || strings.HasPrefix(label, "remove-") {
		source = "local"
	}
	output := map[string]any{"plugin": geminiCycleName, "version": "1.0.0", "source": source, "tree_digest": r.Digest, "manifest_digest": "sha256:" + geminiCycleSHA(r.Authored["plugin.json"]), "dry_run": false, "result": geminiCompletePublicResult(r, label)}
	if !strings.HasPrefix(label, "remove-") {
		output["next_action"] = geminiCompleteNext
	}
	if label == "repeat-add-a" {
		output["security"] = geminiCompleteSecurity(r, true)
		return output
	}
	output["operation_id"] = operation
	target := map[string]any{"target": "gemini", "status": "external_completed", "output": output}
	if !strings.HasPrefix(label, "remove-") {
		target["next_action"] = geminiCompleteNext
	}
	if label == "update-a" || label == "unchanged-update-a" {
		target["selected"] = true
	}
	data := map[string]any{"operation_id": operation, "batch": true, "status": "completed", "succeeded": 1, "failed": 0, "plugin": geminiCycleName, "dry_run": false, "targets": []any{target}}
	if strings.HasPrefix(label, "add-") || label == "update-a" || label == "unchanged-update-a" {
		data["version"], data["source"], data["tree_digest"] = "1.0.0", source, r.Digest
	}
	if strings.HasPrefix(label, "add-") {
		data["action_required"], data["manifest_digest"] = 1, output["manifest_digest"]
		data["security"], data["environment"] = geminiCompleteSecurity(r, false), map[string]any{"reason": "no_save_requested", "saved": false}
	}
	if strings.HasPrefix(label, "remove-") {
		data["status"], data["plugin_data_preserved"], data["data_retained"] = "data_retained", true, true
		data["retained_data"] = []any{map[string]any{"data_receipt_id": r.Binding.DataReceiptID, "physical_backend_id": r.Binding.PhysicalArtifact, "scope": r.scopeName(), "state": "owned"}}
		action := "agentplugins remove " + r.Installation.InstallationID + " --purge-data"
		if r.Scope != "user" {
			action += " --scope project --project-root " + r.Project
		}
		data["retained_data_action"] = "run `" + action + "` to delete retained plugin data"
	}
	return data
}

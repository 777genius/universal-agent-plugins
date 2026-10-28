//go:build linux

package pluginkitairepo_test

import (
	"strings"
)

// The public records are authored from the public command schemas, separately
// from geminiCompletePublicData/Result/Plan. No retained installer output is
// copied into this fixture and no installer process is executed.
func (f *geminiCompleteContractFixture) publicOutput(r *geminiCycleRecipe, stage, group string) map[string]any {
	result := geminiCompleteContractDecode(f.t, []byte(`{"installation_id":"","requires_confirmation":false,"mutated":true}`))
	result["installation_id"] = r.Installation.InstallationID
	if strings.HasPrefix(stage, "remove-") {
		result["plugin"], result["client_id"], result["group_phase"] = "uap-gemini-cycle", "gemini", "external_completed"
		result["deactivation"] = geminiCompleteContractDecode(f.t, []byte(`{"activation":"not_required","artifact_removal_allowed":true,"external_removal_complete":true}`))
		result["affected_surfaces"] = []string{"gemini"}
	} else {
		result["plan"] = f.publicPlan(r)
		actions := []string{"in a running Gemini CLI session use `/mcp reload` and `/skills reload`, or restart Gemini CLI"}
		activation := "active"
		if r.Scope != "user" {
			activation = "prepared"
			actions = append(actions, "Gemini project configuration is installed; native discovery, workspace trust, system policy and MCP initialization remain unobserved; start Gemini CLI in the selected project and inspect skills list and mcp list")
		}
		result["activation"] = map[string]any{"activation": activation, "authentication": "not_checked", "policy": "allowed", "verification": "installation_verified", "user_actions": actions}
		if stage != "repeat-add-a" {
			result["group_phase"] = "external_completed"
		}
		if stage == "repeat-add-a" || stage == "unchanged-update-a" {
			result["mutated"], result["no_change"] = false, true
		}
	}
	source := "direct local source"
	if strings.Contains(stage, "repair-") || strings.HasPrefix(stage, "remove-") {
		source = "local"
	}
	output := geminiCompleteContractDecode(f.t, []byte(`{"plugin":"uap-gemini-cycle","version":"1.0.0","source":"","tree_digest":"","manifest_digest":"","dry_run":false}`))
	output["source"], output["tree_digest"], output["manifest_digest"], output["result"] = source, r.Digest, "sha256:"+geminiCycleSHA(r.Authored["plugin.json"]), result
	next := "in a running Gemini CLI session use `/mcp reload` and `/skills reload`, or restart Gemini CLI; verify this plugin's authentication requirements before using it"
	if !strings.HasPrefix(stage, "remove-") {
		output["next_action"] = next
	}
	if stage == "repeat-add-a" {
		output["security"] = f.publicSecurity(r, "cache")
		return output
	}
	output["operation_id"] = group
	target := map[string]any{"target": "gemini", "status": "external_completed", "output": output}
	if !strings.HasPrefix(stage, "remove-") {
		target["next_action"] = next
	}
	if stage == "update-a" || stage == "unchanged-update-a" {
		target["selected"] = true
	}
	data := geminiCompleteContractDecode(f.t, []byte(`{"operation_id":"","batch":true,"status":"completed","succeeded":1,"failed":0,"plugin":"uap-gemini-cycle","dry_run":false,"targets":[]}`))
	data["operation_id"], data["targets"] = group, []any{target}
	if strings.HasPrefix(stage, "add-") || stage == "update-a" || stage == "unchanged-update-a" {
		data["version"], data["source"], data["tree_digest"] = "1.0.0", source, r.Digest
	}
	if strings.HasPrefix(stage, "add-") {
		data["action_required"], data["manifest_digest"], data["security"] = 1, output["manifest_digest"], f.publicSecurity(r, "local_scan")
		data["environment"] = map[string]any{"reason": "no_save_requested", "saved": false}
	}
	if strings.HasPrefix(stage, "remove-") {
		data["status"], data["data_retained"], data["plugin_data_preserved"] = "data_retained", true, true
		data["retained_data"] = []any{map[string]any{"data_receipt_id": r.Binding.DataReceiptID, "physical_backend_id": r.Binding.PhysicalArtifact, "scope": r.scopeName(), "state": "owned"}}
		action := "agentplugins remove " + r.Installation.InstallationID + " --purge-data"
		if r.Scope != "user" {
			action += " --scope project --project-root " + r.Project
		}
		data["retained_data_action"] = "run `" + action + "` to delete retained plugin data"
	}
	return data
}

func (f *geminiCompleteContractFixture) publicPlan(r *geminiCycleRecipe) map[string]any {
	plan := geminiCompleteContractDecode(f.t, []byte(`{"client_id":"gemini","scope":"","status":"ready","package_mode":"native","activation":"prepared","authentication":"not_checked","policy":"allowed","verification":"package_validated","physical_artifact_id":"","components":[],"user_actions":[],"warnings":["authentication_not_catalog_verified"]}`))
	action := "agentplugins will install and verify Gemini CLI skills and MCP servers automatically; reload them in a running session or restart Gemini CLI"
	if r.Scope != "user" {
		action = "project skills and MCP settings can be configured automatically; native discovery, workspace trust and system policy require observation in Gemini CLI"
	}
	plan["scope"], plan["physical_artifact_id"] = r.scopeName(), r.Binding.PhysicalArtifact
	plan["components"] = []any{map[string]any{"kind": "skill", "name": r.Name, "support": "native"}, map[string]any{"kind": "mcp_server", "name": r.Name, "support": "native"}}
	plan["user_actions"] = []string{"verify the plugin's authentication requirements before using it", action}
	return plan
}

func (f *geminiCompleteContractFixture) publicSecurity(r *geminiCycleRecipe, evidence string) map[string]any {
	row := geminiCompleteContractDecode(f.t, []byte(`{"schema_version":1,"subject":{"tree_digest":"","manifest_digest":""},"scanner":{"id":"lintai","version":"0.1.3"},"policy":{"id":"agent-plugin-install","version":2,"digest":"sha256:9cf869e299d847d7078aeca01f5a182fcdb0144bbf513c83290459991c79e037"},"outcome":"no_blocking_findings","counts":{"blocking":0,"warnings":0,"total":0},"scanned_files":3,"report_digest":"sha256:90f5acb6ff3ac64f669be4f4ba6deea7ae5aa3a9954662441c0e5498b8d56598","evidence_source":""}`))
	row["subject"] = map[string]any{"tree_digest": r.Digest, "manifest_digest": "sha256:" + geminiCycleSHA(r.Authored["plugin.json"])}
	row["evidence_source"] = evidence
	return row
}

func (f *geminiCompleteContractFixture) publicRecords() {
	for _, stage := range []string{"git-a", "git-b", "git-user", "version", "add-user", "add-a", "add-b", "repeat-add-a", "unchanged-update-a", "intact-repair-a", "update-a", "missing-repair-a", "remove-a", "repeat-remove-a", "remove-b", "remove-user"} {
		scope := "a"
		if stage == "version" || strings.HasSuffix(stage, "-user") {
			scope = "user"
		} else if strings.HasSuffix(stage, "-b") {
			scope = "b"
		}
		r := f.views[stage][scope]
		if r == nil {
			r = f.views[""][scope]
		}
		var argv []string
		stdout, stderr, exit := []byte{}, []byte{}, 0
		if strings.HasPrefix(stage, "git-") {
			argv = []string{f.paths["git"], "-c", "core.hooksPath=/dev/null", "init", "--quiet"}
		} else if stage == "version" {
			argv, stdout = []string{f.paths["node"], f.paths["client"], "--version"}, []byte("0.65.0\n")
		} else {
			verb := strings.Split(stage, "-")[0]
			if stage == "repeat-add-a" {
				verb = "add"
			} else if stage == "unchanged-update-a" {
				verb = "update"
			} else if strings.Contains(stage, "repair-") {
				verb = "repair"
			} else if stage == "repeat-remove-a" {
				verb = "remove"
			}
			selector := "uap-gemini-cycle"
			if verb == "add" || stage == "update-a" {
				selector = r.Source
			}
			argv = []string{f.paths["installer"], verb, selector, "--scope", r.scopeName(), "--target", "gemini", "--format", "json"}
			if scope != "user" {
				argv = append(argv, "--project-root", r.Project)
			}
			if verb == "add" {
				argv = append(argv, "--no-save")
			}
			group := "op-" + geminiCycleSHA([]byte("inert operation " + stage))[:32]
			stdout = geminiCompleteContractEncode(f.t, map[string]any{"schema_version": 1, "command": verb, "result": "success", "data": f.publicOutput(r, stage, group)})
			stderr = []byte(geminiCycleProgress(stage))
			if stage == "repeat-remove-a" {
				exit, stdout, stderr = 1, []byte{}, []byte("agentplugins: plugin is not installed for target \"gemini\" in project scope; no target was changed\n")
			}
		}
		f.files[stage+".command.json"] = geminiCompleteContractEncode(f.t, map[string]any{"argv": argv, "cwd": r.Project, "exit": exit, "error": false, "context_error": false, "elapsed_ms": 1, "cleanup": "ordinary full-descendant supervision; forced cleanup is a failure"})
		f.files[stage+".stdout.log"], f.files[stage+".stderr.log"] = stdout, stderr
	}
}

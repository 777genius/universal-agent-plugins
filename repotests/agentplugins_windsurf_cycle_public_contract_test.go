//go:build linux

package pluginkitairepo_test

import (
	"encoding/json"
	"testing"
)

func TestWindsurfCycleRepairAndRemovalUseProductionOutputShapes(t *testing.T) {
	c := windCycleInert(t)
	r := c.w.recipes["a"]
	r.Installation.InstallationID = "held-installation"
	r.Installation.OperationGroupID = "held-operation"
	for _, verb := range []string{"repair", "remove"} {
		t.Run(verb, func(t *testing.T) {
			label := "intact-repair-a"
			status := "completed"
			row := map[string]any{
				"installation_id": "held-installation", "requires_confirmation": false,
				"mutated": true, "group_phase": "external_completed",
			}
			output := map[string]any{
				"operation_id": "held-operation", "plugin": windComponentName,
				"version": windCycleVersion(r), "source": "local", "tree_digest": r.Digest,
				"manifest_digest": r.Installation.Package.ManifestDigest, "dry_run": false, "result": row,
			}
			if verb == "repair" {
				output["next_action"] = "Review the selected native client"
				row["plan"] = map[string]any{
					"client_id": "windsurf", "scope": "project", "status": "ready",
					"package_mode": "prepared_package", "activation": "prepared",
					"authentication": "not_checked", "policy": "allowed", "verification": "package_validated",
					"physical_artifact_id": r.Binding.PhysicalArtifact,
					"components": []any{
						map[string]any{"kind": "skill", "name": windComponentName, "support": "prepared"},
						map[string]any{"kind": "mcp_server", "name": windComponentName, "support": "prepared"},
					},
				}
				row["activation"] = map[string]any{
					"activation": "prepared", "authentication": "not_checked",
					"policy": "allowed", "verification": "installation_verified",
				}
			} else {
				label, status = "remove-a", "data_retained"
				row["plugin"], row["client_id"] = windComponentName, "windsurf"
				row["affected_surfaces"] = []any{"windsurf"}
				row["deactivation"] = map[string]any{
					"activation": "not_required", "artifact_removal_allowed": true, "external_removal_complete": true,
				}
			}
			data := map[string]any{
				"operation_id": "held-operation", "batch": true, "status": status,
				"succeeded": 1, "failed": 0, "plugin": windComponentName, "dry_run": false,
				"targets": []any{map[string]any{"target": "windsurf", "status": "external_completed", "output": output}},
			}
			if verb == "remove" {
				data["data_retained"], data["plugin_data_preserved"] = true, true
			}
			check := func() error {
				raw, err := json.Marshal(map[string]any{"schema_version": 1, "command": verb, "result": "success", "data": data})
				if err != nil {
					t.Fatal(err)
				}
				return c.publicDTO(label, r, raw)
			}
			if err := check(); err != nil {
				t.Fatal("production legacy output refused", err)
			}
			for key, value := range map[string]any{
				"readiness":    "needs_action",
				"next_actions": []any{map[string]any{"code": "verify_native", "actor": "native_client", "message": "Review"}},
			} {
				output[key] = value
				if check() == nil {
					t.Fatal("field from another verb was accepted", key)
				}
				delete(output, key)
			}
			output["scope"] = "project"
			if (check() == nil) != (verb == "remove") {
				t.Fatal("verb-specific scope compatibility differs")
			}
			output["scope"] = "user"
			if check() == nil {
				t.Fatal("foreign scope was accepted")
			}
			delete(output, "scope")
			if verb == "repair" {
				delete(output, "next_action")
				if check() == nil {
					t.Fatal("repair without native guidance was accepted")
				}
			}
		})
	}
}

func TestWindsurfCycleRepeatAddRefusesForeignOperationID(t *testing.T) {
	c := windCycleInert(t)
	r := c.w.recipes["a"]
	r.Installation.InstallationID = "held-installation"
	guidance := "Review the selected native client"
	output := map[string]any{
		"plugin":          windComponentName,
		"version":         windCycleVersion(r),
		"source":          "direct local source",
		"tree_digest":     r.Digest,
		"manifest_digest": r.Installation.Package.ManifestDigest,
		"dry_run":         false,
		"next_action":     guidance,
		"scope":           r.scopeName(),
		"readiness":       "needs_action",
		"next_actions":    []any{map[string]any{"code": "verify_native", "actor": "native_client", "message": guidance}},
		"result": map[string]any{
			"installation_id":       "held-installation",
			"requires_confirmation": false,
			"mutated":               false,
			"no_change":             true,
			"plan": map[string]any{
				"client_id": "windsurf", "scope": r.scopeName(), "status": "ready",
				"package_mode": "prepared_package", "activation": "prepared", "authentication": "not_checked",
				"policy": "allowed", "verification": "package_validated",
				"physical_artifact_id": r.Binding.PhysicalArtifact,
				"components": []any{
					map[string]any{"kind": "skill", "name": windComponentName, "support": "prepared"},
					map[string]any{"kind": "mcp_server", "name": windComponentName, "support": "prepared"},
				},
			},
			"activation": map[string]any{
				"activation": "prepared", "authentication": "not_checked",
				"policy": "allowed", "verification": "installation_verified",
			},
		},
	}
	check := func() error {
		raw, err := json.Marshal(map[string]any{
			"schema_version": 1, "command": "add", "result": "success", "data": output,
		})
		if err != nil {
			t.Fatal(err)
		}
		return c.publicDTO("repeat-add-a", r, raw)
	}
	if err := check(); err != nil {
		t.Fatal("valid scoped repeated add refused", err)
	}
	output["operation_id"] = "injected-group"
	if err := check(); err == nil {
		t.Fatal("foreign operation ID accepted in singleton repeated add")
	}
	delete(output, "operation_id")
	if err := check(); err != nil {
		t.Fatal("repeat-add fixture was not restored", err)
	}
}

"""Read-only Pi paired-artifact core; stdlib, no extraction or source execution."""
import ast
import base64
import hashlib
import io
import json
import ntpath
from pathlib import Path
import posixpath
import re
import zipfile

TRUST_STAGES = [
    "owned_only_project_trusted_by_uap_hook", "project_untrusted_before_grant",
    "confirmed_repeat_granted_owned_project_trust", "changed_owned_content_untrusted",
    "saved_pi_decisions_override_uap_hook", "later_foreign_extension_untrusted",
    "confirmed_update_rerecorded_owned_content", "foreign_project_extension_untrusted",
    "untrusted_project_suppressed", "native_A_B_extension_discovery",
    "native_A_update_B_preserved", "native_A_remove_B_user_foreign_preserved",
    "removal_revoked_project_trust", "last_project_removal_removed_trust_hook",
]
COMPONENT_STAGES = [
    "UAP_project_A_B_installed", "real_Pi_trust_gated_skill_discovery",
    "real_Pi_MCP_initialize_and_tools_list",
    "real_Pi_project_stdio_and_HTTP_initialize_list_call",
    "default_Pi_global_native_absence_after_remove",
    "default_Pi_global_skill_stdio_HTTP_call_update_repair_remove",
    "custom_Pi_global_native_absence_after_remove",
    "custom_Pi_global_skill_stdio_HTTP_call_update_repair_remove",
    "A_update_remove_B_and_foreign_preserved",
]
GATES = [
    "Verify scoped Pi adapter and real UAP facade before native runtime",
    "Verify enterprise policy composition under isolated process environments",
    "Verify Python runner syntax without writing into source checkout",
    "Run genuine pinned native Pi project trust and A/B lifecycle",
    "Upload native Pi evidence including failure",
    "Run real Pi global/project Skills and stdio/HTTP MCP initialize, list and call",
    "Upload Pi Skills/MCP evidence including failure",
]


def audit_pi(blobs, source, run, job, manifest):
    """Caller verifies pinned source Git HEAD/tree and authenticated API metadata.

    blobs has trust/components ZIP bytes. manifest contains head/tree/target,
    run_id/job_id/run_attempt/job_name and artifacts[kind] fixed ID/size/sha256.
    Artifact acquisition metadata is independently validated by caller transport.
    No raw content, paths, URLs, or request IDs enter the bounded return receipt.
    """
    def need(ok, label):
        if not ok:
            raise ValueError(label)

    def sha(body):
        return hashlib.sha256(body).hexdigest()

    def literals(path):
        result = {}
        for node in ast.parse(path.read_bytes()).body:
            if isinstance(node, ast.Assign) and isinstance(node.targets[0], ast.Name):
                try:
                    result[node.targets[0].id] = ast.literal_eval(node.value)
                except (TypeError, ValueError):
                    pass
        return result

    source = Path(source)
    native = literals(source / "scripts/pi_project_native_e2e.py")
    raw = literals(source / "scripts/pi_project_components_native_e2e.py")
    raw_source = ("https://github.com/777genius/universal-agent-plugins/tree/"
                  + raw["PI_RAW_COMMIT"] + "/tests/fixtures/uap_apm/skills")
    need(manifest["target"] in native["PINS"], "pinned_target")
    pin_archive, pin_sha, unused_executable = native["PINS"][manifest["target"]]
    need(run["id"] == manifest["run_id"] and run["head_sha"] == manifest["head"]
         and run["path"] == ".github/workflows/pi-project-native.yml"
         and run["run_attempt"] == manifest["run_attempt"], "run_identity")
    need(job["id"] == manifest["job_id"] and job["run_id"] == run["id"]
         and job["head_sha"] == manifest["head"]
         and job["run_attempt"] == manifest["run_attempt"]
         and job["name"] == manifest["job_name"]
         and job["status"] == "completed" and job["conclusion"] == "success",
         "job_identity_result")
    need(all(step["status"] == "completed" and step["conclusion"] == "success"
             for step in job["steps"]), "actual_steps")
    need(len(job["steps"]) == 13, "actual_step_count")
    for name in GATES:
        matching = [step for step in job["steps"] if step["name"] == name]
        need(len(matching) == 1, "actual_gate_inventory")
    archives = {}
    receipts = {}
    for kind in ("trust", "components"):
        body = blobs[kind]
        fixed = manifest["artifacts"][kind]
        need(isinstance(body, bytes) and len(body) == fixed["size"]
             and len(body) <= 8 * 1024 * 1024
             and sha(body) == fixed["sha256"], "zip_bytes")
        archive = zipfile.ZipFile(io.BytesIO(body))
        names = archive.namelist()
        need(len(names) <= 100 and len(names) == len(set(names))
             and all(name not in ("", ".", "..") and "/" not in name
                     and "\\" not in name and "\x00" not in name and ":" not in name
                     for name in names), "zip_flat_members")
        need(sum(info.file_size for info in archive.infolist()) <= 32 * 1024 * 1024
             and all(not info.is_dir() and not (info.flag_bits & 1)
                     for info in archive.infolist()), "zip_member_bounds")
        need(archive.testzip() is None, "zip_crc")
        archives[kind] = archive
        receipts[kind] = {"artifact_id": fixed["artifact_id"], "bytes": len(body),
                          "sha256": sha(body), "crc": "pass"}
    trust, components = archives["trust"], archives["components"]
    expected_trust = {"a-remove.json", "a-update.json", "a.json", "b-remove.json",
                      "b.json", "c-remove.json", "c-repeat.json", "c-update.json",
                      "c.json", "native-pi-project-evidence.json"}
    need(set(trust.namelist()) == expected_trust, "trust_members")
    t = json.loads(trust.read("native-pi-project-evidence.json"))
    c = json.loads(components.read("pi-project-components-evidence.json"))
    need(t["source_commit"] == c["sha"] == manifest["head"]
         and t["target"] == c["target"] == manifest["target"]
         and t["status"] == c["status"] == "passed", "report_source_result")
    need(t["vendor_auth_qualified"] is False and t["real_model_qualified"] is False
         and c["real_model_qualified"] is False
         and c["mcp_tools_call_qualified"] is True, "qualification_boundaries")
    release = t["pi"]
    need(release == c["release"] and release["archive"] == pin_archive
         and release["sha256"] == pin_sha and release["tag"] == native["VERSION"]
         and re.fullmatch("[0-9a-f]{64}", release["binary_sha256"]),
         "pinned_release")
    need(t["stages"] == TRUST_STAGES and c["stages"] == COMPONENT_STAGES,
         "completed_stage_contract")
    for name in expected_trust - {"native-pi-project-evidence.json"}:
        need(json.loads(trust.read(name))["outcome"] ==
             ("unchanged" if name == "c-repeat.json" else "completed"),
             "trust_lifecycle_outcome")
    path = ntpath if manifest["target"].startswith("windows-") else posixpath
    expected_members = {"pi-project-components-evidence.json",
                        "pi-public-raw-project.json", "pi-public-raw-user.json"}
    hashes = {}
    scopes = {}
    for scope in ("project", "user"):
        r = json.loads(components.read("pi-public-raw-" + scope + ".json"))
        need(r == c["public_raw_skills"][scope]
             and r["scope"] == scope and r["release"] == release,
             "raw_report_binding")
        for flag in ("passed", "public_yes_consent", "foreign_b_user_preserved",
                     "public_add_repeat_ready", "same_iid_binding_payload_and_lock",
                     "no_vendor_trust_override"):
            need(r[flag] is True, "raw_assertion")
        need(r["approval_override"] is False
             and r["model_or_provider_requested"] is False
             and r["native_inventory_count"] == 12
             and r["source"] == raw_source
             and r["skill_sha256"] == sha(raw["PI_RAW_BODY"].encode()),
             "raw_scope_source")
        need(r["proofs"]["add"] == r["proofs"]["repeat"], "raw_repeat_identity")
        proof = r["proofs"]["add"]
        binding = proof["binding"]
        add_command = r["commands"]["add"]
        project_a = add_command["cwd"]
        need(isinstance(project_a, str) and path.isabs(project_a)
             and path.basename(project_a) == "A", "raw_project_a")
        raw_root = path.dirname(project_a)
        project_b = path.join(raw_root, "B")
        outside = path.join(raw_root, "outside")
        user_profile = path.join(raw_root, "home", ".pi", "agent")
        need(project_a == path.join(raw_root, "A")
             and binding["native_profile_root"] ==
             (path.join(project_a, ".pi") if scope == "project" else user_profile),
             "raw_profile_root")
        selected = [(ins, candidate)
                    for ins in proof["state"]["installations"]
                    for candidate in ins.get("clients", {}).values()
                    if candidate.get("client_id") == "pi"
                    and candidate.get("scope") == scope]
        need(len(selected) == 1
             and selected[0][0]["installation_id"] == proof["installation_id"]
             and selected[0][1] == binding, "raw_state_binding_identity")
        installer = add_command["argv"][0]
        need(isinstance(installer, str) and bool(installer), "public_installer")
        def add_argv(name, selected_scope, project):
            argv = [installer, "skills", "add", raw_source, "--skill", name,
                    "--target", "pi", "--scope", selected_scope,
                    "--no-ui", "--yes", "--format", "json"]
            if selected_scope == "project":
                argv += ["--project-root", project]
            return argv
        canonical_commands = {
            "add": (project_a, add_argv(raw["PI_RAW_SKILL"], scope, project_a)),
            "repeat": (project_a, add_argv(raw["PI_RAW_SKILL"], scope, project_a)),
            "remove": (project_a, [installer, "remove", proof["installation_id"],
                                  "--target", "pi", "--scope", scope,
                                  "--format", "json"] +
                       (["--project-root", project_a] if scope == "project" else [])),
        }
        if scope == "project":
            canonical_commands["foreign-b-setup"] = (
                project_b, add_argv("benchmark-secondary", "project", project_b))
        need(binding["client_id"] == "pi" and binding["scope"] == scope
             and (binding["activation"], binding["verification"],
                  binding["authentication"], binding["policy"]) ==
             ("active", "installation_verified", "not_required", "allowed")
             and binding["package_revision"]["resolved_revision"] == raw["PI_RAW_COMMIT"],
             "raw_active_binding")
        need(not any(b.get("client_id") == "pi" and b.get("scope") == scope
                     for ins in r["proofs"]["remove"]["state"]["installations"]
                     for b in ins.get("clients", {}).values()), "raw_removed_binding")
        skill = raw["PI_RAW_SKILL"]
        need(proof["native_tree"][path.join("skills", skill, "SKILL.md")][1]
             == r["skill_sha256"], "raw_skill_payload")
        need(proof["native_tree"][path.join("skills", skill, "references", "fixture.txt")][1]
             == sha(raw["PI_RAW_RESOURCE"]), "raw_resource_payload")
        expected_commands = {"add", "repeat", "remove"}
        if scope == "project":
            expected_commands.add("foreign-b-setup")
        need(set(r["commands"]) == expected_commands, "public_commands")
        for label, command in r["commands"].items():
            expected_cwd, expected_argv = canonical_commands[label]
            need(command["cwd"] == expected_cwd
                 and command["argv"] == expected_argv, "public_command_authority")
            wire = base64.b64decode(command["stdout_base64"], validate=True)
            need(len(wire) <= 1024 * 1024 and sha(wire) == command["stdout_sha256"]
                 and json.loads(wire) == command["dto"], "public_original")
            dto = command["dto"]
            need(dto["schema_version"] == 1 and dto["result"] == "success",
                 "public_result")
            hashes[scope + "-command-" + label] = sha(wire)
            if label in ("add", "repeat"):
                need(dto["readiness"] == "ready" and dto["status"] == "succeeded"
                     and command["argv"][1:3] == ["skills", "add"]
                     and "--yes" in command["argv"]
                     and "--no-ui" in command["argv"], "public_ready_consent")
        need(set(r["inventories"]) == {"baseline", "add", "repeat", "remove"},
             "raw_inventory_phases")
        for phase in ("baseline", "add", "repeat", "remove"):
            need(set(r["inventories"][phase]) == {"a", "b", "outside"},
                 "raw_inventory_places")
            for place in ("a", "b", "outside"):
                name = "raw-" + scope + "-" + phase + "-" + place + ".json"
                expected_members.add(name)
                inv = json.loads(components.read(name))
                need(inv["cwd"] == {"a": project_a, "b": project_b,
                                     "outside": outside}[place]
                     and inv["user_profile"] == user_profile,
                     "native_inventory_root")
                need(inv == r["inventories"][phase][place]
                     and inv["passed"] is True and inv["closure_proved"] is True
                     and inv["binary_unchanged"] is True and inv["schema_version"] == 1
                     and inv["native_binary_sha256"] == release["binary_sha256"],
                     "native_inventory_identity")
                wire = base64.b64decode(inv["stdout_base64"], validate=True)
                need(len(wire) == inv["stdout_bytes"]
                     and len(wire) <= inv["stdout_limit_bytes"] <= 1024 * 1024
                     and sha(wire) == inv["stdout_sha256"], "native_inventory_bytes")
                hashes[name] = sha(wire)
                request_wire = base64.b64decode(inv["request_base64"], validate=True)
                need(0 < len(request_wire) <= 4096 and request_wire.endswith(b"\n")
                     and request_wire.count(b"\n") == 1, "native_request_bound")
                request = json.loads(request_wire)
                need(isinstance(request, dict) and set(request) == {"id", "type"}
                     and request["type"] == "get_commands"
                     and isinstance(request["id"], str)
                     and re.fullmatch("uap-inventory-[0-9a-f]{32}", request["id"]),
                     "native_request_schema")
                responses = [json.loads(line) for line in wire.splitlines() if line.strip()]
                need(all(isinstance(response, dict) for response in responses),
                     "native_response_shape")
                matching = [response for response in responses
                            if isinstance(response.get("id"), str)
                            and response["id"] == request["id"]]
                need(len(matching) == 1 and matching[0] == inv["response"]
                     and isinstance(inv["response"].get("id"), str)
                     and inv["response"]["id"] == request["id"], "native_rpc_join")
                response = matching[0]
                need(response["type"] == "response"
                     and response["command"] == "get_commands"
                     and response["success"] is True, "native_rpc_result")
                rows = response["data"]["commands"]
                need(isinstance(rows, list) and len(rows) <= 512
                     and all(isinstance(row, dict) for row in rows), "native_rows")
                selected = [row for row in rows if row.get("name") == "skill:" + skill]
                present = phase in ("add", "repeat") and (scope == "user" or place == "a")
                need(len(selected) == int(present), "native_selected_presence")
                base = binding["native_profile_root"]
                if present:
                    need(selected == [{
                        "name": "skill:" + skill, "description": raw["PI_RAW_DESCRIPTION"],
                        "source": "skill", "sourceInfo": {
                            "path": path.join(base, "skills", skill, "SKILL.md"),
                            "source": "auto", "scope": scope, "origin": "top-level",
                            "baseDir": base}}], "native_selected_authority")
                users = [row for row in rows
                         if row.get("name") == "skill:uap-pi-foreign-user"]
                need(users == [{
                    "name": "skill:uap-pi-foreign-user",
                    "description": "Foreign user marker", "source": "skill",
                    "sourceInfo": {
                        "path": path.join(inv["user_profile"], "skills",
                                          "uap-pi-foreign-user", "SKILL.md"),
                        "source": "auto", "scope": "user", "origin": "top-level",
                        "baseDir": inv["user_profile"]}}], "native_foreign_user")
                if scope == "project":
                    others = [row for row in rows
                              if row.get("name") == "skill:benchmark-secondary"]
                    expected_b = [{
                        "name": "skill:benchmark-secondary",
                        "description": "An inert second Skill for verifying dependency removal.",
                        "source": "skill", "sourceInfo": {
                            "path": path.join(inv["cwd"], ".pi", "skills",
                                              "benchmark-secondary", "SKILL.md"),
                            "source": "auto", "scope": "project",
                            "origin": "top-level", "baseDir": path.join(inv["cwd"], ".pi")}}]
                    need(others == (expected_b if place == "b" else []),
                         "native_foreign_project_b")
        scopes[scope] = {"native_inventories": 12, "commands": len(expected_commands),
                         "repeat_identity": "pass", "removal": "pass"}
    need(set(components.namelist()) == expected_members, "components_members")
    need(len(hashes) == 31, "original_stream_count")
    return {
        "status": "pass", "audit_kind": "independent paired-artifact check only",
        "head": manifest["head"], "tree": manifest["tree"], "target": manifest["target"],
        "run_id": run["id"], "job_id": job["id"], "archives": receipts,
        "trust_stages": 14, "component_stages": 9, "raw_public_scopes": scopes,
        "original_streams_verified": len(hashes), "actual_steps_success": 13,
        "mcp_tools_call_qualified": True, "vendor_auth_qualified": False,
        "real_model_qualified": False,
        "boundaries": [
            "Real pinned Pi MCP calls use a scripted local model provider.",
            "Non-raw trust/MCP/preservation/cleanup results follow executed pinned assertions and stages; original MCP/provider transcripts and complete filesystem snapshots are not uploaded.",
            "24 raw native inventory and seven public DTO response byte streams are independently decoded and hashed; repeat proof objects retain identical IID/state/binding/native tree/locks.",
            "Raw original serialized state bytes are not uploaded; state_sha256 cannot be recomputed from parsed JSON without assuming serialization.",
            "Tree identity is verified by the wrapper's pinned source checkout, not a native Pi report tree field.",
            "Native release archive and executable bytes are not uploaded; reported binary hashes are only cross-matched across retained release/inventory receipts.",
        ],
    }

"""Inert authored capture envelopes; these tests never execute native clients."""
import base64
import copy
import hashlib
import json
import unittest

import project_native_runtime as runtime
import project_native_windsurf_components as component


def raw(value):
    return json.dumps(value, sort_keys=True, separators=(",", ":")).encode()


def sha(body):
    return hashlib.sha256(body).hexdigest()


def proof(scope, layout, index):
    source = layout["root"] + "/component-source-" + scope
    iid = "01234567-89ab-4cde-8fab-0123456789a" + str(index)
    physical = "uap-windsurf-component-" + sha(iid.encode())[:12]
    target = "user" if scope == "user" else "project"
    state_root = layout["root"] + "/installer-state" if scope == "user" else layout[scope] + "/.uap"
    active, data_path = state_root + "/managed/clients/windsurf/" + physical, state_root + "/plugin-data/" + physical
    bid = "client_" + sha("\x00".join((iid, "windsurf", target, active)).encode())[:24]
    did = "data_" + sha("\x00".join((iid, physical, target)).encode())[:24]
    nonce, source_digest, payload_digest = str(index) * 32, "sha256:" + sha(source.encode()), "sha256:" + sha(active.encode())
    env = {"PLUGIN_ROOT": active, "PLUGIN_DATA": data_path, "UAP_TEST_ROOT": layout["root"], "UAP_TEST_EVENTS": layout["root"] + "/events.jsonl", "UAP_TEST_NONCE": nonce, "UAP_TEST_REVISION": scope + "-one"}
    entry = {"command": active + "/io.github.777genius.agentplugins/managed-stdio-v1/agentplugins", "args": ["--internal-stdio-v1", active, data_path, active, "plugin", "--", "./bin/probe", "Owned Windsurf " + scope, active + "/argument"], "env": env}
    manifest = {"$schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json", "name": "uap-windsurf-component", "version": "1.0.0", "description": "Disposable Windsurf component observation"}
    manifest_digest = "sha256:" + sha(raw(manifest))
    package = {"loader_kind": "agent_plugins", "format_id": "agent-plugins/1.0.0", "schema_uri": manifest["$schema"], "declared_name": "uap-windsurf-component", "version": "1.0.0", "manifest_digest": manifest_digest, "inventory": {"mcp_present": True, "mcp_enabled": True, "skills": ["uap-windsurf-component"], "mcp_servers": ["uap-windsurf-component"]}}
    data = {"data_receipt_id": did, "physical_backend_id": physical, "scope": target, "locator": data_path, "ownership_digest": "sha256:" + sha(("agentplugins-plugin-data-v1\0" + did + "\0" + data_path).encode()), "state": "owned"}
    kind = "windsurf_mcp_entry" if scope == "user" else "devin_project_mcp_server"
    codec = "windsurf-mcpServers" if scope == "user" else "devin-mcpServers"
    profile = layout["home"] + "/.codeium/windsurf" if scope == "user" else layout[scope] + "/.devin"
    mcp_digest = "sha256:" + sha(("agentplugins-native-mcp-v1\0" + codec + "\0uap-windsurf-component\0").encode() + raw(entry))
    objects = [{"object_id": "package:windsurf:" + physical, "kind": "managed_package_directory", "logical_name": "uap-windsurf-component", "path": active, "managed_digest": payload_digest, "protection_class": "managed"},
               {"object_id": ("windsurf:mcp:" if scope == "user" else kind + ":") + "uap-windsurf-component", "kind": kind, "logical_name": "uap-windsurf-component", "path": profile + "/mcp_config.json", "managed_digest": mcp_digest, "protection_class": "managed_entry" if scope == "user" else "managed", **({"source_relative": "mcp.json"} if scope == "user" else {})}]
    if scope != "user":
        body = ("---\nname: uap-windsurf-component\ndescription: " + scope + "-one\n---\nUAP_WINDSURF_" + scope + "-one_" + nonce + "\n").encode()
        objects.append({"object_id": "devin_project_skill_directory:uap-windsurf-component", "kind": "devin_project_skill_directory", "logical_name": "uap-windsurf-component", "path": profile + "/skills/uap-windsurf-component", "source_relative": "skills/uap-windsurf-component", "managed_digest": "sha256:" + sha(b"f\0SKILL.md\0-\0" + body + b"\0"), "protection_class": "managed"})
    binding = {"client_binding_id": bid, "client_id": "windsurf", "scope": target, "target_locator": active, "physical_artifact_id": physical, "materialization": "materialized", "activation": "active" if scope == "user" else "prepared", "authentication": "not_checked", "policy": "allowed", "verification": "installation_verified", "data_receipt_id": did, "affected_surfaces": ["windsurf"], "updated_at":"2026-10-27T00:00:00Z", "package_revision": {"version": "1.0.0", "tree_digest": source_digest, "manifest_digest": manifest_digest}, "native_objects": objects,
               "receipts": [{"operation_id": "inert-001", "operation_group_id": "inert", "sequence": 1, "mutation_type": "directory_swap", "client_binding_id": bid, "active_path": active, "staging_path":state_root+"/managed/clients/windsurf/.agentplugins-staging-inert", "backup_path":state_root+"/managed/clients/windsurf/.agentplugins-backup-"+sha(b"inert-001")[:16], "after_digest": payload_digest, "phase": "committed"}]}
    if scope != "user": binding["native_profile_root"] = profile
    item = {"installation_id": iid, "declared_name": "uap-windsurf-component", "source": {"source_binding_id": "src_" + sha((source + "\0\0").encode())[:24], "requested_source": source, "canonical_source": source, "resolved_revision": "", "tree_digest": source_digest}, "package": package, "clients": {bid: binding}, "origin_mode": "direct", "operation_group_id": "inert", "data_receipts": {did: data}, "created_at": "2026-10-27T00:00:00Z", "updated_at": "2026-10-27T00:00:00Z"}
    state = raw({"schema_version": 5, "installations": [item]})
    return {"scope": scope, "source": source, "revision": scope + "-one", "nonce": nonce, "source_digest": source_digest, "state": base64.b64encode(state).decode(), "state_bytes": len(state), "state_sha256": sha(state), "binding": binding, "native_entry": entry, "data": data, "payload_digest": payload_digest, "user_skill_activated": False}


def fixture(count=23, stage=None):
    source = {"source_commit": "a" * 40, "source_tree": "b" * 40}
    root = "/inert/windsurf"
    layout = {"root": root, "home": root + "/home", "a": root + "/project", "b": root + "/project-b", "outside": root + "/outside", "installer": "/inert/bin/agentplugins", "client": "/inert/vendor/bin/devin", "git": "/usr/bin/git"}
    completed = list(component.LABELS[:count])
    selected = [label[4:] for label in completed if label.startswith("add-")]
    proofs = [proof(scope, layout, index + 1) for index, scope in enumerate(selected)]
    summary = {"client": "windsurf", "version": "3000.11.205", "capture_mode": component.MODE, "capture_review_required": True, "runtime_attempt_complete": False, "test_failed": True, "activation": "prepared", "vendor_authentication_qualified": False, "real_model_qualified": False, "qualification": dict(component.QUAL), "source_identity": {"installer_base_commit": source["source_commit"], "installer_tree": source["source_tree"], "installer_source_state": "committed", "installer_patch_sha256": []}, "component_layout": layout, "completed_labels": completed, "component_proof_count": len(proofs), "attempt_stage": stage or ("capture-review" if count == 23 else completed[-1] if count else "prepare")}
    bodies = {"runtime-attempt.json": raw(summary), "probe-events.jsonl": b""}
    if proofs: bodies["component-proof.jsonl"] = b"".join(raw(row) + b"\n" for row in proofs)
    for label in completed:
        cwd, argv = component._command(label, layout)
        bodies[label + ".command.json"] = raw({"argv": argv, "cwd": cwd, "exit": 0, "error": False, "deadline": False, "elapsed_ms": 1, "cleanup": "full descendant supervision; forced cleanup fails"})
        bodies[label + ".stdout.log"] = b"devin 3000.11.205 (6dea39440ae3)\n" if label == "version" else b"unparsed native/private evidence"
        bodies[label + ".stderr.log"] = b""
    return bodies, source


def rewrite_proof(bodies, change):
    rows = [runtime.strict_json(line) for line in bodies["component-proof.jsonl"].splitlines()]
    change(rows)
    bodies["component-proof.jsonl"] = b"".join(raw(row) + b"\n" for row in rows)


class WindsurfComponentTests(unittest.TestCase):
    def test_complete_and_all_failed_prefixes_remain_unqualified(self):
        for count in range(24):
            bodies, source = fixture(count)
            result = runtime.cli_attempt_members(bodies, source, "windsurf")
            self.assertFalse(result["runtime_attempt_complete"])
            self.assertTrue(result["test_failed"])
        bodies, source = fixture()
        self.assertEqual(len(bodies), 72)
        for name in bodies:
            if name == "probe-events.jsonl": continue
            with self.subTest(missing=name), self.assertRaises(ValueError):
                runtime.cli_attempt_members({k: v for k, v in bodies.items() if k != name}, source, "windsurf")

    def test_modes_flags_labels_triples_and_process_continuation_refuse(self):
        for kind in ("mode", "client", "complete", "passed", "auth", "model", "qualification", "flag-type", "source", "skip", "repeat", "stage", "partial", "orphan", "argv", "cwd", "cleanup", "earlier-error", "earlier-stderr", "version", "bound"):
            bodies, source = fixture(); summary = runtime.strict_json(bodies["runtime-attempt.json"])
            if kind == "mode": summary["capture_mode"] = "unknown"
            elif kind == "client": summary["client"] = "grok"
            elif kind == "complete": summary["runtime_attempt_complete"] = True
            elif kind == "passed": summary["test_failed"] = False
            elif kind == "auth": summary["vendor_authentication_qualified"] = True
            elif kind == "model": summary["real_model_qualified"] = True
            elif kind == "qualification": summary["qualification"]["project-lifecycle"] = True
            elif kind == "flag-type": summary["qualification"]["project-lifecycle"] = 0
            elif kind == "source": summary["source_identity"]["installer_tree"] = "c" * 40
            elif kind == "skip": summary["completed_labels"].pop(4)
            elif kind == "repeat": summary["completed_labels"][2] = "git-a"
            elif kind == "stage": summary["attempt_stage"] = "other"
            elif kind == "partial": del bodies["b-get.stderr.log"]
            elif kind == "orphan": bodies["native-attempt.stdout.log"] = b""
            elif kind in ("argv", "cwd", "cleanup", "earlier-error"):
                row = runtime.strict_json(bodies["empty-a-skills.command.json"])
                row[{"argv": "argv", "cwd": "cwd", "cleanup": "cleanup", "earlier-error": "error"}[kind]] = {"argv": ["/unregistered/client"], "cwd": "/foreign", "cleanup": "assumed", "earlier-error": True}[kind]
                bodies["empty-a-skills.command.json"] = raw(row)
            elif kind == "earlier-stderr": bodies["empty-a-skills.stderr.log"] = b"unknown warning"
            elif kind == "version": bodies["version.stdout.log"] = b"other version\n"
            elif kind == "bound": bodies["b-get.stdout.log"] = b"x" * ((1 << 20) + 1)
            bodies["runtime-attempt.json"] = raw(summary)
            with self.subTest(kind=kind), self.assertRaises(ValueError): runtime.cli_attempt_members(bodies, source, "windsurf")

    def test_proof_corruption_and_same_name_scope_confusion_refuse(self):
        for kind in ("scope", "source", "revision", "state-sha", "data", "entry", "objects", "package", "receipt", "extra-state", "unknown-state", "schema", "binding-key", "user-skill", "unknown-binding", "unknown-receipt", "unknown-data", "intent-automatic", "intent-prepare", "intent-empty", "intent-type"):
            bodies, source = fixture()
            def change(rows):
                row = rows[0]
                if kind in ("scope", "source", "revision", "state-sha", "user-skill"):
                    key = {"state-sha": "state_sha256", "user-skill": "user_skill_activated"}.get(kind, kind)
                    row[key] = True if kind == "user-skill" else "foreign"
                elif kind == "data": row["data"]["locator"] = "/foreign/data"
                elif kind == "entry": row["native_entry"]["env"]["PLUGIN_ROOT"] = "/foreign/package"
                else:
                    state = runtime.strict_json(base64.b64decode(row["state"]))
                    item = state["installations"][0]
                    if kind.startswith("intent-"): row["binding"]["install_intent"] = {"intent-automatic":"automatic", "intent-prepare":"prepare", "intent-empty":"", "intent-type":True}[kind]
                    elif kind == "unknown-binding": row["binding"]["unknown"] = True
                    elif kind == "unknown-receipt": row["binding"]["receipts"][0]["unknown"] = True
                    elif kind == "unknown-data": row["data"]["unknown"] = True;item["data_receipts"]={row["data"]["data_receipt_id"]:row["data"]}
                    elif kind == "objects": row["binding"]["native_objects"][0]["object_id"] = "foreign"
                    elif kind == "package": item["package"]["inventory"]["skills"] = []
                    elif kind == "receipt": row["binding"]["receipts"][0]["phase"] = "pending"
                    elif kind == "extra-state": state["installations"].append(copy.deepcopy(item))
                    elif kind == "unknown-state": state["unexpected"] = True
                    elif kind == "schema": state["schema_version"] = 2
                    elif kind == "binding-key": item["clients"] = {"foreign": row["binding"]}
                    if kind in ("objects", "receipt", "unknown-binding", "unknown-receipt") or kind.startswith("intent-"): item["clients"] = {row["binding"]["client_binding_id"]: row["binding"]}
                    body = raw(state); row.update(state=base64.b64encode(body).decode(), state_bytes=len(body), state_sha256=sha(body))
            rewrite_proof(bodies, change)
            with self.subTest(kind=kind), self.assertRaises(ValueError): runtime.cli_attempt_members(bodies, source, "windsurf")


if __name__ == "__main__":
    unittest.main()

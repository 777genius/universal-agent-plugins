"""Inert Cursor stage/state/protocol joins; no vendor process or credential."""
import base64
import copy
import hashlib
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch
import zipfile

import project_native_cursor_cycle as cycle
import project_native_runtime as runtime


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def wire(raw):
    return {"bytes": len(raw), "sha256": sha(raw), "body": base64.b64encode(raw).decode()}


def compact(value):
    return json.dumps(value, sort_keys=True, separators=(",", ":"), ensure_ascii=False).encode()


def synthetic_cycle():
    # Independent literal recipe. Pin/archive validation has its own refusal
    # contract; this synthetic fixture isolates the public/state/probe joins.
    order = "git-a git-b version empty-a empty-b empty-outside add-a approve-a-one installed-a add-b approve-b-one installed-b installed-outside repeat-add-a unchanged-update-a intact-repair-a stable-a stable-b update-a approve-a-two updated-a updated-b updated-outside damaged-a missing-repair-a repaired-a remove-a removed-a remaining-b removed-outside repeat-remove-a remove-b final-a final-b final-outside".split()
    p = {"root": "/owned", "home": "/owned/home", "a": "/owned/project", "b": "/owned/project-B", "outside": "/owned/outside", "source_a": "/owned/package", "source_b": "/owned/package-B", "nonce": "1" * 32, "installer": "/held/uap", "client": "/pin/dist-package/cursor-agent", "git": "/usr/bin/git", "installer_sha256": sha(b"inert-installer"), "probe_sha256": sha(b"inert-probe")}
    commands, proofs, probes, latest, approval_ids = [], [], [], {}, {"a": [], "b": []}
    prefix, session_count = b"", 0
    schema = "https://agent-plugins.org/schemas/1.0.0/"
    namespace = "io.github.777genius.agentplugins/managed-stdio-v1/"
    def proof(stage, value):
        proofs.append({"stage": stage, "value": value})
    def source(scope, version):
        revision = scope + "-" + version
        nonce = sha((p["nonce"] + "\0" + revision).encode())[:32]
        manifest = compact({"$schema": schema + "plugin.schema.json", "name": "native-proof", "version": version}) + b"\n"
        config = compact({"$schema": schema + "mcp.schema.json", "mcpServers": {"uapcursora": {"type": "stdio", "command": "./bin/probe", "args": ["cursor-component"], "cwd": "${PLUGIN_ROOT}", "env": {"UAP_TEST_ROOT": p["root"], "UAP_TEST_EVENTS": "/owned/events.jsonl", "UAP_TEST_NONCE": nonce, "UAP_TEST_REVISION": revision}}}}) + b"\n"
        value = {"scope": scope, "source": p["source_" + scope], "version": version, "revision": revision, "nonce": nonce, "digest": "sha256:" + sha(manifest + config), "manifest": wire(manifest), "mcp": wire(config), "probe_sha256": p["probe_sha256"]}
        proof("source-" + scope + "-" + version, value)
        return value, manifest
    sources = {scope: source(scope, "1.0.0") for scope in ("a", "b")}
    stores = {}
    for scope in ("a", "b", "outside"):
        key = p[scope].strip("/").replace("/", "-")
        stores[scope] = p["home"] + "/.cursor/projects/" + key + "/"
    proof("original-controls", {"user_config": "/owned/home/.cursor/mcp.json", "user_body": wire(b'{"mcpServers":{}}\n'), "foreign_user": "/owned/home/foreign-owned.txt", "approval_controls": [stores[s] + n for s in ("a", "b", "outside") for n in ("mcp-approvals.json", "mcp-disabled.json")]})
    def server(selected):
        return {"command": selected["active"] + "/" + namespace + "agentplugins", "args": ["--internal-stdio-v1", selected["active"], selected["data"]["locator"], selected["active"], "plugin", "--argv-base64-v1", "Li9iaW4vcHJvYmU=", "Y3Vyc29yLWNvbXBvbmVudA=="], "env": {"PLUGIN_ROOT": selected["active"], "PLUGIN_DATA": selected["data"]["locator"], "UAP_TEST_ROOT": "/owned", "UAP_TEST_EVENTS": "/owned/events.jsonl", "UAP_TEST_NONCE": selected["nonce"], "UAP_TEST_REVISION": selected["revision"]}}
    for index, stage in enumerate(order):
        scope = "b" if stage.endswith("-b") or stage == "approve-b-one" else "a"
        start, end = f"2026-10-29T22:00:{index:02d}Z", f"2026-10-29T22:00:{index:02d}.5Z"
        if stage == "update-a":
            sources["a"] = source("a", "1.1.0")
        if stage == "damaged-a":
            proof("missing-native-a", {"scope": "a", "path": p["a"] + "/.cursor/mcp.json", "before": wire(compact({"mcpServers": {"uapcursora": server(latest["a"])}})), "managed_package_unchanged": True})
        out, err, delta = b"", b"", b""
        cwd = p["outside"] if stage == "version" or "outside" in stage else p[scope]
        if stage.startswith("git-"):
            argv = [p["git"], "init", "--quiet", "--initial-branch=main"]
        elif stage == "version":
            argv, out = [p["client"], "--version"], b"2026.10.22-27dcf5f\n"
        elif stage in {"add-a", "add-b", "repeat-add-a", "unchanged-update-a", "intact-repair-a", "update-a", "missing-repair-a", "remove-a", "repeat-remove-a", "remove-b"}:
            src, manifest = sources[scope]
            version = src["version"]
            old = latest.get(scope)
            changed = stage not in ("repeat-add-a", "unchanged-update-a", "repeat-remove-a")
            removed = stage in ("remove-a", "remove-b", "repeat-remove-a")
            iid = old["installation"]["installation_id"] if old else ("a" if scope == "a" else "b") * 8 + "-1111-2222-3333-444444444444"
            physical = "native-proof-" + sha(iid.encode())[:12]
            active = p[scope] + "/.uap/managed/clients/cursor/" + physical
            did = "data_" + sha((iid + "\0" + physical + "\0project").encode())[:24]
            bid = "client_" + sha((iid + "\0cursor\0project\0" + active).encode())[:24]
            locator = p[scope] + "/.uap/plugin-data/" + physical
            data = old["data"] if old else {"data_receipt_id": did, "physical_backend_id": physical, "scope": "project", "locator": locator, "ownership_digest": "sha256:" + sha(("agentplugins-plugin-data-v1\0" + did + "\0" + locator).encode()), "state": "owned", "created_at": start, "updated_at": start}
            group = "op-" + f"{index:032x}"
            selected = {"scope": scope, "version": version, "nonce": src["nonce"], "revision": src["revision"], "source_digest": src["digest"], "removed": removed, "active": active, "data": data}
            native = {"mcpServers": {} if removed else {"uapcursora": server(selected)}}
            native_raw = compact(native)
            pack = {"loader_kind": "agent_plugins", "format_id": "agent-plugins/1.0.0", "schema_uri": schema + "plugin.schema.json", "declared_name": "native-proof", "version": version, "manifest_digest": "sha256:" + sha(manifest), "inventory": {"mcp_present": True, "mcp_enabled": True, "mcp_servers": ["uapcursora"]}}
            i = {"installation_id": iid, "declared_name": "native-proof", "source": {"source_binding_id": "src_" + sha((p["source_" + scope] + "\0\0").encode())[:24], "requested_source": p["source_" + scope], "canonical_source": p["source_" + scope], "resolved_revision": "", "tree_digest": src["digest"]}, "package": pack, "origin_mode": "direct", "operation_group_id": group, "data_receipts": {did: data}, "clients": {}, "created_at": old["installation"]["created_at"] if old else start, "updated_at": start}
            state = {"schema_version": 5, "installations": [i]}
            old_binding = old["binding"] if old else None
            if changed:
                op = group + "-001"
                base = active.rsplit("/", 1)[0]
                receipt = {"operation_id": op, "operation_group_id": group, "sequence": len(old_binding["receipts"]) + 1 if old_binding else 1, "mutation_type": "directory_remove" if removed else "directory_swap", "client_binding_id": bid, "active_path": active, "backup_path": base + "/.agentplugins-backup-" + sha(op.encode())[:16], "phase": "committed"}
                if old_binding:
                    receipt["before_digest"] = old_binding["native_objects"][0]["managed_digest"]
            if removed:
                i["data_retained"] = True
                state["transaction_receipts"] = [receipt] if changed else old["state"]["transaction_receipts"]
                binding = old_binding
            else:
                package_digest = "sha256:" + sha((scope + version).encode())
                if changed:
                    receipt["staging_path"], receipt["after_digest"] = base + "/.agentplugins-staging-" + sha(op.encode())[:16], package_digest
                binding = {"client_binding_id": bid, "client_id": "cursor", "scope": "project", "target_locator": active, "physical_artifact_id": physical, "materialization": "materialized", "activation": "prepared", "authentication": "not_checked", "policy": "allowed", "verification": "installation_verified", "package_revision": {"version": version, "tree_digest": src["digest"], "manifest_digest": pack["manifest_digest"]}, "data_receipt_id": did, "affected_surfaces": ["cursor"], "native_profile_root": p[scope] + "/.cursor", "native_objects": [{"object_id": "package:cursor:" + physical, "kind": "managed_package_directory", "logical_name": "native-proof", "path": active, "managed_digest": package_digest, "protection_class": "managed"}, {"object_id": "cursor_project_mcp_server:uapcursora", "kind": "cursor_project_mcp_server", "logical_name": "uapcursora", "path": p[scope] + "/.cursor/mcp.json", "managed_digest": "sha256:" + sha(b"agentplugins-native-mcp-v1\0mcpServers\0uapcursora\0" + compact(server(selected))), "protection_class": "managed"}], "receipts": (old_binding["receipts"] if old_binding else []) + ([receipt] if changed else []), "updated_at": start}
                i["clients"] = {bid: binding}
            if not changed:
                state, i, binding = copy.deepcopy(old["state"]), copy.deepcopy(old["installation"]), copy.deepcopy(old_binding)
            value = {key: selected[key] for key in ("scope", "version", "nonce", "revision", "source_digest", "removed")}
            value.update(state=wire(compact(state)), native_config=wire(native_raw), data={"receipt": data, "owner": wire(compact(data)), "caller": wire(("caller-data-" + scope + "-" + p["nonce"] + "\n").encode())})
            if not removed:
                metadata = json.dumps({"protocol": 1, "sha256": p["installer_sha256"], "cliVersion": "0.1.0-development"}, separators=(",", ":")).encode()
                fixed = {"plugin.json": manifest, "mcp.json": json.dumps(native, sort_keys=True, ensure_ascii=False, indent=2).encode() + b"\n", ".cursor-plugin/plugin.json": json.dumps({"name": "native-proof", "version": version, "mcpServers": "./mcp.json"}, sort_keys=True, indent=2).encode() + b"\n", namespace + "metadata.json": metadata, "bin/probe": b"inert-probe", namespace + "agentplugins": b"inert-installer"}
                value["payload"] = {"root": active, "digest": package_digest, "files": {name: {"bytes": len(raw), "sha256": sha(raw), "executable": name in ("bin/probe", namespace + "agentplugins")} for name, raw in fixed.items()}}
            proof(stage, value)
            latest[scope] = {**selected, "installation": i, "state": state, "binding": binding}
            verb = "repair" if "repair" in stage else "update" if "update" in stage else "remove" if "remove" in stage else "add"
            argv = [p["installer"], verb, p["source_" + scope] if verb == "add" else iid] + (["--no-save"] if verb == "add" else []) + ["--scope", "project", "--project-root", p[scope], "--target", "cursor", "--format", "json"]
            if stage == "repeat-remove-a":
                err = b'agentplugins: plugin is not installed for target "cursor" in project scope; no target was changed\n'
            else:
                outcome = {"installation_id": iid, "requires_confirmation": False, "mutated": changed}
                if removed:
                    outcome.update(plugin="native-proof", client_id="cursor", affected_surfaces=["cursor"], group_phase="external_completed", deactivation={"activation": "not_required", "artifact_removal_allowed": True, "external_removal_complete": True})
                else:
                    outcome.update(plan={"client_id": "cursor", "scope": "project", "status": "ready", "package_mode": "native", "activation": "prepared", "authentication": "not_checked", "policy": "allowed", "verification": "package_validated", "physical_artifact_id": physical, "components": [{"kind": "mcp_server", "name": "uapcursora", "support": "native"}]}, activation={"activation": "prepared", "authentication": "not_checked", "policy": "allowed", "verification": "installation_verified"})
                    if stage != "repeat-add-a":
                        outcome["group_phase"] = "external_completed"
                    if stage in ("repeat-add-a", "unchanged-update-a"):
                        outcome["no_change"] = True
                output = {"plugin": "native-proof", "version": version, "source": "local" if verb in ("repair", "remove") else "direct local source", "tree_digest": src["digest"], "manifest_digest": pack["manifest_digest"], "dry_run": False, "result": outcome}
                data_out = output
                if stage != "repeat-add-a":
                    output["operation_id"] = group
                    target = {"target": "cursor", "status": "external_completed", "output": output}
                    if verb == "update":
                        target["selected"] = True
                    data_out = {"operation_id": group, "batch": True, "status": "data_retained" if removed else "completed", "succeeded": 1, "failed": 0, "plugin": "native-proof", "dry_run": False, "targets": [target]}
                    if verb == "add":
                        data_out["environment"] = {"reason": "no_save_requested", "saved": False}
                    if removed:
                        data_out.update(plugin_data_preserved=True, data_retained=True, retained_data=[{k: data[k] for k in ("data_receipt_id", "physical_backend_id", "scope", "state")}])
                out = compact({"schema_version": 1, "command": verb, "result": "success", "data": data_out})
                if stage in ("add-a", "add-b"):
                    err = b"Resolving and validating one Agent Plugin package for every selected target...\nInstalling native-proof for 1 selected clients...\n"
                elif stage == "repeat-add-a":
                    err = b"Resolving and validating one Agent Plugin package for every selected target...\nApplying transactional client package...\n"
                elif "update" in stage:
                    err = b"Resolving and validating one updated Agent Plugin package for every selected target...\n"
                elif "repair" in stage:
                    err = b"Resolving and validating each unique exact installed package revision once...\n"
        else:
            verb = "enable" if stage.startswith("approve-") else "list-tools" if stage in {"installed-a", "installed-b", "stable-a", "stable-b", "updated-a", "updated-b", "repaired-a", "remaining-b"} else "list"
            argv = [p["client"], "mcp", verb] + (["uapcursora"] if verb != "list" else [])
            if verb == "enable":
                selected = latest[scope]
                raw = server(selected)
                parsed = {"command": raw["command"], "args": raw["args"], "env": dict(sorted(raw["env"].items()))}
                fingerprint = "uapcursora-" + sha(json.dumps({"path": p[scope], "server": parsed}, separators=(",", ":")).encode())[:16]
                approval_ids[scope].append(fingerprint)
                proof(stage, {"scope": scope, "path": stores[scope] + "mcp-approvals.json", "body": wire(json.dumps(approval_ids[scope], indent=2).encode()), "fingerprints": list(approval_ids[scope]), "probe_bytes": len(prefix), "probe_sha256": sha(prefix)})
                out = "✓ Enabled and approved MCP server: uapcursora\n".encode()
            elif verb == "list-tools":
                session_count += 1
                pid = 100 + session_count
                delta = b"".join(compact({"method": method, "pid": pid, "session": f"{pid}-{1000+pid}", "nonce": latest[scope]["nonce"], "timestamp": start, "facts": None}) + b"\n" for method in ("initialize", "tools/list", "tools/list"))
                out = b"Tools for uapcursora (1):\n- inspect_runtime (marker, nonce, operation)\n"
            else:
                out = b"No MCP servers configured (expected in .cursor/mcp.json or ~/.cursor/mcp.json)\n"
        commands.append({"stage": stage, "value": {"argv": argv, "cwd": cwd, "exit": 1 if stage == "repeat-remove-a" else 0, "error": False, "deadline": False, "elapsed_ms": 500, "cleanup": "full descendant supervision; forced cleanup fails", "started_at": start, "ended_at": end, "stdout": wire(out), "stderr": wire(err)}})
        probe = {"before_bytes": len(prefix), "before_sha256": sha(prefix), "delta": wire(delta)}
        prefix += delta
        probe.update(after_bytes=len(prefix), after_sha256=sha(prefix))
        probes.append({"stage": stage, "value": probe})
    result = {"cycle_layout": p, "cycle_probe": {"bytes": len(prefix), "sha256": sha(prefix), "sessions": session_count}, "elapsed_ms": 35000}
    return commands, proofs, probes, result


class CursorCycleContracts(unittest.TestCase):
    def test_failed_prefix_stays_false_and_cannot_forge_completion(self):
        source = {"source_commit": "a" * 40, "source_tree": "b" * 40}
        summary = {"client": "cursor", "version": "2026.10.22-27dcf5f", "capture_mode": cycle.MODE, "activation": "prepared", "runtime_attempt_complete": False, "test_failed": True, "cycle_observed_complete": False, "vendor_authentication_qualified": False, "real_model_qualified": False, "qualification": {key: False for key in cycle.FLAGS}, "qualification_scope": cycle.SCOPE, "source_identity": {"installer_base_commit": "a" * 40, "installer_tree": "b" * 40, "installer_source_state": "committed", "installer_patch_sha256": []}, "acquisition": {}, "scanner": {}, "elapsed_ms": 0, "cycle_command_stages": [], "cycle_proof_stages": [], "cycle_probe": {"bytes": 0, "sha256": sha(b""), "sessions": 0}}
        bodies = {name: b"" for name in cycle.MEMBERS}
        bodies["runtime-attempt.json"] = compact(summary)
        self.assertEqual(cycle.members(bodies, source), summary)
        for change in ({"runtime_attempt_complete": True, "test_failed": False, "cycle_observed_complete": True, "qualification": {key: key != "mcp-tools-call" for key in cycle.FLAGS}}, {"qualification": {key: True for key in cycle.FLAGS}}, {"vendor_authentication_qualified": True}, {"unknown": True}, {"capture_mode": "cursor-anything"}, {"cycle_command_stages": ["version"]}):
            with self.assertRaises(ValueError):
                cycle.members({**bodies, "runtime-attempt.json": compact({**summary, **change})}, source)
        with self.assertRaises(ValueError):
            cycle.members({**bodies, "unknown.log": b""}, source)

    def test_probe_validates_every_session_row_and_refuses_calls(self):
        start, end = cycle.stamp("2026-10-29T22:00:00Z"), cycle.stamp("2026-10-29T22:00:01Z")
        def event(method, pid=7, **changes):
            return {"method": method, "nonce": "held", "pid": pid, "session": str(pid) + "-100", "timestamp": "2026-10-29T22:00:00Z", "facts": None, **changes}
        def raw(rows):
            return b"".join(compact(row) + b"\n" for row in rows)
        rows = [event("initialize"), event("tools/list"), event("initialize", 8), event("tools/list", 8), event("tools/list")]
        self.assertEqual(cycle.probe_delta(raw(rows), "held", start, end, set()), {"7-100", "8-100"})
        hostile = [[event("tools/list")], [event("initialize")], [event("initialize"), event("tools/call")],
                   [event("initialize"), event("initialize"), event("tools/list")]]
        for replacement in ({"facts": {}}, {"nonce": "foreign"}, {"pid": True}, {"session": "7-0100"}, {"timestamp": "2026-10-29T22:00:02Z"}, {"unknown": True}):
            altered = copy.deepcopy(rows)
            altered[1].update(replacement)
            hostile.append(altered)
        for captured in hostile:
            with self.assertRaises(ValueError):
                cycle.probe_delta(raw(captured), "held", start, end, set())
        with self.assertRaises(ValueError):
            cycle.probe_delta(raw(rows), "held", start, end, {"8-100"})
        # RFC3339Nano must not lose the final three fractional digits.
        left = cycle.stamp("2026-10-29T22:00:00.000000100Z")
        right = cycle.stamp("2026-10-29T22:00:00.000000200Z")
        reversed_rows = [event("initialize", timestamp="2026-10-29T22:00:00.000000150Z"), event("tools/list", timestamp="2026-10-29T22:00:00.000000149Z")]
        with self.assertRaises(ValueError):
            cycle.probe_delta(raw(reversed_rows), "held", left, right, set())
        outside_rows = [event("initialize", timestamp="2026-10-29T22:00:00.000000099Z"), event("tools/list", timestamp="2026-10-29T22:00:00.000000150Z")]
        with self.assertRaises(ValueError):
            cycle.probe_delta(raw(outside_rows), "held", left, right, set())

    def test_proof_stage_binds_scope_and_updated_revision(self):
        from project_native_cursor_cycle_proof import validate
        for stage, replacement in (("add-a", "add-b"), ("update-a", "intact-repair-a"), ("missing-repair-a", "intact-repair-a")):
            commands, proofs, _, result = synthetic_cycle()
            original = next(row for row in proofs if row["stage"] == replacement)
            next(row for row in proofs if row["stage"] == stage)["value"] = copy.deepcopy(original["value"])
            with self.assertRaises(ValueError):
                validate(proofs, result, commands)

    def test_full_inert_public_state_approval_probe_join(self):
        captured = synthetic_cycle()
        with patch.object(cycle, "pins") as pins:
            cycle.complete(*captured)
            pins.assert_called_once()

    def test_full_serialized_archive_admission_includes_approval_containers(self):
        commands, proofs, probes, result = synthetic_cycle()
        source = {"source_commit": "a" * 40, "source_tree": "b" * 40}
        summary = {"client": "cursor", "version": "2026.10.22-27dcf5f", "capture_mode": cycle.MODE,
                   "activation": "prepared", "runtime_attempt_complete": True, "test_failed": False,
                   "cycle_observed_complete": True, "vendor_authentication_qualified": False,
                   "real_model_qualified": False, "qualification": {key: key != "mcp-tools-call" for key in cycle.FLAGS},
                   "qualification_scope": cycle.SCOPE, "acquisition": {}, "scanner": {},
                   "source_identity": {"installer_base_commit": source["source_commit"], "installer_tree": source["source_tree"], "installer_source_state": "committed", "installer_patch_sha256": []},
                   "cycle_command_stages": [row["stage"] for row in commands],
                   "cycle_proof_stages": [row["stage"] for row in proofs], **result}
        bodies = {name: b"".join(compact(row) + b"\n" for row in rows)
                  for name, rows in zip(cycle.MEMBERS, (commands, proofs, probes))}
        bodies["runtime-attempt.json"] = compact(summary)
        with tempfile.TemporaryDirectory() as temporary, patch.object(cycle, "pins") as pins:
            path = Path(temporary) / "research.zip"
            with zipfile.ZipFile(path, "w", compression=zipfile.ZIP_DEFLATED) as archive:
                for name, body in bodies.items():
                    archive.writestr(name, body)
            admitted, observed = runtime.admit_runtime(path, "cursor", source)
            self.assertEqual(admitted, bodies)
            self.assertEqual(observed, summary)
            pins.assert_called_once()

    def test_approval_containers_are_finite_and_inner_streams_remain_closed(self):
        _, proofs, _, _ = synthetic_cycle()
        for stage in ("approve-a-one", "approve-b-one", "approve-a-two"):
            prefix = copy.deepcopy(proofs[:cycle.PROOFS.index(stage) + 1])
            raw = lambda rows: b"".join(compact(row) + b"\n" for row in rows)
            self.assertEqual(cycle.records(raw(prefix), cycle.PROOFS), prefix)
            for fault in ("extra", "missing", "body-string", "inner-extra", "inner-missing", "inner-bytes", "inner-base64"):
                hostile = copy.deepcopy(prefix)
                value = hostile[-1]["value"]
                if fault == "extra":
                    value["unknown"] = True
                elif fault == "missing":
                    del value["scope"]
                elif fault == "body-string":
                    value["body"] = "unframed"
                elif fault == "inner-extra":
                    value["body"]["unknown"] = True
                elif fault == "inner-missing":
                    del value["body"]["sha256"]
                elif fault == "inner-bytes":
                    value["body"]["bytes"] = True
                else:
                    value["body"]["body"] += "\n"
                with self.subTest(stage=stage, fault=fault), self.assertRaises(ValueError):
                    cycle.records(raw(hostile), cycle.PROOFS)
        foreign = [{"stage": cycle.PROOFS[0], "value": copy.deepcopy(proofs[4]["value"])}]
        with self.assertRaises(ValueError):
            cycle.records(raw(foreign), cycle.PROOFS)
        foreign = [{"stage": cycle.STAGES[0], "value": copy.deepcopy(proofs[4]["value"])}]
        with self.assertRaises(ValueError):
            cycle.records(raw(foreign), cycle.STAGES)

    def test_literal_stages_and_exact_mode_dispatch(self):
        commands, _, _, _ = synthetic_cycle()
        self.assertEqual(tuple(r["stage"] for r in commands), cycle.STAGES)
        for client in ("qoder", "grok", "kiro", "windsurf"):
            with self.assertRaises(ValueError):
                runtime.cli_attempt_members({"runtime-attempt.json": compact({"capture_mode": cycle.MODE})}, {}, client)

    def test_public_stage_no_change_and_group_fields_are_exact(self):
        for stage in ("add-a", "repeat-add-a", "unchanged-update-a", "intact-repair-a", "update-a", "missing-repair-a"):
            for field in ("no_change", "group_phase"):
                for fault in ("presence", "value"):
                    captured = synthetic_cycle()
                    command = next(row["value"] for row in captured[0] if row["stage"] == stage)
                    document = json.loads(cycle.stream(command["stdout"]))
                    output = document["data"] if stage == "repeat-add-a" else document["data"]["targets"][0]["output"]
                    outcome = output["result"]
                    if fault == "presence" and field in outcome:
                        del outcome[field]
                    else:
                        outcome[field] = False if fault == "value" else True
                    command["stdout"] = wire(compact(document))
                    with self.subTest(stage=stage, field=field, fault=fault), patch.object(cycle, "pins"), self.assertRaises(ValueError):
                        cycle.complete(*captured)

    def test_stream_canonical_bounds_and_privacy(self):
        self.assertEqual(cycle.stream({"bytes": 0, "sha256": sha(b""), "body": ""}), b"")
        self.assertEqual(cycle.stream(wire(b"ordinary\n")), b"ordinary\n")
        for value in ({**wire(b"x"), "bytes": True}, {**wire(b"x"), "sha256": "0" * 64}, {**wire(b"x"), "body": "eA==\n"}, wire(b"uap-cursor-private-canary"), wire(b"x" * ((1 << 20) + 1))):
            with self.assertRaises(ValueError):
                cycle.stream(value)

    def test_complete_refuses_each_independent_hostile(self):
        def output(captured, stage):
            return next(row["value"] for row in captured[0] if row["stage"] == stage)
        def proof(captured, stage):
            return next(row["value"] for row in captured[1] if row["stage"] == stage)
        def bad_public(captured):
            row = output(captured, "repeat-add-a")
            value = json.loads(base64.b64decode(row["stdout"]["body"]))
            value["data"]["result"]["mutated"] = True
            row["stdout"] = wire(compact(value))
        def bad_state(captured):
            row = proof(captured, "update-a")
            value = json.loads(base64.b64decode(row["state"]["body"]))
            i = value["installations"][0]
            next(iter(i["clients"].values()))["activation"] = "active"
            row["state"] = wire(compact(value))
        hostiles = [lambda x: output(x, "installed-a")["argv"].append("--approve-all"),
                    lambda x: output(x, "final-outside").update(started_at="2026-10-29T23:00:34Z", ended_at="2026-10-29T23:00:34.5Z"),
                    lambda x: x[3].update(elapsed_ms=17500),
                    lambda x: output(x, "remaining-b").update(cwd="/owned/project"),
                    lambda x: output(x, "repaired-a").update(exit=1),
                    lambda x: proof(x, "approve-a-two")["fingerprints"].reverse(),
                    lambda x: proof(x, "approve-b-one").update(path="/owned/home/.cursor/mcp-approvals.json"),
                    lambda x: proof(x, "source-a-1.1.0").update(probe_sha256="0" * 64),
                    lambda x: proof(x, "missing-native-a").update(path="/owned/project-B/.cursor/mcp.json"),
                    lambda x: proof(x, "remove-a")["data"].update(caller=wire(b"foreign")),
                    lambda x: x[2][8]["value"].update(delta=wire(b"")),
                    lambda x: x[2][16]["value"].update(before_sha256="0" * 64),
                    lambda x: x[3]["cycle_probe"].update(sessions=0), bad_public, bad_state]
        for mutate in hostiles:
            captured = synthetic_cycle()
            mutate(captured)
            with self.subTest(mutate=repr(mutate)), patch.object(cycle, "pins"), self.assertRaises(ValueError):
                cycle.complete(*captured)

    def test_pin_boundary_refuses_unknown_inventory(self):
        with self.assertRaises(ValueError):
            cycle.pins({"acquisition": {"client": "cursor", "acquisition": {}, "runtime_files": []}}, {"client": "/pin/dist-package/cursor-agent"})


if __name__ == "__main__":
    unittest.main()

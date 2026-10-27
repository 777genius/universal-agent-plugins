"""Authored success-envelope and actual-frame hostile controls; no native child."""
import copy
import hashlib
import json
from pathlib import Path
import unittest

import project_native_runtime as runtime
import test_project_native_runtime as fixtures


def raw(value):
    return json.dumps(value, separators=(",", ":"), sort_keys=True).encode()


def stream(body):
    return {"bytes": len(body), "sha256": hashlib.sha256(body).hexdigest(), "body": runtime.base64.b64encode(body).decode()}


def qualified_fixture():
    """Independently author finite envelope semantics around actual ACP frames.

    This is parser input, not genuine execution, filesystem proof or a runtime
    qualification. Existing actual Store/Stager/data contracts remain separate.
    """
    source = fixtures.RuntimeTests.source
    bodies = fixtures.RuntimeTests().cline_cycle_bodies()
    summary = runtime.strict_json(bodies["runtime-attempt.json"])
    summary.update(capture_mode="cline-complete-cycle", cycle_capture_review_required=False,
                   attempt_stage="complete-cycle", runtime_attempt_complete=True, test_failed=False,
                   qualification={key: True for key in runtime.shared.QUALIFICATION_CASES})
    bodies["runtime-attempt.json"] = raw(summary)
    rows = {name: runtime.cline_cycle_rows(bodies[name]) for name in runtime.CLINE_CYCLE_AGGREGATES}
    root = "/inert/cycle"
    project = {"user": root + "/outside", "a": root + "/project", "b": root + "/project-b"}
    native = {**project, "user": root + "/home"}
    config = {scope: path + ("/.cline/data/settings/cline_mcp_settings.json" if scope == "user" else "/.uap/clients/cline/mcp.json") for scope, path in native.items()}
    nonce = {scope: str(index + 1) * 32 for index, scope in enumerate(("user", "a", "b"))}
    sha = lambda body: hashlib.sha256(body).hexdigest()
    recipes, state_by_stage = {}, {}
    for row in rows["cycle-proof.jsonl"]:
        stage = row["stage"]
        if stage.startswith("source-"):
            _, scope, revision = stage.split("-", 2)
            body = ("---\nname: uap-cline-" + scope + "\ndescription: " + revision + "\n---\nUAP_CLINE_" + revision + "_" + nonce[scope] + "\n").encode()
            value = {"source": root + "/cycle-source-" + scope, "digest": "sha256:" + sha(stage.encode()),
                     "files": {"plugin.json": "file:600:" + sha(b"independent manifest"), "skills/uap-cline-" + scope + "/SKILL.md": "file:600:" + sha(body)}}
            recipes[(scope, revision)] = value
            row["value"] = value
            continue
        scope = "user" if stage.endswith("-user") else "b" if stage.endswith("-b") else "a"
        revision = scope + "-one"
        if scope == "a" and stage in ("update-a", "before-missing-a", "missing-repair-a", "remove-a", "repeat-remove-a"):
            revision = "a-two"
        recipe = recipes[(scope, revision)]
        iid = "independent-installation-" + scope
        physical = "uap-cline-cycle-" + sha(iid.encode())[:12]
        scope_name = "user" if scope == "user" else "project"
        state_root = root + "/installer-state" if scope == "user" else project[scope] + "/.uap"
        active = state_root + "/managed/clients/cline/" + physical
        bid = "client_" + sha("\0".join((iid, "cline", scope_name, active)).encode())[:24]
        did = "data_" + sha("\0".join((iid, physical, scope_name)).encode())[:24]
        locator = state_root + "/plugin-data/" + physical
        receipt = {"data_receipt_id": did, "physical_backend_id": physical, "scope": scope_name, "locator": locator,
                   "ownership_digest": "sha256:" + sha(("agentplugins-plugin-data-v1\0" + did + "\0" + locator).encode()), "state": "owned"}
        removed = stage in ("remove-a", "repeat-remove-a", "remove-b", "remove-user")
        binding = {"client_binding_id": bid, "client_id": "cline", "scope": scope_name, "target_locator": active,
                   "physical_artifact_id": physical, "activation": "active" if scope == "user" else "prepared",
                   "materialization": "materialized", "authentication": "not_checked", "policy": "allowed", "verification": "installation_verified", "affected_surfaces": ["cline"], "package_revision": {"version": "1.0.0", "tree_digest": recipe["digest"], "manifest_digest": "sha256:" + sha(b"independent manifest")}, "native_profile_root": "" if scope == "user" else project[scope]}
        if scope != "user":
            binding["data_receipt_id"] = did
        paths = {"managed_package_directory": active, "cline_global_skill_directory" if scope == "user" else "cline_project_skill_directory": native[scope] + "/.cline/skills/uap-cline-" + scope}
        if scope != "user": paths["cline_project_mcp_server"] = config[scope]
        binding["native_objects"] = []
        for kind, path in paths.items():
            skill = "skill_directory" in kind
            oid = "package:cline:" + physical if kind == "managed_package_directory" else "cline-skill:uap-cline-user" if scope == "user" else kind + ":" + ("uapclinea" if kind == "cline_project_mcp_server" else "uap-cline-" + scope)
            logical = "uap-cline-cycle" if kind == "managed_package_directory" else "uapclinea" if kind == "cline_project_mcp_server" else "uap-cline-" + scope
            skill_body = ("---\nname: uap-cline-" + scope + "\ndescription: " + revision + "\n---\nUAP_CLINE_" + revision + "_" + nonce[scope] + "\n").encode()
            obj = {"object_id": oid, "kind": kind, "logical_name": logical, "path": path, "protection_class": "managed", "managed_digest": "sha256:" + sha(b"f\0SKILL.md\0-\0" + skill_body + b"\0" if skill else kind.encode())}
            if skill: obj["source_relative"] = "skills/uap-cline-" + scope
            binding["native_objects"].append(obj)
        installation = {"installation_id": iid, "declared_name": "uap-cline-cycle",
                        "source": {"source_binding_id": "src_" + sha((recipe["source"] + "\0\0").encode())[:24], "requested_source": recipe["source"], "canonical_source": recipe["source"], "resolved_revision": "", "tree_digest": recipe["digest"]},
                        "origin_mode": "direct", "operation_group_id": "inert-operation", "created_at": "2026-10-27T00:00:00Z", "updated_at": "2026-10-27T00:00:00Z", "package": {"loader_kind": "agent_plugins", "format_id": "agent-plugins/1.0.0", "schema_uri": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json", "declared_name": "uap-cline-cycle", "version": "1.0.0", "manifest_digest": "sha256:" + sha(b"independent manifest"), "inventory": {"mcp_present": scope != "user", "mcp_enabled": scope != "user", "skills": ["uap-cline-" + scope], **({"mcp_servers": ["uapclinea"]} if scope != "user" else {})}},
                        "clients": {} if removed else {bid: binding}, "data_retained": removed,
                        "data_receipts": {} if scope == "user" and not removed else {did: receipt}}
        state_by_stage[stage] = (installation, binding)
        row["value"] = {"scope": scope, "revision": revision, "removed": removed, "state": stream(raw({"schema_version": 5, "installations": [installation]})), "binding": binding, "source_digest": recipe["digest"]}
    selected = dict(zip(runtime.CLINE_CYCLE_LISTINGS, ([], [], ["user"], ["user", "a"], ["user", "b"], ["user", "a"], ["user", "a"], ["user", "a"], ["user"], ["user", "b"], ["user"], [], [], [])))
    for row in rows["cycle-discovery.jsonl"]:
        stage, value = row["stage"], row["value"]
        scope = "user" if stage.endswith("-user") else stage[-1]
        value.update(cwd=project[scope], mcp_settings_override=scope != "user")
        value["argv"][4] = project[scope]
        if value["kind"] == "mcp":
            entries = [{"name": "foreign-" + scope, "disabled": True, "path": config[scope], "transportType": "streamableHttp"}]
            if scope != "user" and scope in selected[stage]:
                entries.append({"name": "uapclinea", "disabled": False, "path": config[scope], "transportType": "stdio"})
        else:
            entries = []
            for who in sorted({"user", scope}):
                name = "foreign-" + who
                entries.append({"name": name, "description": "Foreign inert marker", "instructions": "FOREIGN_" + who, "frontmatter": {"name": name, "description": "Foreign inert marker"}, "path": native[who] + "/.cline/skills/" + name + "/SKILL.md"})
            for who in selected[stage]:
                name = "uap-cline-" + who
                revision = "a-two" if who == "a" and stage in ("updated-a", "repaired-a") else who + "-one"
                entries.append({"name": name, "description": revision, "instructions": "UAP_CLINE_" + revision + "_" + nonce[who], "frontmatter": {"name": name, "description": revision}, "path": native[who] + "/.cline/skills/" + name + "/SKILL.md"})
        value["stdout"] = stream(raw(entries))
    measured_path = Path(__file__).resolve().parents[1] / "repotests/testdata/cline-3.0.73-installed-acp/native-acp.jsonl"
    measured = [runtime.strict_json(line) for line in measured_path.read_bytes().splitlines()]
    def record(name, stage, value):
        return {"stage": stage, "value": value}
    for name in ("cycle-native-acp.jsonl", "cycle-native-commands.jsonl", "cycle-provider-requests.jsonl", "cycle-provider-responses.jsonl", "cycle-provider-receipts.jsonl", "cycle-probe-events.jsonl"):
        rows[name] = []
    for index, stage in enumerate(runtime.CLINE_CYCLE_ACP):
        scope = stage[-1]; empty = stage in runtime.CLINE_CYCLE_EMPTY
        token = nonce[scope]; call_id = "uap-cline-cycle-" + sha((stage + token).encode())[:24]
        sentinel = "UAP_CLINE_DONE_" + stage + "_" + token
        prompt = "UAP_CLINE_CYCLE_" + stage + "_" + token
        operation = "empty" if empty else "write" if stage.startswith("installed-") else "read"
        args = {"nonce": token, "marker": "retained-" + token, "operation": operation}
        session = f"1793012345678_{index:05d}_cli"
        started, ended = f"2026-10-27T11:00:{index * 2:02d}Z", f"2026-10-27T11:00:{index * 2 + 1:02d}Z"
        frames = copy.deepcopy(measured if not empty else measured[:2] + measured[7:])
        frames[1]["result"]["sessionId"] = session
        facts, events = None, []
        if not empty:
            proof = {"installed-a": "add-a", "installed-b": "add-b", "updated-a": "update-a", "repaired-a": "missing-repair-a", "remaining-b": "add-b"}[stage]
            ins, binding = state_by_stage[proof]
            facts = {"nonce": token, "revision": "a-two" if stage in ("updated-a", "repaired-a") else scope + "-one", "cwd": binding["target_locator"], "plugin_root": binding["target_locator"], "plugin_data": ins["data_receipts"][binding["data_receipt_id"]]["locator"], "argv": ["Owned Cline " + scope, binding["target_locator"] + "/argument"], "literal": "", "once": "", "marker": args["marker"], "pid": index + 100, "session": "probe-" + stage}
            events = [{"method": method, "nonce": token, "pid": facts["pid"], "session": facts["session"], "timestamp": started, "facts": facts if method == "tools/call" else None} for method in ("initialize", "tools/list", "tools/call")]
        for frame in frames[2:-1]:
            params = frame["params"]; params["sessionId"] = session
            update = params.get("update", params.get("toolCall"))
            if "toolCallId" in update:
                update["toolCallId"] = call_id
            if "rawInput" in update:
                update["rawInput"] = args
                title = raw(args).decode(); update["title"] = "uapclinea__inspect_runtime: " + (title[:57] + "..." if len(title) > 60 else title)
            if update.get("status") == "completed":
                update["rawOutput"] = {"isError": False, "content": [{"type": "text", "text": raw(facts).decode()}]}
            if update.get("sessionUpdate") == "agent_message_chunk":
                update["content"]["text"] = sentinel
            if "updatedAt" in update:
                update["updatedAt"] = ended
        rows["cycle-native-acp.jsonl"].append(record("", stage, stream(b"".join(raw(frame) + b"\n" for frame in frames))))
        rows["cycle-native-commands.jsonl"].append(record("", stage, {"argv": ["/inert/cline", "--auto-approve", "false", "--cwd", project[scope], "--acp"], "cwd": project[scope], "mcp_settings_override": config[scope], "auto_approve": False, "injected_mcp_servers": [], "error": False, "deadline": False, "error_stream": stream(b""), "started_at": started, "ended_at": ended, "session": session, "cleanup": "existing OS strict planned duplex shutdown; entire delegated descendant cgroup"}))
        rows["cycle-probe-events.jsonl"].append(record("", stage, stream(b"".join(raw(event) + b"\n" for event in events))))
        suffix = " Return only " + sentinel + ". Do not call any built-in or MCP tool or change any file." if empty else " Call only uapclinea__inspect_runtime with the exact supplied harmless nonce/operation/marker. Do not call built-in tools or any other MCP server."
        request = {"model": "uap-cline-model", "stream": True, "tools": [{"function": {"name": "read_file"}}] + ([] if empty else [{"function": {"name": "uapclinea__inspect_runtime"}}]), "messages": [{"role": "system", "content": "Inert authored system fixture"}, {"role": "user", "content": prompt + suffix}]}
        for number in range(1 if empty else 2):
            current = copy.deepcopy(request)
            if number:
                current["messages"].append({"role": "assistant", "content": None, "tool_calls": [{"id": call_id, "type": "function", "function": {"name": "uapclinea__inspect_runtime", "arguments": raw(args).decode()}}]})
                current["messages"].append({"role": "tool", "tool_call_id": call_id, "content": raw({"isError": False, "content": [{"type": "text", "text": raw(facts).decode()}]}).decode()})
            rows["cycle-provider-requests.jsonl"].append(record("", stage, {"index": number, "stream": stream(raw(current))}))
            delta = {"role": "assistant", "content": sentinel} if empty or number else {"role": "assistant", "tool_calls": [{"id": call_id, "index": 0, "type": "function", "function": {"name": "uapclinea__inspect_runtime", "arguments": raw(args).decode()}}]}
            chunks = [{"model": "uap-cline-model", "choices": [{"delta": delta, "finish_reason": None, "index": 0}]}, {"model": "uap-cline-model", "choices": [{"delta": {}, "finish_reason": "stop" if empty or number else "tool_calls", "index": 0}]}]
            for chunk in chunks:
                chunk.update(created=1, id="uap-cline-completion-" + token, object="chat.completion.chunk")
            body = b"".join(b"data: " + raw(chunk) + b"\n\n" for chunk in chunks) + b"data: [DONE]\n\n"
            rows["cycle-provider-responses.jsonl"].append(record("", stage, {"index": number, "stream": stream(body)}))
        rows["cycle-provider-receipts.jsonl"].append(record("", stage, {"requests": 1 if empty else 2, "responses": 1 if empty else 2, "empty": empty, "exact_call_issued": not empty, "single_call_approved": not empty, "error": False, "call_id": call_id, "prompt": prompt, "sentinel": sentinel, "operation": operation}))
    for name, entries in rows.items():
        bodies[name] = b"".join(raw(entry) + b"\n" for entry in entries)
    return bodies, source


class CompleteCycleTests(unittest.TestCase):
    def test_reviewed_rehashed_extra_calls_object_authority_and_session_grammar_refuse(self):
        for kind in ("extra-response-call", "extra-history-call", "foreign-object-id", "foreign-logical-name", "foreign-source-relative", "foreign-skill-digest", "foreign-modes", "foreign-models", "extra-config-option", "duplicate-config-option", "typed-capability"):
            bodies, source = qualified_fixture()
            if kind in ("extra-response-call", "extra-history-call"):
                member = "cycle-provider-responses.jsonl" if kind == "extra-response-call" else "cycle-provider-requests.jsonl"
                entries = runtime.cline_cycle_rows(bodies[member])
                index = 2 if kind == "extra-response-call" else 3
                captured = runtime.cline_cycle_stream(entries[index]["value"]["stream"])
                if kind == "extra-response-call":
                    chunks = [runtime.strict_json(line[6:]) for line in captured.splitlines() if line.startswith(b"data: ") and line != b"data: [DONE]"]
                    chunks[0]["choices"][0]["delta"]["tool_calls"].append({"id": "foreign-call", "type": "function", "index": 1, "function": {"name": "foreign__tool", "arguments": "{}"}})
                    captured = b"".join(b"data: " + raw(chunk) + b"\n\n" for chunk in chunks) + b"data: [DONE]\n\n"
                else:
                    request = runtime.strict_json(captured)
                    request["messages"].insert(-1, {"role": "assistant", "content": None, "tool_calls": [{"id": "foreign-call", "type": "function", "function": {"name": "foreign__tool", "arguments": "{}"}}]})
                    captured = raw(request)
                entries[index]["value"]["stream"] = stream(captured)
            elif kind.startswith("foreign-object") or kind in ("foreign-logical-name", "foreign-source-relative", "foreign-skill-digest"):
                member = "cycle-proof.jsonl"; entries = runtime.cline_cycle_rows(bodies[member])
                for entry in entries:
                    value = entry["value"]
                    if value.get("scope") != "a": continue
                    state = runtime.strict_json(runtime.cline_cycle_stream(value["state"]))
                    binding_values = [value["binding"], *state["installations"][0]["clients"].values()]
                    for binding in binding_values:
                        for obj in binding["native_objects"]:
                            if kind == "foreign-object-id" and obj["kind"] == "managed_package_directory": obj["object_id"] = "foreign-object-authority"
                            if kind == "foreign-logical-name": obj["logical_name"] = "foreign-logical-authority"
                            if kind == "foreign-source-relative" and "source_relative" in obj: obj["source_relative"] = "skills/foreign"
                            if kind == "foreign-skill-digest" and "skill_directory" in obj["kind"]: obj["managed_digest"] = "sha256:" + "9" * 64
                    value["state"] = stream(raw(state))
            else:
                member = "cycle-native-acp.jsonl"; entries = runtime.cline_cycle_rows(bodies[member])
                frames = [runtime.strict_json(line) for line in runtime.cline_cycle_stream(entries[2]["value"]).splitlines()]
                if kind == "foreign-modes": frames[1]["result"]["modes"] = {"currentModeId": "plan", "foreign": True}
                elif kind == "foreign-models": frames[1]["result"]["models"]["currentModelId"] = "foreign-model"
                elif kind == "extra-config-option": frames[1]["result"]["configOptions"][0]["foreign"] = True
                elif kind == "duplicate-config-option": frames[1]["result"]["configOptions"].append(copy.deepcopy(frames[1]["result"]["configOptions"][0]))
                else: frames[0]["result"]["agentCapabilities"]["loadSession"] = 1
                entries[2]["value"] = stream(b"".join(raw(frame) + b"\n" for frame in frames))
            bodies[member] = b"".join(raw(entry) + b"\n" for entry in entries)
            with self.subTest(kind=kind), self.assertRaises(ValueError):
                runtime.cline_members(bodies, source)

    def test_full_authored_semantics_require_exact_success_and_keep_old_capture_failed(self):
        bodies, source = qualified_fixture()
        self.assertTrue(runtime.cline_members(bodies, source)["runtime_attempt_complete"])
        old = fixtures.RuntimeTests().cline_cycle_bodies()
        self.assertFalse(runtime.cline_members(old, source)["runtime_attempt_complete"])
        for field, value in (("runtime_attempt_complete", False), ("test_failed", True), ("cycle_capture_review_required", True), ("cycle_observed_complete", False), ("activation", "active"), ("real_model_qualified", True), ("vendor_authentication_qualified", True), ("source_identity", {}), ("cycle_provider_cleanup", {})):
            changed = dict(bodies); summary = runtime.strict_json(changed["runtime-attempt.json"]); summary[field] = value; changed["runtime-attempt.json"] = raw(summary)
            with self.subTest(field=field), self.assertRaises(ValueError):
                runtime.cline_members(changed, source)
        changed = dict(old); summary = runtime.strict_json(bodies["runtime-attempt.json"]); changed["runtime-attempt.json"] = raw(summary)
        with self.assertRaises(ValueError):
            runtime.cline_members(changed, source)

    def test_every_missing_member_and_partial_qualification_refuses(self):
        bodies, source = qualified_fixture()
        for name in bodies:
            with self.subTest(missing=name), self.assertRaises(ValueError):
                runtime.cline_members({key: body for key, body in bodies.items() if key != name}, source)
        for flag in runtime.shared.QUALIFICATION_CASES:
            changed = dict(bodies); summary = runtime.strict_json(changed["runtime-attempt.json"]); summary["qualification"][flag] = False; changed["runtime-attempt.json"] = raw(summary)
            with self.subTest(flag=flag), self.assertRaises(ValueError):
                runtime.cline_members(changed, source)

    def test_raw_semantic_mutations_cannot_hide_behind_rehashed_streams(self):
        for kind in ("empty-permission", "empty-event", "foreign-session", "call-id", "result", "probe-time", "data-root", "source", "state-schema", "extra-user-data", "revision", "selection", "provider-result", "cleanup", "repeat-removal"):
            bodies, source = qualified_fixture()
            member = "cycle-native-acp.jsonl"
            entries = runtime.cline_cycle_rows(bodies[member])
            if kind in ("empty-permission", "foreign-session", "call-id", "result"):
                index = 0 if kind == "empty-permission" else 2
                frames = [runtime.strict_json(line) for line in runtime.cline_cycle_stream(entries[index]["value"]).splitlines()]
                if kind == "empty-permission":
                    frames.insert(2, {"jsonrpc": "2.0", "id": 99, "method": "session/request_permission", "params": {}})
                elif kind == "foreign-session": frames[2]["params"]["sessionId"] = "foreign"
                elif kind == "call-id": frames[3]["params"]["toolCall"]["toolCallId"] = "foreign"
                else: frames[6]["params"]["update"]["rawOutput"]["isError"] = True
                entries[index]["value"] = stream(b"".join(raw(frame) + b"\n" for frame in frames))
            elif kind in ("empty-event", "probe-time"):
                member = "cycle-probe-events.jsonl"; entries = runtime.cline_cycle_rows(bodies[member])
                if kind == "empty-event": entries[0]["value"] = stream(b'{"method":"tools/call"}\n')
                else:
                    events = [runtime.strict_json(line) for line in runtime.cline_cycle_stream(entries[2]["value"]).splitlines()]; events[-1]["timestamp"] = "2020-01-01T00:00:00Z"; entries[2]["value"] = stream(b"".join(raw(event) + b"\n" for event in events))
            elif kind in ("data-root", "source", "state-schema", "extra-user-data", "revision"):
                member = "cycle-proof.jsonl"; entries = runtime.cline_cycle_rows(bodies[member]); value = entries[3 if kind == "extra-user-data" else 4]["value"]; state = runtime.strict_json(runtime.cline_cycle_stream(value["state"])); ins = state["installations"][0]
                if kind == "data-root": next(iter(ins["data_receipts"].values()))["locator"] = "/foreign/data"
                elif kind == "source": ins["source"]["canonical_source"] = "/foreign/source"
                elif kind == "state-schema": state["schema_version"] = 2
                elif kind == "extra-user-data": ins["data_receipts"] = {"foreign": {}}
                else: value["revision"] = "a-two"
                value["state"] = stream(raw(state))
            elif kind == "selection":
                member = "cycle-discovery.jsonl"; entries = runtime.cline_cycle_rows(bodies[member]); entries[6]["value"]["stdout"] = stream(b"[]")
            elif kind == "provider-result":
                member = "cycle-provider-requests.jsonl"; entries = runtime.cline_cycle_rows(bodies[member]); value = entries[3]["value"]; request = runtime.strict_json(runtime.cline_cycle_stream(value["stream"])); request["messages"][-1]["content"] = "{}"; value["stream"] = stream(raw(request))
            elif kind == "cleanup":
                summary = runtime.strict_json(bodies["runtime-attempt.json"]); summary["cycle_provider_cleanup"]["forced_close"] = True; bodies["runtime-attempt.json"] = raw(summary)
            else:
                bodies["repeat-remove-a.stderr.log"] = b"different refusal\n"
            bodies[member] = b"".join(raw(entry) + b"\n" for entry in entries)
            with self.subTest(kind=kind), self.assertRaises(ValueError):
                runtime.cline_members(bodies, source)


if __name__ == "__main__":
    unittest.main()

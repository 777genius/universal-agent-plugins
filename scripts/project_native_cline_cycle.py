"""Finite semantic success joins for the reviewed Cline3.0.73 full cycle.

The caller owns closed member/size/privacy/source admission. This module reads
only those private captured bytes; it never opens a native profile or executes
a process. Uncaptured live filesystem equality remains a source-attested gate.
"""
import collections
from datetime import datetime
import hashlib
import json
from pathlib import PurePosixPath
import re


def require(condition):
    if not condition:
        raise ValueError("Cline complete-cycle semantic join differs")


def digest(body):
    return hashlib.sha256(body).hexdigest()


def exact(value, expected):
    if type(value) is not type(expected):
        return False
    if type(value) is dict:
        return set(value) == set(expected) and all(exact(value[key], item) for key, item in expected.items())
    if type(value) is list:
        return len(value) == len(expected) and all(exact(left, right) for left, right in zip(value, expected))
    return value == expected


def stamp(value):
    require(type(value) is str and value.endswith("Z"))
    result = datetime.fromisoformat(value[:-1] + "+00:00")
    require(result.tzinfo is not None)
    return result


def grouped(rows):
    result = collections.defaultdict(list)
    for row in rows:
        result[row["stage"]].append(row["value"])
    return result


def states(rows, decode, stream):
    proofs = {row["stage"]: row["value"] for row in rows}
    root = PurePosixPath(proofs["source-a-a-one"]["source"]).parent
    require(root.is_absolute() and str(root) == str(PurePosixPath(str(root))))
    projects = {"a": str(root / "project"), "b": str(root / "project-b"), "user": str(root / "outside")}
    native_roots = {**projects, "user": str(root / "home")}
    configs = {scope: str(PurePosixPath(path) / (".cline/data/settings/cline_mcp_settings.json" if scope == "user" else ".uap/clients/cline/mcp.json")) for scope, path in native_roots.items()}
    installations, bindings, data_ids, result = {}, {}, {}, {}
    for stage, value in proofs.items():
        if stage.startswith("source-"):
            continue
        state = decode(stream(value["state"]))
        require(type(state) is dict and {"schema_version", "installations"} <= set(state) <= {"schema_version", "installations", "transaction_receipts"})
        require(type(state["schema_version"]) is int and state["schema_version"] == 5 and len(state["installations"]) == 1)
        ins = state["installations"][0]
        required = {"installation_id", "declared_name", "source", "package", "clients", "created_at", "updated_at", "origin_mode", "operation_group_id"}
        require(type(ins) is dict and required <= set(ins) <= required | {"data_receipts", "data_retained"})
        require(ins["origin_mode"] == "direct" and all(type(ins[key]) is str and ins[key] for key in ("created_at", "updated_at", "operation_group_id")))
        scope, revision, binding = value["scope"], value["revision"], value["binding"]
        require(scope in projects and revision == ("user-one" if scope == "user" else "b-one" if scope == "b" else "a-two" if stage in ("update-a", "before-missing-a", "missing-repair-a", "remove-a", "repeat-remove-a") else "a-one"))
        source = proofs["source-" + scope + "-" + revision]
        require(source["source"] == str(root / ("cycle-source-" + scope)))
        require(re.fullmatch(r"sha256:[0-9a-f]{64}", source["digest"]) is not None)
        iid = ins["installation_id"]
        require(type(iid) is str and iid and installations.setdefault(scope, iid) == iid)
        require(ins["declared_name"] == "uap-cline-cycle")
        require(ins["source"] == {"source_binding_id": "src_" + digest((source["source"] + "\0\0").encode())[:24], "requested_source": source["source"], "canonical_source": source["source"], "resolved_revision": "", "tree_digest": source["digest"]})
        require(value["source_digest"] == source["digest"])
        manifest = "sha256:" + source["files"]["plugin.json"].split(":")[-1]
        inventory = {"mcp_present": scope != "user", "mcp_enabled": scope != "user", "skills": ["uap-cline-" + scope]}
        if scope != "user":
            inventory["mcp_servers"] = ["uapclinea"]
        require(exact(ins["package"], {"loader_kind": "agent_plugins", "format_id": "agent-plugins/1.0.0", "schema_uri": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json", "declared_name": "uap-cline-cycle", "version": "1.0.0", "manifest_digest": manifest, "inventory": inventory}))
        physical = "uap-cline-cycle-" + digest(iid.encode())[:12]
        scope_name = "user" if scope == "user" else "project"
        state_root = str(root / "installer-state") if scope == "user" else projects[scope] + "/.uap"
        active = state_root + "/managed/clients/cline/" + physical
        bid = "client_" + digest("\0".join((iid, "cline", scope_name, active)).encode())[:24]
        require(binding["target_locator"] == active and binding["physical_artifact_id"] == physical and binding["client_binding_id"] == bid)
        require(binding["client_id"] == "cline" and binding["scope"] == scope_name)
        require(binding["materialization"] == "materialized" and binding["authentication"] == "not_checked" and binding["policy"] == "allowed" and binding["verification"] == "installation_verified" and binding["affected_surfaces"] == ["cline"])
        require(binding["package_revision"] == {"version": "1.0.0", "tree_digest": source["digest"], "manifest_digest": manifest})
        skill_kind = "cline_global_skill_directory" if scope == "user" else "cline_project_skill_directory"
        expected_paths = {"managed_package_directory": active, skill_kind: native_roots[scope] + "/.cline/skills/uap-cline-" + scope}
        if scope != "user":
            expected_paths["cline_project_mcp_server"] = configs[scope]
        objects = binding["native_objects"]
        require(len(objects) == len(expected_paths) and {obj["kind"] for obj in objects} == set(expected_paths))
        for obj in objects:
            kind = obj["kind"]
            oid = "package:cline:" + physical if kind == "managed_package_directory" else "cline-skill:uap-cline-user" if scope == "user" else kind + ":" + ("uapclinea" if kind == "cline_project_mcp_server" else "uap-cline-" + scope)
            logical = "uap-cline-cycle" if kind == "managed_package_directory" else "uapclinea" if kind == "cline_project_mcp_server" else "uap-cline-" + scope
            keys = {"object_id", "kind", "logical_name", "path", "managed_digest", "protection_class"} | ({"source_relative"} if kind == skill_kind else set())
            require(set(obj) == keys and obj["object_id"] == oid and obj["logical_name"] == logical)
            require(obj["path"] == expected_paths[kind] and obj["protection_class"] == "managed" and re.fullmatch(r"sha256:[0-9a-f]{64}", obj["managed_digest"]) is not None)
            if kind == skill_kind:
                require(obj["source_relative"] == "skills/uap-cline-" + scope)
        require(bindings.setdefault(scope, bid) == bid)
        data = ins.get("data_receipts", {})
        if value["removed"]:
            require(ins["clients"] == {} and ins.get("data_retained") is True)
        else:
            require(ins["clients"] == {bid: binding} and ins.get("data_retained", False) is False)
            require(binding["activation"] == ("active" if scope == "user" else "prepared"))
            require(binding.get("native_profile_root", "") == ("" if scope == "user" else projects[scope]))
            require(binding["package_revision"]["tree_digest"] == source["digest"])
        if scope == "user" and not value["removed"]:
            require(binding.get("data_receipt_id", "") == "" and data == {})
        else:
            did = "data_" + digest("\0".join((iid, physical, scope_name)).encode())[:24]
            require(set(data) == {did} and data_ids.setdefault(scope, did) == did)
            receipt = data[did]
            locator = state_root + "/plugin-data/" + physical
            require(receipt["data_receipt_id"] == did and receipt["scope"] == scope_name and receipt["state"] == "owned")
            require(receipt["locator"] == locator and receipt["physical_backend_id"] == physical)
            require(receipt["ownership_digest"] == "sha256:" + digest(("agentplugins-plugin-data-v1\0" + did + "\0" + locator).encode()))
            if not value["removed"]:
                require(binding["data_receipt_id"] == did)
        result[stage] = (ins, binding, revision)
    require(len(set(installations.values())) == len(set(bindings.values())) == len(set(data_ids.values())) == 3)
    require(result["remove-a"][0] == result["repeat-remove-a"][0])
    require(all(result[stage][0] == result["add-a"][0] for stage in ("repeat-add-a", "unchanged-update-a")))
    require(result["missing-repair-a"][0]["source"] == result["update-a"][0]["source"])
    require(result["update-a"][0]["source"]["tree_digest"] != result["add-a"][0]["source"]["tree_digest"])
    return result, proofs, projects, native_roots, configs


def listing(rows, proofs, projects, native_roots, configs, decode, stream):
    selected = {"empty-a": [], "empty-b": [], "installed-user": ["user"], "installed-a": ["user", "a"], "installed-b": ["user", "b"], "stable-a": ["user", "a"], "updated-a": ["user", "a"], "repaired-a": ["user", "a"], "removed-a": ["user"], "remaining-b": ["user", "b"], "removed-b": ["user"], "removed-user": [], "final-a": [], "final-b": []}
    skill_bodies = {}
    for row in rows:
        stage, value = row["stage"], row["value"]
        scope = "user" if stage in ("installed-user", "removed-user") else stage[-1]
        require(value["cwd"] == projects[scope] and value["mcp_settings_override"] is (scope != "user"))
        entries = decode(stream(value["stdout"]))
        require(type(entries) is list)
        by_name = {entry["name"]: entry for entry in entries}
        require(len(by_name) == len(entries))
        if value["kind"] == "mcp":
            expected = {"foreign-" + scope: {"name": "foreign-" + scope, "disabled": True, "path": configs[scope], "transportType": "streamableHttp"}}
            if scope != "user" and scope in selected[stage]:
                expected["uapclinea"] = {"name": "uapclinea", "disabled": False, "path": configs[scope], "transportType": "stdio"}
            require(by_name == expected)
            continue
        require(set(by_name) == {"foreign-" + item for item in ("user", scope)} | {"uap-cline-" + item for item in selected[stage]})
        for name, entry in by_name.items():
            require(set(entry) == {"name", "description", "instructions", "frontmatter", "path"})
            require(all(type(entry[key]) is str for key in ("name", "description", "instructions", "path")))
            require(entry["frontmatter"] == {"name": name, "description": entry["description"]})
            who = name.removeprefix("uap-cline-").removeprefix("foreign-")
            require(entry["path"] == native_roots[who] + "/.cline/skills/" + name + "/SKILL.md")
            if name.startswith("foreign-"):
                require(entry["description"] == "Foreign inert marker" and entry["instructions"] == "FOREIGN_" + who)
                continue
            revision = "a-two" if who == "a" and stage in ("updated-a", "repaired-a") else who + "-one"
            require(entry["description"] == revision)
            require(re.fullmatch("UAP_CLINE_" + re.escape(revision) + r"_[0-9a-f]{32}", entry["instructions"]) is not None)
            raw = ("---\nname: " + name + "\ndescription: " + revision + "\n---\n" + entry["instructions"] + "\n").encode()
            require(proofs["source-" + who + "-" + revision]["files"]["skills/" + name + "/SKILL.md"] == "file:600:" + digest(raw))
            require(skill_bodies.setdefault((who, revision), raw) == raw)
    return skill_bodies


def skill_ownership(state_rows, skill_bodies):
    for ins, binding, revision in state_rows.values():
        scope = "user" if binding["scope"] == "user" else "b" if binding["native_profile_root"].endswith("/project-b") else "a"
        body = skill_bodies[(scope, revision)]
        framed = b"f\0SKILL.md\0-\0" + body + b"\0"
        kind = "cline_global_skill_directory" if scope == "user" else "cline_project_skill_directory"
        require(next(obj for obj in binding["native_objects"] if obj["kind"] == kind)["managed_digest"] == "sha256:" + digest(framed))


def acp_frames(raw, command, receipt, empty, decode):
    frames = [decode(line) for line in raw.splitlines()]
    require(len(frames) == (5 if empty else 10))
    require(all(type(frame) is dict and frame.get("jsonrpc") == "2.0" and "error" not in frame for frame in frames))
    require(set(frames[0]) == set(frames[1]) == {"jsonrpc", "id", "result"})
    require(type(frames[0]["id"]) is int and frames[0]["id"] == 1)
    require(type(frames[1]["id"]) is int and frames[1]["id"] == 2)
    require(frames[0]["result"]["agentInfo"] == {"name": "cline", "version": "3.0.73"})
    require(set(frames[0]["result"]) == {"protocolVersion", "agentCapabilities", "agentInfo", "authMethods"})
    require(type(frames[0]["result"]["protocolVersion"]) is int and frames[0]["result"]["protocolVersion"] == 1)
    require(exact(frames[0]["result"]["agentCapabilities"], {"loadSession": True, "promptCapabilities": {"image": True, "audio": False, "embeddedContext": False}}))
    require(frames[0]["result"]["authMethods"] == [{"id": "cline", "name": "Sign in with Cline"}, {"id": "cline-pass", "name": "Sign in with ClinePass"}, {"id": "openai-codex", "name": "Sign in with ChatGPT Subscription"}])
    session = command["session"]
    require(frames[1]["result"]["sessionId"] == session)
    require(set(frames[1]["result"]) == {"sessionId", "modes", "models", "configOptions"})
    modes = [{"id": "plan", "name": "Plan", "description": "Explore the codebase and plan changes without modifying files"}, {"id": "act", "name": "Act", "description": "Make changes to the codebase"}]
    require(exact(frames[1]["result"]["modes"], {"availableModes": modes, "currentModeId": "act"}))
    require(exact(frames[1]["result"]["models"], {"availableModels": [{"modelId": "uap-cline-model", "name": "UAP scripted model"}], "currentModelId": "uap-cline-model"}))
    raw_options = frames[1]["result"]["configOptions"]
    require(type(raw_options) is list and len(raw_options) == 4)
    options = {entry["id"]: entry for entry in raw_options}
    require(len(options) == 4 and set(options) == {"provider", "model", "mode", "auto_approve"})
    require(options["auto_approve"]["currentValue"] is False)
    require([options[key]["currentValue"] for key in ("provider", "model", "mode")] == ["uap-scripted", "uap-cline-model", "act"])
    expected_options = {
        "provider": {"category": "model", "currentValue": "uap-scripted", "description": "The authentication provider to use", "id": "provider", "name": "Provider", "options": [{"name": "Cline Usage-Billing", "value": "cline"}, {"name": "ClinePass", "value": "cline-pass"}, {"name": "OpenAI ChatGPT Subscription", "value": "openai-codex"}], "type": "select"},
        "model": {"category": "model", "currentValue": "uap-cline-model", "id": "model", "name": "Model", "options": [{"name": "UAP scripted model", "value": "uap-cline-model"}], "type": "select"},
        "mode": {"category": "mode", "currentValue": "act", "description": "Controls whether the agent can modify files", "id": "mode", "name": "Session Mode", "options": [], "type": "select"},
        "auto_approve": {"currentValue": False, "description": "Automatically approve all tool calls without asking for permission", "id": "auto_approve", "name": "Auto-approve tools", "type": "boolean"}}
    expected_options["mode"]["options"] = [{"description": entry["description"], "name": entry["name"], "value": entry["id"]} for entry in modes]
    require(exact(options, expected_options))
    require(type(frames[-1]["id"]) is int and frames[-1] == {"jsonrpc": "2.0", "id": 3, "result": {"stopReason": "end_turn"}})
    middle = frames[2:-1]
    for frame in middle:
        require(frame["params"]["sessionId"] == session)
        require(set(frame) == ({"jsonrpc", "id", "method", "params"} if frame["method"] == "session/request_permission" else {"jsonrpc", "method", "params"}))
        require(set(frame["params"]) == ({"sessionId", "toolCall", "options"} if frame["method"] == "session/request_permission" else {"sessionId", "update"}))
    updates = [frame["params"]["update"] for frame in middle if frame["method"] == "session/update"]
    require(updates[-2] == {"sessionUpdate": "agent_message_chunk", "content": {"type": "text", "text": receipt["sentinel"]}})
    require(set(updates[-1]) == {"sessionUpdate", "updatedAt"} and updates[-1]["sessionUpdate"] == "session_info_update")
    require(stamp(command["started_at"]) <= stamp(updates[-1]["updatedAt"]) <= stamp(command["ended_at"]))
    if empty:
        require([frame["method"] for frame in middle] == ["session/update", "session/update"])
        return None, None
    require([frame["method"] for frame in middle] == ["session/update", "session/request_permission"] + ["session/update"] * 5)
    require([(entry["sessionUpdate"], entry.get("status")) for entry in updates[:-2]] == [("tool_call_update", "pending"), ("tool_call_update", "in_progress"), ("tool_call", "pending"), ("tool_call_update", "completed")])
    permission = middle[1]["params"]
    require(type(middle[1]["id"]) in (int, str) and type(middle[1]["id"]) is not bool)
    expected_options = [{"kind": "allow_once", "name": "Allow once", "optionId": "allow_once"}, {"kind": "allow_always", "name": "Allow always", "optionId": "allow_always"}, {"kind": "reject_once", "name": "Reject", "optionId": "reject_once"}]
    require(permission["options"] == expected_options and receipt["single_call_approved"] is True)
    require(all(entry["toolCallId"] == receipt["call_id"] for entry in updates[:-2]))
    require(permission["toolCall"]["toolCallId"] == receipt["call_id"])
    require(permission["toolCall"]["rawInput"] == updates[0]["rawInput"] == updates[2]["rawInput"])
    args = updates[0]["rawInput"]
    title = json.dumps(args, sort_keys=True, separators=(",", ":"))
    title = "uapclinea__inspect_runtime: " + (title[:57] + "..." if len(title) > 60 else title)
    for update in (updates[0], updates[2]):
        require(set(update) == {"sessionUpdate", "toolCallId", "title", "kind", "status", "rawInput"})
        require(update["title"] == title and update["kind"] == "other")
    require(permission["toolCall"] == {key: value for key, value in updates[0].items() if key != "sessionUpdate"})
    require(set(updates[1]) == {"sessionUpdate", "toolCallId", "status"})
    require(set(updates[3]) == {"sessionUpdate", "toolCallId", "status", "rawOutput"})
    result = updates[3]["rawOutput"]
    require(set(result) == {"content", "isError"} and result["isError"] is False and len(result["content"]) == 1)
    require(result["content"][0]["type"] == "text" and set(result["content"][0]) == {"type", "text"})
    return decode(result["content"][0]["text"]), updates[0]["rawInput"]


def native(rows, state_rows, projects, configs, decode, stream):
    acp, commands, requests, responses, probes, receipts = [grouped(rows[name]) for name in ("cycle-native-acp.jsonl", "cycle-native-commands.jsonl", "cycle-provider-requests.jsonl", "cycle-provider-responses.jsonl", "cycle-probe-events.jsonl", "cycle-provider-receipts.jsonl")]
    sessions, pids, probe_sessions, nonces = set(), set(), set(), {}
    previous_end = None
    for stage, captures in acp.items():
        command, receipt = commands[stage][0], receipts[stage][0]
        scope, empty = stage[-1], receipt["empty"]
        require(command["cwd"] == projects[scope] and command["mcp_settings_override"] == configs[scope])
        start, end = stamp(command["started_at"]), stamp(command["ended_at"])
        require(start < end and (previous_end is None or previous_end < start))
        previous_end = end
        require(command["session"] not in sessions)
        sessions.add(command["session"])
        facts, args = acp_frames(stream(captures[0]), command, receipt, empty, decode)
        request_bodies = [decode(stream(value["stream"])) for value in requests[stage]]
        events = [decode(line) for line in stream(probes[stage][0]).splitlines()]
        require(all(value["model"] == "uap-cline-model" and value["stream"] is True for value in request_bodies))
        nonce = receipt["prompt"].split("_")[-1]
        require(re.fullmatch(r"[0-9a-f]{32}", nonce) is not None and nonces.setdefault(scope, nonce) == nonce)
        require(receipt["prompt"] == "UAP_CLINE_CYCLE_" + stage + "_" + nonce)
        require(receipt["call_id"] == "uap-cline-cycle-" + digest((stage + nonce).encode())[:24])
        require(receipt["sentinel"] == "UAP_CLINE_DONE_" + stage + "_" + nonce)
        suffix = " Return only " + receipt["sentinel"] + ". Do not call any built-in or MCP tool or change any file." if empty else " Call only uapclinea__inspect_runtime with the exact supplied harmless nonce/operation/marker. Do not call built-in tools or any other MCP server."
        require(request_bodies[0]["messages"][-1] == {"role": "user", "content": receipt["prompt"] + suffix})
        require(len(request_bodies[0]["messages"]) == 2 and set(request_bodies[0]["messages"][0]) == {"role", "content"} and request_bodies[0]["messages"][0]["role"] == "system" and type(request_bodies[0]["messages"][0]["content"]) is str)
        for request in request_bodies:
            names = [tool["function"]["name"] for tool in request["tools"]]
            require(len(names) == len(set(names)) and all(type(name) is str for name in names))
            require({name for name in names if "__" in name} == (set() if empty else {"uapclinea__inspect_runtime"}))
        if empty:
            require(not events and facts is None and args is None)
            require(all(not tool["function"]["name"].endswith("__inspect_runtime") for tool in request_bodies[0]["tools"]))
        else:
            require([entry["method"] for entry in events] == ["initialize", "tools/list", "tools/call"])
            require(args == {"marker": "retained-" + nonce, "nonce": nonce, "operation": receipt["operation"]})
            require(events[-1]["facts"] == facts)
            require(set(facts) == {"nonce", "revision", "cwd", "argv", "plugin_root", "plugin_data", "literal", "once", "marker", "pid", "session"})
            tool = request_bodies[1]["messages"][-1]
            history = request_bodies[1]["messages"]
            require(len(history) == 4 and history[:2] == request_bodies[0]["messages"])
            assistant = history[-2]
            require(set(assistant) == {"role", "content", "tool_calls"} and assistant["role"] == "assistant" and assistant["content"] is None and len(assistant["tool_calls"]) == 1)
            call = assistant["tool_calls"][0]
            require(set(call) == {"id", "type", "function"} and call["id"] == receipt["call_id"] and call["type"] == "function")
            require(set(call["function"]) == {"name", "arguments"} and call["function"]["name"] == "uapclinea__inspect_runtime" and decode(call["function"]["arguments"]) == args)
            require({key: value for key, value in request_bodies[1].items() if key != "messages"} == {key: value for key, value in request_bodies[0].items() if key != "messages"})
            require(tool["role"] == "tool" and tool["tool_call_id"] == receipt["call_id"])
            tool_result = decode(tool["content"])
            require(set(tool) == {"role", "tool_call_id", "content"} and set(tool_result) == {"content", "isError"} and tool_result["isError"] is False and len(tool_result["content"]) == 1 and set(tool_result["content"][0]) == {"type", "text"} and tool_result["content"][0]["type"] == "text")
            require(decode(tool_result["content"][0]["text"]) == facts)
            proof = {"installed-a": "add-a", "installed-b": "add-b", "updated-a": "update-a", "repaired-a": "missing-repair-a", "remaining-b": "add-b"}[stage]
            ins, binding, revision = state_rows[proof]
            data = ins["data_receipts"][binding["data_receipt_id"]]
            require(facts["nonce"] == nonce and facts["revision"] == revision and facts["marker"] == args["marker"])
            require(facts["cwd"] == facts["plugin_root"] == binding["target_locator"] and facts["plugin_data"] == data["locator"])
            require(facts["argv"] == ["Owned Cline " + scope, binding["target_locator"] + "/argument"] and facts["literal"] == facts["once"] == "")
            require(type(facts["pid"]) is int and facts["pid"] > 0 and facts["pid"] not in pids and facts["session"] not in probe_sessions)
            pids.add(facts["pid"])
            probe_sessions.add(facts["session"])
            require(all((event["nonce"], event["pid"], event["session"]) == (nonce, facts["pid"], facts["session"]) and start <= stamp(event["timestamp"]) <= end for event in events))
        for index, value in enumerate(responses[stage]):
            lines = stream(value["stream"]).splitlines()
            require(all(not line or line.startswith(b"data: ") for line in lines))
            payloads = [line[6:] for line in lines if line.startswith(b"data: ")]
            require(len(payloads) == 3 and payloads[-1] == b"[DONE]")
            chunks = [decode(part) for part in payloads[:-1]]
            require(all(set(chunk) == {"choices", "created", "id", "model", "object"} and chunk["model"] == "uap-cline-model" and type(chunk["created"]) is int and chunk["created"] == 1 and chunk["id"] == "uap-cline-completion-" + nonce and chunk["object"] == "chat.completion.chunk" for chunk in chunks))
            require(len(chunks[0]["choices"]) == 1 and set(chunks[0]["choices"][0]) == {"delta", "finish_reason", "index"} and chunks[0]["choices"][0]["finish_reason"] is None and type(chunks[0]["choices"][0]["index"]) is int and chunks[0]["choices"][0]["index"] == 0)
            require(chunks[1]["choices"] == [{"delta": {}, "finish_reason": "stop" if empty or index == 1 else "tool_calls", "index": 0}])
            delta = chunks[0]["choices"][0]["delta"]
            if empty or index == 1:
                require(delta == {"content": receipt["sentinel"], "role": "assistant"})
            else:
                require(set(delta) == {"role", "tool_calls"} and delta["role"] == "assistant" and len(delta["tool_calls"]) == 1)
                call = delta["tool_calls"][0]
                require(set(call) == {"id", "index", "type", "function"} and type(call["index"]) is int and call["index"] == 0 and call["type"] == "function" and set(call["function"]) == {"name", "arguments"})
                require(call["id"] == receipt["call_id"] and call["function"]["name"] == "uapclinea__inspect_runtime" and decode(call["function"]["arguments"]) == args)


def validate_success(rows, decode, stream):
    """Reject malformed success; source-attested outgoing writer grants once.

    ACP captures inbound stdout, not outgoing permission replies. The exact
    inbound permission and approved receipt are joined here; the unchanged Go
    writer and independent outgoing-buffer replay establish allow_once bytes.
    """
    def load(raw):
        require(type(raw) in (str, bytes))
        return decode(raw.encode("utf-8", errors="strict") if type(raw) is str else raw)
    try:
        state_rows, proofs, projects, native_roots, configs = states(rows["cycle-proof.jsonl"], load, stream)
        skill_bodies = listing(rows["cycle-discovery.jsonl"], proofs, projects, native_roots, configs, load, stream)
        skill_ownership(state_rows, skill_bodies)
        native(rows, state_rows, projects, configs, load, stream)
    except (KeyError, IndexError, TypeError, AttributeError, OverflowError, UnicodeError, RecursionError) as error:
        raise ValueError("Cline complete-cycle semantic structure refused") from error


def validate_raw_skills(record, rows, decode, stream):
    """Require the additional public raw route AND the original mixed cycle.

    Captures are data only. Live foreign files, OS runner closure and tree
    comparisons remain source-attested gates, never inferred from a flag.
    """
    try:
        raw_skills(record, rows, decode, stream)
    except (KeyError, IndexError, TypeError, AttributeError, OverflowError, UnicodeError, RecursionError) as error:
        raise ValueError("Cline raw Skills semantic structure refused") from error


def raw_skills(record, rows, decode, stream):
    require(type(record) is dict and set(record) == {"source", "skill_sha256", "commands", "proofs", "inventories", "probe_before", "probe_after", "repeat_before", "repeat_after"})
    source = "https://github.com/777genius/universal-agent-plugins/tree/b5d80336b53397be5bbb72c84f680e5885a6ac0d/tests/fixtures/uap_apm/skills"
    key = "github-skill:777genius/universal-agent-plugins:benchmark-note:tests/fixtures/uap_apm/skills/benchmark-note"
    name = "skill-4e68c33bfb91dc370d5c3745c0ba746428ff227ed3b3466f9c9d1552"
    description = "An inert note for checking Agent Skills 1.0 installation."
    skill = b"---\nname: benchmark-note\ndescription: An inert note for checking Agent Skills 1.0 installation.\n---\n\nRead the adjacent references/fixture.txt when explicitly asked. Do not access networks, credentials, or external tools.\n"
    resource = b"Portable Agent Skills 1.0 fixture. This exact file must be preserved without transformation.\n"
    require(record["source"] == source and record["skill_sha256"] == digest(skill) == "53e39c4f1dcaa4c2ed1196654678ede091b71fb837f77a1beb07ba556abc1198")
    proofs = {row["stage"]: row["value"] for row in rows["cycle-proof.jsonl"]}
    root = PurePosixPath(proofs["source-a-a-one"]["source"]).parent
    projects = {"a": str(root / "project"), "b": str(root / "project-b"), "user": str(root / "outside")}
    old = decode(stream(proofs["repeat-remove-a"]["state"]))["installations"][0]
    require(stream(record["probe_before"]) == stream(record["probe_after"]) == b"".join(stream(row["value"]) for row in rows["cycle-probe-events.jsonl"]))
    require(type(record["proofs"]) is dict and set(record["proofs"]) == {"add", "repeat", "remove"})
    installed = []
    for phase in ("add", "repeat", "remove"):
        proof = record["proofs"][phase]
        require(set(proof) == ({"state"} if phase == "remove" else {"state", "skill", "resource", "binding"}))
        state = decode(stream(proof["state"]))
        require(type(state) is dict and type(state["schema_version"]) is int and state["schema_version"] == 5 and len(state["installations"]) == 2)
        others = [item for item in state["installations"] if item["installation_id"] == old["installation_id"]]
        require(exact(others, [old]))
        selected = [item for item in state["installations"] if item["installation_id"] != old["installation_id"]]
        require(len(selected) == 1)
        ins = selected[0]
        require(ins["declared_name"] == name and type(ins["installation_id"]) is str and ins["installation_id"])
        origin = ins["source"]
        require(origin["stable_source_key"] == key and origin["source_binding_id"] == "src_" + digest(key.encode())[:24] and origin["repository"] == "777genius/universal-agent-plugins" and origin["package_subpath"] == "tests/fixtures/uap_apm/skills/benchmark-note" and origin["resolved_revision"] == "b5d80336b53397be5bbb72c84f680e5885a6ac0d")
        require(re.fullmatch(r"sha256:[0-9a-f]{64}", origin["tree_digest"]) is not None)
        require(exact(ins["package"]["inventory"], {"mcp_present": False, "mcp_enabled": False, "skills": ["benchmark-note"]}))
        if phase == "remove":
            require(ins["clients"] == {} and ins.get("data_retained") is True and ins["installation_id"] == installed[0]["installation_id"] and exact(origin, installed[0]["source"]) and exact(ins["package"], installed[0]["package"]))
            continue
        require(stream(proof["skill"]) == skill and stream(proof["resource"]) == resource and len(ins["clients"]) == 1 and ins.get("data_retained", False) is False)
        binding = proof["binding"]
        require(exact(list(ins["clients"].values()), [binding]))
        physical = name[:51] + "-" + digest(ins["installation_id"].encode())[:12]
        active = projects["a"] + "/.uap/managed/clients/cline/" + physical
        bid = "client_" + digest("\0".join((ins["installation_id"], "cline", "project", active)).encode())[:24]
        require(binding["client_binding_id"] == bid and list(ins["clients"]) == [bid] and binding["physical_artifact_id"] == physical and binding["target_locator"] == active and binding["native_profile_root"] == projects["a"])
        require(all(binding[field] == value for field, value in {"client_id": "cline", "scope": "project", "activation": "active", "authentication": "not_required", "policy": "allowed", "materialization": "materialized", "verification": "installation_verified"}.items()))
        objects = {obj["kind"]: obj for obj in binding["native_objects"]}
        require(len(binding["native_objects"]) == len(objects) == 2 and set(objects) == {"managed_package_directory", "cline_project_skill_directory"})
        native = projects["a"] + "/.cline/skills/benchmark-note"
        skill_digest = "sha256:" + digest(b"f\0SKILL.md\0-\0" + skill + b"\0d\0references\0f\0references/fixture.txt\0-\0" + resource + b"\0")
        require(exact(objects["cline_project_skill_directory"], {"object_id": "cline_project_skill_directory:benchmark-note", "kind": "cline_project_skill_directory", "logical_name": "benchmark-note", "path": native, "source_relative": "skills/benchmark-note", "managed_digest": skill_digest, "protection_class": "managed"}))
        package_object = objects["managed_package_directory"]
        require(set(package_object) == {"object_id", "kind", "logical_name", "path", "managed_digest", "protection_class"} and package_object["object_id"] == "package:cline:" + physical and package_object["logical_name"] == name and package_object["path"] == active and package_object["protection_class"] == "managed" and re.fullmatch(r"sha256:[0-9a-f]{64}", package_object["managed_digest"]) is not None)
        installed.append(ins)
    require(exact(installed[0], installed[1]) and stream(record["proofs"]["add"]["state"]) == stream(record["proofs"]["repeat"]["state"]))
    snapshot = record["repeat_before"]
    require(exact(snapshot, record["repeat_after"]) and set(snapshot) == {"native", "managed", "state-v2.json", "uap.toml", "uap.lock"})
    require(stream(snapshot["state-v2.json"]) == stream(record["proofs"]["add"]["state"]) and stream(snapshot["uap.toml"]) and stream(snapshot["uap.lock"]))
    manifest = (json.dumps({"$schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json", "name": name}, separators=(",", ":"), sort_keys=True) + "\n").encode()
    expected_files = {"SKILL.md": skill, "references/fixture.txt": resource}
    for label, files, directories in (("native", expected_files, {".", "references"}), ("managed", {"plugin.json": manifest, **{"skills/benchmark-note/" + path: value for path, value in expected_files.items()}}, {".", "skills", "skills/benchmark-note", "skills/benchmark-note/references"})):
        tree = snapshot[label]
        require(type(tree) is dict and set(tree) == set(files) | directories)
        require(all(type(tree[path]) is str and tree[path].startswith("directory:") for path in directories))
        require(all(type(tree[path]) is str and re.fullmatch(r"file:[0-7]+:" + digest(body), tree[path]) is not None for path, body in files.items()))
    require(installed[0]["package"]["manifest_digest"] == "sha256:" + digest(manifest))
    commands = record["commands"]
    order = ["before-a", "before-b", "before-user", "add", "add-a", "add-b", "add-user", "repeat", "repeat-a", "repeat-b", "repeat-user", "remove", "remove-a", "remove-b", "remove-user"]
    require(type(commands) is list and [row["stage"] for row in commands] == order)
    add_argv = None
    public = {}
    for command in commands:
        require(set(command) == {"stage", "argv", "cwd", "exit", "error", "deadline", "stdout", "stderr"} and type(command["exit"]) is int and command["exit"] == 0 and command["error"] is False and command["deadline"] is False)
        argv, phase = command["argv"], command["stage"]
        require(type(argv) is list and all(type(part) is str for part in argv) and PurePosixPath(argv[0]).is_absolute())
        if phase in ("add", "repeat", "remove"):
            require(command["cwd"] == projects["user"])
            expected = ["skills", "add", source, "--skill", "benchmark-note", "--target", "cline", "--scope", "project", "--project-root", projects["a"], "--yes", "--no-ui", "--format", "json"]
            if phase == "remove": expected = ["remove", installed[0]["installation_id"], "--target", "cline", "--scope", "project", "--project-root", projects["a"], "--yes", "--format", "json"]
            require(argv[1:] == expected)
            if phase == "add": add_argv = argv
            if phase == "repeat": require(argv == add_argv)
            doc = decode(stream(command["stdout"]))
            stream(command["stderr"])
            require(type(doc) is dict and type(doc["schema_version"]) is int and doc["schema_version"] == 1 and doc["command"] == ("agentplugins remove" if phase == "remove" else "skills add") and doc["result"] == "success")
            if phase != "remove":
                require(doc["status"] == "succeeded" and doc["readiness"] == "ready" and not doc.get("next_actions") and len(doc["items"]) == 1)
                item = doc["items"][0]
                require(item["alias"] == "benchmark-note" and item["targetId"] == "cline" and item["destinationPath"] == active and item["status"] == ("unchanged" if phase == "repeat" else "succeeded"))
            public[phase] = command
        else:
            scope = phase.split("-")[1]
            require(command["cwd"] == projects[scope] and argv[1:] == ["--auto-approve", "false", "--cwd", projects[scope], "config", "skills", "--json"] and stream(command["stderr"]) == b"")
    inventories = record["inventories"]
    require(type(inventories) is list and [(row["stage"], row["scope"]) for row in inventories] == [(phase, scope) for phase in ("before", "add", "repeat", "remove") for scope in ("a", "b", "user")])
    baseline = {"a": "final-a", "b": "final-b", "user": "removed-user"}
    for row in inventories:
        require(set(row) == {"stage", "scope", "stdout"})
        require(exact(row["stdout"], commands[order.index(row["stage"] + "-" + row["scope"])]["stdout"]))
        prior = next(value["value"] for value in rows["cycle-discovery.jsonl"] if value["stage"] == baseline[row["scope"]] and value["value"]["kind"] == "skills")
        expected = {item["path"]: item for item in decode(stream(prior["stdout"]))}
        if row["scope"] == "a" and row["stage"] in ("add", "repeat"):
            path = native + "/SKILL.md"
            expected[path] = {"name": "benchmark-note", "description": description, "instructions": skill.split(b"---\n", 2)[2].decode().strip(), "frontmatter": {"name": "benchmark-note", "description": description}, "path": path}
        catalog = decode(stream(row["stdout"]))
        require(type(catalog) is list and len(catalog) == len(expected) and exact({item["path"]: item for item in catalog}, expected))

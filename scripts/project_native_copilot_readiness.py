"""Source-bound installed Copilot single-tool readiness, all five flags false."""
import hashlib
import json
from pathlib import PurePosixPath
import re

from project_native_copilot_events import MODE, PROMPT, decode, instant, metadata, natural, obj, require, same, telemetry, uuid, values

COMMANDS = ("git-A", "git-B", "version", "add-A", "native-attempt")
TRIPLES = {label + suffix for label in COMMANDS for suffix in (".command.json", ".stdout.log", ".stderr.log")}
EXACT = TRIPLES | {"runtime-attempt.json", "mcp.json.installed", "uapcopilota-SKILL.md.installed", "probe-events.jsonl", "provider-request-1.json", "provider-request-2.json"}
SUMMARY = set("acquisition activation binding client data_locator elapsed_ms installation_source probe_facts provider_kind provider_returned provider_error qualification real_model_qualified runtime_attempt_complete scanner source_identity test_failed vendor_authentication_qualified mode provider_closed readiness_observed native_command_started_at native_command_ended_at native_session_id provider_call_id original_config_change".split())


def canonical(path):
    require(type(path) is str and path.startswith("/") and not path.startswith("//")
            and str(PurePosixPath(path)) == path and ".." not in PurePosixPath(path).parts
            and len(path) <= 4096 and all(ord(c) >= 32 and c != "\x7f" for c in path))
    return path


def compact(value):
    return json.dumps(value, ensure_ascii=False, sort_keys=True, separators=(",", ":")).encode()


def digest(body):
    return "sha256:" + hashlib.sha256(body).hexdigest()


def members(bodies, source):
    from project_native_runtime import strict_json
    require("runtime-attempt.json" in bodies and not set(bodies) - EXACT)
    result = strict_json(bodies["runtime-attempt.json"])
    require(type(result) is dict and not set(result) - SUMMARY)
    values(result, {"mode": MODE, "client": "copilot", "provider_kind": "scripted_loopback", "activation": "prepared",
                    "vendor_authentication_qualified": False, "real_model_qualified": False,
                    "qualification": {name: False for name in ("project-lifecycle", "native-restart-removal", "mcp-initialize", "mcp-tools-list", "mcp-tools-call")}})
    for key in ("test_failed", "runtime_attempt_complete", "provider_closed", "readiness_observed"):
        require(type(result.get(key)) is bool)
    natural(result.get("elapsed_ms"), 180000)
    expected_source = {"installer_base_commit": source["source_commit"], "installer_tree": source["source_tree"],
                       "installer_source_state": "committed", "installer_patch_sha256": []}
    if "source_identity" in result:
        require(same(result["source_identity"], expected_source))
    commands = {}
    for label in COMMANDS:
        triple = {label + suffix for suffix in (".command.json", ".stdout.log", ".stderr.log")}
        require(not set(bodies) & triple or triple <= set(bodies))
        if triple <= set(bodies):
            row = obj(strict_json(bodies[label + ".command.json"]), "argv cwd exit elapsed_ms error context_error cleanup")
            require(type(row["argv"]) is list and 1 <= len(row["argv"]) <= 64
                    and all(type(arg) is str and len(arg) <= 4096 for arg in row["argv"]))
            canonical(row["cwd"])
            require(type(row["exit"]) is int and type(row["error"]) is bool and type(row["context_error"]) is bool)
            natural(row["elapsed_ms"], 65000)
            values(row, {"cleanup": "ordinary full-descendant supervision; forced cleanup is a failure"})
            commands[label] = row
    if result["test_failed"]:
        require(result["runtime_attempt_complete"] is False)
        return result
    require(result["runtime_attempt_complete"] and result["readiness_observed"] and result["provider_closed"]
            and result.get("provider_returned") is True and "provider_error" not in result
            and "original_config_change" not in result and set(bodies) == EXACT
            and same(result.get("source_identity"), expected_source) and result["elapsed_ms"] < 150000)
    for row in commands.values():
        values(row, {"exit": 0, "error": False, "context_error": False})
    root, project, home, active, data, nonce = authority(bodies, result, commands)
    start, end = instant(result["native_command_started_at"]), instant(result["native_command_ended_at"])
    require(commands["native-attempt"]["elapsed_ms"] >= (end - start) // 10**6
            and result["elapsed_ms"] >= sum(row["elapsed_ms"] for row in commands.values()))
    rows, events, session = decode(bodies["native-attempt.stdout.log"], start, end)
    require(session == result["native_session_id"])
    metadata(events, root, project)
    telemetry(rows, events)
    facts = exchange(bodies, result, events, session, nonce, start, end)
    expected = {"nonce": nonce, "revision": "A-one", "cwd": active, "argv": ["argument with spaces", active + "/argument", "${UNKNOWN}"],
                "plugin_root": active, "plugin_data": data, "literal": "literal ${UNKNOWN} " + home,
                "once": active + "/${UNKNOWN}", "marker": "retained-" + nonce, "pid": facts["pid"], "session": facts["session"]}
    require(same(facts, expected) and same(facts, result["probe_facts"]))
    natural(facts["pid"], 1 << 31)
    require(facts["pid"] > 0 and type(facts["session"]) is str and 1 <= len(facts["session"]) <= 256
            and facts["session"] != session and facts["session"] != result["provider_call_id"])
    events_raw = bodies["probe-events.jsonl"]
    require(events_raw.endswith(b"\n"))
    probes = events_raw[:-1].split(b"\n")
    require(len(probes) == 3)
    last = start
    for i, method in enumerate(("initialize", "tools/list", "tools/call")):
        probe = obj(strict_json(probes[i]), "method pid session nonce timestamp facts")
        values(probe, {"method": method, "pid": facts["pid"], "session": facts["session"], "nonce": nonce,
                       "facts": facts if i == 2 else None})
        when = instant(probe["timestamp"])
        require(last <= when <= end)
        last = when
    return result


def authority(bodies, result, commands):
    from project_native_runtime import strict_json
    project = commands["git-A"]["cwd"]
    root, home = str(PurePosixPath(project).parent), str(PurePosixPath(project).parent / "home")
    require(project == root + "/project" and commands["git-B"]["cwd"] == root + "/project-B")
    git = canonical(commands["git-A"]["argv"][0])
    for label in ("git-A", "git-B"):
        require(commands[label]["argv"] == [git, "-c", "core.hooksPath=/dev/null", "init", "--quiet"]
                and bodies[label + ".stdout.log"] == bodies[label + ".stderr.log"] == b"")
    acquisition = obj(result["acquisition"], "client acquisition runtime_files")
    values(acquisition, {"client": "copilot", "acquisition": {"sha256": "9aeb4430e9ac9523546db07b227c7aba179a4710a55f9ac29ad143b0f9e1b325", "pin_verified": True,
           "members": [{"name": "copilot", "kind": "file", "bytes": 198445888, "sha256": "f19b3554a6b62605db3a26d5255ebe2a3ee39fcd6a357edb18301ea5037f6f0d"}]}})
    files = acquisition["runtime_files"]
    require(type(files) is list and len(files) == 1)
    runtime = obj(files[0], "path sha256")
    client = canonical(runtime["path"])
    require(runtime["sha256"] == acquisition["acquisition"]["members"][0]["sha256"])
    require(commands["version"]["argv"] == [client, "--version"] and commands["version"]["cwd"] == root + "/outside"
            and bodies["version.stderr.log"] == b"" and bodies["version.stdout.log"] == b"GitHub Copilot CLI 1.0.103.\nRun 'copilot update' to check for updates.\n")
    add = commands["add-A"]
    require(add["cwd"] == project and add["argv"] == [canonical(add["argv"][0]), "add", root + "/package", "--no-save", "--scope", "project", "--project-root", project, "--target", "copilot", "--format", "json"]
            and bodies["add-A.stderr.log"] == b"Resolving and validating one Agent Plugin package for every selected target...\nInstalling native-proof for 1 selected clients...\n")
    require(commands["native-attempt"]["cwd"] == project and commands["native-attempt"]["argv"] == [client, "-p", PROMPT, "--output-format", "json", "--no-auto-update", "--disable-client-telemetry", "--disable-builtin-mcps", "--available-tools=uapcopilota-inspect_runtime", "--allow-tool=uapcopilota(inspect_runtime)", "--deny-tool=shell", "--deny-tool=write", "--deny-tool=read", "--deny-tool=url", "--deny-tool=memory"]
            and bodies["native-attempt.stderr.log"] == b"")
    binding = obj(result["binding"], "client_binding_id client_id scope target_locator physical_artifact_id materialization activation authentication policy verification package_revision data_receipt_id affected_surfaces native_profile_root native_objects receipts updated_at")
    values(binding, {"client_id": "copilot", "scope": "project", "materialization": "materialized", "activation": "prepared", "authentication": "not_checked", "policy": "allowed", "verification": "installation_verified", "affected_surfaces": ["copilot"], "native_profile_root": project})
    physical = binding["physical_artifact_id"]
    require(type(physical) is str and re.fullmatch(r"native-proof-[0-9a-f]{12}", physical))
    active, data = project + "/.uap/managed/clients/copilot/" + physical, project + "/.uap/plugin-data/" + physical
    require(binding["target_locator"] == active and result["data_locator"] == data)
    require(type(binding["client_binding_id"]) is str and re.fullmatch(r"client_[0-9a-f]{24}", binding["client_binding_id"])
            and type(binding["data_receipt_id"]) is str and re.fullmatch(r"data_[0-9a-f]{24}", binding["data_receipt_id"]))
    require(instant(binding["updated_at"]) <= instant(result["native_command_started_at"]))
    source = obj(result["installation_source"], "source_binding_id requested_source canonical_source resolved_revision tree_digest")
    values(source, {"requested_source": root + "/package", "canonical_source": root + "/package", "resolved_revision": ""})
    require(type(source["tree_digest"]) is str and re.fullmatch(r"sha256:[0-9a-f]{64}", source["tree_digest"]))
    revision = obj(binding["package_revision"], "version tree_digest manifest_digest")
    values(revision, {"version": "1.0.0", "tree_digest": source["tree_digest"]})
    require(type(revision["manifest_digest"]) is str and re.fullmatch(r"sha256:[0-9a-f]{64}", revision["manifest_digest"]))
    facts = obj(result["probe_facts"], "nonce revision cwd argv plugin_root plugin_data literal once marker pid session")
    nonce = facts["nonce"]
    require(type(nonce) is str and re.fullmatch(r"[0-9a-f]{32}", nonce))
    entry = {"type": "stdio", "tools": ["*"], "command": active + "/io.github.777genius.agentplugins/managed-stdio-v1/agentplugins",
             "args": ["--internal-stdio-v1", active, data, active, "plugin", "--", "./bin/probe", "argument with spaces", active + "/argument", "${UNKNOWN}"],
             "env": {"PLUGIN_ROOT": active, "PLUGIN_DATA": data, "UAP_TEST_ROOT": root, "UAP_TEST_EVENTS": root + "/events.jsonl", "UAP_TEST_NONCE": nonce, "UAP_TEST_REVISION": "A-one", "UAP_TEST_LITERAL": "literal ${UNKNOWN} $HOME", "UAP_TEST_ONCE": active + "/${UNKNOWN}"}}
    require(same(strict_json(bodies["mcp.json.installed"]), {"mcpServers": {"uapcopilota": entry}}))
    skill = ("---\nname: uapcopilota\ndescription: Disposable Copilot project proof\n---\nUAP_COPILOT_A_ONE_" + nonce + "\n").encode()
    require(bodies["uapcopilota-SKILL.md.installed"] == skill)
    objects = binding["native_objects"]
    require(type(objects) is list and len(objects) == 3)
    owned = {"copilot_project_mcp_server": (project + "/.github/mcp.json", digest(b"agentplugins-native-mcp-v1\0copilot-mcpServers\0uapcopilota\0" + compact(entry))),
             "copilot_project_skill_directory": (project + "/.github/skills/uapcopilota", digest(b"f\0SKILL.md\0-\0" + skill + b"\0"))}
    seen = set()
    for item in objects:
        require(type(item) is dict and type(item.get("kind")) is str and item["kind"] not in seen)
        seen.add(item["kind"])
        if item["kind"] in owned:
            path, sha = owned[item["kind"]]
            values(item, {"object_id": item["kind"] + ":uapcopilota", "logical_name": "uapcopilota", "path": path, "managed_digest": sha, "protection_class": "managed"})
        else:
            values(item, {"kind": "managed_package_directory", "path": active, "logical_name": "native-proof", "protection_class": "managed"})
    require(set(owned) <= seen)
    public_receipt(bodies["add-A.stdout.log"], result, binding, source, revision, active, project, physical, objects)
    require(same(result["scanner"], {"archive_sha256": "2b3d176db752433b904a4b42375543ff398f4841d22e48f7d4f23ded925b72da",
            "binary_sha256": "b6723451f155eb49e84acb34d1f06de929bddfcff6238cd4e38cdc6519396fba", "platform": "linux-amd64",
            "route": "prepopulated supported scanner cache; ordinary CLI security assessment", "source_tag": "v0.1.3", "version": "0.1.3"}))
    return root, project, home, active, data, nonce


def public_receipt(body, result, binding, source, revision, active, project, physical, objects):
    from project_native_runtime import strict_json
    envelope = obj(strict_json(body), "schema_version command result data")
    values(envelope, {"schema_version": 1, "command": "add", "result": "success"})
    data = envelope["data"]
    require(type(data) is dict)
    values(data, {"batch": True, "status": "completed", "succeeded": 1, "failed": 0, "action_required": 1,
                  "plugin": "native-proof", "version": "1.0.0", "dry_run": False,
                  "tree_digest": source["tree_digest"], "manifest_digest": revision["manifest_digest"],
                  "environment": {"reason": "no_save_requested", "saved": False}})
    targets = data["targets"]
    require(type(targets) is list and len(targets) == 1 and type(targets[0]) is dict)
    values(targets[0], {"target": "copilot", "status": "external_completed"})
    output = targets[0]["output"]
    require(type(output) is dict)
    values(output, {"operation_id": data["operation_id"], "plugin": "native-proof", "version": "1.0.0", "dry_run": False,
                    "tree_digest": source["tree_digest"], "manifest_digest": revision["manifest_digest"]})
    outcome = output["result"]
    require(type(outcome) is dict)
    uuid(outcome["installation_id"])
    values(outcome, {"requires_confirmation": False, "mutated": True, "group_phase": "external_completed"})
    values(outcome["plan"], {"client_id": "copilot", "scope": "project", "status": "ready", "package_mode": "native",
           "activation": "prepared", "authentication": "not_checked", "policy": "allowed", "verification": "package_validated", "physical_artifact_id": physical})
    values(outcome["activation"], {"activation": "prepared", "authentication": "not_checked", "policy": "allowed", "verification": "installation_verified"})
    receipts = binding["receipts"]
    require(type(receipts) is list and len(receipts) == 1)
    receipt = obj(receipts[0], "operation_id operation_group_id sequence mutation_type client_binding_id active_path staging_path backup_path after_digest phase")
    operation = data["operation_id"]
    require(type(operation) is str and re.fullmatch(r"op-[0-9a-f]{32}", operation))
    values(receipt, {"operation_id": operation + "-001", "operation_group_id": operation, "sequence": 1, "mutation_type": "directory_swap",
                    "client_binding_id": binding["client_binding_id"], "active_path": active, "phase": "committed"})
    parent = project + "/.uap/managed/clients/copilot/"
    require(type(receipt["staging_path"]) is str and re.fullmatch(re.escape(parent) + r"\.agentplugins-staging-[0-9a-f]{16}", receipt["staging_path"])
            and receipt["backup_path"] == receipt["staging_path"].replace("/.agentplugins-staging-", "/.agentplugins-backup-"))
    for item in objects:
        keys = "object_id kind logical_name path managed_digest protection_class"
        if item["kind"] == "copilot_project_skill_directory":
            keys += " source_relative"
            values(item, {"source_relative": "skills/uapcopilota"})
        obj(item, keys)
        if item["kind"] == "managed_package_directory":
            values(item, {"object_id": "package:copilot:" + physical, "managed_digest": receipt["after_digest"]})
            require(type(item["managed_digest"]) is str and re.fullmatch(r"sha256:[0-9a-f]{64}", item["managed_digest"]))


def exchange(bodies, result, data, session, nonce, start, end):
    from project_native_runtime import strict_json
    call, tool = "uap_" + nonce + "_attempt", "uapcopilota-inspect_runtime"
    sentinel = "UAP_DONE_" + nonce + "_attempt"
    require(result["provider_call_id"] == call and session != call)
    args = {"marker": "retained-" + nonce, "nonce": nonce, "operation": "write"}
    user = data[7]
    values(user, {"content": PROMPT, "delivery": "idle", "supportedNativeDocumentMimeTypes": []})
    uuid(user["parentAgentTaskId"])
    transformed = user["transformedContent"]
    prefix, suffix = "<current_datetime>", "</current_datetime>\n\n" + PROMPT
    require(type(transformed) is str and transformed.startswith(prefix) and transformed.endswith(suffix)
            and start <= instant(transformed[len(prefix):-len(suffix)]) <= end)
    ids = [uuid(data[i]["messageId"]) for i in (7, 12, 22)]
    require(len(set(ids)) == 3 and session not in ids and uuid(user["interactionId"]) != session)
    for i, event in enumerate(data):
        if "turnId" in event:
            require(event["turnId"] == ("0" if i <= 16 else "1"))
        if "interactionId" in event:
            require(event["interactionId"] == user["interactionId"])
        if "model" in event:
            require(event["model"] == "uap-fixture-model")
    for i in (12, 22):
        values(data[i], {"originatingMessageId": user["messageId"], "rte": False, "apiCallId": "chatcmpl-" + nonce})
    values(data[12], {"content": "", "toolRequests": [{"toolCallId": call, "name": tool, "arguments": args, "type": "function", "toolTitle": "inspect_runtime", "mcpServerName": "uapcopilota", "mcpToolName": "inspect_runtime"}]})
    values(data[22], {"content": sentinel, "toolRequests": []})
    values(data[19], {"messageId": data[22]["messageId"]})
    values(data[20], {"messageId": data[22]["messageId"], "deltaContent": sentinel})
    values(data[10], {"toolCallId": call, "toolName": tool, "toolType": "function"})
    require(type(data[10]["inputDelta"]) is str and same(strict_json(data[10]["inputDelta"].encode()), args))
    values(data[13], {"toolCallId": call, "toolName": tool, "arguments": args, "toolTitle": "inspect_runtime", "mcpServerName": "uapcopilota", "mcpConfigServerName": "uapcopilota", "mcpTransport": "stdio", "mcpConfigSource": "workspace", "mcpToolName": "inspect_runtime"})
    values(data[15], {"toolCallId": call, "rte": False, "success": True})
    output = obj(data[15]["result"], "content detailedContent contents")
    require(type(output["content"]) is str)
    values(output, {"detailedContent": output["content"], "contents": [{"type": "text", "text": output["content"]}]})
    facts = obj(strict_json(output["content"].encode()), "nonce revision cwd argv plugin_root plugin_data literal once marker pid session")
    require(data[15]["toolTelemetry"]["metrics"]["mcp_result_content_bytes"] == len(output["content"].encode()))
    requests = [obj(strict_json(bodies[f"provider-request-{i}.json"]), "model messages temperature top_p frequency_penalty presence_penalty tools stream stream_options") for i in (1, 2)]
    expected_tool = [{"type": "function", "function": {"name": tool, "description": "Inspect only disposable fixture runtime facts.",
                     "parameters": {"type": "object", "properties": {key: {"type": "string"} for key in ("marker", "nonce", "operation")}, "required": ["nonce"], "additionalProperties": False}}}]
    for request in requests:
        values(request, {"model": "uap-fixture-model", "temperature": 0, "top_p": 0.95, "frequency_penalty": 0,
                         "presence_penalty": 0, "tools": expected_tool, "stream": True, "stream_options": {"include_usage": True}})
    first, second = requests[0].pop("messages"), requests[1].pop("messages")
    require(same(requests[0], requests[1]) and type(first) is list and len(first) == 2
            and type(second) is list and len(second) == 4 and same(first, second[:2]))
    system = obj(first[0], "role content")
    require(system["role"] == "system" and type(system["content"]) is str and 1 <= len(system["content"]) <= 2 << 20)
    require(same(first[1], {"role": "user", "content": transformed}))
    require(same(second[2], {"role": "assistant", "content": None, "refusal": None, "tool_calls": [{"id": call, "type": "function", "function": {"name": tool, "arguments": compact(args).decode()}}]}))
    require(same(second[3], {"role": "tool", "tool_call_id": call, "content": output["content"]}))
    return facts

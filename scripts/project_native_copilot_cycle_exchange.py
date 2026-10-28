"""Whole native catalogs, model wire exchange and append-only probe joins."""
from project_native_copilot_events import DISABLED, GRAMMAR, decode, telemetry, uuid
from project_native_copilot_cycle import AGGREGATES, CALLS, NATIVE, REGISTRY, compact, instant, load, natural, obj, require, same, sha, stream, timed, values
from project_native_copilot_cycle_proof import foreign, nonce

NO_CALL = (3, 4, 5, 6, 7, 8, 9, 19, 20, 21, 22, 23, 24, 25, 26)
BUILTINS = {"customize-cloud-agent": "b96d547993e3bf5142f5c01773688c2cad8c12a4cfddd3ca54cddf43838b101f", "github-pr-media": "6911e8b48dad09cfe8ed00b26792dc96939bc2ec9ab7b523d1d41e1109d8c444"}
PROOF_FOR = {"installed-a": "after-native-installed-a", "installed-b": "after-native-installed-b", "stable-a": "after-native-stable-a", "updated-a": "after-native-updated-a", "repaired-a": "after-native-repaired-a", "remaining-b": "after-native-remaining-b"}


def no_call_decode(raw, start, end):
    require(raw.endswith(b"\n") and 0 <= end - start <= 60 * 10**9)
    lines = raw[:-1].split(b"\n")
    require(len(lines) == 15)
    rows, data, ids, last = [], [], set(), start
    for i, (line, original) in enumerate(zip(lines, NO_CALL)):
        ephemeral = original < 7 or original == 9 or 19 <= original <= 21 or original in (23, 25)
        keys = "type timestamp sessionId exitCode usage" if i == 14 else "type data id timestamp parentId" + (" ephemeral" if ephemeral else "")
        row = obj(load(line), keys)
        when = instant(row["timestamp"])
        require(last <= when <= end)
        last = when
        rows.append(row)
        if i == 14:
            values(row, {"type": "result", "exitCode": 0})
            require(uuid(row["sessionId"]) not in ids)
            continue
        require(row["type"] == GRAMMAR[original][0] and uuid(row["id"]) not in ids)
        ids.add(row["id"])
        uuid(row["parentId"])
        if ephemeral:
            require(row["ephemeral"] is True)
        data.append(obj(row["data"], GRAMMAR[original][1]))
    session, external = rows[-1]["sessionId"], set()
    for i in (0, 5, 9):
        parent = rows[i]["parentId"]
        require(parent not in ids and parent not in external and parent != session)
        external.add(parent)
    require(all(rows[i]["parentId"] == rows[0]["parentId"] for i in range(1, 5)))
    for child, parent in ((6, 5), (7, 5), (8, 5), (11, 10), (12, 10), (13, 12)):
        require(rows[child]["parentId"] == rows[parent]["id"])
    require(rows[10]["parentId"] == rows[9]["parentId"])
    usage(rows[-1]["usage"])
    return rows, data, session


def usage(value):
    obj(value, "premiumRequests totalApiDurationMs sessionDurationMs codeChanges")
    values(value, {"premiumRequests": 0, "codeChanges": {"linesAdded": 0, "linesRemoved": 0, "filesModified": []}})
    natural(value["totalApiDurationMs"], 60000)
    natural(value["sessionDurationMs"], 60000)


def catalog(data, layout, recipes, stage):
    call = stage in CALLS
    offset = 3 if call else 0
    base, proofs = recipes["base_nonce"], recipes["proofs"]
    key = CALLS[stage][0] if call else "user" if stage in ("installed-user", "remaining-user") else stage[-1]
    project = layout["outside"] if key == "user" else layout[key]
    want = {}
    def add(name, description, source, path, invocable=True):
        want[name] = {"name": name, "commandName": name, "description": description, "source": source, "path": path, "userInvocable": invocable, "enabled": True}
    for who in (("user", "b") if key == "b" else ("user",)):
        name, description, path, _ = foreign(layout, base, who)
        add(name, description, "personal-copilot" if who == "user" else "project", path)
    if stage not in ("final-a", "final-b"):
        user = proofs["add-user"]
        add("uapcopilotuser", "Disposable Copilot user user-one " + nonce(base, "user"), "plugin", user["skill_path"])
    if call:
        value = proofs[PROOF_FOR[stage]]
        skill = "uapcopilot" + key
        add(skill, f'Disposable Copilot {key} {value["revision"]} {value["nonce"]}', "project", value["skill_path"])
        for index, expected in enumerate(({"serverName": skill, "status": "pending"}, {"serverName": skill, "status": "connected", "configSource": "workspace"}, {"serverName": skill, "toolCount": 1})):
            require(same(data[index], expected))
    for name in BUILTINS:
        add(name, None, "builtin", layout["root"] + "/xdg-cache/copilot/pkg/linux-x64/1.0.103/builtin/" + name + "/SKILL.md", False)
    skills = data[offset]["skills"]
    require(type(skills) is list and len(skills) == len(want))
    seen = set()
    for item in skills:
        obj(item, "name commandName description source path userInvocable enabled")
        name = item["name"]
        require(type(name) is str and name in want and name not in seen)
        seen.add(name)
        if name in BUILTINS:
            require(type(item["description"]) is str and sha(item["description"].encode()) == BUILTINS[name])
            want[name]["description"] = item["description"]
        require(same(item, want[name]))
    values(data[offset + 1], {"infoType": "configuration", "message": DISABLED})
    values(data[offset + 3], {"model": "uap-fixture-model"})
    servers = [{"name": name, "status": "disabled"} for name in ("WorkIQ", "browser-use", "github-copilot-browser", "github-mcp-server", "githubiq")]
    if call:
        servers.append({"name": skill, "status": "connected", "source": "workspace", "serverMetadata": {"instructions": None}})
    require(same(data[offset + 2]["servers"], servers))
    return project


def request(raw, tool, transformed, args, call, facts=None):
    row = load(raw)
    required = set("model messages temperature top_p frequency_penalty presence_penalty stream stream_options".split())
    require(type(row) is dict and required <= set(row) <= required | {"tools"})
    values(row, {"model": "uap-fixture-model", "temperature": 0, "top_p": 0.95, "frequency_penalty": 0, "presence_penalty": 0, "stream": True, "stream_options": {"include_usage": True}})
    if tool:
        expected = [{"type": "function", "function": {"name": tool, "description": "Inspect only disposable fixture runtime facts.", "parameters": {"type": "object", "properties": {key: {"type": "string"} for key in ("marker", "nonce", "operation")}, "required": ["nonce"], "additionalProperties": False}}}]
        require(same(row["tools"], expected))
    else:
        require("tools" not in row or same(row["tools"], []))
    messages = row["messages"]
    require(type(messages) is list and len(messages) == (4 if facts is not None else 2))
    system = obj(messages[0], "role content")
    require(system["role"] == "system" and type(system["content"]) is str and 1 <= len(system["content"].encode()) <= 1 << 20)
    require(same(messages[1], {"role": "user", "content": transformed}))
    if facts is not None:
        require(same(messages[2], {"role": "assistant", "content": None, "refusal": None, "tool_calls": [{"id": call, "type": "function", "function": {"name": tool, "arguments": compact(args).decode()}}]}))
        require(same(messages[3], {"role": "tool", "tool_call_id": call, "content": facts}))
    return messages[:2]


def response(raw, n, delta, finish):
    def chunk(change, reason):
        return b"data: " + compact({"id": "chatcmpl-" + n, "object": "chat.completion.chunk", "created": 1, "model": "uap-fixture-model", "choices": [{"index": 0, "delta": change, "finish_reason": reason}]}) + b"\n\n"
    require(raw == chunk(delta, None) + chunk({}, finish) + b"data: [DONE]\n\n")


def messages(data, stage, n, start, end, call):
    prompt = "Report only the disposable fixture challenge for phase " + stage + "."
    user = data[7 if call else 4]
    values(user, {"content": prompt, "delivery": "idle", "supportedNativeDocumentMimeTypes": []})
    uuid(user["parentAgentTaskId"])
    transformed = user["transformedContent"]
    prefix, suffix = "<current_datetime>", "</current_datetime>\n\n" + prompt
    require(type(transformed) is str and transformed.startswith(prefix) and transformed.endswith(suffix)
            and start <= instant(transformed[len(prefix):-len(suffix)]) <= end)
    for i, item in enumerate(data):
        if "turnId" in item:
            require(item["turnId"] == ("1" if call and i > 16 else "0"))
        if "interactionId" in item:
            require(item["interactionId"] == user["interactionId"])
        if "model" in item:
            require(item["model"] == "uap-fixture-model")
    uuid(user["messageId"])
    uuid(user["interactionId"])
    final = data[22 if call else 10]
    uuid(final["messageId"])
    require(final["messageId"] != user["messageId"])
    values(final, {"originatingMessageId": user["messageId"], "rte": False, "apiCallId": "chatcmpl-" + n, "content": "UAP_DONE_" + n + "_" + stage, "toolRequests": []})
    values(data[19 if call else 7], {"messageId": final["messageId"]})
    values(data[20 if call else 8], {"messageId": final["messageId"], "deltaContent": final["content"]})
    if not call:
        values(data[9], {"outcome": "success", "editClassifierVersion": 1, "containsBuiltInFileEditRequest": False})
        natural(data[9]["dispatchDurationMs"], 60000)
        require(same(data[11], {"model": "uap-fixture-model", "isByok": True, "result": "success"}))
    return user, final


def tool_facts(data, tool, skill, call, args, n, stage):
    user = data[7]
    values(data[12], {"originatingMessageId": user["messageId"], "rte": False, "apiCallId": "chatcmpl-" + n, "content": "", "toolRequests": [{"toolCallId": call, "name": tool, "arguments": args, "type": "function", "toolTitle": "inspect_runtime", "mcpServerName": skill, "mcpToolName": "inspect_runtime"}]})
    uuid(data[12]["messageId"])
    require(len({user["messageId"], data[12]["messageId"], data[22]["messageId"]}) == 3)
    values(data[10], {"toolCallId": call, "toolName": tool, "toolType": "function"})
    require(type(data[10]["inputDelta"]) is str and same(load(data[10]["inputDelta"].encode()), args))
    values(data[13], {"toolCallId": call, "toolName": tool, "arguments": args, "toolTitle": "inspect_runtime", "mcpServerName": skill, "mcpConfigServerName": skill, "mcpTransport": "stdio", "mcpConfigSource": "workspace", "mcpToolName": "inspect_runtime"})
    values(data[15], {"toolCallId": call, "rte": False, "success": True})
    output = obj(data[15]["result"], "content detailedContent contents")
    require(type(output["content"]) is str)
    values(output, {"detailedContent": output["content"], "contents": [{"type": "text", "text": output["content"]}]})
    require(data[15]["toolTelemetry"]["metrics"]["mcp_result_content_bytes"] == len(output["content"].encode()))
    return output["content"]


def probe(value, prefix, expected, start, end, native_ids, probe_ids, call):
    obj(value, "before_bytes before_sha256 after_bytes after_sha256 delta")
    delta = stream(value["delta"])
    values(value, {"before_bytes": len(prefix), "before_sha256": sha(prefix), "after_bytes": len(prefix + delta), "after_sha256": sha(prefix + delta)})
    if expected is None:
        require(delta == b"")
        return prefix
    facts = obj(load(expected[0].encode()), "nonce revision cwd argv plugin_root plugin_data literal once marker pid session")
    held, home = expected[1:]
    active, data, n = held["active"], held["data_path"], held["nonce"]
    values(facts, {"nonce": n, "revision": held["revision"], "cwd": active, "argv": ["argument with spaces", active + "/argument", "${UNKNOWN}"], "plugin_root": active, "plugin_data": data, "literal": "literal ${UNKNOWN} " + home, "once": active + "/${UNKNOWN}", "marker": "retained-" + n})
    natural(facts["pid"], (1 << 31) - 1)
    import re
    session = facts["session"]
    require(facts["pid"] > 0 and type(session) is str and re.fullmatch(r"[1-9][0-9]{0,9}-[1-9][0-9]{0,19}", session)
            and session not in native_ids and session not in probe_ids and session != call and delta.endswith(b"\n"))
    lines = delta[:-1].split(b"\n")
    require(len(lines) == 3)
    last = start
    for i, method in enumerate(("initialize", "tools/list", "tools/call")):
        row = obj(load(lines[i]), "method pid session nonce timestamp facts")
        values(row, {"method": method, "pid": facts["pid"], "session": session, "nonce": n, "facts": facts if i == 2 else None})
        when = instant(row["timestamp"])
        require(last <= when <= end)
        last = when
    probe_ids.add(session)
    return prefix + delta


def validate(data, layout, recipes):
    def indexed(name):
        return {row["stage"]: row["value"] for row in data[name]}
    commands, events = indexed(AGGREGATES[1]), indexed(AGGREGATES[0])
    receipts, probes = indexed(AGGREGATES[5]), indexed(AGGREGATES[6])
    requests, responses = {}, {}
    for name, dest in ((AGGREGATES[3], requests), (AGGREGATES[4], responses)):
        for row in data[name]:
            value = obj(row["value"], "index body")
            dest.setdefault(row["stage"], []).append(stream(value["body"]))
            require(value["index"] == len(dest[row["stage"]]))
    prefix, native_ids, probe_ids, elapsed = b"", set(), set(), 0
    windows = []
    for stage in NATIVE:
        record, raw, start, end = timed(commands[stage], True)
        require(raw == stream(events[stage]))
        call = stage in CALLS
        n = nonce(recipes["base_nonce"], CALLS[stage][0]) if call else nonce(recipes["base_nonce"], stage)
        call_id, sentinel = "uap_" + n + "_" + stage, "UAP_DONE_" + n + "_" + stage
        rows, event_data, session = decode(raw, start, end, startup=True) if call else no_call_decode(raw, start, end)
        require(session not in native_ids and session not in probe_ids and session != call_id)
        project = catalog(event_data, layout, recipes, stage)
        args = [layout["client"], "-p", "Report only the disposable fixture challenge for phase " + stage + ".", "--output-format", "json", "--no-auto-update", "--disable-client-telemetry", "--disable-builtin-mcps", "--excluded-tools=" + DISABLED.removeprefix("Disabled tools: ").replace(", ", ",")]
        tool, arguments, facts, held = "", None, None, None
        if call:
            key, operation = CALLS[stage]
            skill = "uapcopilot" + key
            tool = skill + "-inspect_runtime"
            arguments = {"marker": "retained-" + n, "nonce": n, "operation": operation}
            args.append("--allow-tool=" + skill + "(inspect_runtime)")
            telemetry(rows, event_data)
            facts = tool_facts(event_data, tool, skill, call_id, arguments, n, stage)
            held = recipes["proofs"][PROOF_FOR[stage]]
        args += ["--deny-tool=" + kind for kind in ("shell", "write", "read", "url", "memory")]
        values(record, {"argv": args, "cwd": project})
        user, final = messages(event_data, stage, n, start, end, call)
        require(session not in (user["messageId"], user["interactionId"], final["messageId"]) and (not call or session != event_data[12]["messageId"]))
        count = 2 if call else 1
        receipt = obj(receipts[stage], "requests responses error returned shutdown serve_closed connections_closed forced")
        values(receipt, {"requests": count, "responses": count, "error": False, "returned": True, "shutdown": True, "serve_closed": True, "connections_closed": True, "forced": False})
        require(len(requests[stage]) == len(responses[stage]) == count)
        initial = request(requests[stage][0], tool, user["transformedContent"], arguments, call_id)
        if call:
            require(same(request(requests[stage][1], tool, user["transformedContent"], arguments, call_id, facts), initial))
            response(responses[stage][0], n, {"role": "assistant", "tool_calls": [{"index": 0, "id": call_id, "type": "function", "function": {"name": tool, "arguments": compact(arguments).decode()}}]}, "tool_calls")
        response(responses[stage][-1], n, {"role": "assistant", "content": sentinel}, "stop")
        prefix = probe(probes[stage], prefix, (facts, held, layout["home"]) if call else None, start, end, native_ids | {session}, probe_ids, call_id)
        native_ids.add(session)
        elapsed += record["elapsed_ms"]
        windows.append((start, end))
    require(len(native_ids) == 14 and len(probe_ids) == 6)
    require(all(windows[i][1] <= windows[i + 1][0] for i in range(13)))
    user = recipes["proofs"]["add-user"]
    last = None
    for row in data[AGGREGATES[2]]:
        stage = row["stage"]
        record, raw, start, end = timed(row["value"], True, maximum=35000)
        values(record, {"argv": [layout["client"], "plugin", "list"], "cwd": layout["outside"]})
        require(last is None or last <= start)
        last = end
        if stage in ("empty-user", "remove-user", "repeat-remove-user"):
            expected = "No plugins installed.\n\nUse 'copilot plugin install <source>' to install a plugin.\nBuilt-in plugins:\n  • browser-use (v0.0.4) [disabled]\n"
        else:
            market = "agentplugins-" + sha(user["binding"]["physical_artifact_id"].encode())[:12]
            expected = "Built-in plugins:\n  • browser-use (v0.0.4) [disabled]\nLive Plugins (loaded from a local marketplace directory, never copied):\n  • native-proof@" + market + " (v1.0.0) (enabled)\n      from " + user["active"] + "\n"
        require(raw == expected.encode())
        elapsed += record["elapsed_ms"]
    return elapsed

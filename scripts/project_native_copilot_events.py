"""Finite measured Copilot1.0.103 readiness events; never lifecycle proof."""
import calendar
import datetime
import hashlib
import re

MODE = "copilot-installed-tool-readiness"
PROMPT = "Call only the installed inspect_runtime tool once for the fixture challenge."
UUID = re.compile(r"[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}\Z")
STAMP = re.compile(r"(\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d)(?:\.(\d{1,9}))?(Z|[+-]\d\d:\d\d)\Z")
GRAMMAR = (
    ("session.mcp_server_status_changed", "serverName status"),
    ("session.mcp_server_status_changed", "serverName status configSource"),
    ("session.mcp_server_tools_loaded", "serverName toolCount"),
    ("session.skills_loaded", "skills"), ("session.info", "infoType message"),
    ("session.mcp_servers_loaded", "servers"), ("session.tools_updated", "model"),
    ("user.message", "content transformedContent messageId supportedNativeDocumentMimeTypes delivery interactionId turnId parentAgentTaskId"),
    ("assistant.turn_start", "turnId interactionId"), ("model.call_start", "turnId model"),
    ("assistant.tool_call_delta", "toolCallId toolName toolType inputDelta"),
    ("model.call_finished", "turnId dispatchDurationMs outcome editClassifierVersion interactionId containsBuiltInFileEditRequest"),
    ("assistant.message", "messageId originatingMessageId model content toolRequests interactionId turnId rte apiCallId"),
    ("tool.execution_start", "toolCallId toolName arguments turnId model toolTitle mcpServerName mcpConfigServerName mcpTransport mcpConfigSource mcpToolName"),
    ("model.call_final_result", "model isByok result"),
    ("tool.execution_complete", "toolCallId model interactionId turnId rte success result toolTelemetry"),
    ("assistant.turn_end", "turnId"), ("assistant.turn_start", "turnId interactionId"),
    ("model.call_start", "turnId model"), ("assistant.message_start", "messageId"),
    ("assistant.message_delta", "messageId deltaContent"),
    ("model.call_finished", "turnId dispatchDurationMs outcome editClassifierVersion interactionId containsBuiltInFileEditRequest"),
    ("assistant.message", "messageId originatingMessageId model content toolRequests interactionId turnId rte apiCallId"),
    ("model.call_final_result", "model isByok result"), ("assistant.turn_end", "turnId"), ("assistant.idle", ""),
)
DISABLED = "Disabled tools: bash, create, dynamic_workflows_manage, edit, fetch_copilot_cli_documentation, glob, grep, list_agents, list_bash, read_agent, read_bash, run_dynamic_workflow, session_store_sql, skill, sql, stop_bash, task, view, write_agent"


def require(condition):
    if not condition:
        raise ValueError("Copilot readiness measured semantic boundary")


def same(a, b):
    if type(a) is not type(b):
        return False
    if type(a) is dict:
        return a.keys() == b.keys() and all(same(a[key], b[key]) for key in a)
    if type(a) is list:
        return len(a) == len(b) and all(same(x, y) for x, y in zip(a, b))
    return a == b


def obj(value, keys):
    require(type(value) is dict and set(value) == set(keys.split()))
    return value


def values(row, expected):
    require(all(key in row and same(row[key], value) for key, value in expected.items()))


def natural(value, maximum):
    require(type(value) is int and 0 <= value <= maximum)


def uuid(value):
    require(type(value) is str and UUID.fullmatch(value) is not None)
    return value


def instant(value):
    require(type(value) is str)
    match = STAMP.fullmatch(value)
    require(match is not None)
    parsed = datetime.datetime.fromisoformat(match[1] + match[3].replace("Z", "+00:00"))
    return calendar.timegm(parsed.utctimetuple()) * 10**9 + int((match[2] or "").ljust(9, "0"))


def decode(body, start, end):
    from project_native_runtime import strict_json
    require(type(body) is bytes and len(body) <= 2 << 20 and body.endswith(b"\n")
            and 0 <= end - start <= 60 * 10**9)
    lines = body[:-1].split(b"\n")
    require(len(lines) == 27)
    rows, data, ids, last = [], [], set(), start
    for i, line in enumerate(lines):
        ephemeral = i < 7 or i in (9, 10, 11, 14, 18, 19, 20, 21, 23, 25)
        keys = "type data id timestamp parentId" + (" ephemeral" if ephemeral else "")
        if i == 26:
            keys = "type timestamp sessionId exitCode usage"
        row = obj(strict_json(line), keys)
        when = instant(row["timestamp"])
        require(last <= when <= end)
        last = when
        rows.append(row)
        if i == 26:
            values(row, {"type": "result", "exitCode": 0})
            require(uuid(row["sessionId"]) not in ids)
            continue
        require(row["type"] == GRAMMAR[i][0] and uuid(row["id"]) not in ids)
        ids.add(row["id"])
        uuid(row["parentId"])
        if ephemeral:
            require(row["ephemeral"] is True)
        data.append(obj(row["data"], GRAMMAR[i][1]))
    session, external = rows[26]["sessionId"], set()
    for i in (0, 8, 11, 21):
        parent = rows[i]["parentId"]
        require(parent not in ids and parent not in external and parent != session)
        external.add(parent)
    require(all(rows[i]["parentId"] == rows[0]["parentId"] for i in range(1, 8)))
    for child, parent in ((9, 8), (10, 8), (13, 12), (14, 13), (15, 13), (16, 15),
                          (17, 16), (18, 17), (19, 17), (20, 17), (23, 22), (24, 22), (25, 24)):
        require(rows[child]["parentId"] == rows[parent]["id"])
    require(rows[12]["parentId"] == rows[11]["parentId"] and rows[22]["parentId"] == rows[21]["parentId"])
    return rows, data, session


def metadata(data, root, project):
    values(data[0], {"serverName": "uapcopilota", "status": "pending"})
    values(data[1], {"serverName": "uapcopilota", "status": "connected", "configSource": "workspace"})
    values(data[2], {"serverName": "uapcopilota", "toolCount": 1})
    values(data[4], {"infoType": "configuration", "message": DISABLED})
    values(data[6], {"model": "uap-fixture-model"})
    skills = data[3]["skills"]
    require(type(skills) is list and len(skills) == 3)
    names = ("uapcopilota", "customize-cloud-agent", "github-pr-media")
    hashes = (None, "b96d547993e3bf5142f5c01773688c2cad8c12a4cfddd3ca54cddf43838b101f", "6911e8b48dad09cfe8ed00b26792dc96939bc2ec9ab7b523d1d41e1109d8c444")
    for i, skill in enumerate(skills):
        obj(skill, "name commandName description source userInvocable enabled path")
        path = project + "/.github/skills/" + names[i] + "/SKILL.md" if i == 0 else root + "/xdg-cache/copilot/pkg/linux-x64/1.0.103/builtin/" + names[i] + "/SKILL.md"
        values(skill, {"name": names[i], "commandName": names[i], "source": "project" if i == 0 else "builtin",
                       "userInvocable": i == 0, "enabled": True, "path": path})
        require(type(skill["description"]) is str)
        require(skill["description"] == "Disposable Copilot project proof" if i == 0 else
                hashlib.sha256(skill["description"].encode()).hexdigest() == hashes[i])
    expected = [{"name": name, "status": "disabled"} for name in
                ("WorkIQ", "browser-use", "github-copilot-browser", "github-mcp-server", "githubiq")]
    expected.append({"name": "uapcopilota", "status": "connected", "source": "workspace", "serverMetadata": {"instructions": None}})
    require(same(data[5]["servers"], expected))


def telemetry(rows, data):
    for i in (11, 21):
        natural(data[i]["dispatchDurationMs"], 60000)
        values(data[i], {"outcome": "success", "editClassifierVersion": 1, "containsBuiltInFileEditRequest": False})
    for i in (14, 23):
        values(data[i], {"model": "uap-fixture-model", "isByok": True, "result": "success"})
    metrics = obj(obj(data[15]["toolTelemetry"], "metrics")["metrics"],
                  "mcp_result_content_bytes context_token_limit context_current_tokens large_output_threshold_bytes result_token_count")
    for value in metrics.values():
        natural(value, 1 << 31)
    usage = obj(rows[26]["usage"], "premiumRequests totalApiDurationMs sessionDurationMs codeChanges")
    values(usage, {"premiumRequests": 0, "codeChanges": {"linesAdded": 0, "linesRemoved": 0, "filesModified": []}})
    natural(usage["totalApiDurationMs"], 60000)
    natural(usage["sessionDurationMs"], 60000)

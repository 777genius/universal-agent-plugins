"""Independent success-only replay of Gemini catalogs, wire traffic and probes."""
import posixpath
import re
import json

import project_native_gemini_cycle as cycle
from project_native_gemini_complete import (NS, closed, digest, load, natural, receipt, records,
                                            require, same, stream, text, window)
from project_native_gemini_complete_proof import view_stage, visible

MODEL = "gemini-2.5-flash"
WAIT = ("Set to true to wait for all previously requested tools in this turn to complete before starting. "
        "Set to false (or omit) to run in parallel. Use true when this tool depends on the output of previous tools.")
DESCRIPTIONS = {
    "antigravity-support": "Use when the user asks questions, seeks help, or requests instructions related to installing, setting up, or migrating to Antigravity CLI. This skill provides the latest up to date details, requirements, and commands sourced from the official Antigravity CLI documentation.",
    "skill-creator": "Guide for creating effective skills. This skill should be used when users want to create a new skill (or update an existing skill) that extends Gemini CLI's capabilities with specialized knowledge, workflows, or tool integrations.",
}
RIPGREP = "Ripgrep is not available. Falling back to GrepTool.\n"
TERMINAL = ("Warning: Basic terminal detected (TERM=dumb). Visual rendering will be limited. For the best experience, use a terminal emulator with truecolor support.\n"
            "Warning: 256-color support not detected. Using a terminal with at least 256-color support is recommended for a better visual experience.\n" + RIPGREP)
STARTUP = TERMINAL + "[STARTUP] Phase 'cleanup_ops' was started but never ended. Skipping metrics.\n"
MISSING = STARTUP + "[STARTUP] Cannot measure phase 'cleanup_ops': start mark 'startup:cleanup_ops:start' not found (likely cleared by reset).\n"
# unicode.IsSpace, which strings.TrimSpace uses. Python's default strip also
# removes U+001C..U+001F; those are not whitespace in the preserved Go grammar.
GO_WHITE_SPACE = "\t\n\v\f\r \u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000"


def phase(stage, context):
    selected = cycle.scope(stage)
    view = context["views"][view_stage(stage)]
    r = view[selected]["recipe"]
    advertised = [view[key]["recipe"] for key in visible(view, selected)]
    operation = "write" if stage in ("installed-user", "installed-a", "installed-b") else (
        "read" if stage in ("restarted-a", "updated-a", "repaired-a") else "none")
    tool = "mcp_" + r["name"] + "_inspect_runtime" if operation != "none" else ""
    wire = "uap_cycle_" + digest((stage + r["nonce"]).encode())[:24] if tool else ""
    return {"stage": stage, "scope": selected, "recipe": r, "advertised": advertised,
            "operation": operation, "tool": tool, "wire": wire, "id": tool + "__" + wire if tool else "",
            "tools": ["mcp_" + item["name"] + "_inspect_runtime" for item in advertised],
            "prompt": "Disposable Gemini fixture stage " + stage + ". Call only the preselected installed inspect_runtime tool when requested.",
            "sentinel": "UAP_GEMINI_DONE_" + stage + "_" + r["nonce"],
            "args": {"nonce": r["nonce"], "operation": operation, "marker": "retained-" + r["nonce"]} if tool else {}}


def number(value, low, high, integer=False):
    require(type(value) in ((int,) if integer else (int, float)) and low <= value <= high, "wire numeric type/bound")


def contents(rows, q):
    require(type(rows) is list and 0 < len(rows) <= 128, "content count")
    parts = []
    for row in rows:
        closed(row, ("parts",), ("role",))
        require("role" not in row or row["role"] in ("user", "model"), "content role")
        require(type(row["parts"]) is list and 0 < len(row["parts"]) <= 256, "part count")
        for part in row["parts"]:
            require(type(part) is dict, "part union")
            if "text" in part:
                closed(part, ("text",))
                require(text(part["text"], 1 << 20), "part text")
            elif "functionCall" in part:
                closed(part, ("functionCall", "thoughtSignature"))
                require(row.get("role") == "model" and part["thoughtSignature"] == "skip_thought_signature_validator"
                        and same(part["functionCall"], {"id": q["wire"], "name": q["tool"], "args": q["args"]}), "wire call identity/signature")
            elif "functionResponse" in part:
                closed(part, ("functionResponse",))
                value = closed(part["functionResponse"], ("id", "name", "response"))
                output = closed(value["response"], ("output",))
                require(value["id"] == q["wire"] and value["name"] == q["tool"]
                        and text(output["output"], 1 << 20), "wire return identity")
            else:
                require(False, "unknown part union")
            parts.append(part)
    return parts


def generation(value):
    bounds = {"temperature": (0, 2), "topP": (0, 1), "topK": (0, 2048), "candidateCount": (1, 8),
              "maxOutputTokens": (1, 1 << 20), "seed": (-(1 << 31), 1 << 31),
              "presencePenalty": (-2, 2), "frequencyPenalty": (-2, 2)}
    closed(value, (), (*bounds, "stopSequences", "responseMimeType", "thinkingConfig"))
    for key, val in value.items():
        if key in bounds:
            number(val, *bounds[key], key in ("topK", "candidateCount", "maxOutputTokens", "seed"))
        elif key == "responseMimeType":
            require(val in ("text/plain", "application/json"), "response MIME")
        elif key == "stopSequences":
            require(type(val) is list and len(val) <= 16 and all(text(v, 256) for v in val), "stop sequence")
        else:
            closed(val, (), ("includeThoughts", "thinkingBudget", "thinkingLevel"))
            if "includeThoughts" in val:
                require(type(val["includeThoughts"]) is bool, "thinking boolean")
            if "thinkingBudget" in val:
                number(val["thinkingBudget"], -1, 32768, True)
            if "thinkingLevel" in val:
                require(val["thinkingLevel"] in ("THINKING_LEVEL_UNSPECIFIED", "LOW", "HIGH"), "thinking level")


def request(body, q):
    value = closed(load(body), ("contents", "tools"), ("generationConfig", "systemInstruction", "safetySettings", "toolConfig", "cachedContent"))
    groups = value["tools"]
    require(type(groups) is list and len(groups) == 1, "declaration group")
    declarations = closed(groups[0], ("functionDeclarations",))["functionDeclarations"]
    require(type(declarations) is list and len(declarations) == len(q["tools"]), "declaration count")
    seen = set()
    for row in declarations:
        closed(row, ("name", "description", "parametersJsonSchema"))
        require(type(row["name"]) is str and row["name"] in q["tools"] and row["name"] not in seen
                and row["description"] == "Inspect only disposable fixture runtime facts.", "owned declaration")
        seen.add(row["name"])
        schema = {"type": "object", "properties": {k: {"type": "string"} for k in ("nonce", "operation", "marker")},
                  "required": ["nonce"], "additionalProperties": False}
        schema["properties"]["wait_for_previous"] = {"type": "boolean", "description": WAIT}
        require(same(row["parametersJsonSchema"], schema), "source-bound argument schema")
    parts = contents(value["contents"], q)
    if "generationConfig" in value:
        generation(value["generationConfig"])
    if "systemInstruction" in value:
        contents([value["systemInstruction"]], q)
    if "cachedContent" in value:
        require(text(value["cachedContent"], 1024), "cached content")
    if "toolConfig" in value:
        config = closed(closed(value["toolConfig"], ("functionCallingConfig",))["functionCallingConfig"], ("mode",), ("allowedFunctionNames",))
        require(config["mode"] in ("MODE_UNSPECIFIED", "AUTO", "ANY", "NONE", "VALIDATED"), "tool mode")
        if "allowedFunctionNames" in config:
            names = config["allowedFunctionNames"]
            require(type(names) is list and all(type(v) is str for v in names)
                    and sorted(names) == sorted(q["tools"]), "allowed tool identities")
    if "safetySettings" in value:
        rows = value["safetySettings"]
        require(type(rows) is list and len(rows) <= 8, "safety count")
        for row in rows:
            closed(row, ("category", "threshold"))
            require(row["category"] in ("HARM_CATEGORY_UNSPECIFIED", "HARM_CATEGORY_HARASSMENT", "HARM_CATEGORY_HATE_SPEECH", "HARM_CATEGORY_SEXUALLY_EXPLICIT", "HARM_CATEGORY_DANGEROUS_CONTENT", "HARM_CATEGORY_CIVIC_INTEGRITY")
                    and row["threshold"] in ("HARM_BLOCK_THRESHOLD_UNSPECIFIED", "BLOCK_LOW_AND_ABOVE", "BLOCK_MEDIUM_AND_ABOVE", "BLOCK_ONLY_HIGH", "BLOCK_NONE", "OFF"), "safety enum")
    return value, parts


def facts(value, r, home):
    closed(value, ("nonce", "revision", "cwd", "argv", "plugin_root", "plugin_data", "literal", "once", "marker", "pid", "session"))
    expected = {"nonce": r["nonce"], "revision": r["revision"], "cwd": r["active"],
                "argv": ["argument with spaces", r["active"] + "/argument", "${UNKNOWN}"], "plugin_root": r["active"],
                "plugin_data": r["data"], "literal": "literal  " + home, "once": r["active"] + "/", "marker": "retained-" + r["nonce"]}
    require(same({k: value[k] for k in expected}, expected) and text(value["session"], 256, True), "held runtime facts")
    natural(value["pid"], (1 << 63) - 1, 1)
    return value


def envelope_tokens(body):
    # Equality only, after request() has admitted the bounded strict JSON and
    # full schema. Match Go json.Number's original token identity without
    # changing numeric authority or prohibiting legitimate float options.
    require(type(body) is bytes and 0 < len(body) <= 1 << 20, "envelope byte bound")
    def number_token(value):
        return ("json-number", value)
    value = json.loads(body.decode("utf-8"), parse_int=number_token, parse_float=number_token)
    return {key: item for key, item in value.items() if key != "contents"}


def provider(rows, q, home):
    reqs, resps, retained = rows
    count = 1 if q["operation"] == "none" else 2
    require(len(reqs) == len(resps) == count, "provider exact count")
    first, first_envelope = None, None
    for index, body in enumerate(reqs):
        value, parts = request(body, q)
        calls = [p for p in parts if "functionCall" in p]
        returns = [p for p in parts if "functionResponse" in p]
        if index == 0:
            require(not calls and not returns, "history before selected call")
            first = value
            first_envelope = envelope_tokens(body)
            last = first["contents"][-1]
            require(last.get("role") == "user" and same(last["parts"][-1], {"text": q["prompt"]}), "exact provider prompt")
        else:
            require(same(envelope_tokens(body), first_envelope)
                    and len(value["contents"]) == len(first["contents"]) + 2
                    and same(value["contents"][:-2], first["contents"]), "whole immutable request history")
            for row, role in zip(value["contents"][-2:], ("model", "user")):
                require(row.get("role") == role and len(row["parts"]) == 1, "closed call/result rows")
            require(len(calls) == len(returns) == 1, "exact history call/result")
            output = returns[0]["functionResponse"]["response"]["output"]
            prefix, suffix = "<untrusted_context>\n", "\n</untrusted_context>"
            require(output.startswith(prefix) and output.endswith(suffix), "return wrapper")
            inner = output[len(prefix):-len(suffix)]
            require("<untrusted_context>" not in inner and "</untrusted_context>" not in inner, "duplicate wrapper")
            q["facts"] = facts(load(inner.encode()), q["recipe"], home)
        raw = resps[index]
        require(raw.startswith(b"data: ") and raw.endswith(b"\n\n"), "whole SSE framing")
        inner = raw[6:-2]
        require(inner == inner.strip() and b"\r" not in inner and b"\n" not in inner, "one SSE row")
        part = {"text": q["sentinel"]}
        if q["operation"] != "none" and index == 0:
            part = {"functionCall": {"id": q["id"], "name": q["tool"], "args": q["args"]}}
        expected = {"candidates": [{"index": 0, "content": {"role": "model", "parts": [part]}, "finishReason": "STOP"}],
                    "usageMetadata": {"promptTokenCount": 1, "candidatesTokenCount": 1, "totalTokenCount": 2}}
        require(same(load(inner), expected), "exact emitted SSE")
    require(same(retained, {"scope": q["scope"], "operation": q["operation"], "tools": q["tools"], "native_call_id": q["id"],
                           "wire_call_id": q["wire"], "prompt": q["prompt"], "sentinel": q["sentinel"], "requests": count,
                           "responses": count, "issued": count == 2, "returned": count == 2, "error": False}), "provider receipt join")


def json_lines(raw):
    require(bool(raw) and len(raw) <= 1 << 20 and raw.endswith(b"\n"), "whole JSONL")
    return [load(line) for line in raw[:-1].split(b"\n")]


def stats(value, call):
    fields = ("total_tokens", "input_tokens", "output_tokens", "cached", "input")
    closed(value, (*fields, "duration_ms", "tool_calls", "models"))
    for key in (*fields, "duration_ms", "tool_calls"):
        natural(value[key], 1 << 40)
    require(type(value["models"]) is dict and set(value["models"]) == {MODEL}
            and value["tool_calls"] == int(call), "exact native model/tool stats")
    row = closed(value["models"][MODEL], fields)
    for val in row.values():
        natural(val, 1 << 40)


def native(raw, q, home, start, end, sessions):
    state, assistant, session, previous = 0, "", "", start
    for row in json_lines(raw):
        previous = window(row["timestamp"], previous, end)
        kind = row["type"]
        if kind == "init":
            closed(row, ("type", "timestamp", "session_id", "model"))
            require(state == 0 and row["model"] == MODEL and text(row["session_id"], 256, True)
                    and row["session_id"] not in sessions, "native initial session/model")
            session, state = row["session_id"], 1
        elif kind == "message":
            closed(row, ("type", "timestamp", "role", "content"), ("delta",))
            require(text(row["content"], 1 << 20) and ("delta" not in row or type(row["delta"]) is bool), "native message shape")
            if row["role"] == "user":
                require(state == 1 and row["content"] == q["prompt"], "native prompt")
                state = 2
            else:
                require(row["role"] == "assistant" and (state in (4, 5) or (state == 2 and q["operation"] == "none")), "native assistant order")
                assistant += row["content"]
                require(len(assistant.encode()) <= len(q["sentinel"].encode()), "native sentinel bound")
                state = 5
        elif kind == "tool_use":
            closed(row, ("type", "timestamp", "tool_name", "tool_id", "parameters"))
            require(state == 2 and q["operation"] != "none" and row["tool_name"] == q["tool"]
                    and row["tool_id"] == q["id"] and same(row["parameters"], q["args"]), "native exact selected call")
            state = 3
        elif kind == "tool_result":
            closed(row, ("type", "timestamp", "tool_id", "status", "output"))
            require(state == 3 and row["tool_id"] == q["id"] and row["status"] == "success"
                    and text(row["output"], 1 << 20), "native exact result")
            require(same(facts(load(row["output"].encode()), q["recipe"], home), q["facts"]), "native/provider facts join")
            state = 4
        elif kind == "result":
            closed(row, ("type", "timestamp", "status"), ("stats",))
            require(state == 5 and assistant == q["sentinel"] and row["status"] == "success", "native terminal closure")
            if "stats" in row:
                stats(row["stats"], q["operation"] != "none")
            state = 6
        else:
            require(False, "unknown native event")
    require(state == 6, "native incomplete session")
    sessions.add(session)
    return session


def catalog(raw, stderr, stage, q, context, paths):
    value = re.sub(r"\x1b\[[0-9;]*m", "", raw.decode("utf-8"))
    require("\0" not in value and "\x1b" not in value, "catalog control")
    lines = [line for line in value.replace("\r\n", "\n").strip(GO_WHITE_SPACE).split("\n") if line.strip(GO_WHITE_SPACE)]
    require(lines and lines[0] == "Discovered Agent Skills:" and (len(lines) - 1) % 3 == 0
            and (len(lines) - 1) // 3 <= 256, "catalog whole framing")
    actual = {}
    for i in range(1, len(lines), 3):
        match = re.fullmatch(r"(\S.*?) \[(Enabled|Disabled)\]( \[Built-in\])?", lines[i])
        require(match is not None and match[1] not in actual and lines[i + 1].startswith("  Description: ")
                and lines[i + 2].startswith("  Location:    "), "catalog exact row")
        actual[match[1]] = (lines[i + 1][15:], lines[i + 2][15:], match[2] == "Enabled", bool(match[3]))
    want = {name: (description, posixpath.dirname(paths["client"]) + "/builtin/" + name + "/SKILL.md", True, True)
            for name, description in DESCRIPTIONS.items()}
    view = context["views"][view_stage(stage)]
    for key in dict.fromkeys(("user", q["scope"])):
        r = view[key]["recipe"]
        want["foreign-" + key] = ("Foreign inert marker", r["profile"] + "/skills/foreign-" + key + "/SKILL.md", False, False)
        if key in visible(view, q["scope"]):
            want[r["name"]] = (r["revision"], r["skill_path"], True, False)
    require(actual == want, "complete pinned/foreign/owned catalog")
    refresh = "Scheduling MCP context refresh...\nExecuting MCP context refresh...\nMCP context refresh complete.\n"
    pairs = ["Registering notification handlers for server '" + r["name"] + "'. Capabilities: { tools: {} }\n"
             "Server '" + r["name"] + "' has tools but did not declare 'listChanged' capability. Listening anyway for robustness...\n" for r in q["advertised"]]
    allowed = ["", RIPGREP, RIPGREP + refresh]
    if pairs:
        allowed.append(RIPGREP + "".join(pairs) + refresh)
    if len(pairs) == 2:
        allowed.append(RIPGREP + "".join(reversed(pairs)) + refresh)
    require(len(stderr) <= 1024 and stderr.decode() in allowed, "finite listing stderr")


def probes(raw, q, selected, home, start, end, sessions, pids):
    require(not raw or raw.endswith(b"\n"), "probe whole framing")
    rows = [load(line) for line in raw[:-1].split(b"\n")] if raw else []
    require(len(rows) <= 6, "probe finite count")
    allowed = {r["nonce"]: r for r in q["advertised"]}
    groups, clocks = {}, {}
    for row in rows:
        closed(row, ("method", "pid", "session", "nonce", "timestamp", "facts"))
        require(type(row["nonce"]) is str and row["nonce"] in allowed, "removed/unselected component")
        nonce, r = row["nonce"], allowed[row["nonce"]]
        pid = natural(row["pid"], (1 << 63) - 1, 1)
        require(text(row["session"], 256, True), "probe session")
        clocks[nonce] = window(row["timestamp"], clocks.get(nonce, start), end)
        if nonce not in groups:
            require(row["session"] not in sessions and pid not in pids, "fresh probe PID/session")
            groups[nonce] = {"pid": pid, "session": row["session"], "methods": []}
            sessions.add(row["session"])
            pids.add(pid)
        group = groups[nonce]
        require(group["pid"] == pid and group["session"] == row["session"], "stable component process")
        chosen = selected and q["operation"] != "none" and nonce == q["recipe"]["nonce"]
        methods = ["initialize", "tools/list"] + (["tools/call"] if chosen else [])
        index = len(group["methods"])
        require(index < len(methods) and row["method"] == methods[index], "probe exact method sequence")
        group["methods"].append(row["method"])
        if row["method"] == "tools/call":
            value = facts(row["facts"], r, home)
            require(same(value, q["facts"]) and value["pid"] == pid and value["session"] == row["session"], "all protocol facts/process joins")
        else:
            require(row["facts"] is None, "noncall facts")
    for nonce in allowed:
        if nonce not in groups:
            require(not selected, "advertised native component absent")
        else:
            expected = 3 if selected and q["operation"] != "none" and nonce == q["recipe"]["nonce"] else 2
            require(len(groups[nonce]["methods"]) == expected, "incomplete component exchange")


def verify(bodies, paths, start, end, context):
    commands = records(bodies["cycle-native-commands.jsonl"], cycle.OBSERVATIONS)
    native_rows = records(bodies["cycle-native-events.jsonl"], cycle.NATIVE)
    discovery = records(bodies["cycle-discovery.jsonl"], cycle.LISTINGS)
    receipts = records(bodies["cycle-provider-receipts.jsonl"], cycle.NATIVE)
    probes_rows = records(bodies["cycle-probe-events.jsonl"], [s.removeprefix("native:") for s in cycle.OBSERVATIONS])
    phases = {stage: phase(stage, context) for stage in cycle.NATIVE}
    streams = {}
    for name in ("cycle-provider-requests.jsonl", "cycle-provider-responses.jsonl"):
        expected = [stage for stage in cycle.NATIVE for _ in range(1 if phases[stage]["operation"] == "none" else 2)]
        rows = records(bodies[name], expected)
        streams[name] = {stage: [] for stage in cycle.NATIVE}
        for row in rows:
            value = closed(row["value"], ("index", "stream"))
            items = streams[name][row["stage"]]
            require(type(value["index"]) is int and value["index"] == len(items), "provider raw index")
            items.append(stream(value["stream"]))
    for row in receipts:
        stage = row["stage"]
        provider((streams["cycle-provider-requests.jsonl"][stage], streams["cycle-provider-responses.jsonl"][stage], row["value"]), phases[stage], paths["home"])
    previous, native_index, listing_index = start, 0, 0
    sessions, probe_sessions, pids = set(), set(), set()
    for index, row in enumerate(commands):
        kind, stage = row["stage"].split(":", 1)
        value = closed(row["value"], ("kind", "scope", "argv", "cwd", "exit", "error", "context_error", "started_at", "finished_at", "elapsed_ms", "cleanup", "stdout", "stderr"))
        selected = kind == "native"
        q = phases[stage] if selected else phase(stage, context)
        begin = window(value["started_at"], previous, end)
        finish = window(value["finished_at"], begin, end)
        previous = finish
        require(value["kind"] == kind and value["scope"] == q["scope"]
                and type(value["elapsed_ms"]) is int and value["elapsed_ms"] == (finish - begin) // 1_000_000, "observation identity/clock")
        argv = [paths["node"], paths["client"], "skills", "list", "--all"]
        if selected:
            argv = [paths["node"], paths["client"], "--prompt", q["prompt"], "--approval-mode", "default", "--output-format", "stream-json", "--model", MODEL]
        cwd = paths["outside" if q["scope"] == "user" else q["scope"]]
        receipt({k: value[k] for k in ("argv", "cwd", "exit", "error", "context_error", "elapsed_ms", "cleanup")}, argv, cwd, 60000 if selected else 10000)
        stdout, stderr = stream(value["stdout"]), stream(value["stderr"])
        if selected:
            event = native_rows[native_index]
            native_index += 1
            ev = closed(event["value"], ("scope", "session", "stream"))
            require(event["stage"] == stage and ev["scope"] == q["scope"] and stream(ev["stream"]) == stdout
                    and stderr.decode() in ("", RIPGREP, TERMINAL, STARTUP, MISSING), "native raw/finite stderr join")
            require(native(stdout, q, paths["home"], begin, finish, sessions) == ev["session"], "retained native session join")
        else:
            event = discovery[listing_index]
            listing_index += 1
            ev = closed(event["value"], ("scope", "stdout", "stderr"))
            require(event["stage"] == stage and ev["scope"] == q["scope"] and stream(ev["stdout"]) == stdout
                    and stream(ev["stderr"]) == stderr, "catalog raw duplicate join")
            catalog(stdout, stderr, stage, q, context, paths)
        probe = probes_rows[index]
        ev = closed(probe["value"], ("scope", "stream"))
        require(ev["scope"] == q["scope"], "probe scope")
        probes(stream(ev["stream"]), q, selected, paths["home"], begin, finish, probe_sessions, pids)

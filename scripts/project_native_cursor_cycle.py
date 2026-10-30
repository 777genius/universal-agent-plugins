"""Closed Cursor MCP lifecycle bytes; this reader opens no files or children."""
import base64
import hashlib
import json
from pathlib import PurePosixPath
import re

MODE = "cursor-complete-mcp-cycle"
MEMBERS = ("cursor-cycle-commands.jsonl", "cursor-cycle-proofs.jsonl", "cursor-cycle-probe.jsonl")
STAGES = tuple("git-a git-b version empty-a empty-b empty-outside add-a approve-a-one installed-a add-b approve-b-one installed-b installed-outside repeat-add-a unchanged-update-a intact-repair-a stable-a stable-b update-a approve-a-two updated-a updated-b updated-outside damaged-a missing-repair-a repaired-a remove-a removed-a remaining-b removed-outside repeat-remove-a remove-b final-a final-b final-outside".split())
PROOFS = tuple("source-a-1.0.0 source-b-1.0.0 original-controls add-a approve-a-one add-b approve-b-one repeat-add-a unchanged-update-a intact-repair-a source-a-1.1.0 update-a approve-a-two missing-native-a missing-repair-a remove-a repeat-remove-a remove-b".split())
TOOLS = frozenset("installed-a installed-b stable-a stable-b updated-a updated-b repaired-a remaining-b".split())
PUBLIC = frozenset("add-a add-b repeat-add-a unchanged-update-a intact-repair-a update-a missing-repair-a remove-a repeat-remove-a remove-b".split())
CHANGED = PUBLIC - {"repeat-add-a", "unchanged-update-a", "repeat-remove-a"}
FLAGS = tuple("project-lifecycle native-restart-removal mcp-initialize mcp-tools-list mcp-tools-call".split())
EMPTY = b"No MCP servers configured (expected in .cursor/mcp.json or ~/.cursor/mcp.json)\n"
TOOL_TEXT = b"Tools for uapcursora (1):\n- inspect_runtime (marker, nonce, operation)\n"
ENABLE_TEXT = "✓ Enabled and approved MCP server: uapcursora\n".encode()
REFUSAL = b'agentplugins: plugin is not installed for target "cursor" in project scope; no target was changed\n'
CLEANUP = "full descendant supervision; forced cleanup fails"
SCOPE = "MCP-only: two project installations, fresh initialize/tools-list sessions; no tools-call, skills, runtime cwd/argv facts, vendor authentication or model"


def require(condition):
    if not condition:
        raise ValueError("Cursor complete MCP lifecycle evidence differs")


def obj(value, required, optional=""):
    require(type(value) is dict and set(required.split()) <= set(value) <= set((required + " " + optional).split()))


def load(raw):
    from project_native_runtime import strict_json
    return strict_json(raw)


def private(raw):
    from project_native_runtime import cline_cycle_private
    return cline_cycle_private(raw, b"uap-cursor-private-")


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def compact(value):
    return json.dumps(value, sort_keys=True, separators=(",", ":"), ensure_ascii=False).encode()


def same(a, b):
    if type(a) is not type(b):
        return False
    if type(a) is dict:
        return set(a) == set(b) and all(same(a[k], v) for k, v in b.items())
    if type(a) is list:
        return len(a) == len(b) and all(same(x, y) for x, y in zip(a, b))
    return a == b


def canonical(path):
    require(type(path) is str and path.startswith("/") and path != "/" and str(PurePosixPath(path)) == path and ".." not in PurePosixPath(path).parts and "\0" not in path)
    return path


def stream(value):
    obj(value, "bytes sha256 body")
    require(type(value["bytes"]) is int and 0 <= value["bytes"] <= 1 << 20 and type(value["body"]) is str
            and len(value["body"]) <= ((1 << 20) + 2) // 3 * 4 and type(value["sha256"]) is str)
    raw = base64.b64decode(value["body"], validate=True)
    require(len(raw) == value["bytes"] and sha(raw) == value["sha256"] and base64.b64encode(raw).decode() == value["body"] and not private(raw))
    raw.decode("utf-8", errors="strict")
    return raw


def embedded(value):
    if type(value) is dict:
        if "body" in value:
            stream(value)
        else:
            for item in value.values():
                embedded(item)
    elif type(value) is list:
        for item in value:
            embedded(item)


def records(raw, order):
    require(type(raw) is bytes and len(raw) <= 1 << 20 and (not raw or raw.endswith(b"\n")) and not private(raw))
    rows = [load(line) for line in raw.splitlines()]
    require(len(rows) <= len(order))
    for row, stage in zip(rows, order):
        obj(row, "stage value")
        require(row["stage"] == stage and type(row["value"]) is dict)
        if order == PROOFS and stage in ("approve-a-one", "approve-b-one", "approve-a-two"):
            obj(row["value"], "scope path body fingerprints probe_bytes probe_sha256")
            stream(row["value"]["body"])
            for key, item in row["value"].items():
                if key != "body":
                    embedded(item)
        else:
            embedded(row["value"])
    return rows


def stamp(value):
    from project_native_copilot_events import instant
    require(type(value) is str and re.fullmatch(r"\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d(?:\.\d{1,9})?Z", value))
    return instant(value)


def command(value, ordinary):
    obj(value, "argv cwd exit error deadline elapsed_ms cleanup started_at ended_at stdout stderr")
    require(type(value["argv"]) is list and value["argv"] and all(type(a) is str and "\0" not in a for a in value["argv"])
            and type(value["exit"]) is int and type(value["error"]) is bool and type(value["deadline"]) is bool
            and type(value["elapsed_ms"]) is int and 0 <= value["elapsed_ms"] <= 35000 and value["cleanup"] == CLEANUP)
    canonical(value["cwd"])
    start, end = stamp(value["started_at"]), stamp(value["ended_at"])
    duration = end-start
    require(0 <= duration <= 35 * 10**9 and duration // 10**6 == value["elapsed_ms"])
    if ordinary:
        require(value["error"] is False and value["deadline"] is False)
    return stream(value["stdout"]), stream(value["stderr"]), start, end


def probe_delta(raw, nonce, start, end, seen):
    require(raw and raw.endswith(b"\n") and len(raw) <= 1 << 20)
    sessions, pids, last = {}, {}, start
    for line in raw.splitlines():
        row = load(line)
        obj(row, "method nonce pid session timestamp facts")
        require(row["nonce"] == nonce and row["facts"] is None and type(row["pid"]) is int and row["pid"] > 0
                and type(row["session"]) is str and re.fullmatch(str(row["pid"]) + r"-[1-9][0-9]*", row["session"]) and row["session"] not in seen)
        require(int(row["session"].split("-")[1]) < 1 << 64)
        when = stamp(row["timestamp"])
        require(last <= when <= end)
        last = when
        session = row["session"]
        if row["method"] == "initialize":
            require(session not in sessions and row["pid"] not in pids)
            sessions[session], pids[row["pid"]] = [row["pid"], 0], session
        else:
            require(row["method"] == "tools/list" and session in sessions and sessions[session][0] == row["pid"])
            sessions[session][1] += 1
    require(sessions and all(value[1] >= 1 for value in sessions.values()))
    return set(sessions)


def pins(result, paths):
    identity = result["acquisition"]
    obj(identity, "client acquisition runtime_files")
    require(identity["client"] == "cursor")
    acq = identity["acquisition"]
    obj(acq, "bytes installer_executed integrity members pin_verified resolved_origin sha256 url")
    url = "https://downloads.cursor.com/lab/2026.10.22-27dcf5f/linux/x64/agent-cli-package.tar.gz"
    require(acq["bytes"] == 184859176 and type(acq["bytes"]) is int and acq["installer_executed"] is False and acq["pin_verified"] is True
            and acq["sha256"] == "85296944e4600a83c4b764f578d1502bf9c416db200d33a593e8f3ecabfdc294"
            and acq["url"] == url and acq["resolved_origin"] == "https://downloads.cursor.com"
            and acq["integrity"] == "sha256:" + acq["sha256"])
    rows = acq["members"]
    require(type(rows) is list and len(rows) == 565 and sha(compact(rows)) == "0646a2eff536987e17b914607380879094058ac4f8c6b54d011cf5fe884d7b97")
    files = {row["name"]: row["sha256"] for row in rows if row["kind"] == "file"}
    root = str(PurePosixPath(paths["client"]).parent.parent)
    require(paths["client"] == root + "/dist-package/cursor-agent" and len(files) == 445
            and same(identity["runtime_files"], [{"path": root + "/" + row["name"], "sha256": row["sha256"]} for row in rows if row["kind"] == "file"]))
    require(same(result["scanner"], {"archive_sha256": "2b3d176db752433b904a4b42375543ff398f4841d22e48f7d4f23ded925b72da", "binary_sha256": "b6723451f155eb49e84acb34d1f06de929bddfcff6238cd4e38cdc6519396fba", "platform": "linux-amd64", "route": "prepopulated supported scanner cache; ordinary CLI security assessment", "source_tag": "v0.1.3", "version": "0.1.3"}))


def argv(stage, paths, proofs):
    scope = "b" if stage.endswith("-b") or stage == "approve-b-one" else "a"
    cwd = paths["outside"] if "outside" in stage or stage == "version" else paths[scope]
    if stage.startswith("git-"):
        return [paths["git"], "init", "--quiet", "--initial-branch=main"], cwd
    if stage == "version":
        return [paths["client"], "--version"], cwd
    if stage in PUBLIC:
        verb = "repair" if "repair" in stage else "update" if "update" in stage else "remove" if "remove" in stage else "add"
        selector = paths["source_" + scope] if verb == "add" else proofs["add-" + scope]["installation"]["installation_id"]
        return [paths["installer"], verb, selector] + (["--no-save"] if verb == "add" else []) + ["--scope", "project", "--project-root", cwd, "--target", "cursor", "--format", "json"], cwd
    verb = "list-tools" if stage in TOOLS else "enable" if stage.startswith("approve-") else "list"
    return [paths["client"], "mcp", verb] + (["uapcursora"] if verb != "list" else []), cwd


def progress(stage):
    if stage in ("add-a", "add-b"):
        return b"Resolving and validating one Agent Plugin package for every selected target...\nInstalling native-proof for 1 selected clients...\n"
    if stage == "repeat-add-a":
        return b"Resolving and validating one Agent Plugin package for every selected target...\nApplying transactional client package...\n"
    if "update" in stage:
        return b"Resolving and validating one updated Agent Plugin package for every selected target...\n"
    if "repair" in stage:
        return b"Resolving and validating each unique exact installed package revision once...\n"
    return b""


def complete(commands, proof_rows, probes, result):
    from project_native_cursor_cycle_proof import validate, public_result
    paths, proofs = validate(proof_rows, result, commands)
    pins(result, paths)
    prefix, sessions, elapsed, previous, groups = b"", set(), 0, None, set()
    first = None
    for frame, probe in zip(commands, probes):
        stage, value = frame["stage"], frame["value"]
        out, err, start, end = command(value, True)
        if first is None:
            first = start
        require(previous is None or previous <= start)
        previous, elapsed = end, elapsed + value["elapsed_ms"]
        expected, cwd = argv(stage, paths, proofs)
        require(value["argv"] == expected and value["cwd"] == cwd and value["exit"] == (1 if stage == "repeat-remove-a" else 0))
        if stage in PUBLIC:
            if stage == "repeat-remove-a":
                require(out == b"" and err == REFUSAL)
            else:
                require(err == progress(stage))
                group = public_result(out, stage, proofs[stage])
                if group:
                    require(group not in groups)
                    groups.add(group)
        else:
            require(err == b"")
            expected_out = b"" if stage.startswith("git-") else b"2026.10.22-27dcf5f\n" if stage == "version" else TOOL_TEXT if stage in TOOLS else ENABLE_TEXT if stage.startswith("approve-") else EMPTY
            require(out == expected_out)
        require(probe["stage"] == stage)
        value = probe["value"]
        obj(value, "before_bytes before_sha256 after_bytes after_sha256 delta")
        require(type(value["before_bytes"]) is int and value["before_bytes"] == len(prefix) and value["before_sha256"] == sha(prefix))
        delta = stream(value["delta"])
        if stage in TOOLS:
            selected = "add-b" if stage.endswith("-b") else "update-a" if stage in ("updated-a", "repaired-a") else "add-a"
            sessions |= probe_delta(delta, proofs[selected]["nonce"], start, end, sessions)
        else:
            require(delta == b"")
        prefix += delta
        require(len(prefix) <= 1 << 20 and type(value["after_bytes"]) is int and value["after_bytes"] == len(prefix) and value["after_sha256"] == sha(prefix))
        if stage.startswith("approve-"):
            p = proofs[stage]
            require(p["probe_bytes"] == len(prefix) and p["probe_sha256"] == sha(prefix))
    require(first is not None and previous - first < 150 * 10**9 and previous - first < (result["elapsed_ms"] + 1) * 10**6
            and elapsed <= result["elapsed_ms"] and same(result["cycle_probe"], {"bytes": len(prefix), "sha256": sha(prefix), "sessions": len(sessions)}))


def members(bodies, source):
    try:
        return admit(bodies, source)
    except (KeyError, TypeError, AttributeError, IndexError, RecursionError, OverflowError, UnicodeError) as error:
        raise ValueError("Cursor complete MCP structure refused") from error


def admit(bodies, source):
    require(set(bodies) == {"runtime-attempt.json", *MEMBERS} and all(type(raw) is bytes and len(raw) <= 1 << 20 and not private(raw) for raw in bodies.values()) and sum(map(len, bodies.values())) <= 16 << 20)
    result = load(bodies["runtime-attempt.json"])
    obj(result, "acquisition activation client elapsed_ms qualification real_model_qualified runtime_attempt_complete scanner source_identity test_failed vendor_authentication_qualified version capture_mode cycle_command_stages cycle_proof_stages cycle_observed_complete cycle_probe qualification_scope", "cycle_layout cycle_stage")
    require(result["client"] == "cursor" and result["version"] == "2026.10.22-27dcf5f" and result["capture_mode"] == MODE and result["activation"] == "prepared"
            and result["vendor_authentication_qualified"] is False and result["real_model_qualified"] is False and result["qualification_scope"] == SCOPE)
    require(same(result["source_identity"], {"installer_base_commit": source["source_commit"], "installer_tree": source["source_tree"], "installer_source_state": "committed", "installer_patch_sha256": []}))
    done = result["runtime_attempt_complete"]
    require(type(done) is bool and type(result["test_failed"]) is bool and type(result["cycle_observed_complete"]) is bool and type(result["elapsed_ms"]) is int and 0 <= result["elapsed_ms"] <= 180000
            and same(result["qualification"], {flag: done and flag != "mcp-tools-call" for flag in FLAGS}))
    commands, proofs, probes = [records(bodies[name], order) for name, order in zip(MEMBERS, (STAGES, PROOFS, STAGES))]
    require(same(result["cycle_command_stages"], [r["stage"] for r in commands]) and same(result["cycle_proof_stages"], [r["stage"] for r in proofs]) and len(probes) <= len(commands))
    for row in commands:
        command(row["value"], False)
    for row in probes:
        obj(row["value"], "before_bytes before_sha256 after_bytes after_sha256 delta")
    if not done:
        require(result["test_failed"] is True and result["cycle_observed_complete"] is False)
        return result
    require(result["test_failed"] is False and result["cycle_observed_complete"] is True and result["elapsed_ms"] < 150000 and len(commands) == len(probes) == len(STAGES) and len(proofs) == len(PROOFS))
    complete(commands, proofs, probes, result)
    return result

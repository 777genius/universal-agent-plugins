"""Exact Kiro 2.34 authenticated fixture receipts; no files, processes, or credentials."""
import copy
import datetime
import hashlib
import json
from pathlib import PurePosixPath
import re

MODE = "kiro-authenticated-project-mcp-cycle"
NAME = "uap-kiro-attempt"
BASE = ("git-a", "git-b", "version", "add-a")
STEPS = ("chat-help", "agent-help", "add-b", "validate-a", "validate-b",
         "native-write-a", "native-write-b", "repair-a", "native-restart-a",
         "remove-a", "native-removed-a", "native-remaining-b", "remove-b", "native-removed-b")
PROOFS = ("add-a", "add-b", "repair-a", "remove-a", "remove-b")
POSITIVE = frozenset(("native-write-a", "native-write-b", "native-restart-a", "native-remaining-b"))
FLAGS = ("project-lifecycle", "native-restart-removal", "mcp-initialize", "mcp-tools-list", "mcp-tools-call")
CLEANUP = "full descendant supervision; forced cleanup fails"
PIN = "11dce9286cf7401079cf4eac02a3d5982daac16a346d880109ec7b4a0b941040"


def require(value):
    if not value:
        raise ValueError("Kiro authenticated native lifecycle receipt differs")


def load(raw):
    from project_native_runtime import strict_json
    return strict_json(raw)


def obj(value, required, optional=""):
    require(type(value) is dict and set(required.split()) <= set(value)
            <= set((required + " " + optional).split()))


def same(left, right):
    if type(left) is not type(right):
        return False
    if type(left) is dict:
        return set(left) == set(right) and all(same(left[key], right[key]) for key in left)
    if type(left) is list:
        return len(left) == len(right) and all(same(a, b) for a, b in zip(left, right))
    return left == right


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def canonical(value):
    require(type(value) is str and len(value) <= 4096 and value.startswith("/")
            and value != "/" and str(PurePosixPath(value)) == value
            and ".." not in PurePosixPath(value).parts and "\0" not in value)
    return value


def instant(value):
    require(type(value) is str)
    match = re.fullmatch(r"(\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d)(?:\.(\d{1,9}))?Z", value)
    require(match is not None)
    when = datetime.datetime.strptime(match[1], "%Y-%m-%dT%H:%M:%S").replace(tzinfo=datetime.timezone.utc)
    return int(when.timestamp()) * 1_000_000_000 + int((match[2] or "").ljust(9, "0"))


def agent(key):
    selector = "@" + NAME + "/inspect_runtime"
    return {"name": "uap-kiro-native-" + key, "description": "Disposable UAP native MCP fixture",
            "prompt": "Use only the named disposable MCP inspect_runtime tool, exactly as the user requests. Never run shell, file, web, or subagent tools.",
            "tools": [selector], "allowedTools": [selector], "mcpServers": {},
            "resources": [], "hooks": {}, "toolAliases": {}}


def source_identity(result, source):
    require(same(result["source_identity"], {
        "installer_base_commit": source["source_commit"], "installer_tree": source["source_tree"],
        "installer_source_state": "committed", "installer_patch_sha256": []}))
    parent = source["client_identity"]
    require(parent["client"] == "kiro" and parent["acquisition"]["sha256"] == PIN
            and parent["acquisition"]["pin_verified"] is True)
    members = parent["acquisition"]["members"]
    require(type(members) is list and 1 <= len(members) <= 4096)
    projection = {"client": "kiro", "acquisition": {"sha256": PIN, "pin_verified": True, "members": [
        {"Name": row["name"], "Kind": row["kind"], "Bytes": row.get("bytes", 0),
         "sha256": row.get("sha256", "")} for row in members]},
        "node": None, "runtime_files": parent["runtime_files"]}
    require(same(result["acquisition"], projection))
    scanner = result["scanner"]
    obj(scanner, "version source_tag binary_sha256 archive_sha256 platform route")
    require(scanner["version"] == "0.1.3" and scanner["source_tag"] == "v0.1.3"
            and scanner["platform"] == "linux-amd64"
            and scanner["archive_sha256"] == source["scanner_archive_sha256"]
            and re.fullmatch("[0-9a-f]{64}", scanner["binary_sha256"]) is not None
            and scanner["route"] == "prepopulated supported scanner cache; ordinary CLI security assessment")


def config(value, name, removed):
    obj(value, "mcpServers")
    servers = value["mcpServers"]
    require(type(servers) is dict and set(servers) == (set() if removed else {name}))
    if removed:
        return None
    row = servers[name]
    obj(row, "command args env")
    return row


def role_proof(row, info, previous):
    obj(row, "stage role project state_json state_sha256 config_json config_sha256 data removed binding")
    key, stage = row["role"], row["stage"]
    require(key in ("a", "b") and stage in PROOFS and stage.endswith("-" + key))
    project = info["projects"][key]
    require(row["project"] == project and type(row["removed"]) is bool
            and row["removed"] == stage.startswith("remove-"))
    for name in ("state", "config"):
        body = row[name + "_json"]
        require(type(body) is str and len(body.encode()) <= 1 << 20
                and sha(body.encode()) == row[name + "_sha256"])
    state = load(row["state_json"].encode())
    obj(state, "schema_version installations", "transaction_receipts environment_receipts")
    require(type(state["schema_version"]) is int and state["schema_version"] == 5
            and type(state["installations"]) is list and len(state["installations"]) == 1
            and state.get("environment_receipts", []) == [])
    installed = state["installations"][0]
    obj(installed, "installation_id declared_name source package clients created_at updated_at",
        "local_chatgpt_mapping install_preferences origin_mode directory operation_group_id data_receipts data_retained needs_rebind")
    require(installed["declared_name"] == NAME and type(installed["installation_id"]) is str
            and bool(installed["installation_id"]) and type(installed["clients"]) is dict)
    source = installed["source"]
    require(type(source) is dict and source["canonical_source"] == info["source"]
            and bool(source["source_binding_id"]) and bool(source["tree_digest"]))
    package = installed["package"]
    require(type(package) is dict and package["declared_name"] == NAME
            and package["version"] == "1.0.0" and bool(package["manifest_digest"]))
    binding = row["binding"]
    require(type(binding) is dict and binding["client_id"] == "kiro" and binding["scope"] == "project"
            and binding["activation"] == "prepared" and binding["verification"] == "installed"
            and binding["native_profile_root"] == project + "/.kiro")
    artifact = binding["physical_artifact_id"]
    require(type(artifact) is str and artifact and "/" not in artifact and "\\" not in artifact and artifact not in (".", ".."))
    active = project + "/.uap/managed/clients/kiro/" + artifact
    require(binding["target_locator"] == active)
    data = installed.get("data_receipts")
    require(type(data) is dict and set(data) == {binding["data_receipt_id"]})
    allocation = data[binding["data_receipt_id"]]
    require(allocation["data_receipt_id"] == binding["data_receipt_id"] and allocation["physical_backend_id"] == artifact
            and allocation["scope"] == "project" and allocation["state"] == "owned"
            and bool(allocation["ownership_digest"]) and allocation["locator"] == row["data"])
    require(canonical(row["data"]).startswith(project + "/.uap/plugin-data/"))
    entry = config(load(row["config_json"].encode()), NAME, row["removed"])
    if row["removed"]:
        require(not installed["clients"] and installed.get("data_retained") is True)
    else:
        require(len(installed["clients"]) == 1 and any(same(value, binding) for value in installed["clients"].values()))
        expected = {"command": active + "/io.github.777genius.agentplugins/managed-stdio-v1/agentplugins",
                    "args": ["--internal-stdio-v1", active, row["data"], active, "plugin", "--", "./bin/probe", "Owned kiro", active + "/argument"],
                    "env": {"PLUGIN_ROOT": active, "PLUGIN_DATA": row["data"], "UAP_TEST_ROOT": info["root"],
                            "UAP_TEST_EVENTS": info["root"] + "/events.jsonl", "UAP_TEST_NONCE": info["nonce"],
                            "UAP_TEST_REVISION": "a-one"}}
        require(same(entry, expected))
    if previous is not None:
        expected = copy.deepcopy(previous["installation"])
        expected["updated_at"] = installed["updated_at"]
        if "operation_group_id" in installed:
            expected["operation_group_id"] = installed["operation_group_id"]
        if row["removed"]:
            expected["clients"], expected["data_retained"] = {}, True
        else:
            require(set(expected["clients"]) == set(installed["clients"]))
            for client, old in expected["clients"].items():
                now = installed["clients"][client]
                receipts, old_receipts = now.get("receipts", []), old.get("receipts", [])
                require(same(receipts[:len(old_receipts)], old_receipts))
                old["updated_at"] = now["updated_at"]
                if "receipts" in now:
                    old["receipts"] = receipts
        require(same(expected, installed) and row["data"] == previous["data"])
        if row["removed"]:
            require(same(binding, previous["binding"]))
    return {"installation": installed, "binding": binding, "data": row["data"], "removed": row["removed"]}


def arguments(label, info, paths, roles):
    key = "b" if label.endswith("-b") else "a"
    project, name = info["projects"][key], info["name"]
    if label in ("git-a", "git-b"):
        return project, [paths["git"], "-c", "core.hooksPath=/dev/null", "init", "--quiet"]
    if label == "version":
        return info["root"] + "/outside", [paths["client"], "--version"]
    if label in ("chat-help", "agent-help"):
        return info["root"] + "/outside", [paths["client"], *({"chat-help": ["chat", "--help"], "agent-help": ["agent", "validate", "--help"]}[label])]
    if label.startswith("validate-"):
        return project, [paths["client"], "agent", "validate", "--path", project + "/.kiro/agents/uap-kiro-native-" + key + ".json"]
    if label in ("add-a", "add-b"):
        return project, [paths["installer"], "add", info["source"], "--no-save", "--scope", "project", "--project-root", project, "--target", "kiro", "--format", "json"]
    if label in ("repair-a", "remove-a", "remove-b"):
        return project, [paths["installer"], label.split("-")[0], roles[key]["installation"]["installation_id"],
                         "--scope", "project", "--project-root", project, "--target", "kiro", "--format", "json", "--yes"]
    operation = "write" if label.startswith("native-write-") else "read"
    marker = "kiro-" + key + "-" + info["nonce"]
    prompt = "Call @" + name + "/inspect_runtime exactly once with nonce " + info["nonce"] + ", operation " + operation + " and marker " + marker + ". Do not use any other tool."
    if label.startswith("native-removed-"):
        prompt = "Call @" + name + "/inspect_runtime only if that tool is available, with nonce " + info["nonce"] + " and operation read. If unavailable, finish without using any tool. Do not use any other tool."
    return project, [paths["client"], "chat", "--v2", "--no-interactive", "--agent", "uap-kiro-native-" + key,
                     "--trust-tools", "@" + name + "/inspect_runtime", prompt]



def public_json(raw, verb):
    value = load(raw)
    obj(value, "schema_version command result data")
    require(type(value["schema_version"]) is int and value["schema_version"] == 1
            and value["command"] == verb and value["result"] == "success"
            and type(value["data"]) is dict)


def event_delta(raw, info, role, command, seen, positive):
    if not positive:
        require(raw == b"")
        return
    require(raw and raw.endswith(b"\n") and len(raw) <= 1 << 20)
    lines = raw[:-1].split(b"\n")
    require(3 <= len(lines) <= 6)
    rows = [load(line) for line in lines]
    obj(rows[0], "method pid session nonce timestamp facts")
    session, pid = rows[0]["session"], rows[0]["pid"]
    require(type(pid) is int and 0 < pid < 1 << 31 and type(session) is str
            and re.fullmatch(str(pid) + r"-[0-9]{1,20}", session) and session not in seen)
    seen.add(session)
    start, end, last = instant(command["started_at"]), instant(command["ended_at"]), instant(command["started_at"])
    birth = int(session.split("-")[1])
    require(start <= birth <= end)
    for index, row in enumerate(rows):
        obj(row, "method pid session nonce timestamp facts")
        method = "initialize" if index == 0 else "tools/call" if index == len(rows)-1 else "tools/list"
        when = instant(row["timestamp"])
        require(row["method"] == method and type(row["pid"]) is int and row["pid"] == pid
                and row["session"] == session and row["nonce"] == info["nonce"] and last <= when <= end and birth <= when)
        last = when
        if method != "tools/call":
            require(row["facts"] is None)
            continue
        key = "b" if command["cwd"] == info["projects"]["b"] else "a"
        active = role["binding"]["target_locator"]
        expected = {"nonce": info["nonce"], "revision": "a-one", "cwd": active,
                    "argv": ["Owned kiro", active + "/argument"], "plugin_root": active,
                    "plugin_data": role["data"], "literal": "", "once": "",
                    "marker": "kiro-" + key + "-" + info["nonce"], "pid": pid, "session": session}
        require(same(row["facts"], expected))


def members(bodies, source):
    require(type(bodies) is dict and "runtime-attempt.json" in bodies and "kiro-cycle-proof.json" in bodies)
    result = load(bodies["runtime-attempt.json"])
    obj(result, "client version runtime_attempt_complete activation vendor_authentication_qualified real_model_qualified qualification source_identity acquisition scanner test_failed elapsed_ms capture_mode kiro_cycle",
        "attempt_stage installation_source binding installed_skill installed_mcp data_locator")
    complete = result["runtime_attempt_complete"]
    require(type(complete) is bool and result["client"] == "kiro" and result["version"] == "2.34.0"
            and result["capture_mode"] == MODE and result["activation"] == "prepared"
            and result["test_failed"] is (not complete)
            and result["vendor_authentication_qualified"] is complete and result["real_model_qualified"] is complete
            and same(result["qualification"], {name: complete for name in FLAGS})
            and type(result["elapsed_ms"]) is int and 0 <= result["elapsed_ms"] <= 180000)
    source_identity(result, source)
    info = result["kiro_cycle"]
    obj(info, "steps complete positive_sessions nonce root projects home source name")
    root = canonical(info["root"])
    require(info["complete"] is complete and re.fullmatch("[0-9a-f]{32}", info["nonce"])
            and same(info["projects"], {"a": root + "/project", "b": root + "/project-b"})
            and info["home"] == root + "/home" and info["source"] == root + "/package"
            and info["name"] in ("", NAME) and type(info["steps"]) is list
            and info["steps"] == list(STEPS[:len(info["steps"])])
            and type(info["positive_sessions"]) is int and 0 <= info["positive_sessions"] <= 4)
    proofs = load(bodies["kiro-cycle-proof.json"])
    require(type(proofs) is list and len(proofs) <= len(PROOFS))
    roles, by_stage = {}, {}
    for index, row in enumerate(proofs):
        require(type(row) is dict and row.get("stage") == PROOFS[index])
        key = row["role"]
        roles[key] = role_proof(row, info, roles.get(key))
        by_stage[row["stage"]] = roles[key]
    if "a" in roles and "b" in roles:
        require(roles["a"]["data"] != roles["b"]["data"]
                and roles["a"]["binding"]["target_locator"] != roles["b"]["binding"]["target_locator"])
    allowed = {"runtime-attempt.json", "kiro-cycle-proof.json", "probe-events.jsonl"}
    for key in ("a", "b"):
        member = "kiro-agent-" + key + ".json"
        allowed.add(member)
        if member in bodies:
            require(same(load(bodies[member]), agent(key)))
    commands, order = {}, BASE + STEPS
    for label in order:
        triple = {label + suffix for suffix in (".command.json", ".stdout.log", ".stderr.log")}
        allowed |= triple
        require(not set(bodies) & triple or triple <= set(bodies))
        if triple <= set(bodies):
            row = load(bodies[label + ".command.json"])
            extra = "" if label in BASE else "started_at ended_at event_offset"
            obj(row, "argv cwd exit error deadline elapsed_ms cleanup " + extra)
            require(type(row["argv"]) is list and row["argv"] and all(type(x) is str for x in row["argv"])
                    and type(row["cwd"]) is str and type(row["exit"]) is int and type(row["error"]) is bool
                    and type(row["deadline"]) is bool and type(row["elapsed_ms"]) is int
                    and 0 <= row["elapsed_ms"] <= 35000 and row["cleanup"] == CLEANUP)
            if label not in BASE:
                require(instant(row["started_at"]) <= instant(row["ended_at"])
                        and type(row["event_offset"]) is int and row["event_offset"] >= 0)
            commands[label] = row
    require(not set(bodies) - allowed and list(commands) == list(order[:len(commands)]))
    require(info["steps"] == [label for label in commands if label not in BASE])
    if not complete:
        return result
    require(list(commands) == list(order) and len(proofs) == 5 and info["positive_sessions"] == 4
            and info["name"] == NAME and all(role["removed"] for role in roles.values())
            and all("kiro-agent-" + key + ".json" in bodies for key in ("a", "b")))
    paths = {"git": commands["git-a"]["argv"][0], "client": commands["version"]["argv"][0],
             "installer": commands["add-a"]["argv"][0]}
    for path in paths.values():
        canonical(path)
    runtime_files = {row["path"]: row["sha256"] for row in source["client_identity"]["runtime_files"]}
    require(paths["client"].endswith("/kirocli/bin/kiro-cli")
            and runtime_files.get(paths["client"]) == "94cae7e349f76869a19eae335a5b392fba8515743b1c2822b9ea33128c067cc6")
    events = bodies.get("probe-events.jsonl", b"")
    seen, offset = set(), 0
    for index, label in enumerate(order):
        command = commands[label]
        expected_cwd, expected_argv = arguments(label, info, paths, roles)
        require(command["cwd"] == expected_cwd and same(command["argv"], expected_argv)
                and command["exit"] == 0 and command["error"] is False and command["deadline"] is False)
        if label in BASE:
            continue
        require(command["event_offset"] == offset)
        next_offset = commands[order[index+1]]["event_offset"] if index + 1 < len(order) else len(events)
        require(offset <= next_offset <= len(events))
        if label.startswith("native-"):
            key = "b" if label.endswith("-b") else "a"
            event_delta(events[offset:next_offset], info, roles[key], command, seen, label in POSITIVE)
        else:
            require(next_offset == offset)
        offset = next_offset
    require(offset == len(events) and len(seen) == 4)
    for label in PROOFS:
        public_json(bodies[label + ".stdout.log"], label.split("-")[0])
    require(bodies["version.stdout.log"] == b"kiro-cli 2.34.0\n" and bodies["version.stderr.log"] == b"")
    for token in (b"--v2", b"--no-interactive", b"--agent", b"--trust-tools"):
        require(token in bodies["chat-help.stdout.log"])
    require(b"--path" in bodies["agent-help.stdout.log"])
    return result

"""Failed-only direct post-add Copilot list; raw listing grants no authority."""
from project_native_copilot_events import natural, obj, require, same, values
from project_native_copilot_user import FLAGS, FIELDS, authority, config_facts, metadata

MODE = "copilot-post-add-registry-first-capture"
COMMANDS = ("git-A", "git-B", "version", "add-user", "native-list")
USER_META = "user-profile-metadata.json"
LIST_META = "native-list-profile-metadata.json"
VERSION = b"GitHub Copilot CLI 1.0.103.\nRun 'copilot update' to check for updates.\n"
STAGES = {"prepare": (range(4), 0, False), "before-user": ((3,), 1, False),
          "public-user": ((3, 4), 1, False), "public-result": ((4,), 1, False),
          "after-user-metadata": ((4,), 1, True), "before-list": ((4,), 2, True),
          "native-list": ((4, 5), 2, True), "after-list-metadata": ((5,), 2, True),
          "review-barrier": ((5,), 2, True)}


def public(body):
    from project_native_runtime import strict_json
    row = obj(strict_json(body), "schema_version command result data")
    require(type(row["schema_version"]) is int and row["schema_version"] == 1
            and row["command"] == "add" and row["result"] == "success" and type(row["data"]) is dict)


def commands(bodies, result, layout):
    from project_native_runtime import strict_json
    prefix = result["commands"]
    require(type(prefix) is list and len(prefix) <= 5 and prefix == list(COMMANDS[:len(prefix)]))
    expected = {
        "git-A": (layout["a"], [layout["git"], "-c", "core.hooksPath=/dev/null", "init", "--quiet"]),
        "git-B": (layout["b"], [layout["git"], "-c", "core.hooksPath=/dev/null", "init", "--quiet"]),
        "version": (layout["outside"], [layout["client"], "--version"]),
        "add-user": (layout["outside"], [layout["installer"], "add", layout["package"], "--no-save", "--scope", "user", "--target", "copilot", "--format", "json"]),
        "native-list": (layout["outside"], [layout["client"], "plugin", "list"]),
    }
    rows, allowed = {}, {"runtime-attempt.json"}
    for label in COMMANDS:
        triple = {label + suffix for suffix in (".command.json", ".stdout.log", ".stderr.log")}
        require(triple <= set(bodies) if label in prefix else not triple & set(bodies))
        if label not in prefix:
            continue
        allowed |= triple
        row = obj(strict_json(bodies[label + ".command.json"]), "argv cwd exit elapsed_ms error context_error cleanup")
        require(same(row["argv"], expected[label][1]) and row["cwd"] == expected[label][0]
                and type(row["exit"]) is int and -1 <= row["exit"] <= 255
                and type(row["error"]) is bool and type(row["context_error"]) is bool
                and row["cleanup"] == "ordinary full-descendant supervision; forced cleanup is a failure")
        natural(row["elapsed_ms"], 35000)
        current = label == prefix[-1] and (result["stage"] == "prepare"
            or result["stage"] == "public-user" and label == "add-user"
            or result["stage"] == "native-list" and label == "native-list")
        if not current:
            values(row, {"exit": 0, "error": False, "context_error": False})
            if label.startswith("git-"):
                require(bodies[label + ".stdout.log"] == bodies[label + ".stderr.log"] == b"")
        rows[label] = row
    require(sum(row["elapsed_ms"] for row in rows.values()) <= result["elapsed_ms"])
    return rows, allowed


def metadata_status(bodies, name, stage):
    from project_native_runtime import strict_json
    metadata(bodies[name], stage)
    return strict_json(bodies[name])["status"]


def members(bodies, source):
    from project_native_runtime import strict_json
    require(type(bodies) is dict and "runtime-attempt.json" in bodies and len(bodies) <= 18
            and all(type(body) is bytes and len(body) <= 1 << 20 for body in bodies.values())
            and sum(map(len, bodies.values())) <= 16 << 20)
    for body in bodies.values():
        body.decode("utf-8", errors="strict")
    result = strict_json(bodies["runtime-attempt.json"])
    require(type(result) is dict and FIELDS <= set(result) <= FIELDS | {"original_config_change"})
    values(result, {"client": "copilot", "mode": MODE, "test_failed": True, "runtime_attempt_complete": False,
                    "vendor_authentication_qualified": False, "real_model_qualified": False, "probe_status": "absent",
                    "qualification": {flag: False for flag in FLAGS}})
    stage = result["stage"]
    require(type(stage) is str and stage in STAGES)
    natural(result["elapsed_ms"], 180000)
    layout = authority(result, source)
    rows, allowed = commands(bodies, result, layout)
    lengths, count, validated = STAGES[stage]
    require(len(rows) in lengths)
    statuses = []
    for name, label in ((USER_META, "add-user"), (LIST_META, "native-list"))[:count]:
        require(name in bodies)
        allowed.add(name)
        statuses.append(metadata_status(bodies, name, label))
    require(set(bodies) == allowed)
    if stage != "prepare":
        require(bodies["version.stdout.log"] == VERSION and bodies["version.stderr.log"] == b"")
    if validated:
        public(bodies["add-user.stdout.log"])
    if stage in ("before-user", "after-user-metadata"):
        require(statuses[0] == "refused")
    if count == 2:
        require(statuses[0] == "observed")
    if stage in ("before-list", "after-list-metadata"):
        require(statuses[1] == "refused")
    if stage == "review-barrier":
        require(statuses[1] == "observed")
    if "original_config_change" in result:
        require(stage in ("prepare", "public-user", "native-list"))
        config_facts(result["original_config_change"])
    return result

"""Closed failed-only empty Copilot listing; opaque native output is not absence."""
from project_native_copilot_events import natural, obj, require, same, values
from project_native_copilot_user import FLAGS, FIELDS, authority, config_facts, metadata

MODE = "copilot-empty-registry-first-capture"
COMMANDS = ("git-A", "git-B", "version", "native-list")
TRIPLES = {name + suffix for name in COMMANDS for suffix in (".command.json", ".stdout.log", ".stderr.log")}
ALLOWED = TRIPLES | {"runtime-attempt.json", "user-profile-metadata.json"}


def commands(bodies, result, layout):
    from project_native_runtime import strict_json
    prefix = result["commands"]
    require(type(prefix) is list and len(prefix) <= 4 and prefix == list(COMMANDS[:len(prefix)]))
    expected = {
        "git-A": (layout["a"], [layout["git"], "-c", "core.hooksPath=/dev/null", "init", "--quiet"]),
        "git-B": (layout["b"], [layout["git"], "-c", "core.hooksPath=/dev/null", "init", "--quiet"]),
        "version": (layout["outside"], [layout["client"], "--version"]),
        "native-list": (layout["outside"], [layout["client"], "plugin", "list"]),
    }
    rows = {}
    for label in COMMANDS:
        triple = {label + suffix for suffix in (".command.json", ".stdout.log", ".stderr.log")}
        require(triple <= set(bodies) if label in prefix else not triple & set(bodies))
        if label not in prefix:
            continue
        row = obj(strict_json(bodies[label + ".command.json"]), "argv cwd exit elapsed_ms error context_error cleanup")
        require(same(row["argv"], expected[label][1]) and row["cwd"] == expected[label][0]
                and type(row["exit"]) is int and -1 <= row["exit"] <= 255
                and type(row["error"]) is bool and type(row["context_error"]) is bool
                and row["cleanup"] == "ordinary full-descendant supervision; forced cleanup is a failure")
        natural(row["elapsed_ms"], 35000)
        if label != prefix[-1] or result["stage"] in {"before-list", "review-barrier"} or (result["stage"] != "prepare" and label != "native-list"):
            values(row, {"exit": 0, "error": False, "context_error": False})
        rows[label] = row
    require(sum(row["elapsed_ms"] for row in rows.values()) <= result["elapsed_ms"])
    return rows


def members(bodies, source):
    from project_native_runtime import strict_json
    require("runtime-attempt.json" in bodies and set(bodies) <= ALLOWED and len(bodies) <= 14)
    require(all(type(body) is bytes and len(body) <= 1 << 20 for body in bodies.values()))
    result = strict_json(bodies["runtime-attempt.json"])
    require(type(result) is dict and FIELDS <= set(result) <= FIELDS | {"original_config_change"})
    values(result, {"client": "copilot", "mode": MODE, "test_failed": True, "runtime_attempt_complete": False,
                    "vendor_authentication_qualified": False, "real_model_qualified": False, "probe_status": "absent",
                    "qualification": {flag: False for flag in FLAGS}})
    require(type(result["stage"]) is str and result["stage"] in {"prepare", "before-list", "native-list", "review-barrier"})
    natural(result["elapsed_ms"], 180000)
    layout = authority(result, source)
    rows = commands(bodies, result, layout)
    if result["stage"] == "prepare":
        require(len(rows) <= 3 and "user-profile-metadata.json" not in bodies)
    else:
        require(len(rows) >= 3 and "user-profile-metadata.json" in bodies)
        metadata(bodies["user-profile-metadata.json"], "native-list")
        require(bodies["version.stdout.log"] == b"GitHub Copilot CLI 1.0.103.\nRun 'copilot update' to check for updates.\n"
                and bodies["version.stderr.log"] == b"")
        for label in ("git-A", "git-B"):
            require(bodies[label + ".stdout.log"] == bodies[label + ".stderr.log"] == b"")
    if result["stage"] == "before-list":
        require(len(rows) == 3 and same(strict_json(bodies["user-profile-metadata.json"]), {"stage": "native-list", "status": "refused"}))
    if result["stage"] == "review-barrier":
        require(len(rows) == 4 and "original_config_change" not in result)
    if "original_config_change" in result:
        config_facts(result["original_config_change"])
    return result

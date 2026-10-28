"""Closed Gemini complete-cycle success admission over retained evidence only.

Filesystem and process containment are attested by the pinned live checks and
the finite final receipt. This reader never runs a client or reopens a profile.
"""
from datetime import datetime
import base64
import hashlib
import json
import math
import posixpath
import re

import project_native_gemini_cycle as cycle
from project_native_gemini_cycle_bytes import private, stream as failed_stream
from project_native_runtime import strict_json

MODE = "gemini-complete-cycle"
NS = 1_000_000_000
SHA = re.compile(r"[0-9a-f]{64}\Z")
BUILTINS = {
    "antigravity-support": "2b866bce962f642be8c0dd74ffc2548fa29f2100bc945518b2f738b7d7d6fd38",
    "skill-creator": "aab93935325978cb5a54b4907ae1ca11d7b4a903925ed0cc1739345392280db9",
}
CLIENT_SOURCE = {
    "commit": "6006c14a4decfe4b891c6263ad6ad5992e404b5b",
    "genai_commit": "38cac5bbf4941ec5fa760238bd423c0ecc2c6f04",
    "genai_converters_blob": "509660270e9cfe2173850ba0a8e9e4ddbeb3b3af",
    "genai_types_blob": "24ed38f1aaace9165a0dfb4bcbfe794ca94a75de",
    "mcp_tool_blob": "997f1957eed193f61bf6090b607d3ad402da7742",
    "output_types_blob": "1d4159a1f8e545ec2da2b8df413cb6778b3f79dc",
    "tools_blob": "ab2870984aba77e77fc5bacb25009cd47698582e",
}
SCANNER = {
    "archive_sha256": "2b3d176db752433b904a4b42375543ff398f4841d22e48f7d4f23ded925b72da",
    "binary_sha256": "b6723451f155eb49e84acb34d1f06de929bddfcff6238cd4e38cdc6519396fba",
    "platform": "linux-amd64", "route": "prepopulated supported scanner cache; ordinary CLI security assessment",
    "source_tag": "v0.1.3", "version": "0.1.3",
}


def require(condition, reason="complete evidence differs"):
    if not condition:
        raise ValueError("Gemini complete " + reason)


def same(left, right):
    if type(left) is not type(right):
        return False
    if type(left) is dict:
        return left.keys() == right.keys() and all(same(left[k], right[k]) for k in left)
    if type(left) is list:
        return len(left) == len(right) and all(same(a, b) for a, b in zip(left, right))
    return left == right


def closed(value, required, optional=()):
    require(type(value) is dict and set(required) <= value.keys()
            and not value.keys() - set(required) - set(optional), "closed object")
    return value


def text(value, limit, nonempty=False):
    return (type(value) is str and "\0" not in value and len(value.encode("utf-8")) <= limit
            and (not nonempty or bool(value)))


def natural(value, high, low=0):
    require(type(value) is int and low <= value <= high, "integer bound/type")
    return value


def load(body):
    require(type(body) is bytes and 0 < len(body) <= 1 << 20, "JSON byte bound")
    value = strict_json(body)
    nodes = [0]

    def visit(item, depth):
        nodes[0] += 1
        require(depth <= 32 and nodes[0] <= 16384, "JSON structure bound")
        if type(item) is dict:
            require(all(len(k.encode("utf-8")) <= 4096 for k in item), "JSON key byte bound")
            for child in item.values():
                visit(child, depth + 1)
        elif type(item) is list:
            for child in item:
                visit(child, depth + 1)
        elif type(item) in (int, float):
            require(math.isfinite(float(item)), "finite JSON number")
        elif type(item) is str:
            item.encode("utf-8", errors="strict")

    visit(value, 0)
    return value


def instant(value):
    require(text(value, 64, True), "timestamp type")
    match = re.fullmatch(r"([0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2})(?:\.([0-9]{1,9}))?(Z|[+-](?:[01][0-9]|2[0-3]):[0-5][0-9])", value)
    require(match is not None and int(match[1][:4]) >= 1, "timestamp grammar")
    base = datetime.fromisoformat(match[1] + match[3].replace("Z", "+00:00"))
    return int(base.timestamp()) * NS + int((match[2] or "").ljust(9, "0"))


def window(value, start, end):
    stamp = instant(value)
    require(start <= stamp <= end, "timestamp outside original interval")
    return stamp


def digest(body):
    return hashlib.sha256(body).hexdigest()


def stream(row):
    body = failed_stream(row)
    require(base64.b64encode(body).decode("ascii") == row["body"], "canonical whole base64")
    return body


def records(body, expected):
    require(type(body) is bytes and len(body) <= 1 << 20 and body.endswith(b"\n")
            and not private(body), "whole aggregate/private boundary")
    values = [closed(load(line), ("stage", "value")) for line in body[:-1].split(b"\n")]
    require([v["stage"] for v in values] == list(expected), "fixed aggregate chronology")
    require(all(type(v["value"]) is dict for v in values), "aggregate value type")
    return values


def receipt(row, argv, cwd, maximum, exit_code=0):
    closed(row, ("argv", "cwd", "exit", "error", "context_error", "elapsed_ms", "cleanup"))
    require(same(row["argv"], argv) and row["cwd"] == cwd and type(row["exit"]) is int
            and row["exit"] == exit_code and row["error"] is False and row["context_error"] is False
            and row["cleanup"] == cycle.CLEANUP, "ordinary command receipt")
    natural(row["elapsed_ms"], maximum)
    require(type(argv) is list and 0 < len(argv) <= 32 and all(text(a, 4096, True) for a in argv)
            and text(cwd, 4096, True), "command text bounds")


def completion(result):
    row = closed(result.get("cycle_completion"), ("schema_version", "started_at", "work_deadline_at", "finished_at",
                 "work_context_error", "original_checks_error", "open_authorities", "open_events", "provider"))
    require(len(json.dumps(row, ensure_ascii=False).encode()) <= 8192, "completion receipt bound")
    require(type(row["schema_version"]) is int and row["schema_version"] == 1
            and row["work_context_error"] is False and row["original_checks_error"] is False
            and type(row["open_authorities"]) is int and row["open_authorities"] == 0
            and row["open_events"] is False, "original final checks")
    expected = {"shutdown_complete": True, "forced_close": False, "serve_returned": True, "error": False,
                "busy": 0, "current_stage": "", "requests": 16, "completed_stages": list(cycle.NATIVE)}
    require(same(row["provider"], expected), "ordinary final provider closure")
    start, deadline, end = (instant(row[k]) for k in ("started_at", "work_deadline_at", "finished_at"))
    require(deadline == start + 150 * NS and start <= end <= deadline, "original work origin")
    elapsed = natural(result["elapsed_ms"], 180000)
    require((end - start) // 1_000_000 <= elapsed, "elapsed origin")
    return start, end


def pins(result, paths):
    require(same(result.get("client_source"), CLIENT_SOURCE) and same(result.get("scanner"), SCANNER), "pinned sources/scanner")
    require(same(result.get("gemini_detection_link"), {"path": paths["root"] + "/detection-bin/gemini", "target": paths["client"]}), "detection link")
    identity = closed(result.get("acquisition"), ("client", "acquisition", "node", "runtime_files"))
    require(identity["client"] == "gemini", "acquired client")
    files = {}
    for key, archive_sha in (("acquisition", "bb5e7e879608bd9b3fe261930d3886fa56aacbf471da1655f2978eb35b705a73"),
                             ("node", "fd8e59d5a511510f6a298afb548f18c7d2b1be404d8b4a27d94fbe49f56cb2d6")):
        archive = closed(identity[key], ("sha256", "pin_verified", "members"))
        require(archive["sha256"] == archive_sha and archive["pin_verified"] is True
                and type(archive["members"]) is list and 0 < len(archive["members"]) <= 512, "pinned archive closure")
        seen = set()
        for item in archive["members"]:
            closed(item, ("name", "kind", "bytes", "sha256"))
            name = item["name"]
            require(text(name, 4096, True) and name != "." and not name.startswith("/") and "\\" not in name
                    and posixpath.normpath(name) == name and ".." not in name.split("/") and name not in seen, "canonical archive member")
            seen.add(name)
            natural(item["bytes"], 1 << 30)
            if item["kind"] == "directory":
                require(item["bytes"] == 0 and item["sha256"] == "", "archive directory")
                continue
            require(item["kind"] == "file" and type(item["sha256"]) is str and SHA.fullmatch(item["sha256"]), "archive file")
            if key == "node":
                require(len(archive["members"]) == 1 and name == "node-v24.21.0-linux-x64/bin/node"
                        and item["bytes"] == 126595440
                        and item["sha256"] == "7fde7b8afa198da66257f42ee2001d874c7355631e6d1579a5fb5ef1f246df4c", "pinned Node member")
                path = paths["node"]
            else:
                path = posixpath.dirname(paths["client"]) + "/" + name
            require(path not in files, "runtime path alias")
            files[path] = item["sha256"]
    require(files.get(paths["client"]) == "41847af2158ced805c4a3e098a05b9a2c26fb1a49b12583009b2eaf6a4059a59"
            and files.get(paths["node"]) == "7fde7b8afa198da66257f42ee2001d874c7355631e6d1579a5fb5ef1f246df4c", "pinned entrypoints")
    for name, sha in BUILTINS.items():
        require(files.get(posixpath.dirname(paths["client"]) + "/builtin/" + name + "/SKILL.md") == sha, "pinned builtin")
    runtime = identity["runtime_files"]
    require(type(runtime) is list and len(runtime) == len(files), "runtime inventory count")
    actual = {}
    for row in runtime:
        closed(row, ("path", "sha256"))
        require(type(row["path"]) is str and row["path"] not in actual, "runtime path uniqueness")
        actual[row["path"]] = row["sha256"]
    require(same(actual, files), "full immutable runtime-file closure")


def members(bodies, source, result):
    try:
        return _members(bodies, source, result)
    except (KeyError, TypeError, AttributeError, IndexError, OverflowError, UnicodeError, RecursionError) as error:
        raise ValueError("Gemini complete structural refusal") from error


def _members(bodies, source, result):
    required = {"runtime-attempt.json", *cycle.AGGREGATES}
    required |= {label + suffix for label in cycle.PUBLIC for suffix in (".command.json", ".stdout.log", ".stderr.log")}
    require(type(bodies) is dict and set(bodies) == required and len(bodies) == 57
            and all(type(v) is bytes and len(v) <= 2 << 20 and not private(v) for v in bodies.values())
            and sum(map(len, bodies.values())) <= 16 << 20, "closed member/size/privacy boundary")
    decoded = load(bodies["runtime-attempt.json"])
    require(same(decoded, result), "actual summary alias")
    closed(result, ("acquisition", "activation", "attempt_stage", "capture_mode", "client", "client_source",
                    "cycle_capture_review_required", "cycle_layout", "cycle_listing_stages", "cycle_native_stages",
                    "cycle_observed_complete", "cycle_provider_completed", "cycle_public_stages", "elapsed_ms",
                    "gemini_detection_link", "provider_kind", "qualification", "real_model_qualified", "runtime_attempt_complete",
                    "scanner", "source_identity", "test_failed", "vendor_authentication_qualified", "cycle_completion"))
    require(result["client"] == "gemini" and result["provider_kind"] == "scripted_loopback"
            and result["capture_mode"] == MODE and result["attempt_stage"] == "complete-cycle"
            and result["activation"] == "prepared" and result["cycle_capture_review_required"] is False
            and result["cycle_observed_complete"] is True and result["runtime_attempt_complete"] is True
            and result["test_failed"] is False and result["vendor_authentication_qualified"] is False
            and result["real_model_qualified"] is False and same(result["qualification"], dict.fromkeys(cycle.QUAL, True)), "success-only tuple")
    require(type(source) is dict and all(type(source[k]) is str and re.fullmatch(r"[0-9a-f]{40}", source[k]) for k in ("source_commit", "source_tree")), "held source identity")
    expected = {"installer_base_commit": source["source_commit"], "installer_tree": source["source_tree"], "installer_source_state": "committed", "installer_patch_sha256": []}
    require(same(result["source_identity"], expected), "exact current source/tree")
    for key, values in (("cycle_public_stages", cycle.PUBLIC), ("cycle_listing_stages", cycle.LISTINGS),
                        ("cycle_native_stages", cycle.NATIVE), ("cycle_provider_completed", cycle.NATIVE)):
        require(same(result[key], list(values)), "full fixed matrices")
    paths = cycle.layout(result["cycle_layout"])
    require(all(text(v, 4096, True) and "\\" not in v for v in paths.values()), "layout text bounds")
    start, end = completion(result)
    pins(result, paths)
    from project_native_gemini_complete_proof import verify as verify_proof
    from project_native_gemini_complete_protocol import verify as verify_protocol
    context = verify_proof(bodies, paths, start, end)
    verify_protocol(bodies, paths, start, end, context)
    return result

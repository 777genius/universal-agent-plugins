"""Closed failed-only public Copilot user-registration observation; no runtime claim."""
import json
import re

from project_native_copilot_events import natural, obj, require, same, values
from project_native_copilot_readiness import canonical

MODE = "copilot-user-registry-first-capture"
COMMANDS = ("git-A", "git-B", "version", "add-user")
TRIPLES = {name + suffix for name in COMMANDS for suffix in (".command.json", ".stdout.log", ".stderr.log")}
ALLOWED = TRIPLES | {"runtime-attempt.json", "user-profile-metadata.json", "probe-events.jsonl"}
FIELDS = set("client mode acquisition scanner source_identity runtime_attempt_complete vendor_authentication_qualified real_model_qualified qualification commands stage layout probe_status test_failed elapsed_ms".split())
FLAGS = ("project-lifecycle", "native-restart-removal", "mcp-initialize", "mcp-tools-list", "mcp-tools-call")
SCANNER = {"archive_sha256": "2b3d176db752433b904a4b42375543ff398f4841d22e48f7d4f23ded925b72da",
           "binary_sha256": "b6723451f155eb49e84acb34d1f06de929bddfcff6238cd4e38cdc6519396fba", "platform": "linux-amd64",
           "route": "prepopulated supported scanner cache; ordinary CLI security assessment", "source_tag": "v0.1.3", "version": "0.1.3"}


def metadata(body):
    from project_native_runtime import strict_json
    require(len(body) <= 2048)
    row = strict_json(body)
    if same(row, {"stage": "add-user", "status": "refused"}):
        return
    obj(row, "stage status before_entries after_entries changed_entries omitted_count entries")
    values(row, {"stage": "add-user", "status": "observed"})
    for key in ("before_entries", "after_entries"):
        natural(row[key], 1024)
        require(row[key] >= 1)
    natural(row["changed_entries"], row["before_entries"] + row["after_entries"])
    natural(row["omitted_count"], row["changed_entries"])
    entries = row["entries"]
    require(type(entries) is list and len(entries) <= 16 and row["changed_entries"] == len(entries) + row["omitted_count"]
            and (not row["changed_entries"] or len(entries) > 0))
    keys = []
    for entry in entries:
        obj(entry, "path_sha256 before_kind after_kind before_permissions after_permissions before_present after_present owner_equal group_equal identity_equal mode_equal")
        require(type(entry["path_sha256"]) is str and re.fullmatch(r"[0-9a-f]{64}", entry["path_sha256"]))
        keys.append(entry["path_sha256"])
        for key in ("before_present", "after_present", "owner_equal", "group_equal", "identity_equal", "mode_equal"):
            require(type(entry[key]) is bool)
        require(entry["before_present"] or entry["after_present"])
        for prefix in ("before", "after"):
            require(type(entry[prefix + "_kind"]) is str and entry[prefix + "_kind"] in {"absent", "file", "directory"})
            natural(entry[prefix + "_permissions"], 0o7777)
            require(entry[prefix + "_present"] == (entry[prefix + "_kind"] != "absent"))
            require(entry[prefix + "_present"] or entry[prefix + "_permissions"] == 0)
        both = entry["before_present"] and entry["after_present"]
        if both:
            require(entry["owner_equal"] and entry["group_equal"])
            require(entry["mode_equal"] == (entry["before_kind"] == entry["after_kind"]
                    and entry["before_permissions"] == entry["after_permissions"]))
        else:
            require(not any(entry[key] for key in ("owner_equal", "group_equal", "identity_equal", "mode_equal")))
    require(keys == sorted(set(keys)))


def config_facts(row):
    """Retain only the existing finite config projection, never error/value text."""
    hashes = set("expected_sha256 observed_sha256 key_sha256 before_value_sha256 after_value_sha256 prefix_sha256".split())
    counts = set("baseline_bytes observed_bytes prefix_bytes comment_lines suffix_bytes before_fields after_fields changed_fields omitted_count".split())
    booleans = set("baseline_json_valid observed_json_valid semantically_equal trusted_folders_match".split())
    reasons = set("valid syntax unexpected-eof duplicate-key body-limit structure-limit key object-end array-end unexpected-delimiter nonfinite-number trailing-data other".split())
    statuses = set("deadline guard-read-refused baseline-refused read-refused unstable invalid-json non-object change-limit captured suffix-refused absent prefix-refused value-refused entry-limit diagnostic-bound".split())
    kinds = {"absent", "null", "boolean", "number", "string", "array", "object"}
    def visit(value, depth=0):
        require(type(value) is dict and len(value) <= 20 and depth <= 3)
        for key, item in value.items():
            if key in hashes:
                require(type(item) is str and re.fullmatch(r"[0-9a-f]{64}", item))
            elif key in counts:
                natural(item, 16384)
            elif key in booleans:
                require(type(item) is bool)
            elif key == "status":
                require(type(item) is str and item in statuses)
            elif key in {"baseline_json_reason", "observed_json_reason", "suffix_json_reason"}:
                require(type(item) is str and item in reasons)
            elif key == "strict_json_failure":
                require(type(item) is str and item in {"both", "baseline", "observed"})
            elif key in {"before_type", "after_type"}:
                require(type(item) is str and item in kinds)
            elif key in {"leading_comment_probe", "member_delta"}:
                visit(item, depth + 1)
            elif key in {"changed_members", "members"}:
                require(item is None or type(item) is list and len(item) <= 32)
                for member in item or []:
                    visit(member, depth + 1)
            else:
                require(False)
    require(len(json.dumps(row, separators=(",", ":")).encode()) <= 16384)
    visit(row)


def authority(result, source):
    layout = obj(result["layout"], "root home a b outside package installer client git")
    for value in layout.values():
        canonical(value)
    root = layout["root"]
    require(root != "/" and all(layout[key] == root + "/" + tail for key, tail in
            (("home", "home"), ("a", "project"), ("b", "project-B"), ("outside", "outside"), ("package", "package"))))
    expected = {"client": "copilot", "acquisition": {"sha256": "9aeb4430e9ac9523546db07b227c7aba179a4710a55f9ac29ad143b0f9e1b325", "pin_verified": True,
                "members": [{"name": "copilot", "kind": "file", "bytes": 198445888, "sha256": "f19b3554a6b62605db3a26d5255ebe2a3ee39fcd6a357edb18301ea5037f6f0d"}]},
                "runtime_files": [{"path": layout["client"], "sha256": "f19b3554a6b62605db3a26d5255ebe2a3ee39fcd6a357edb18301ea5037f6f0d"}]}
    require(same(result["acquisition"], expected) and same(result["scanner"], SCANNER))
    require(same(result["source_identity"], {"installer_base_commit": source["source_commit"], "installer_tree": source["source_tree"],
                                          "installer_source_state": "committed", "installer_patch_sha256": []}))
    return layout


def commands(bodies, result, layout):
    from project_native_runtime import strict_json
    prefix = result["commands"]
    require(type(prefix) is list and len(prefix) <= 4 and prefix == list(COMMANDS[:len(prefix)]))
    expected = {
        "git-A": (layout["a"], [layout["git"], "-c", "core.hooksPath=/dev/null", "init", "--quiet"]),
        "git-B": (layout["b"], [layout["git"], "-c", "core.hooksPath=/dev/null", "init", "--quiet"]),
        "version": (layout["outside"], [layout["client"], "--version"]),
        "add-user": (layout["outside"], [layout["installer"], "add", layout["package"], "--no-save", "--scope", "user", "--target", "copilot", "--format", "json"]),
    }
    rows = {}
    for label in COMMANDS:
        triple = {label + suffix for suffix in (".command.json", ".stdout.log", ".stderr.log")}
        require(triple <= set(bodies) if label in prefix else not triple & set(bodies))
        if label not in prefix:
            continue
        row = obj(strict_json(bodies[label + ".command.json"]), "argv cwd exit elapsed_ms error context_error cleanup")
        require(same(row["argv"], expected[label][1]) and row["cwd"] == expected[label][0] and type(row["exit"]) is int
                and -1 <= row["exit"] <= 255 and type(row["error"]) is bool and type(row["context_error"]) is bool
                and row["cleanup"] == "ordinary full-descendant supervision; forced cleanup is a failure")
        natural(row["elapsed_ms"], 35000)
        if (label != prefix[-1] or result["stage"] in {"before-user", "public-result", "review-barrier"}
                or (result["stage"] != "prepare" and label != "add-user")):
            values(row, {"exit": 0, "error": False, "context_error": False})
        rows[label] = row
    require(sum(row["elapsed_ms"] for row in rows.values()) <= result["elapsed_ms"])
    return rows


def members(bodies, source):
    from project_native_runtime import strict_json
    require("runtime-attempt.json" in bodies and set(bodies) <= ALLOWED and len(bodies) <= 15)
    result = strict_json(bodies["runtime-attempt.json"])
    require(type(result) is dict and FIELDS <= set(result) <= FIELDS | {"original_config_change"})
    values(result, {"client": "copilot", "mode": MODE, "test_failed": True, "runtime_attempt_complete": False,
                    "vendor_authentication_qualified": False, "real_model_qualified": False,
                    "qualification": {flag: False for flag in FLAGS}})
    require(type(result["stage"]) is str and result["stage"] in {"prepare", "before-user", "public-user", "public-result", "review-barrier"})
    natural(result["elapsed_ms"], 180000)
    layout = authority(result, source)
    rows = commands(bodies, result, layout)
    if result["stage"] == "prepare":
        require(len(rows) <= 3 and "user-profile-metadata.json" not in bodies)
    else:
        require(len(rows) >= 3 and "user-profile-metadata.json" in bodies)
        metadata(bodies["user-profile-metadata.json"])
        require(bodies["version.stdout.log"] == b"GitHub Copilot CLI 1.0.103.\nRun 'copilot update' to check for updates.\n" and bodies["version.stderr.log"] == b"")
        for label in ("git-A", "git-B"):
            require(bodies[label + ".stdout.log"] == bodies[label + ".stderr.log"] == b"")
    if result["stage"] == "before-user":
        require(len(rows) == 3 and same(strict_json(bodies["user-profile-metadata.json"]), {"stage": "add-user", "status": "refused"}))
    if result["stage"] in {"public-result", "review-barrier"}:
        require(len(rows) == 4 and "original_config_change" not in result)
    if result["stage"] == "review-barrier":
        public = strict_json(bodies["add-user.stdout.log"])
        require(type(public) is dict and type(public.get("schema_version")) is int and public["schema_version"] == 1
                and public.get("command") == "add" and public.get("result") == "success" and "data" in public)
    require(type(result["probe_status"]) is str and result["probe_status"] in {"absent", "captured", "refused"}
            and (result["probe_status"] == "captured") == ("probe-events.jsonl" in bodies))
    if "original_config_change" in result:
        config_facts(result["original_config_change"])
    return result

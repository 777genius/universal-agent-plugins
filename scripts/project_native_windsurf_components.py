"""Closed failed-only Windsurf component capture; native bodies stay unparsed."""
import base64
import hashlib
import json
import posixpath
import re

MODE = "windsurf-component-listing-first-capture"
NAME = "uap-windsurf-component"
LABELS = ("git-a", "git-b", "version", "empty-user-skills", "empty-user-mcp", "empty-a-skills",
          "empty-a-mcp", "empty-b-skills", "empty-b-mcp", "add-user", "user-skills", "user-mcp",
          "user-get", "add-a", "a-skills", "a-mcp", "a-get", "b-before-skills", "b-before-mcp",
          "add-b", "b-skills", "b-mcp", "b-get")
QUAL = {key: False for key in ("project-lifecycle", "native-restart-removal", "mcp-initialize",
                              "mcp-tools-list", "mcp-tools-call")}
SUFFIXES = (".command.json", ".stdout.log", ".stderr.log")
SHA = re.compile(r"[0-9a-f]{64}\Z")
DELTA_ROLES = ("home", "tmp", "xdg-config", "xdg-cache", "xdg-data", "xdg-state")
DELTA_RANK = ("xdg-config", "home", "xdg-data", "xdg-state", "xdg-cache", "tmp")


def _sha(body):
    return hashlib.sha256(body).hexdigest()


def _object(value, required, optional=()):
    if type(value) is not dict or set(required) - value.keys() or value.keys() - set(required) - set(optional):
        raise ValueError("Windsurf component closed object differs")
    return value


def _layout(value):
    keys = ("root", "home", "a", "b", "outside", "installer", "client", "git")
    _object(value, keys)
    for path in value.values():
        if (type(path) is not str or not path.startswith("/") or path.startswith("//")
                or posixpath.normpath(path) != path or "\x00" in path):
            raise ValueError("Windsurf component canonical layout differs")
    root = value["root"]
    for key, relative in (("home", "home"), ("a", "project"), ("b", "project-b"), ("outside", "outside")):
        if value[key] != root + "/" + relative:
            raise ValueError("Windsurf component independent authority layout differs")
    if len(set(value.values())) != len(value):
        raise ValueError("Windsurf component layout aliases")
    return value


def _scope(label):
    if label in ("git-a", "add-a") or label.startswith(("a-", "empty-a-")):
        return "a"
    if label in ("git-b", "add-b") or label.startswith(("b-", "empty-b-")):
        return "b"
    return "user"


def _command(label, layout):
    scope = _scope(label)
    cwd = layout["outside" if scope == "user" else scope]
    if label in ("git-a", "git-b"):
        return cwd, [layout["git"], "-c", "core.hooksPath=/dev/null", "init", "--quiet"]
    if label == "version":
        return cwd, [layout["client"], "--version"]
    if label.startswith("add-"):
        args = [layout["installer"], "add", layout["root"] + "/component-source-" + scope,
                "--no-save", "--scope", "user" if scope == "user" else "project",
                "--target", "windsurf", "--format", "json"]
        if scope != "user":
            args += ["--project-root", cwd]
        return cwd, args
    args = ["skills", "list", "--json"] if label.endswith("-skills") else ["mcp", "list"]
    if label.endswith("-get"):
        args = ["mcp", "get", NAME]
    return cwd, [layout["client"], *args]


def _delta_number(value, maximum):
    if type(value) is not int or not 0 <= value <= maximum:
        raise ValueError("Windsurf delta integer bound differs")


def _delta_counts(value):
    _object(value, ("added", "removed", "changed"))
    for count in value.values():
        _delta_number(count, 8192)
    return sum(value.values())


def _delta_value(value):
    if type(value) is not dict or value.get("kind") not in ("absent", "directory", "file"):
        raise ValueError("Windsurf delta ordinary kind differs")
    kind = value["kind"]
    fields = ("kind",) if kind == "absent" else ("kind", "mode", "record_sha256")
    if kind == "file":
        fields += ("bytes", "content_sha256")
    _object(value, fields)
    if kind != "absent":
        _delta_number(value["mode"], 0o177777)
        if value["mode"] & 0o170000 != (0o040000 if kind == "directory" else 0o100000):
            raise ValueError("Windsurf delta mode differs from ordinary kind")
        if type(value["record_sha256"]) is not str or not SHA.fullmatch(value["record_sha256"]):
            raise ValueError("Windsurf delta held record hash differs")
    if kind == "file":
        _delta_number(value["bytes"], 64 << 20)
        if type(value["content_sha256"]) is not str or not SHA.fullmatch(value["content_sha256"]):
            raise ValueError("Windsurf delta content hash differs")
    return kind


def _delta_detail(value):
    _object(value, ("root", "key_sha256", "operation", "before", "after"))
    if (value["root"] not in DELTA_ROLES or type(value["key_sha256"]) is not str
            or not SHA.fullmatch(value["key_sha256"]) or value["operation"] not in ("added", "removed", "changed")):
        raise ValueError("Windsurf delta role/key/operation differs")
    before, after = _delta_value(value["before"]), _delta_value(value["after"])
    operation = value["operation"]
    if ((operation == "added" and (before != "absent" or after == "absent"))
            or (operation == "removed" and (before == "absent" or after != "absent"))
            or (operation == "changed" and (before == "absent" or after == "absent"
                or value["before"]["record_sha256"] == value["after"]["record_sha256"]))):
        raise ValueError("Windsurf delta operation/presence differs")
    special = value["root"] == "xdg-config" and value["key_sha256"] == _sha(b"xdg-config\0devin/config.json")
    return (0 if special else DELTA_RANK.index(value["root"]) + 1, value["key_sha256"])


def _delta_page(value, offset):
    _object(value, ("status", "before", "after", "total", "roots", "omitted_count", "details"))
    if len(json.dumps(value, sort_keys=True, separators=(",", ":")).encode()) > 2 << 10:
        raise ValueError("Windsurf delta page serialization bound differs")
    if value["status"] not in ("captured", "details-bound"):
        raise ValueError("Windsurf paged delta must describe a complete unequal frame")
    for field in ("before", "after"):
        _delta_number(value[field], 4096)
        if value[field] < len(DELTA_ROLES):
            raise ValueError("Windsurf delta lacks complete root frame")
    total = _delta_counts(value["total"])
    _object(value["roots"], DELTA_ROLES)
    for counts in value["roots"].values():
        _delta_counts(counts)
    if (not 0 < total <= 8192 or value["after"] != value["before"] + value["total"]["added"] - value["total"]["removed"]
            or value["total"]["changed"] + value["total"]["removed"] > value["before"]
            or value["total"]["changed"] + value["total"]["added"] > value["after"]
            or any(sum(row[field] for row in value["roots"].values()) != value["total"][field] for field in ("added", "removed", "changed"))):
        raise ValueError("Windsurf delta complete counts differ")
    expected = 0 if value["status"] == "details-bound" else min(3, max(0, total - offset))
    if type(value["details"]) is not list or len(value["details"]) != expected:
        raise ValueError("Windsurf fixed delta page cardinality differs")
    _delta_number(value["omitted_count"], 8192)
    if value["omitted_count"] != total - len(value["details"]):
        raise ValueError("Windsurf delta per-page omission differs")
    for detail in value["details"]:
        _delta_detail(detail)


def _delta_pages(result):
    if "profile_delta_pages" not in result:
        return
    completed, stage = result["completed_labels"], result["attempt_stage"]
    if (not completed or stage != completed[-1] or stage in ("git-a", "git-b", "add-user", "add-a", "add-b")
            or "profile_delta" not in result):
        raise ValueError("Windsurf delta pages lack their actual failed native triple")
    pages = result["profile_delta_pages"]
    if type(pages) is not list or len(pages) != 5:
        raise ValueError("Windsurf delta requires five fixed pages")
    original = result["profile_delta"]
    _delta_page(original, 0)
    keys, ranks = set(), []
    observed = {role: {field: 0 for field in ("added", "removed", "changed")} for role in DELTA_ROLES}
    for index, page in enumerate(pages):
        _delta_page(page, index * 3)
        if (index == 0 and page != original) or any(page[field] != original[field] for field in ("before", "after", "total", "roots")):
            raise ValueError("Windsurf delta pages do not join the unchanged original projection")
        for detail in page["details"]:
            key = detail["key_sha256"]
            if key in keys:
                raise ValueError("Windsurf duplicate paged delta key")
            keys.add(key)
            ranks.append(_delta_detail(detail))
            observed[detail["root"]][detail["operation"]] += 1
    if ranks != sorted(ranks) or any(observed[role][field] > original["roots"][role][field] for role in DELTA_ROLES for field in observed[role]):
        raise ValueError("Windsurf paged delta rank/root counts differ")


def _proof(row, scope, layout, decode):
    _object(row, ("scope", "source", "revision", "nonce", "source_digest", "state", "state_bytes",
                  "state_sha256", "binding", "native_entry", "data", "payload_digest", "user_skill_activated"))
    source = layout["root"] + "/component-source-" + scope
    if (row["scope"] != scope or row["source"] != source or row["revision"] != scope + "-one"
            or type(row["nonce"]) is not str or not re.fullmatch(r"[0-9a-f]{32}", row["nonce"])
            or row["user_skill_activated"] is not False or type(row["state_bytes"]) is not int
            or not 0 < row["state_bytes"] <= 64 << 10 or type(row["state"]) is not str):
        raise ValueError("Windsurf component independent source proof differs")
    raw = base64.b64decode(row["state"], validate=True)
    if len(raw) != row["state_bytes"] or _sha(raw) != row["state_sha256"]:
        raise ValueError("Windsurf component raw state hash differs")
    state = _object(decode(raw), ("schema_version", "installations"), ("transaction_receipts", "environment_receipts"))
    if (state["schema_version"] != 5 or type(state["schema_version"]) is not int
            or type(state["installations"]) is not list or len(state["installations"]) != 1
            or state.get("transaction_receipts", []) != [] or state.get("environment_receipts", []) != []):
        raise ValueError("Windsurf component current state closure differs")
    item = state["installations"][0]
    _object(item, ("installation_id", "declared_name", "source", "package", "clients", "created_at", "updated_at"),
            ("origin_mode", "operation_group_id", "data_receipts"))
    identifier = item["installation_id"]
    if (type(identifier) is not str or not re.fullmatch(r"[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}", identifier)
            or item["declared_name"] != NAME or item.get("origin_mode") != "direct"):
        raise ValueError("Windsurf component installation authority differs")
    want_source = {"source_binding_id": "src_" + _sha((source + "\x00\x00").encode())[:24],
                   "requested_source": source, "canonical_source": source, "resolved_revision": "", "tree_digest": row["source_digest"]}
    if item["source"] != want_source or not re.fullmatch(r"sha256:[0-9a-f]{64}", row["source_digest"]):
        raise ValueError("Windsurf component source binding differs")
    physical = NAME + "-" + _sha(identifier.encode())[:12]
    state_root = (layout["root"] + "/installer-state") if scope == "user" else layout[scope] + "/.uap"
    active, data_path = state_root + "/managed/clients/windsurf/" + physical, state_root + "/plugin-data/" + physical
    scope_name = "user" if scope == "user" else "project"
    binding_id = "client_" + _sha("\x00".join((identifier, "windsurf", scope_name, active)).encode())[:24]
    data_id = "data_" + _sha("\x00".join((identifier, physical, scope_name)).encode())[:24]
    binding = row["binding"]
    if type(binding) is not dict or item["clients"] != {binding_id: binding}:
        raise ValueError("Windsurf component single binding differs")
    expected = {"client_binding_id": binding_id, "client_id": "windsurf", "scope": scope_name,
                "target_locator": active, "physical_artifact_id": physical, "materialization": "materialized",
                "activation": "active" if scope == "user" else "prepared", "verification": "installation_verified",
                "authentication": "not_checked", "policy": "allowed", "data_receipt_id": data_id,
                "affected_surfaces": ["windsurf"]}
    if any(binding.get(key) != value for key, value in expected.items()):
        raise ValueError("Windsurf component exact binding facts differ")
    if binding.get("native_profile_root", "") != ("" if scope == "user" else layout[scope] + "/.devin"):
        raise ValueError("Windsurf component original user/project profile differs")
    receipt = row["data"]
    if (type(receipt) is not dict or item.get("data_receipts") != {data_id: receipt}
            or any(receipt.get(key) != value for key, value in {
                "data_receipt_id": data_id, "physical_backend_id": physical, "scope": scope_name,
                "locator": data_path, "state": "owned",
                "ownership_digest": "sha256:" + _sha(("agentplugins-plugin-data-v1\x00" + data_id + "\x00" + data_path).encode())}.items())):
        raise ValueError("Windsurf component owned data differs")
    env = {"PLUGIN_ROOT": active, "PLUGIN_DATA": data_path, "UAP_TEST_ROOT": layout["root"],
           "UAP_TEST_EVENTS": layout["root"] + "/events.jsonl", "UAP_TEST_NONCE": row["nonce"], "UAP_TEST_REVISION": scope + "-one"}
    entry = {"command": active + "/io.github.777genius.agentplugins/managed-stdio-v1/agentplugins",
             "args": ["--internal-stdio-v1", active, data_path, active, "plugin", "--", "./bin/probe", "Owned Windsurf " + scope, active + "/argument"], "env": env}
    if row["native_entry"] != entry or not re.fullmatch(r"sha256:[0-9a-f]{64}", row["payload_digest"]):
        raise ValueError("Windsurf component source-bound launcher differs")
    _binding_closure(item, binding, row, scope, layout, entry, active, physical)
    return identifier, binding_id, physical, data_id


def _binding_closure(item, binding, row, scope, layout, entry, active, physical):
    # InstallIntentAutomatic is empty and omitempty: the fresh writer omits it.
    _object(binding, ("client_binding_id", "client_id", "scope", "target_locator", "physical_artifact_id", "materialization",
                      "activation", "authentication", "policy", "verification", "data_receipt_id", "affected_surfaces",
                      "package_revision", "native_objects", "receipts", "updated_at"), ("native_profile_root",))
    if type(binding["updated_at"]) is not str:
        raise ValueError("Windsurf component automatic binding metadata differs")
    _object(row["data"], ("data_receipt_id", "physical_backend_id", "scope", "locator", "ownership_digest", "state"),
            ("created_at", "updated_at"))
    manifest = {"$schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json", "name": NAME,
                "version": "1.0.0", "description": "Disposable Windsurf component observation"}
    manifest_sha = "sha256:" + _sha(json.dumps(manifest, sort_keys=True, separators=(",", ":")).encode())
    package = {"loader_kind": "agent_plugins", "format_id": "agent-plugins/1.0.0", "schema_uri": manifest["$schema"],
               "declared_name": NAME, "version": "1.0.0", "manifest_digest": manifest_sha,
               "inventory": {"skills": [NAME], "mcp_present": True, "mcp_enabled": True, "mcp_servers": [NAME]}}
    if item["package"] != package or binding.get("package_revision") != {"version": "1.0.0", "tree_digest": row["source_digest"], "manifest_digest": manifest_sha}:
        raise ValueError("Windsurf component exact authored package differs")
    kind = "windsurf_mcp_entry" if scope == "user" else "devin_project_mcp_server"
    codec = "windsurf-mcpServers" if scope == "user" else "devin-mcpServers"
    profile = layout["home"] + "/.codeium/windsurf" if scope == "user" else layout[scope] + "/.devin"
    canonical = json.dumps(entry, sort_keys=True, separators=(",", ":"), ensure_ascii=False).encode()
    mcp_digest = "sha256:" + _sha(b"agentplugins-native-mcp-v1\0" + codec.encode() + b"\0" + NAME.encode() + b"\0" + canonical)
    owned = [{"object_id": "package:windsurf:" + physical, "kind": "managed_package_directory", "logical_name": NAME,
              "path": active, "managed_digest": row["payload_digest"], "protection_class": "managed"},
             {"object_id": ("windsurf:mcp:" if scope == "user" else kind + ":") + NAME, "kind": kind,
              "logical_name": NAME, "path": profile + "/mcp_config.json", "managed_digest": mcp_digest,
              "protection_class": "managed_entry" if scope == "user" else "managed", **({"source_relative": "mcp.json"} if scope == "user" else {})}]
    if scope != "user":
        skill = ("---\nname: " + NAME + "\ndescription: " + scope + "-one\n---\nUAP_WINDSURF_" + scope + "-one_" + row["nonce"] + "\n").encode()
        kind = "devin_project_skill_directory"
        owned.append({"object_id": kind + ":" + NAME, "kind": kind, "logical_name": NAME,
                      "path": profile + "/skills/" + NAME, "source_relative": "skills/" + NAME,
                      "managed_digest": "sha256:" + _sha(b"f\0SKILL.md\0-\0" + skill + b"\0"), "protection_class": "managed"})
    actual = binding.get("native_objects")
    if type(actual) is not list or len(actual) != len(owned) or sorted(actual, key=lambda v: v["kind"]) != sorted(owned, key=lambda v: v["kind"]):
        raise ValueError("Windsurf component exact native ownership differs")
    receipts = binding.get("receipts")
    if type(receipts) is not list or len(receipts) != 1:
        raise ValueError("Windsurf component committed receipt count differs")
    receipt = _object(receipts[0], ("operation_id", "operation_group_id", "sequence", "mutation_type", "client_binding_id",
                                   "active_path", "staging_path", "backup_path", "after_digest", "phase"), ("before_digest",))
    group = item.get("operation_group_id")
    base = posixpath.dirname(active)
    if (type(group) is not str or not group or receipt["operation_id"] != group + "-001"
            or type(receipt["sequence"]) is not int or receipt.get("before_digest", "") != ""
            or type(receipt["staging_path"]) is not str or posixpath.dirname(receipt["staging_path"]) != base
            or not posixpath.basename(receipt["staging_path"]).startswith(".agentplugins-staging-")
            or receipt["backup_path"] != base + "/.agentplugins-backup-" + _sha(receipt["operation_id"].encode())[:16]):
        raise ValueError("Windsurf component source-bound committed receipt paths differ")
    if any(receipt.get(key) != value for key, value in {"operation_group_id": item.get("operation_group_id"), "sequence": 1,
            "mutation_type": "directory_swap", "client_binding_id": binding["client_binding_id"], "active_path": active,
            "after_digest": row["payload_digest"], "phase": "committed"}.items()):
        raise ValueError("Windsurf component committed receipt differs")


def _members(bodies, source, result, decode):
    if (result.get("client") != "windsurf" or result.get("version") != "3000.11.205"
            or result.get("capture_mode") != MODE or result.get("capture_review_required") is not True
            or result.get("runtime_attempt_complete") is not False or result.get("test_failed") is not True
            or result.get("activation") != "prepared" or result.get("qualification") != QUAL
            or any(type(v) is not bool for v in result.get("qualification", {}).values())
            or result.get("vendor_authentication_qualified") is not False or result.get("real_model_qualified") is not False
            or result.get("source_identity") != {"installer_base_commit": source["source_commit"], "installer_tree": source["source_tree"],
                                                 "installer_source_state": "committed", "installer_patch_sha256": []}):
        raise ValueError("Windsurf component capture must remain failed and unqualified")
    layout = _layout(result.get("component_layout"))
    completed = result.get("completed_labels")
    if (type(completed) is not list or completed != list(LABELS[:len(completed)]) or len(completed) > len(LABELS)
            or type(result.get("component_proof_count")) is not int):
        raise ValueError("Windsurf component exact prefix differs")
    stage = result.get("attempt_stage")
    permitted = {"prepare"} if not completed else {completed[-1]}
    if len(completed) < len(LABELS):
        permitted.add(LABELS[len(completed)])
    else:
        permitted.add("capture-review")
    if stage not in permitted:
        raise ValueError("Windsurf component actual failure stage differs")
    allowed = {"runtime-attempt.json", "probe-events.jsonl", "component-proof.jsonl"}
    for index, label in enumerate(completed):
        triple = {label + suffix for suffix in SUFFIXES}
        allowed |= triple
        if not triple <= bodies.keys():
            raise ValueError("Windsurf component partial triple")
        row = _object(decode(bodies[label + SUFFIXES[0]]), ("argv", "cwd", "exit", "error", "deadline", "elapsed_ms", "cleanup"))
        cwd, argv = _command(label, layout)
        if (row["argv"] != argv or row["cwd"] != cwd or type(row["exit"]) is not int
                or type(row["error"]) is not bool or type(row["deadline"]) is not bool
                or type(row["elapsed_ms"]) is not int or row["elapsed_ms"] < 0
                or row["cleanup"] != "full descendant supervision; forced cleanup fails"):
            raise ValueError("Windsurf component command receipt differs")
        earlier = index < len(completed) - 1 or stage == "capture-review" or stage != label
        native = label not in ("git-a", "git-b", "add-user", "add-a", "add-b")
        if earlier and (row["exit"] != 0 or row["error"] or row["deadline"] or (native and bodies[label + SUFFIXES[2]])):
            raise ValueError("Windsurf component earlier failed command continued")
        if earlier and label == "version" and bodies[label + SUFFIXES[1]] != b"devin 3000.11.205 (6dea39440ae3)\n":
            raise ValueError("Windsurf component exact version differs")
    if ("runtime-attempt.json" not in bodies or set(bodies) - allowed or len(bodies) > 72
            or any(type(raw) is not bytes or len(raw) > 1 << 20 for raw in bodies.values())
            or sum(map(len, bodies.values())) > 16 << 20):
        raise ValueError("Windsurf component closed member bound differs")
    raw = bodies.get("component-proof.jsonl", b"")
    if raw and not raw.endswith(b"\n"):
        raise ValueError("Windsurf component proof framing differs")
    rows = raw.splitlines()
    expected = [label.removeprefix("add-") for label in completed if label.startswith("add-")]
    if (len(rows) != result["component_proof_count"] or len(rows) > len(expected)
            or len(rows) < len(expected) - int(bool(completed and stage == completed[-1] and completed[-1].startswith("add-")))):
        raise ValueError("Windsurf component proof prefix differs")
    identities = []
    for row, scope in zip(rows, expected):
        identities.append(_proof(decode(row), scope, layout, decode))
    if any(len({value[index] for value in identities}) != len(identities) for index in range(4)):
        raise ValueError("Windsurf component same-name authority overlaps")
    if stage == "capture-review" and (len(completed) != 23 or len(rows) != 3):
        raise ValueError("Windsurf complete capture lacks required stages")
    _delta_pages(result)
    return result


def members(bodies, source, result, decode):
    try:
        return _members(bodies, source, result, decode)
    except (KeyError, TypeError, AttributeError, IndexError, OverflowError, UnicodeError, RecursionError) as error:
        raise ValueError("Windsurf component structural refusal") from error

"""Closed skills-only Grok lifecycle evidence; no filesystem access or child."""
import base64
from datetime import datetime
import hashlib
import json
from pathlib import PurePosixPath
import re

MODE = "grok-complete-skills-cycle"
PUBLIC = tuple("git-a git-b git-outside version add-a add-b repeat-add-a unchanged-update-a intact-repair-a update-a missing-repair-a remove-a repeat-remove-a remove-b".split())
INSPECT = tuple("empty-outside empty-a empty-b installed-a installed-b installed-outside stable-a updated-a updated-b updated-outside damaged-a repaired-a removed-a remaining-b removed-outside repeat-removed-a removed-b final-a final-b final-outside".split())
PROOF = tuple("source-a-a-one source-b-b-one fixture add-a add-b repeat-add-a unchanged-update-a intact-repair-a before-source-revision-a source-a-a-two update-a before-missing-a damaged-a missing-repair-a remove-a repeat-remove-a remove-b final".split())
MEMBERS = ("grok-cycle-public.jsonl", "grok-cycle-inspect.jsonl", "grok-cycle-proof.jsonl")
FLAGS = tuple("project-lifecycle native-restart-removal mcp-initialize mcp-tools-list mcp-tools-call".split())
SCHEMA = "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json"
CLEANUP = "full descendant supervision; forced cleanup fails"


def require(condition):
    if not condition:
        raise ValueError("Grok complete skills-cycle evidence join differs")


def load(raw):
    from project_native_runtime import strict_json
    return strict_json(raw)


def private(raw):
    from project_native_runtime import grok_private_credential
    return grok_private_credential(raw)


def same(left, right):
    if type(left) is not type(right):
        return False
    if type(left) is dict:
        return set(left) == set(right) and all(same(left[key], value) for key, value in right.items())
    if type(left) is list:
        return len(left) == len(right) and all(same(a, b) for a, b in zip(left, right))
    return left == right


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def compact(value):
    return json.dumps(value, separators=(",", ":"), sort_keys=True, ensure_ascii=False).encode()


def canonical(value):
    require(type(value) is str and value.startswith("/") and value != "/" and str(PurePosixPath(value)) == value
            and ".." not in PurePosixPath(value).parts and "\0" not in value)
    return value


def stream(value):
    require(type(value) is dict and set(value) == {"bytes", "sha256", "body"}
            and type(value["bytes"]) is int and 0 <= value["bytes"] <= 1 << 20
            and type(value["sha256"]) is str and re.fullmatch(r"[0-9a-f]{64}", value["sha256"])
            and type(value["body"]) is str and len(value["body"]) <= ((1 << 20) + 2) // 3 * 4)
    raw = base64.b64decode(value["body"], validate=True)
    require(len(raw) == value["bytes"] and sha(raw) == value["sha256"]
            and base64.b64encode(raw).decode() == value["body"] and not private(raw))
    return raw


def embedded(value):
    if type(value) is dict:
        if {"body", "sha256"} <= set(value):
            stream(value)
        else:
            for item in value.values():
                embedded(item)
    elif type(value) is list:
        for item in value:
            embedded(item)


def records(raw, order):
    require(type(raw) is bytes and len(raw) <= 1 << 20 and (not raw or raw.endswith(b"\n")) and not private(raw))
    result = [load(line) for line in raw.splitlines()]
    require(len(result) <= len(order))
    for index, row in enumerate(result):
        require(type(row) is dict and set(row) == {"stage", "value"} and row["stage"] == order[index]
                and type(row["value"]) is dict)
        embedded(row["value"])
    return result


def snapshot(value, files, mode, directories=0o700):
    expected = {"."}
    for path in files:
        expected.add(path)
        expected.update(str(parent) for parent in PurePosixPath(path).parents)
    require(type(value) is dict and set(value) == expected)
    for path, entry in value.items():
        keys = set("kind mode uid gid device inode modified_ns changed_ns".split())
        is_file = path in files
        require(type(entry) is dict and set(entry) == keys | ({"content"} if is_file else set()))
        for field in keys - {"kind"}:
            require(type(entry[field]) is int and 0 <= entry[field] < 1 << 64)
        require(entry["inode"] > 0 and entry["mode"] == (mode if is_file else directories))
        require(entry["kind"] == ("file" if is_file else "directory"))
        if is_file:
            require(entry["modified_ns"] > 0 and entry["changed_ns"] > 0 and stream(entry["content"]) == files[path])
        else:
            require(entry["modified_ns"] == entry["changed_ns"] == 0)


def tree_digest(files, source):
    directories = {str(parent) for path in files for parent in PurePosixPath(path).parents} - {"."}
    out = bytearray()
    def frame(raw):
        out.extend(len(raw).to_bytes(8, "big"))
        out.extend(raw)
    if source:
        frame(b"agentplugins.package-tree\0sha256\0v1")
    for path in sorted(set(files) | directories):
        raw, is_file = files.get(path, b""), path in files
        if source:
            for item in ("entry", path, "file" if is_file else "directory", "100644" if is_file else "040000", ""):
                frame(item.encode())
            out.extend(len(raw).to_bytes(8, "big"))
        else:
            out.extend(f'{"file" if is_file else "dir"}\0{path}\0false\0{len(raw)}\0'.encode())
        out.extend(raw)
    return "sha256:" + sha(out)


def authored(scope, revision, nonce):
    name = "uap-grok-cycle-" + scope
    manifest = compact({"$schema": SCHEMA, "name": name, "version": "1.0.0", "description": "Disposable Grok skills cycle"})
    skill = f"---\nname: {name}\ndescription: {revision} {nonce}\n---\nUAP_GROK_{revision}_{nonce}\n".encode()
    return {"plugin.json": manifest, "skills/" + name + "/SKILL.md": skill}


def projected(scope, revision, nonce):
    files = authored(scope, revision, nonce)
    manifest = {"name": "uap-grok-cycle-" + scope, "version": "1.0.0", "description": "Disposable Grok skills cycle", "skills": "./skills/"}
    files[".grok-plugin/plugin.json"] = (json.dumps(manifest, sort_keys=True, ensure_ascii=False, indent=2) + "\n").encode()
    return files


def fixture(value):
    require(set(value) == set("root home project_a project_b outside nonce fixed".split()))
    root, nonce = canonical(value["root"]), value["nonce"]
    require(type(nonce) is str and re.fullmatch(r"[0-9a-f]{32}", nonce))
    paths = {"home": root + "/home", "project_a": root + "/project", "project_b": root + "/project-b", "outside": root + "/outside"}
    require(all(value[key] == path for key, path in paths.items()))
    config = b'[marketplace]\ndefault_skills_installs_purged = true\n[plugins]\nenabled = ["uap-grok-cycle-a", "uap-grok-cycle-b", "uap-grok-original-user"]\ndisabled = ["uap-grok-foreign-a", "uap-grok-foreign-b"]\n[features]\nturn_summary = false\n'
    trust = "".join("[folders." + json.dumps(paths[key], ensure_ascii=False) + "]\ntrusted = true\n" for key in ("project_a", "project_b")).encode()
    fixed = {paths["home"] + "/.grok/config.toml": {".": config}, paths["home"] + "/.grok/trusted_folders.toml": {".": trust}}
    for scope in ("a", "b", "user"):
        name = "uap-grok-original-user" if scope == "user" else "uap-grok-foreign-" + scope
        native = paths["home"] if scope == "user" else paths["project_" + scope]
        description = "Original user " + nonce if scope == "user" else "Foreign inert marker"
        body = "USER_" + nonce if scope == "user" else "FOREIGN_" + scope + "_" + nonce
        fixed[native + "/.grok/plugins/" + name] = {".grok-plugin/plugin.json": compact({"name": name, "version": "0.9.0", "skills": "./skills/"}), "skills/" + name + "/SKILL.md": f"---\nname: {name}\ndescription: {description}\n---\n{body}\n".encode()}
        if scope != "user":
            fixed[native + "/.uap/plugin-data/caller-owned"] = {"keep.txt": f"CALLER_{scope}_{nonce}\n".encode()}
    require(type(value["fixed"]) is dict and set(value["fixed"]) == set(fixed))
    for path, files in fixed.items():
        snapshot(value["fixed"][path], files, 0o600)
    return paths, nonce, value["fixed"]


def stamp(value):
    require(type(value) is str and re.fullmatch(r"\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d(?:\.\d{1,9})?Z", value))
    return datetime.fromisoformat(value[:-1] + "+00:00")


def binding(value, ins, scope, revision, digest, paths, nonce):
    iid, name = ins["installation_id"], "uap-grok-cycle-" + scope
    physical = name + "-" + sha(iid.encode())[:12]
    active = paths["project_" + scope] + "/.grok/plugins/" + physical
    bid = "client_" + sha("\0".join((iid, "grok", "project", active)).encode())[:24]
    required = set("client_binding_id client_id scope target_locator physical_artifact_id materialization activation authentication policy verification package_revision affected_surfaces native_profile_root native_objects receipts updated_at".split())
    require(type(value) is dict and set(value) == required)
    expected = {"client_binding_id": bid, "client_id": "grok", "scope": "project", "target_locator": active, "physical_artifact_id": physical,
        "materialization": "materialized", "activation": "prepared", "authentication": "not_checked", "policy": "allowed", "verification": "package_validated", "affected_surfaces": ["grok"],
        "native_profile_root": paths["project_" + scope] + "/.grok", "package_revision": {"version": "1.0.0", "tree_digest": digest, "manifest_digest": ins["package"]["manifest_digest"]}}
    require(all(same(value[key], item) for key, item in expected.items()))
    managed = tree_digest(projected(scope, revision, nonce), False)
    require(same(value["native_objects"], [{"object_id": "package:grok:" + physical, "kind": "managed_package_directory", "logical_name": name, "path": active, "managed_digest": managed, "protection_class": "managed"}]))
    stamp(value["updated_at"])
    return active, physical


def check_receipt(row, bound, sequence, kind, before, after):
    require(type(row) is dict and type(row.get("operation_group_id")) is str
            and re.fullmatch(r"op-[0-9a-f]{32}", row["operation_group_id"]))
    operation = row["operation_group_id"] + "-001"
    suffix = sha(operation.encode())[:16]
    expected = {"operation_id": operation, "operation_group_id": row["operation_group_id"], "sequence": sequence,
                "mutation_type": kind, "client_binding_id": bound["client_binding_id"], "phase": "committed",
                "active_path": bound["target_locator"], "backup_path": str(PurePosixPath(bound["target_locator"]).parent / (".agentplugins-backup-" + suffix))}
    if kind == "directory_swap":
        expected["staging_path"] = bound["native_profile_root"] + "/.agentplugins-staging-" + suffix
    if before:
        expected["before_digest"] = before
    if after:
        expected["after_digest"] = after
    require(same(row, expected))


def check_swap_receipts(bound, scope, stage, nonce, prior):
    count = 1 if scope == "b" else 2 if stage in ("intact-repair-a", "before-source-revision-a") else 3 if stage in ("update-a", "before-missing-a") else 4 if stage in ("missing-repair-a", "remove-a", "repeat-remove-a") else 1
    rows = bound["receipts"]
    require(type(rows) is list and len(rows) == count)
    one = tree_digest(projected(scope, scope + "-one", nonce), False)
    two = tree_digest(projected(scope, "a-two", nonce), False) if scope == "a" else one
    before, after, seen = ("", one, one, ""), (one, one, two, two), set()
    for index, row in enumerate(rows):
        check_receipt(row, bound, index + 1, "directory_swap", before[index], after[index])
        require(row["operation_id"] not in seen)
        seen.add(row["operation_id"])
    if prior is not None:
        previous = prior["binding"]["receipts"]
        require(same(rows[:len(previous)], previous))


def proofs(rows):
    by_stage = {row["stage"]: row["value"] for row in rows}
    paths, nonce, fixed = fixture(by_stage["fixture"])
    sources = {}
    for scope, revision in (("a", "a-one"), ("b", "b-one"), ("a", "a-two")):
        row = by_stage["source-" + scope + "-" + revision]
        files = authored(scope, revision, nonce)
        digest = tree_digest(files, True)
        require(set(row) == {"scope", "source", "revision", "digest", "files"} and row["scope"] == scope and row["revision"] == revision
                and row["source"] == str(PurePosixPath(paths["home"]).parent / ("cycle-source-" + scope)) and row["digest"] == digest)
        snapshot(row["files"], files, 0o600)
        sources[scope, revision] = row
    current, result, identities = {}, {}, {}
    for stage in PROOF:
        if stage in ("fixture", "final") or stage.startswith("source-"):
            continue
        row = by_stage[stage]
        require(same(row["preserved"], fixed))
        if stage == "damaged-a":
            require(set(row) == {"scope", "removed_path", "before", "absent", "preserved"} and row["scope"] == "a" and row["absent"] is True)
            require(same(row["before"], current["a"]["payload"]) and row["removed_path"] == current["a"]["binding"]["target_locator"])
            continue
        require(set(row) == set("scope revision removed state binding source_digest payload preserved".split()))
        scope = "b" if stage.endswith("-b") else "a"
        revision = "b-one" if scope == "b" else "a-two" if PROOF.index(stage) >= PROOF.index("update-a") else "a-one"
        removed = stage in ("remove-a", "repeat-remove-a", "remove-b")
        require(row["scope"] == scope and row["revision"] == revision and row["removed"] is removed)
        source = sources[scope, revision]
        require(row["source_digest"] == source["digest"])
        state = load(stream(row["state"]))
        require(type(state) is dict and {"schema_version", "installations"} <= set(state) <= {"schema_version", "installations", "transaction_receipts"}
                and type(state["schema_version"]) is int and state["schema_version"] == 5 and type(state["installations"]) is list and len(state["installations"]) == 1)
        ins = state["installations"][0]
        required = set("installation_id declared_name source package origin_mode operation_group_id clients created_at updated_at".split())
        require(type(ins) is dict and required <= set(ins) <= required | {"data_receipts", "data_retained"})
        iid, name = ins["installation_id"], "uap-grok-cycle-" + scope
        require(type(iid) is str and re.fullmatch(r"[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}", iid))
        require(ins["declared_name"] == name and ins["origin_mode"] == "direct" and stamp(ins["created_at"]) <= stamp(ins["updated_at"]))
        require(re.fullmatch(r"op-[0-9a-f]{32}", ins["operation_group_id"]))
        require(same(ins["source"], {"source_binding_id": "src_" + sha((source["source"] + "\0\0").encode())[:24], "requested_source": source["source"], "canonical_source": source["source"], "resolved_revision": "", "tree_digest": source["digest"]}))
        require(same(ins["package"], {"loader_kind": "agent_plugins", "format_id": "agent-plugins/1.0.0", "schema_uri": SCHEMA, "declared_name": name, "version": "1.0.0", "manifest_digest": "sha256:" + sha(authored(scope, revision, nonce)["plugin.json"]), "inventory": {"mcp_present": False, "mcp_enabled": False, "skills": [name]}}))
        active, physical = binding(row["binding"], ins, scope, revision, source["digest"], paths, nonce)
        bound = row["binding"]
        check_swap_receipts(bound, scope, stage, nonce, current.get(scope))
        transaction = state.get("transaction_receipts", [])
        require(type(transaction) is list)
        if removed:
            require(len(transaction) == 1)
            check_receipt(transaction[0], bound, len(bound["receipts"]) + 1, "directory_remove", tree_digest(projected(scope, revision, nonce), False), "")
            require(transaction[0]["operation_id"] not in {item["operation_id"] for item in bound["receipts"]})
            require(ins["operation_group_id"] == transaction[0]["operation_group_id"])
        else:
            require(transaction == [] and ins["operation_group_id"] == bound["receipts"][-1]["operation_group_id"])
        identity = iid, bound["client_binding_id"], physical
        require(identities.setdefault(scope, identity) == identity)
        require(same(ins["clients"], {} if removed else {identity[1]: row["binding"]}) and ins.get("data_retained", False) is removed)
        if not removed:
            require(ins.get("data_receipts", {}) == {})
            snapshot(row["payload"], projected(scope, revision, nonce), 0o644, 0o755)
        else:
            require(same(row["binding"], current[scope]["binding"]))
            data_id = "data_" + sha("\0".join((iid, physical, "project")).encode())[:24]
            data_path = paths["project_" + scope] + "/.uap/plugin-data/" + physical
            receipts = ins.get("data_receipts")
            require(type(receipts) is dict and set(receipts) == {data_id})
            receipt = receipts[data_id]
            expected = {"data_receipt_id": data_id, "physical_backend_id": physical, "scope": "project", "locator": data_path, "ownership_digest": "sha256:" + sha(("agentplugins-plugin-data-v1\0" + data_id + "\0" + data_path).encode()), "state": "owned"}
            require(type(receipt) is dict and set(receipt) == set(expected) | {"created_at", "updated_at"} and all(same(receipt[key], item) for key, item in expected.items()))
            require(stamp(receipt["created_at"]) <= stamp(receipt["updated_at"]))
            marker = stream(row["payload"][".agentplugins-data-owner.json"]["content"])
            require(same(load(marker), receipt))
            snapshot(row["payload"], {".agentplugins-data-owner.json": marker}, 0o600)
        if stage in ("repeat-add-a", "unchanged-update-a", "before-source-revision-a", "before-missing-a", "repeat-remove-a"):
            require(same(row["state"], current[scope]["state"]) and same(row["payload"], current[scope]["payload"]))
        current[scope], result[stage] = row, row
    require(len(set(item[0] for item in identities.values())) == len(set(item[1] for item in identities.values())) == 2)
    final = by_stage["final"]
    require(set(final) == {"preserved", "retained_a", "retained_b", "absent_a", "absent_b"} and same(final["preserved"], fixed))
    for scope in ("a", "b"):
        require(final["absent_" + scope] == current[scope]["binding"]["target_locator"] and same(final["retained_" + scope], current[scope]["payload"]))
    return paths, nonce, result


def command(row):
    require(type(row) is dict and set(row) == set("argv cwd exit error deadline elapsed_ms cleanup stdout stderr".split()))
    require(type(row["argv"]) is list and 1 <= len(row["argv"]) <= 64 and all(type(arg) is str and len(arg) <= 4096 and "\0" not in arg for arg in row["argv"]))
    canonical(row["cwd"])
    require(type(row["exit"]) is int and -1 <= row["exit"] <= 255 and type(row["error"]) is bool and type(row["deadline"]) is bool
            and type(row["elapsed_ms"]) is int and 0 <= row["elapsed_ms"] <= 45000 and row["cleanup"] == CLEANUP)
    return stream(row["stdout"]), stream(row["stderr"])


def inspect(raw, cwd, trusted, expected):
    row = load(raw)
    from project_native_runtime import grok_selection_identity_keys
    require(grok_selection_identity_keys(row, "grokVersion cwd projectRoot projectTrusted plugins skills mcpServers"))
    require(type(row) is dict and row.get("grokVersion") == "1.0.70" and row.get("cwd") == cwd
            and row.get("projectRoot") in (cwd, cwd + "/") and row.get("projectTrusted") is trusted and same(row.get("mcpServers"), []))
    wanted_plugins, wanted_skills = {}, {}
    for name, scope, path, description, count, loaded in expected:
        wanted_plugins[name] = {"name": name, "scope": scope, "path": path, "enabled": True, "provides": {"skills": count, "agents": 0, "hooks": False, "mcpServers": 0}}
        if loaded:
            wanted_skills[name] = {"name": name, "description": description, "source": {"type": "plugin", "plugin_name": name, "path": path + "/./skills/" + name + "/SKILL.md"}, "userInvocable": True}
    for key, wanted in (("plugins", wanted_plugins), ("skills", wanted_skills)):
        actual = row.get(key)
        require(type(actual) is list and len(actual) == len(wanted))
        seen = set()
        for value in actual:
            require(type(value) is dict and type(value.get("name")) is str and value["name"] in wanted and value["name"] not in seen)
            name = value["name"]
            expected_row = wanted[name]
            if key == "skills" and "disabled" in value:
                require(value["disabled"] is False)
                expected_row = {**expected_row, "disabled": False}
            require(same(value, expected_row))
            seen.add(name)


def public_result(raw, stage, verb, selected):
    row = load(raw)
    require(type(row) is dict and set(row) == {"schema_version", "command", "result", "data"}
            and same(row["schema_version"], 1) and row["command"] == verb and row["result"] == "success")
    installation = load(stream(selected["state"]))["installations"][0]
    scope, binding_row = selected["scope"], selected["binding"]
    name, digest = "uap-grok-cycle-" + scope, selected["source_digest"]
    data = row["data"]
    if stage == "repeat-add-a":
        required = {"plugin", "version", "source", "tree_digest", "manifest_digest", "dry_run", "result"}
        require(verb == "add" and type(data) is dict and required <= set(data) <= required | {"security", "next_action"})
        output, group = data, ""
    else:
        required = {"batch", "status", "succeeded", "failed", "plugin", "dry_run", "targets"}
        require(type(data) is dict and required <= set(data))
        require(all(same(data[key], value) for key, value in {"batch": True, "succeeded": 1, "failed": 0, "plugin": name, "dry_run": False}.items()))
        require(data["status"] == ("data_retained" if verb == "remove" else "completed") and type(data["targets"]) is list and len(data["targets"]) == 1)
        target = data["targets"][0]
        require(type(target) is dict and target.get("target") == "grok" and target.get("status") == "external_completed")
        if verb == "update":
            require(target.get("selected") is True)
        output, group = target["output"], data.get("operation_id")
    expected = {"plugin": name, "version": "1.0.0", "source": "local" if verb in ("repair", "remove") else "direct local source", "tree_digest": digest, "manifest_digest": installation["package"]["manifest_digest"], "dry_run": False}
    require(type(output) is dict and all(same(output.get(key), value) for key, value in expected.items()))
    outcome, changed = output["result"], stage not in ("repeat-add-a", "unchanged-update-a")
    if stage != "repeat-add-a":
        require(type(group) is str and re.fullmatch(r"op-[0-9a-f]{32}", group) and output.get("operation_id") == group)
        if changed:
            require(group == installation["operation_group_id"])
    require(type(outcome) is dict and outcome.get("installation_id") == installation["installation_id"]
            and outcome.get("requires_confirmation") is False and outcome.get("mutated") is changed)
    found = []
    def visit(value):
        if type(value) is dict:
            for key, item in value.items():
                if key == "mutated":
                    require(item is changed)
                    found.append(item)
                if key == "no_change":
                    require(type(item) is bool and (not changed or item is False))
                checks = {"tree_digest": digest, "manifest_digest": installation["package"]["manifest_digest"], "installation_id": installation["installation_id"],
                          "client_id": "grok", "target": "grok", "scope": "project", "client_binding_id": binding_row["client_binding_id"],
                          "physical_artifact_id": binding_row["physical_artifact_id"], "target_locator": binding_row["target_locator"]}
                if key in checks:
                    require(same(item, checks[key]))
                visit(item)
        elif type(value) is list:
            for item in value:
                visit(item)
    visit(data)
    require(found)
    if verb == "remove":
        require(data.get("plugin_data_preserved") is True and data.get("data_retained") is True)
        receipt = next(iter(installation["data_receipts"].values()))
        require(same(data.get("retained_data"), [{key: receipt[key] for key in ("data_receipt_id", "physical_backend_id", "scope", "state")}]))
    return group


def complete(public, inspections, proof_rows):
    paths, nonce, proof = proofs(proof_rows)
    commands = {row["stage"]: row["value"] for row in public}
    git, client, installer = commands["git-a"]["argv"][0], commands["version"]["argv"][0], commands["add-a"]["argv"][0]
    require(len({canonical(value) for value in (git, client, installer)}) == 3)
    elapsed, groups = 0, set()
    for stage, row in commands.items():
        out, err = command(row)
        scope = "b" if stage.endswith("-b") else "a"
        cwd = paths["outside"] if stage in ("version", "git-outside") else paths["project_" + scope]
        if stage.startswith("git-"):
            argv = [git, "-c", "core.hooksPath=/dev/null", "init", "--quiet"]
            require(out == err == b"")
        elif stage == "version":
            argv = [client, "--version"]
            require(out == b"grok 1.0.70 (f50904eaee6c)\n" and err == b"")
        else:
            verb = "repair" if "repair" in stage else stage.removeprefix("repeat-").removeprefix("unchanged-").split("-")[0]
            selected = proof[stage]
            name = "uap-grok-cycle-" + scope
            selector = str(PurePosixPath(paths["home"]).parent / ("cycle-source-" + scope)) if verb == "add" or stage == "update-a" else load(stream(selected["state"]))["installations"][0]["installation_id"] if verb == "repair" else name
            argv = [installer, verb, selector] + (["--no-save"] if verb == "add" else []) + ["--scope", "project", "--project-root", cwd, "--target", "grok", "--format", "json"]
            if stage == "repeat-remove-a":
                require(out == b"" and err == b'agentplugins: plugin is not installed for target "grok" in project scope; no target was changed\n')
            else:
                group = public_result(out, stage, verb, selected)
                if group:
                    require(group not in groups)
                    groups.add(group)
                if stage.startswith("add-"):
                    expected_progress = "Resolving and validating one Agent Plugin package for every selected target...\nInstalling " + name + " for 1 selected clients...\n"
                elif stage == "repeat-add-a":
                    expected_progress = "Resolving and validating one Agent Plugin package for every selected target...\nApplying transactional client package...\n"
                elif verb == "update":
                    expected_progress = "Resolving and validating one updated Agent Plugin package for every selected target...\n"
                elif verb == "repair":
                    expected_progress = "Resolving and validating each unique exact installed package revision once...\n"
                else:
                    expected_progress = ""
                require(err == expected_progress.encode())
        require(row["argv"] == argv and row["cwd"] == cwd and row["exit"] == (1 if stage == "repeat-remove-a" else 0)
                and row["error"] is False and row["deadline"] is False and row["elapsed_ms"] <= 35000)
        elapsed += row["elapsed_ms"]
    for capture in inspections:
        stage, row = capture["stage"], capture["value"]
        scope = "outside" if stage.endswith("outside") else stage[-1]
        cwd = paths["outside"] if scope == "outside" else paths["project_" + scope]
        require(row["argv"] == [client, "inspect", "--json"] and row["cwd"] == cwd and row["exit"] == 0 and row["error"] is False and row["deadline"] is False)
        require(row["elapsed_ms"] <= 35000)
        raw, err = command(row)
        require(err == b"")
        name = "uap-grok-original-user"
        expected = [(name, "user", paths["home"] + "/.grok/plugins/" + name, "Original user " + nonce, 1, True)]
        if scope != "outside":
            foreign = "uap-grok-foreign-" + scope
            expected.append((foreign, "project", cwd + "/.grok/plugins/" + foreign, "Foreign inert marker", 1, False))
            owned = stage in ("installed-a", "installed-b", "stable-a", "updated-a", "updated-b", "repaired-a", "remaining-b")
            if owned:
                key = "add-b" if scope == "b" else "update-a" if stage in ("updated-a", "damaged-a", "repaired-a") else "add-a"
                selected = proof[key]
                expected.append(("uap-grok-cycle-" + scope, "project", selected["binding"]["target_locator"], selected["revision"] + " " + nonce, 1, True))
        inspect(raw, cwd, scope != "outside", expected)
        elapsed += row["elapsed_ms"]
    return client, elapsed


def members(bodies, source):
    try:
        return admit(bodies, source)
    except (KeyError, TypeError, AttributeError, IndexError, RecursionError, OverflowError, UnicodeError) as error:
        raise ValueError("Grok complete skills-cycle structure refused") from error


def admit(bodies, source):
    require(set(bodies) == {"runtime-attempt.json", *MEMBERS} and all(len(raw) <= 1 << 20 and not private(raw) for raw in bodies.values()))
    result = load(bodies["runtime-attempt.json"])
    fields = set("client version runtime_attempt_complete activation vendor_authentication_qualified real_model_qualified qualification source_identity acquisition scanner elapsed_ms test_failed capture_mode attempt_stage cycle_public_stages cycle_inspect_stages cycle_proof_stages cycle_observed_complete".split())
    require(type(result) is dict and set(result) == fields)
    require(result["client"] == "grok" and result["version"] == "1.0.70" and result["capture_mode"] == MODE and result["attempt_stage"] == "complete-cycle"
            and result["activation"] == "prepared" and result["vendor_authentication_qualified"] is False and result["real_model_qualified"] is False)
    complete_flag = result["runtime_attempt_complete"]
    require(type(complete_flag) is bool and type(result["test_failed"]) is bool and type(result["cycle_observed_complete"]) is bool
            and type(result["elapsed_ms"]) is int and 0 <= result["elapsed_ms"] <= 195000)
    require(same(result["source_identity"], {"installer_base_commit": source["source_commit"], "installer_tree": source["source_tree"], "installer_source_state": "committed", "installer_patch_sha256": []}))
    require(same(result["qualification"], {key: complete_flag and key in FLAGS[:2] for key in FLAGS}))
    rows = [records(bodies[name], order) for name, order in zip(MEMBERS, (PUBLIC, INSPECT, PROOF))]
    for key, captured in zip(("cycle_public_stages", "cycle_inspect_stages", "cycle_proof_stages"), rows):
        require(same(result[key], [row["stage"] for row in captured]))
    for captured in rows[:2]:
        for row in captured:
            command(row["value"])
    if not complete_flag:
        require(result["test_failed"] is True)
        return result
    require(result["test_failed"] is False and result["cycle_observed_complete"] is True and result["elapsed_ms"] < 150000
            and all(len(captured) == len(order) for captured, order in zip(rows, (PUBLIC, INSPECT, PROOF))))
    client, elapsed = complete(*rows)
    require(elapsed <= result["elapsed_ms"])
    acquisition = result["acquisition"]
    require(same(acquisition, {"client": "grok", "acquisition": {"sha256": "11d27a30adfb75c4b9b180072ab27412a1916f054ff5497ab70fb9a77d304bcc", "pin_verified": True,
        "members": [{"Name": "grok", "Kind": "file", "Bytes": 195700192, "sha256": "22d281c5964c5444147328096ba2a221f3f0874c6858e27c825656a80f6e58e9"}]}, "node": None,
        "runtime_files": [{"path": client, "sha256": "22d281c5964c5444147328096ba2a221f3f0874c6858e27c825656a80f6e58e9"}]}))
    require(same(result["scanner"], {"archive_sha256": "2b3d176db752433b904a4b42375543ff398f4841d22e48f7d4f23ded925b72da", "binary_sha256": "b6723451f155eb49e84acb34d1f06de929bddfcff6238cd4e38cdc6519396fba", "platform": "linux-amd64", "route": "prepopulated supported scanner cache; ordinary CLI security assessment", "source_tag": "v0.1.3", "version": "0.1.3"}))
    return result

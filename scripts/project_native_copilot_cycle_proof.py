"""Captured source/state/native/data joins for the Copilot complete cycle.

Binary-framed whole-package digests are source-attested by the Go verifier.
Every text leaf is reconstructed here; binary leaves join held hash/size receipts.
Only the text-only User source and installed digests can be recomputed here.
"""
from pathlib import PurePosixPath
import re

from project_native_copilot_cycle import HEX, PROOF, canonical, compact, instant, load, natural, obj, require, same, sha, stream, values

PLUGIN_SCHEMA = "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json"
MCP_SCHEMA = "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json"
LAUNCHER = "io.github.777genius.agentplugins/managed-stdio-v1"
DIR = "directory:20000000"


def version(revision):
    return "1.1.0" if revision == "a-two" else "1.0.0"


def nonce(base, key):
    return sha((base + key).encode())[:32]


def recipe(layout, base, key, revision):
    n, skill = nonce(base, key), "uapcopilot" + key
    manifest = compact({"$schema": PLUGIN_SCHEMA, "name": "native-proof", "version": version(revision), "description": "Disposable Copilot cycle"})
    body = f"---\nname: {skill}\ndescription: Disposable Copilot {key} {revision} {n}\n---\nUAP_COPILOT_{revision}_{n}\n".encode()
    files = {"plugin.json": manifest, "skills/" + skill + "/SKILL.md": body}
    if key != "user":
        entry = {"type": "stdio", "command": "./bin/probe", "args": ["argument with spaces", "${PLUGIN_ROOT}/argument", "${UNKNOWN}"], "cwd": "${PLUGIN_ROOT}",
                 "env": {"UAP_TEST_ROOT": layout["root"], "UAP_TEST_EVENTS": layout["root"] + "/events.jsonl", "UAP_TEST_NONCE": n, "UAP_TEST_REVISION": revision, "UAP_TEST_LITERAL": "literal ${UNKNOWN} $HOME", "UAP_TEST_ONCE": "${PLUGIN_ROOT}/${UNKNOWN}"}}
        files["mcp.json"] = compact({"$schema": MCP_SCHEMA, "mcpServers": {skill: entry}})
    return files


def tree(files, mode="600", dirs="700", binary=None):
    result = {".": DIR + dirs}
    for path, raw in files.items():
        result[path] = "file:" + mode + ":" + sha(raw)
    if binary:
        result.update(binary)
    for path in list(result):
        for parent in PurePosixPath(path).parents:
            if str(parent) != ".":
                result.setdefault(str(parent), DIR + dirs)
    return result


def text_digest(files, source=False):
    entries = tree(files)
    out = bytearray()
    def frame(raw):
        out.extend(len(raw).to_bytes(8, "big"))
        out.extend(raw)
    if source:
        frame(b"agentplugins.package-tree\0sha256\0v1")
    for path in sorted(set(entries) - {"."}):
        is_file = path in files
        raw = files[path] if is_file else b""
        if source:
            for value in ("entry", path, "file" if is_file else "directory", "100644" if is_file else "040000", ""):
                frame(value.encode())
            out.extend(len(raw).to_bytes(8, "big"))
        else:
            out.extend(f'{"file" if is_file else "dir"}\0{path}\0false\0{len(raw)}\0'.encode())
        out.extend(raw)
    return "sha256:" + sha(out)


def fixture(value, result):
    obj(value, "nonce layout immutable")
    base = value["nonce"]
    require(type(base) is str and re.fullmatch(r"[0-9a-f]{32}", base))
    layout = obj(value["layout"], "root home a b outside installer client git probe")
    for path in layout.values():
        canonical(path)
    root = layout["root"]
    require(root != "/")
    values(layout, {"home": root + "/home", "a": root + "/project", "b": root + "/project-B", "outside": root + "/outside"})
    require(len(set(layout.values())) == len(layout) and result["nonce"] == base and same(result["layout"], layout))
    immutable = obj(value["immutable"], "installer probe")
    for key, row in immutable.items():
        obj(row, "path bytes sha256")
        require(row["path"] == layout[key] and type(row["sha256"]) is str and HEX.fullmatch(row["sha256"]))
        natural(row["bytes"], 64 << 20)
        require(row["bytes"] > 0)
    return layout, base, immutable


def foreign(layout, base, key):
    name = "uap-foreign-" + key
    description = "Original user skill must remain unchanged" if key == "user" else "Original project B skill must remain unchanged"
    path = layout["home"] + "/.copilot/skills/" + name + "/SKILL.md" if key == "user" else layout["b"] + "/.github/skills/" + name + "/SKILL.md"
    body = f"---\nname: {name}\ndescription: {description}\nuser-invocable: true\n---\nUAP_FOREIGN_{key.upper()}_{base}\n".encode()
    return name, description, path, body


def prepare(proofs, result):
    layout, base, immutable = fixture(proofs["fixture"], result)
    sources = {}
    for key, revision in (("user", "user-one"), ("a", "a-one"), ("b", "b-one"), ("a", "a-two")):
        value = obj(proofs["source-" + key + "-" + revision], "source digest files")
        authored = recipe(layout, base, key, revision)
        binary = {"bin/probe": "file:700:" + immutable["probe"]["sha256"]} if key != "user" else None
        require(value["source"] == layout["root"] + "/cycle-source-" + key and type(value["digest"]) is str
                and re.fullmatch(r"sha256:[0-9a-f]{64}", value["digest"]) and same(value["files"], tree(authored, binary=binary)))
        if key == "user":
            require(value["digest"] == text_digest(authored, source=True))
        sources[key, revision] = value
    require(sources["a", "a-one"]["digest"] != sources["a", "a-two"]["digest"])
    for key in ("user", "a", "b"):
        value = obj(proofs["caller-" + key], "path body files")
        parent = layout["root"] + "/installer-state" if key == "user" else layout[key] + "/.uap"
        raw = f"CALLER_{key}_{base}\n".encode()
        require(value["path"] == parent + "/plugin-data/caller-owned/keep.txt" and stream(value["body"]) == raw
                and same(value["files"], tree({"keep.txt": raw})))
    for key in ("user", "b"):
        value = obj(proofs["foreign-" + key], "name description source path body files")
        name, description, path, body = foreign(layout, base, key)
        values(value, {"name": name, "description": description, "source": "personal-copilot" if key == "user" else "project", "path": path})
        require(stream(value["body"]) == body and same(value["files"], tree({"SKILL.md": body})))
    return layout, base, immutable, sources


def mcp(layout, base, key, revision, active, data, native=True):
    entry = {"type": "stdio", "command": active + "/" + LAUNCHER + "/agentplugins",
             "args": ["--internal-stdio-v1", active, data, active, "plugin", "--", "./bin/probe", "argument with spaces", active + "/argument", "${UNKNOWN}"],
             "env": {"PLUGIN_ROOT": active, "PLUGIN_DATA": data, "UAP_TEST_ROOT": layout["root"], "UAP_TEST_EVENTS": layout["root"] + "/events.jsonl", "UAP_TEST_NONCE": nonce(base, key), "UAP_TEST_REVISION": revision, "UAP_TEST_LITERAL": "literal ${UNKNOWN} $HOME", "UAP_TEST_ONCE": active + "/${UNKNOWN}"}}
    if native:
        entry["tools"] = ["*"]
    return entry


def binding(value, installation, key, revision, source, layout):
    required = set("client_binding_id client_id scope target_locator physical_artifact_id materialization activation authentication policy verification package_revision affected_surfaces native_objects receipts updated_at".split())
    require(type(value) is dict and required <= set(value) <= required | {"data_receipt_id", "native_profile_root"})
    iid = installation["installation_id"]
    physical = "native-proof-" + sha(iid.encode())[:12]
    scope = "user" if key == "user" else "project"
    state_root = layout["root"] + "/installer-state" if key == "user" else layout[key] + "/.uap"
    active, data = state_root + "/managed/clients/copilot/" + physical, state_root + "/plugin-data/" + physical
    bid = "client_" + sha("\0".join((iid, "copilot", scope, active)).encode())[:24]
    values(value, {"client_binding_id": bid, "client_id": "copilot", "scope": scope, "target_locator": active, "physical_artifact_id": physical,
        "materialization": "materialized", "activation": "active" if key == "user" else "prepared", "authentication": "not_checked", "policy": "allowed", "verification": "installation_verified", "affected_surfaces": ["copilot", "vscode"] if key == "user" else ["copilot"],
        "package_revision": {"version": version(revision), "tree_digest": source["digest"], "manifest_digest": "sha256:" + source["files"]["plugin.json"].split(":")[-1]}})
    require(value.get("native_profile_root", "") == ("" if key == "user" else layout[key]))
    instant(value["updated_at"])
    return active, data, physical


def state(value, source, key, revision, layout):
    state = load(stream(value["state"]))
    require(type(state) is dict and {"schema_version", "installations"} <= set(state) <= {"schema_version", "installations", "transaction_receipts"})
    require(type(state["schema_version"]) is int and state["schema_version"] == 5 and type(state["installations"]) is list and len(state["installations"]) == 1)
    ins = state["installations"][0]
    required = set("installation_id declared_name source package origin_mode operation_group_id clients created_at updated_at".split())
    require(type(ins) is dict and required <= set(ins) <= required | {"data_receipts", "data_retained"})
    from project_native_copilot_events import uuid
    uuid(ins["installation_id"])
    require(instant(ins["created_at"]) <= instant(ins["updated_at"]) and type(ins["operation_group_id"]) is str and re.fullmatch(r"op-[0-9a-f]{32}", ins["operation_group_id"]))
    source_path = layout["root"] + "/cycle-source-" + key
    inventory = {"mcp_present": key != "user", "mcp_enabled": key != "user", "skills": ["uapcopilot" + key]}
    if key != "user":
        inventory["mcp_servers"] = ["uapcopilot" + key]
    values(ins, {"declared_name": "native-proof", "origin_mode": "direct", "source": {"source_binding_id": "src_" + sha((source_path + "\0\0").encode())[:24], "requested_source": source_path, "canonical_source": source_path, "resolved_revision": "", "tree_digest": source["digest"]},
                 "package": {"loader_kind": "agent_plugins", "format_id": "agent-plugins/1.0.0", "schema_uri": PLUGIN_SCHEMA, "declared_name": "native-proof", "version": version(revision), "manifest_digest": "sha256:" + source["files"]["plugin.json"].split(":")[-1], "inventory": inventory}})
    b = value["binding"]
    active, data, physical = binding(b, ins, key, revision, source, layout)
    require(same(ins["clients"], {} if value["removed"] else {b["client_binding_id"]: b})
            and ins.get("data_retained", False) is value["removed"])
    receipts(b["receipts"], b, active)
    if "transaction_receipts" in state:
        receipts(state["transaction_receipts"], b, active)
    return ins, active, data, physical


def receipts(rows, binding, active):
    require(type(rows) is list and len(rows) <= 64)
    required = set("operation_id sequence mutation_type client_binding_id phase".split())
    optional = set("operation_group_id active_path staging_path backup_path before_digest after_digest data_receipt_id physical_directory_proof profile_owners".split())
    for row in rows:
        require(type(row) is dict and required <= set(row) <= required | optional and type(row["sequence"]) is int and 1 <= row["sequence"] <= 64
                and row["client_binding_id"] == binding["client_binding_id"] and row["phase"] == "committed")
        require(type(row["operation_id"]) is str and re.fullmatch(r"op-[0-9a-f]{32}-[0-9]{3}", row["operation_id"]))
        for key in ("before_digest", "after_digest"):
            if key in row:
                require(type(row[key]) is str and re.fullmatch(r"sha256:[0-9a-f]{64}", row[key]))
        if "active_path" in row:
            require(row["active_path"] == active)
        for key in ("staging_path", "backup_path"):
            if key in row:
                name = "staging" if key == "staging_path" else "backup"
                require(type(row[key]) is str and re.fullmatch(re.escape(str(PurePosixPath(active).parent)) + r"/\.agentplugins-" + name + r"-[0-9a-f]{16}", row[key]))


def data_snapshot(value, ins, binding, key, removed, written, path, physical, base):
    value = obj(value, "allocated path receipt owner marker files")
    values(value, {"path": path, "allocated": key != "user" or removed})
    if not value["allocated"]:
        require(all(value[name] is None for name in ("receipt", "owner", "marker", "files")) and ins.get("data_receipts", {}) == {} and binding.get("data_receipt_id", "") == "")
        return
    did = "data_" + sha("\0".join((ins["installation_id"], physical, "user" if key == "user" else "project")).encode())[:24]
    receipt = value["receipt"]
    required = set("data_receipt_id physical_backend_id scope locator ownership_digest state".split())
    require(type(receipt) is dict and required <= set(receipt) <= required | {"created_at", "updated_at"})
    values(receipt, {"data_receipt_id": did, "physical_backend_id": physical, "scope": "user" if key == "user" else "project", "locator": path, "state": "owned", "ownership_digest": "sha256:" + sha(("agentplugins-plugin-data-v1\0" + did + "\0" + path).encode())})
    for name in ("created_at", "updated_at"):
        if name in receipt:
            instant(receipt[name])
    require(same(ins.get("data_receipts"), {did: receipt}) and (key == "user" or binding["data_receipt_id"] == did))
    owner = stream(value["owner"])
    require(same(load(owner), receipt))
    files = {".agentplugins-data-owner.json": owner}
    if written:
        marker = ("retained-" + nonce(base, key)).encode()
        require(key != "user" and stream(value["marker"]) == marker)
        files["native-marker.txt"] = marker
    else:
        require(value["marker"] is None)
    require(same(value["files"], tree(files)))


def native_snapshot(value, b, source, key, revision, removed, active, data, physical, layout, base, immutable):
    obj(value, "active managed_files objects skill_path skill config_path config skill_files launcher_metadata")
    skill = "uapcopilot" + key
    skill_path = active + "/skills/" + skill + "/SKILL.md" if key == "user" else layout[key] + "/.github/skills/" + skill + "/SKILL.md"
    config = "" if key == "user" else layout[key] + "/.github/mcp.json"
    values(value, {"active": active, "skill_path": skill_path, "config_path": config, "objects": b["native_objects"]})
    authored = recipe(layout, base, key, revision)
    skill_body = authored["skills/" + skill + "/SKILL.md"]
    generated = {}
    if key == "user":
        market = {"name": "agentplugins-" + sha(physical.encode())[:12], "owner": {"name": "Agent Plugins CLI"}, "metadata": {"description": "Managed local marketplace for native-proof", "version": version(revision)}, "plugins": [{"name": "native-proof", "description": "Disposable Copilot cycle", "version": version(revision), "source": "."}]}
        import json
        generated[".github/plugin/marketplace.json"] = (json.dumps(market, ensure_ascii=False, sort_keys=True, indent=2) + "\n").encode()
        require(value["config"] is None and value["skill_files"] is None and value["launcher_metadata"] is None)
    else:
        entry = mcp(layout, base, key, revision, active, data)
        require(same(load(stream(value["config"])), {"mcpServers": {} if removed else {skill: entry}}))
        files = {".": DIR + "700"}
        if key == "b":
            _, _, _, foreign_body = foreign(layout, base, "b")
            files.update({"uap-foreign-b": DIR + "700", "uap-foreign-b/SKILL.md": "file:600:" + sha(foreign_body)})
        if not removed:
            files.update({skill: DIR + "755", skill + "/SKILL.md": "file:644:" + sha(skill_body)})
        require(same(value["skill_files"], files))
        import json
        generated["mcp.json"] = (json.dumps({"mcpServers": {skill: mcp(layout, base, key, revision, active, data, False)}}, ensure_ascii=False, sort_keys=True, indent=2) + "\n").encode()
    objects = b["native_objects"]
    require(type(objects) is list and len(objects) == (1 if key == "user" else 3))
    by_kind = {item["kind"]: item for item in objects}
    require(len(by_kind) == len(objects))
    package = by_kind["managed_package_directory"]
    obj(package, "object_id kind logical_name path managed_digest protection_class")
    values(package, {"object_id": "package:copilot:" + physical, "kind": "managed_package_directory", "logical_name": "native-proof", "path": active, "protection_class": "managed"})
    require(type(package["managed_digest"]) is str and re.fullmatch(r"sha256:[0-9a-f]{64}", package["managed_digest"]))
    if key != "user":
        for kind, path, digest in (("copilot_project_skill_directory", str(PurePosixPath(skill_path).parent), "sha256:" + sha(b"f\0SKILL.md\0-\0" + skill_body + b"\0")), ("copilot_project_mcp_server", config, "sha256:" + sha(("agentplugins-native-mcp-v1\0copilot-mcpServers\0" + skill + "\0").encode() + compact(entry)))):
            expected = {"object_id": kind + ":" + skill, "kind": kind, "logical_name": skill, "path": path, "managed_digest": digest, "protection_class": "managed"}
            if kind.endswith("directory"):
                expected["source_relative"] = "skills/" + skill
            require(same(by_kind[kind], expected))
    if removed:
        require(value["managed_files"] is None and value["skill"] is None and value["launcher_metadata"] is None)
        return
    require(stream(value["skill"]) == skill_body)
    files = {**authored, **generated}
    binary = None
    if key != "user":
        metadata = stream(value["launcher_metadata"])
        meta = obj(load(metadata), "protocol sha256 cliVersion")
        values(meta, {"protocol": 1, "sha256": immutable["installer"]["sha256"]})
        require(type(meta["cliVersion"]) is str and 1 <= len(meta["cliVersion"]) <= 128)
        files[LAUNCHER + "/metadata.json"] = metadata
        binary = {"bin/probe": "file:755:" + immutable["probe"]["sha256"], LAUNCHER + "/agentplugins": "file:700:" + immutable["installer"]["sha256"]}
    expected_tree = tree(files, "644", "755", binary)
    if key != "user":
        expected_tree["io.github.777genius.agentplugins"] = DIR + "700"
        expected_tree[LAUNCHER] = DIR + "700"
        expected_tree[LAUNCHER + "/metadata.json"] = "file:600:" + sha(files[LAUNCHER + "/metadata.json"])
    else:
        require(package["managed_digest"] == text_digest(files))
    require(same(value["managed_files"], expected_tree))
    require(any(row.get("after_digest") == package["managed_digest"] for row in b["receipts"]))


def validate(rows, result):
    require([row["stage"] for row in rows] == list(PROOF))
    proofs = {row["stage"]: row["value"] for row in rows}
    layout, base, immutable, sources = prepare(proofs, result)
    completed, identities, current, written = {}, {}, {}, set()
    for stage in PROOF[9:]:
        value = proofs[stage]
        if stage.startswith("source-"):
            continue
        if stage == "missing-a":
            before = completed["before-missing-a"]
            obj(value, "path source_digest object before absent")
            skill = next(item for item in before["binding"]["native_objects"] if item["kind"] == "copilot_project_skill_directory")
            body = recipe(layout, base, "a", "a-two")["skills/uapcopilota/SKILL.md"]
            values(value, {"path": layout["a"] + "/.github/skills/uapcopilota", "source_digest": sources["a", "a-two"]["digest"], "object": skill, "before": tree({"SKILL.md": body}, "644", "755"), "absent": True})
            continue
        obj(value, "scope revision version removed state binding source_digest native data")
        key = value["scope"]
        require(key in ("user", "a", "b") and key == ("b" if stage.endswith("-b") else "user" if stage.endswith("-user") else "a"))
        revision = "a-two" if key == "a" and PROOF.index(stage) >= PROOF.index("update-a") else key + "-one"
        removed = stage in ("remove-a", "repeat-remove-a", "remove-b", "repeat-remove-b", "remove-user", "repeat-remove-user")
        values(value, {"revision": revision, "version": version(revision), "removed": removed, "source_digest": sources[key, revision]["digest"]})
        ins, active, data, physical = state(value, sources[key, revision], key, revision, layout)
        identity = (ins["installation_id"], value["binding"]["client_binding_id"], physical)
        require(identities.setdefault(key, identity) == identity)
        if stage.startswith("after-native-"):
            written.add(key)
        data_snapshot(value["data"], ins, value["binding"], key, removed, key in written, data, physical, base)
        native_snapshot(value["native"], value["binding"], sources[key, revision], key, revision, removed, active, data, physical, layout, base, immutable)
        if key in current:
            previous = current[key]
            require(ins["installation_id"] == previous["installation"]["installation_id"])
            if previous["data"]["allocated"]:
                require(same(value["data"]["receipt"], previous["data"]["receipt"]) and same(value["data"]["owner"], previous["data"]["owner"]))
            if stage.startswith("repeat-") or stage in ("unchanged-update-a", "before-source-revision-a", "before-missing-a"):
                require(same(value["state"], previous["state"]) and same(value["native"], previous["native"]))
        value = {**value, "installation": ins, "active": active, "data_path": data, "skill_path": value["native"]["skill_path"], "nonce": nonce(base, key), "project": layout["outside"] if key == "user" else layout[key]}
        completed[stage], current[key] = value, value
    require(len(set(item[0] for item in identities.values())) == len(set(item[1] for item in identities.values())) == len(set(item[2] for item in identities.values())) == 3)
    recipes = {"base_nonce": base, "proofs": completed, "sources": sources}
    return layout, completed, recipes

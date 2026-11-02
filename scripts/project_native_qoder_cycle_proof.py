"""Text-only source, selected package, original controls and retention proofs."""
from pathlib import PurePosixPath
import re

from project_native_qoder_cycle import DESCRIPTION, NAME, PROOF, SCHEMA, canonical, compact, digest, instant, natural, obj, require, same, sha, stream

DIR = "directory:20000000"
STATE_STAGES = tuple("add-a add-b repeat-add-a unchanged-update-a intact-repair-a before-source-revision-a update-a before-missing-a missing-repair-a remove-a repeat-remove-a remove-b".split())
UUID = re.compile(r"[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}\Z")


def nonce(base, key):
    return sha((base + key).encode())[:32]


def authored(base, key, revision):
    n = nonce(base, key)
    return {"plugin.json": compact({"$schema": SCHEMA, "name": NAME, "version": "1.0.0", "description": DESCRIPTION}),
            "skills/" + NAME + "/SKILL.md": f"---\nname: {NAME}\ndescription: Disposable Qoder {key} {revision} {n}\n---\nUAP_QODER_{revision}_{n}\n".encode()}


def projected(base, key, revision):
    import json
    files = authored(base, key, revision)
    files[".agentplugins-qoder-native/.qoder-plugin/plugin.json"] = (json.dumps({"name": NAME, "version": "1.0.0", "description": DESCRIPTION, "skills": "./skills/"}, sort_keys=True, indent=2) + "\n").encode()
    files[".agentplugins-qoder-native/skills/" + NAME + "/SKILL.md"] = files["skills/" + NAME + "/SKILL.md"]
    return files


def tree(files, mode="600", dirs="700"):
    result = {".": DIR + dirs}
    for path, body in files.items():
        result[path] = "file:" + mode + ":" + sha(body)
        for parent in PurePosixPath(path).parents:
            if str(parent) != ".":
                result.setdefault(str(parent), DIR + dirs)
    return result


def tree_digest(files, source=False):
    directories = {str(p) for path in files for p in PurePosixPath(path).parents} - {"."}
    out = bytearray()
    def frame(raw):
        out.extend(len(raw).to_bytes(8, "big"))
        out.extend(raw)
    if source:
        frame(b"agentplugins.package-tree\0sha256\0v1")
    for path in sorted(set(files) | directories):
        is_file, body = path in files, files.get(path, b"")
        if source:
            for value in ("entry", path, "file" if is_file else "directory", "100644" if is_file else "040000", ""):
                frame(value.encode())
            out.extend(len(body).to_bytes(8, "big"))
        else:
            out.extend(f'{"file" if is_file else "dir"}\0{path}\0false\0{len(body)}\0'.encode())
        out.extend(body)
    return "sha256:" + sha(out)


def fixture(value, summary):
    obj(value, "nonce layout owner_uid owner_gid")
    base, layout = value["nonce"], obj(value["layout"], "root home a b outside installer client git")
    require(type(base) is str and re.fullmatch(r"[0-9a-f]{32}", base))
    for path in layout.values():
        canonical(path)
    require(len(set(layout.values())) == len(layout) and same(summary["layout"], layout) and summary["nonce"] == base)
    require(all(layout[key] == layout["root"] + "/" + suffix for key, suffix in (("home", "home"), ("a", "project"), ("b", "project-b"), ("outside", "outside"))))
    natural(value["owner_uid"], (1 << 32) - 1)
    natural(value["owner_gid"], (1 << 32) - 1)
    return layout, base, value["owner_uid"], value["owner_gid"]


def control(layout, base, key):
    name = "uap-qoder-original-user" if key == "user" else "uap-qoder-foreign-" + key
    parent = layout["home"] if key == "user" else layout[key]
    path, description = parent + "/.qoder/plugins/" + name, "Original Qoder " + key + " " + base
    manifest = compact({"name": name, "version": "0.9.0", "description": description, "skills": "./skills/"})
    body = f"---\nname: {name}\ndescription: {description}\n---\nUAP_QODER_ORIGINAL_{key.upper()}_{base}\n".encode()
    entry = dict(scope="user" if key == "user" else "project", installPath=path, version="0.9.0")
    if key != "user":
        entry["projectPath"] = parent
    return name, path, description, manifest, body, entry


def originals(proofs, layout, base):
    profile = {".": "directory:20000000700", ".auth": "directory:20000000700"}
    registry, enabled = {}, {}
    for key in ("user", "a", "b"):
        row = obj(proofs["original-" + key], "key path manifest skill files entry")
        name, path, _, manifest, body, entry = control(layout, base, key)
        files = {".qoder-plugin/plugin.json": manifest, "skills/" + name + "/SKILL.md": body}
        require(row["key"] == key and row["path"] == path and stream(row["manifest"]) == manifest
                and stream(row["skill"]) == body and same(row["entry"], entry) and same(row["files"], tree(files)))
        registry[name + "@local"], enabled[name + "@local"] = [entry], False
        if key == "user":
            for relative, value in tree(files).items():
                profile["plugins/" + name + ("/" + relative if relative != "." else "")] = value.replace("file:600:", "file:600:" + str(len(files[relative])) + ":") if relative in files else value
    row = obj(proofs["original-registry"], "path registry settings_path settings")
    root = layout["home"] + "/.qoder"
    raw_registry = compact({"version": 2, "plugins": registry})
    settings = compact({"general": {"enableAutoUpdate": False}, "enabledPlugins": enabled})
    require(row["path"] == root + "/plugins/installed_plugins_v2.json" and row["settings_path"] == root + "/settings.json"
            and stream(row["registry"]) == raw_registry and stream(row["settings"]) == settings)
    profile.update({"settings.json": "file:600:" + str(len(settings)) + ":" + sha(settings),
                    "plugins": "directory:20000000700", "plugins/installed_plugins_v2.json": "file:600:" + str(len(raw_registry)) + ":" + sha(raw_registry),
                    ".auth/.credential-transaction": "file:600:0:" + sha(b"")})
    for path in ("tmp", "tmp/telemetry", "tmp/telemetry/active-runs"):
        profile[path] = "directory:20000000700"
    row = obj(proofs["original-profile"], "logs_absent inventory")
    require(row["logs_absent"] is True and type(row["inventory"]) is dict)
    actual = row["inventory"]
    for path, mode in (("installation_id", "644"), (".auth/machine_id", "600")):
        # These private fresh UUID bytes are held/validated by Go, never exported.
        value = actual.get(path)
        require(type(value) is str and re.fullmatch("file:" + mode + r":36:[0-9a-f]{64}", value))
        profile[path] = value
    require(same(actual, profile))
    return profile


def receipt(row, bound, sequence, kind, before, after):
    require(type(row) is dict and type(row.get("operation_group_id")) is str and re.fullmatch(r"op-[0-9a-f]{32}", row["operation_group_id"]))
    operation = row["operation_group_id"] + "-001"
    suffix = sha(operation.encode())[:16]
    parent = str(PurePosixPath(bound["target_locator"]).parent)
    expected = dict(operation_id=operation, operation_group_id=row["operation_group_id"], sequence=sequence, mutation_type=kind,
                    client_binding_id=bound["client_binding_id"], active_path=bound["target_locator"], backup_path=parent + "/.agentplugins-backup-" + suffix, phase="committed")
    if kind == "directory_swap":
        expected["staging_path"] = parent + "/.agentplugins-staging-" + suffix
    if before:
        expected["before_digest"] = before
    if after:
        expected["after_digest"] = after
    require(same(row, expected))


def state(row, stage, key, revision, layout, base, prior):
    from project_native_qoder_cycle import load
    obj(row, "scope revision removed source_digest state binding delivery data")
    removed = stage in ("remove-a", "repeat-remove-a", "remove-b")
    require(row["scope"] == key and row["revision"] == revision and row["removed"] is removed)
    source_files = authored(base, key, revision)
    source_path, source_digest = layout["root"] + "/cycle-source-" + key, tree_digest(source_files, True)
    require(row["source_digest"] == source_digest)
    document = obj(load(stream(row["state"])), "schema_version installations", "transaction_receipts")
    require(type(document["schema_version"]) is int and document["schema_version"] == 5 and type(document["installations"]) is list and len(document["installations"]) == 1)
    ins = obj(document["installations"][0], "installation_id declared_name source package clients origin_mode operation_group_id created_at updated_at", "data_receipts data_retained")
    iid = ins["installation_id"]
    require(type(iid) is str and UUID.fullmatch(iid) and instant(ins["created_at"]) <= instant(ins["updated_at"]))
    require(type(ins["operation_group_id"]) is str and re.fullmatch(r"op-[0-9a-f]{32}", ins["operation_group_id"]))
    manifest_digest = "sha256:" + sha(source_files["plugin.json"])
    require(ins["declared_name"] == NAME and ins["origin_mode"] == "direct"
            and same(ins["source"], dict(source_binding_id="src_" + sha((source_path + "\0\0").encode())[:24], requested_source=source_path, canonical_source=source_path, resolved_revision="", tree_digest=source_digest))
            and same(ins["package"], dict(loader_kind="agent_plugins", format_id="agent-plugins/1.0.0", schema_uri=SCHEMA, declared_name=NAME, version="1.0.0", manifest_digest=manifest_digest, inventory=dict(mcp_present=False, mcp_enabled=False, skills=[NAME]))))
    physical = NAME + "-" + sha(iid.encode())[:12]
    root = layout[key] + "/.uap/managed/clients/qoder"
    active, view = root + "/" + physical, root + "/" + physical + "/.agentplugins-qoder-native"
    bid = "client_" + sha("\0".join((iid, "qoder", "project", active)).encode())[:24]
    b = obj(row["binding"], "client_binding_id client_id scope target_locator physical_artifact_id materialization activation authentication policy verification package_revision affected_surfaces native_profile_root native_objects receipts updated_at")
    expected = dict(client_binding_id=bid, client_id="qoder", scope="project", target_locator=active, physical_artifact_id=physical,
                    materialization="materialized", activation="prepared", authentication="not_checked", policy="allowed", verification="installation_verified", affected_surfaces=["qoder"], native_profile_root=root,
                    package_revision=dict(version="1.0.0", tree_digest=source_digest, manifest_digest=manifest_digest))
    require(all(same(b[field], value) for field, value in expected.items()))
    instant(b["updated_at"])
    payload = projected(base, key, revision)
    installed_digest = tree_digest(payload)
    require(same(b["native_objects"], [dict(object_id="package:qoder:" + physical, kind="managed_package_directory", logical_name=NAME, path=active, managed_digest=installed_digest, protection_class="managed")]))
    require(same(ins["clients"], {} if removed else {bid: b}) and ins.get("data_retained", False) is removed)
    count = 1 if key == "b" else 2 if stage in ("intact-repair-a", "before-source-revision-a") else 3 if stage in ("update-a", "before-missing-a") else 4 if stage in ("missing-repair-a", "remove-a", "repeat-remove-a") else 1
    require(type(b["receipts"]) is list and len(b["receipts"]) == count)
    one, two = tree_digest(projected(base, key, key + "-one")), tree_digest(projected(base, key, "a-two"))
    before, after, seen = ("", one, one, ""), (one, one, two, two), set()
    for index, value in enumerate(b["receipts"]):
        receipt(value, b, index + 1, "directory_swap", before[index], after[index])
        require(value["operation_id"] not in seen)
        seen.add(value["operation_id"])
    if prior:
        require(ins["installation_id"] == prior["installation"]["installation_id"] and same(b["receipts"][:len(prior["binding"]["receipts"])], prior["binding"]["receipts"]))
    transactions = document.get("transaction_receipts", [])
    require(type(transactions) is list and len(transactions) == int(removed))
    if removed:
        receipt(transactions[0], b, count + 1, "directory_remove", installed_digest, "")
        require(transactions[0]["operation_group_id"] == ins["operation_group_id"] and transactions[0]["operation_id"] not in seen)
    else:
        require(ins["operation_group_id"] == b["receipts"][-1]["operation_group_id"])
    delivery = obj(row["delivery"], "active view files manifest skill")
    require(delivery["active"] == active and delivery["view"] == view)
    if removed:
        require(all(delivery[field] is None for field in ("files", "manifest", "skill")))
    else:
        require(same(delivery["files"], tree(payload, "644", "755")) and stream(delivery["manifest"]) == payload[".agentplugins-qoder-native/.qoder-plugin/plugin.json"]
                and stream(delivery["skill"]) == source_files["skills/" + NAME + "/SKILL.md"])
    data(row["data"], ins, b, removed, layout[key], physical)
    return {**row, "installation": ins, "active": active, "view": view, "physical": physical}


def data(value, ins, bound, removed, project, physical):
    from project_native_qoder_cycle import load
    obj(value, "allocated path receipt owner files")
    path = project + "/.uap/plugin-data/" + physical
    require(value["allocated"] is removed and value["path"] == path)
    if not removed:
        require(ins.get("data_receipts", {}) == {} and all(value[field] is None for field in ("receipt", "owner", "files")))
        return
    did = "data_" + sha("\0".join((ins["installation_id"], physical, "project")).encode())[:24]
    r = obj(value["receipt"], "data_receipt_id physical_backend_id scope locator ownership_digest state created_at updated_at")
    expected = dict(data_receipt_id=did, physical_backend_id=physical, scope="project", locator=path, ownership_digest="sha256:" + sha(("agentplugins-plugin-data-v1\0" + did + "\0" + path).encode()), state="owned")
    require(all(same(r[field], wanted) for field, wanted in expected.items()) and instant(r["created_at"]) <= instant(r["updated_at"]))
    require(same(ins.get("data_receipts"), {did: r}) and bound.get("data_receipt_id", "") == "")
    owner = stream(value["owner"])
    require(same(load(owner), r) and same(value["files"], tree({".agentplugins-data-owner.json": owner})))


def validate(rows, summary):
    require([row["stage"] for row in rows] == list(PROOF))
    proofs = {row["stage"]: row["value"] for row in rows}
    layout, base, uid, gid = fixture(proofs["fixture"], summary)
    original = originals(proofs, layout, base)
    for key, revision in (("a", "a-one"), ("b", "b-one"), ("a", "a-two")):
        value = obj(proofs["source-" + key + "-" + revision], "source digest files")
        files = authored(base, key, revision)
        require(value["source"] == layout["root"] + "/cycle-source-" + key and value["digest"] == tree_digest(files, True) and same(value["files"], tree(files)))
    for key in ("a", "b"):
        value = obj(proofs["caller-" + key], "path body files")
        body = ("CALLER_" + key + "_" + base + "\n").encode()
        require(value["path"] == layout[key] + "/.uap/plugin-data/caller-owned/keep.txt" and stream(value["body"]) == body and same(value["files"], tree({"keep.txt": body})))
    current, selected = {}, {}
    for stage in STATE_STAGES:
        key = "b" if stage.endswith("-b") else "a"
        revision = "b-one" if key == "b" else "a-two" if STATE_STAGES.index(stage) >= STATE_STAGES.index("update-a") else "a-one"
        value = state(proofs[stage], stage, key, revision, layout, base, current.get(key))
        if stage in ("repeat-add-a", "unchanged-update-a", "before-source-revision-a", "before-missing-a", "repeat-remove-a"):
            require(same(proofs[stage]["state"], current[key]["state"]) and same(proofs[stage]["delivery"], current[key]["delivery"])
                    and same(proofs[stage]["data"], current[key]["data"]))
        current[key], selected[stage] = value, value
    require(len({value["installation"]["installation_id"] for value in current.values()}) == len({value["binding"]["client_binding_id"] for value in current.values()}) == len({value["physical"] for value in current.values()}) == 2)
    missing = obj(proofs["missing-a"], "path source_digest object before absent")
    before = selected["before-missing-a"]
    require(missing["absent"] is True and missing["path"] == before["active"] and missing["source_digest"] == before["source_digest"]
            and same(missing["object"], before["binding"]["native_objects"][0]) and same(missing["before"], before["delivery"]["files"]))
    return layout, base, uid, gid, original, selected, proofs["log-ledger"]

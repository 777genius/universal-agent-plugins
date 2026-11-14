"""Independent fixed-recipe/state proofs for failed-only Gemini cycle capture."""
import hashlib
import json
import posixpath
import re

from project_native_gemini_cycle_bytes import stream, strict_json

NAME = "uap-gemini-cycle"
SHA = re.compile(r"sha256:[0-9a-f]{64}\Z")


def digest(body):
    return hashlib.sha256(body).hexdigest()


def canonical(value):
    return json.dumps(value, ensure_ascii=False, sort_keys=True, separators=(",", ":")).encode()


def closed(value, required, optional=()):
    if type(value) is not dict or set(required) - value.keys() or value.keys() - set(required) - set(optional):
        raise ValueError("Gemini cycle proof closed object differs")
    return value


def source_recipe(row, paths):
    value = closed(row["value"], ("scope", "source", "name", "revision", "nonce", "digest", "files", "skill", "manifest", "mcp"))
    selected = value["scope"]
    names = {"user": "uapgeminiu", "a": "uapgeminia", "b": "uapgeminib"}
    if (selected not in names or value["name"] != names[selected] or value["source"] != paths["root"] + "/cycle-source-" + selected
            or value["revision"] not in ({"user": ("U-one",), "a": ("A-one", "A-two"), "b": ("B-one",)}[selected])
            or row["stage"] != "source-" + selected + "-" + value["revision"]
            or type(value["nonce"]) is not str or re.fullmatch(r"[0-9a-f]{32}", value["nonce"]) is None
            or type(value["digest"]) is not str or SHA.fullmatch(value["digest"]) is None):
        raise ValueError("Gemini independent source recipe identity differs")
    manifest = {"$schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json", "name": NAME,
                "version": "1.0.0", "description": "Disposable Gemini lifecycle proof"}
    skill = ("---\nname: " + names[selected] + "\ndescription: " + value["revision"] + "\n---\nUAP_GEMINI_" + value["revision"] + "_" + value["nonce"] + "\n").encode()
    env = {"UAP_TEST_ROOT": paths["root"], "UAP_TEST_EVENTS": paths["root"] + "/events.jsonl", "UAP_TEST_NONCE": value["nonce"],
           "UAP_TEST_REVISION": value["revision"], "UAP_TEST_LITERAL": "literal ${UNKNOWN} $HOME", "UAP_TEST_ONCE": "${PLUGIN_ROOT}/${UNKNOWN}"}
    mcp = {"$schema": "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json", "mcpServers": {names[selected]: {
        "type": "stdio", "command": "./bin/probe", "args": ["argument with spaces", "${PLUGIN_ROOT}/argument", "${UNKNOWN}"], "cwd": "${PLUGIN_ROOT}", "env": env}}}
    bodies = {"plugin.json": stream(value["manifest"]), "skills/" + names[selected] + "/SKILL.md": stream(value["skill"]), "mcp.json": stream(value["mcp"])}
    if strict_json(bodies["plugin.json"]) != manifest or bodies["skills/" + names[selected] + "/SKILL.md"] != skill or strict_json(bodies["mcp.json"]) != mcp:
        raise ValueError("Gemini raw source manifest/skill/MCP recipe differs")
    files = value["files"]
    if type(files) is not dict or set(files) != {*bodies, "bin/probe"}: raise ValueError("Gemini complete source file inventory differs")
    for name, item in files.items():
        closed(item, ("bytes", "sha256", "executable"))
        if (type(item["bytes"]) is not int or not 0 <= item["bytes"] <= 32 << 20 or type(item["sha256"]) is not str
                or re.fullmatch(r"[0-9a-f]{64}", item["sha256"]) is None or item["executable"] is not (name == "bin/probe")):
            raise ValueError("Gemini source file identity type differs")
        if name in bodies and (item["bytes"] != len(bodies[name]) or item["sha256"] != digest(bodies[name])):
            raise ValueError("Gemini raw source file/hash join differs")
    return value


def state_proof(row, paths, recipe, previous):
    value = closed(row["value"], ("scope", "revision", "removed", "source_digest", "state", "binding"))
    if value["scope"] != recipe["scope"] or value["revision"] != recipe["revision"] or value["source_digest"] != recipe["digest"] or type(value["removed"]) is not bool:
        raise ValueError("Gemini state proof does not bind prior independent source")
    state = closed(strict_json(stream(value["state"])), ("schema_version", "installations"), ("transaction_receipts", "environment_receipts"))
    if type(state["schema_version"]) is not int or state["schema_version"] != 5 or type(state["installations"]) is not list or len(state["installations"]) != 1 or state.get("environment_receipts", []) != []:
        raise ValueError("Gemini single current typed state differs")
    item = closed(state["installations"][0], ("installation_id", "declared_name", "source", "package", "origin_mode", "clients", "created_at", "updated_at", "data_receipts"), ("operation_group_id", "data_retained"))
    iid = item["installation_id"]
    if (type(iid) is not str or re.fullmatch(r"[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}", iid) is None
            or item["declared_name"] != NAME or item["origin_mode"] != "direct"
            or type(item["created_at"]) is not str or type(item["updated_at"]) is not str):
        raise ValueError("Gemini installation identity/type differs")
    source = recipe["source"]
    if item["source"] != {"source_binding_id": "src_" + digest((source + "\0\0").encode())[:24], "requested_source": source,
                          "canonical_source": source, "resolved_revision": "", "tree_digest": recipe["digest"]}:
        raise ValueError("Gemini full source binding differs")
    package = {"loader_kind": "agent_plugins", "format_id": "agent-plugins/1.0.0", "schema_uri": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json",
               "declared_name": NAME, "version": "1.0.0", "manifest_digest": "sha256:" + recipe["files"]["plugin.json"]["sha256"],
               "inventory": {"skills": [recipe["name"]], "mcp_present": True, "mcp_enabled": True, "mcp_servers": [recipe["name"]]}}
    if item["package"] != package: raise ValueError("Gemini exact package/inventory differs")
    selected = recipe["scope"]; target = "user" if selected == "user" else "project"
    root = paths["root"] + "/installer-state" if selected == "user" else paths[selected] + "/.uap"
    profile = paths["home"] + "/.gemini" if selected == "user" else paths[selected] + "/.gemini"
    physical = NAME + "-" + digest(iid.encode())[:12]
    active, data = root + "/managed/clients/gemini/" + physical, root + "/plugin-data/" + physical
    bid = "client_" + digest("\0".join((iid, "gemini", target, active)).encode())[:24]
    did = "data_" + digest("\0".join((iid, physical, target)).encode())[:24]
    _data(item["data_receipts"], did, physical, target, data)
    binding = _binding(value["binding"], recipe, package, paths, profile, physical, active, data, bid, did, target)
    if previous:
        prior_i, prior_b = previous
        if prior_i["installation_id"] != iid or any(prior_b[k] != binding[k] for k in ("client_binding_id", "physical_artifact_id", "data_receipt_id", "target_locator")) or prior_i["data_receipts"] != item["data_receipts"]:
            raise ValueError("Gemini lifecycle stable source/data/binding IDs changed")
    if value["removed"]:
        if item["clients"] != {} or item.get("data_retained") is not True: raise ValueError("Gemini removal lacks exact retained-data closure")
        receipts = state.get("transaction_receipts")
        if type(receipts) is not list or len(receipts) != 1: raise ValueError("Gemini exact removal receipt count differs")
        _receipt(receipts[0], bid, active, len(binding["receipts"]) + 1, True, binding["receipts"][-1]["after_digest"])
    elif item["clients"] != {bid: binding} or item.get("data_retained", False) is not False or state.get("transaction_receipts", []) != []:
        raise ValueError("Gemini installed binding/state receipt closure differs")
    return item, binding


def _data(value, did, physical, target, path):
    if type(value) is not dict or set(value) != {did}: raise ValueError("Gemini exactly one independent data receipt required")
    d = closed(value[did], ("data_receipt_id", "physical_backend_id", "scope", "locator", "ownership_digest", "state"), ("created_at", "updated_at"))
    expected = {"data_receipt_id": did, "physical_backend_id": physical, "scope": target, "locator": path,
                "ownership_digest": "sha256:" + digest(("agentplugins-plugin-data-v1\0" + did + "\0" + path).encode()), "state": "owned"}
    if any(d[k] != v for k, v in expected.items()) or any(type(d[k]) is not str for k in d): raise ValueError("Gemini independent owned data receipt differs")


def _binding(b, recipe, package, paths, profile, physical, active, data, bid, did, target):
    closed(b, ("client_binding_id", "client_id", "scope", "target_locator", "physical_artifact_id", "materialization", "activation", "authentication", "policy", "verification", "package_revision", "data_receipt_id", "affected_surfaces", "native_profile_root", "native_objects", "receipts", "updated_at"))
    expected = {"client_binding_id": bid, "client_id": "gemini", "scope": target, "target_locator": active, "physical_artifact_id": physical,
                "materialization": "materialized", "activation": "active" if target == "user" else "prepared", "authentication": "not_checked", "policy": "allowed",
                "verification": "installation_verified", "data_receipt_id": did, "affected_surfaces": ["gemini"], "native_profile_root": profile,
                "package_revision": {"version": "1.0.0", "tree_digest": recipe["digest"], "manifest_digest": package["manifest_digest"]}}
    if any(b[k] != v for k, v in expected.items()) or type(b["updated_at"]) is not str: raise ValueError("Gemini exact binding facts differ")
    _objects(b, recipe, paths, profile, physical, active, data)
    receipts = b["receipts"]
    if type(receipts) is not list or not 1 <= len(receipts) <= 16: raise ValueError("Gemini committed receipt chain bound differs")
    before = ""
    for index, receipt in enumerate(receipts):
        _receipt(receipt, bid, active, index + 1, False, before);before = receipt["after_digest"]
    return b


def _receipt(value, bid, active, sequence, removed, before):
    required = ("operation_id", "operation_group_id", "sequence", "mutation_type", "client_binding_id", "active_path", "backup_path", "phase")
    closed(value, (*required, *(("before_digest",) if removed else ("staging_path", "after_digest"))), ("before_digest",))
    group = value["operation_group_id"]
    if (type(group) is not str or not group or value["operation_id"] != group + "-001" or type(value["sequence"]) is not int or value["sequence"] != sequence
            or value["mutation_type"] != ("directory_remove" if removed else "directory_swap") or value["client_binding_id"] != bid
            or value["active_path"] != active or value["phase"] != "committed" or value.get("before_digest", "") != before
            or value["backup_path"] != posixpath.dirname(active) + "/.agentplugins-backup-" + digest(value["operation_id"].encode())[:16]):
        raise ValueError("Gemini exact source-bound receipt identity/phase/path differs")
    if not removed and (type(value["after_digest"]) is not str or SHA.fullmatch(value["after_digest"]) is None
                        or type(value["staging_path"]) is not str or posixpath.dirname(value["staging_path"]) != posixpath.dirname(active)
                        or not posixpath.basename(value["staging_path"]).startswith(".agentplugins-staging-")):
        raise ValueError("Gemini committed receipt payload/staging path differs")


def _objects(b, recipe, paths, profile, physical, active, data):
    env = {"PLUGIN_ROOT": active, "PLUGIN_DATA": data, "UAP_TEST_ROOT": paths["root"], "UAP_TEST_EVENTS": paths["root"] + "/events.jsonl",
           "UAP_TEST_NONCE": recipe["nonce"], "UAP_TEST_REVISION": recipe["revision"], "UAP_TEST_LITERAL": "literal ${UNKNOWN} $HOME", "UAP_TEST_ONCE": active + "/${UNKNOWN}"}
    entry = {"command": active + "/bin/probe", "cwd": active, "args": ["argument with spaces", active + "/argument", "${UNKNOWN}"], "env": env}
    name = recipe["name"]
    objects = b["native_objects"]
    if type(objects) is not list or len(objects) != 3: raise ValueError("Gemini exact native object count differs")
    payload = b["receipts"][-1]["after_digest"]
    skill = stream(recipe["skill"])
    expected = [{"object_id": "package:gemini:" + physical, "kind": "managed_package_directory", "logical_name": NAME, "path": active, "managed_digest": payload, "protection_class": "managed"},
                {"object_id": "gemini-mcp:" + name, "kind": "gemini_global_mcp_server", "logical_name": name, "path": profile + "/settings.json", "managed_digest": "sha256:" + digest(("agentplugins-native-mcp-v1\0gemini-mcpServers\0" + name + "\0").encode() + canonical(entry)), "protection_class": "managed"},
                {"object_id": "gemini-skill:" + name, "kind": "gemini_global_skill_directory", "logical_name": name, "path": profile + "/skills/" + name, "source_relative": "skills/" + name, "managed_digest": "sha256:" + digest(b"f\0SKILL.md\0-\0" + skill + b"\0"), "protection_class": "managed"}]
    if sorted(objects, key=lambda v: v["kind"]) != sorted(expected, key=lambda v: v["kind"]): raise ValueError("Gemini exact independent native objects differ")


def proofs(rows, paths):
    recipes, states = {}, {}
    identities = {}
    for row in rows:
        if row["stage"].startswith("source-"):
            recipe = source_recipe(row, paths)
            prior = recipes.get(recipe["scope"])
            if prior and (recipe["scope"] != "a" or prior["revision"] != "A-one" or recipe["revision"] != "A-two" or prior["nonce"] != recipe["nonce"] or prior["source"] != recipe["source"] or prior["files"]["bin/probe"] != recipe["files"]["bin/probe"]):
                raise ValueError("Gemini selected source revision authority differs")
            recipes[recipe["scope"]] = recipe
        else:
            selected = row["value"]["scope"]
            if selected not in recipes: raise ValueError("Gemini state preceded independent source recipe")
            state = state_proof(row, paths, recipes[selected], states.get(selected))
            iid, b = state
            key = (iid["installation_id"], b["client_binding_id"], b["physical_artifact_id"], b["data_receipt_id"])
            if any(other != selected and any(a == z for a, z in zip(value, key)) for other, value in identities.items()):
                raise ValueError("Gemini same-name user/project authority overlaps")
            states[selected], identities[selected] = state, key
    return recipes,states

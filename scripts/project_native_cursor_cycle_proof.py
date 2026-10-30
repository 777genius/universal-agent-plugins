"""Cursor-specific source, state, projection, approval and public DTO joins."""
import json
import re

from project_native_cursor_cycle import (CHANGED, compact, load, obj, require, same, sha, stamp, stream, canonical)

SCHEMA = "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json"
MCP_SCHEMA = "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json"
NAMESPACE = "io.github.777genius.agentplugins/managed-stdio-v1/"


def digest(value):
    require(type(value) is str and re.fullmatch(r"sha256:[0-9a-f]{64}", value))
    return value


def authored(paths, scope, version):
    revision = scope + "-" + version
    nonce = sha((paths["nonce"] + "\0" + revision).encode())[:32]
    server = {"type": "stdio", "command": "./bin/probe", "args": ["cursor-component"], "cwd": "${PLUGIN_ROOT}", "env": {"UAP_TEST_ROOT": paths["root"], "UAP_TEST_EVENTS": paths["events"], "UAP_TEST_NONCE": nonce, "UAP_TEST_REVISION": revision}}
    return {"plugin.json": compact({"$schema": SCHEMA, "name": "native-proof", "version": version}) + b"\n", "mcp.json": compact({"$schema": MCP_SCHEMA, "mcpServers": {"uapcursora": server}}) + b"\n"}, nonce, revision


def server(paths, selected):
    active, data = selected["active"], selected["data"]["locator"]
    return {"command": active + "/" + NAMESPACE + "agentplugins", "args": ["--internal-stdio-v1", active, data, active, "plugin", "--argv-base64-v1", "Li9iaW4vcHJvYmU=", "Y3Vyc29yLWNvbXBvbmVudA=="], "env": {"PLUGIN_ROOT": active, "PLUGIN_DATA": data, "UAP_TEST_ROOT": paths["root"], "UAP_TEST_EVENTS": paths["events"], "UAP_TEST_NONCE": selected["nonce"], "UAP_TEST_REVISION": selected["revision"]}}


def approval(paths, scope, selected):
    raw = server(paths, selected)
    # Exact pinned Zod shape order, followed by the source record's sorted keys.
    parsed = {"command": raw["command"], "args": raw["args"], "env": dict(sorted(raw["env"].items()))}
    encoded = json.dumps({"path": paths[scope], "server": parsed}, separators=(",", ":"), ensure_ascii=False).encode()
    require(encoded.isascii() and b"${" not in encoded)
    key = re.sub(r"[^a-zA-Z0-9]+", "-", paths[scope]).strip("-")
    return paths["home"] + "/.cursor/projects/" + key + "/mcp-approvals.json", "uapcursora-" + sha(encoded)[:16]


def receipt(row, selected, group, count, before, after):
    require(type(group) is str and re.fullmatch(r"op-[0-9a-f]{32}", group))
    op = group + "-001"
    base = selected["active"].rsplit("/", 1)[0]
    expected = {"operation_id": op, "operation_group_id": group, "sequence": count, "mutation_type": "directory_remove" if after is None else "directory_swap", "client_binding_id": selected["binding_id"], "active_path": selected["active"], "backup_path": base + "/.agentplugins-backup-" + sha(op.encode())[:16], "phase": "committed"}
    if after is not None:
        expected["staging_path"] = base + "/.agentplugins-staging-" + sha(op.encode())[:16]
        expected["after_digest"] = digest(after)
    if before:
        expected["before_digest"] = digest(before)
    require(same(row, expected))


def payload(value, paths, selected, files):
    obj(value, "root files digest")
    require(value["root"] == selected["active"])
    digest(value["digest"])
    mcp = json.dumps({"mcpServers": {"uapcursora": server(paths, selected)}}, sort_keys=True, ensure_ascii=False, indent=2).encode() + b"\n"
    manifest = json.dumps({"name": "native-proof", "version": selected["version"], "mcpServers": "./mcp.json"}, sort_keys=True, ensure_ascii=False, indent=2).encode() + b"\n"
    metadata = json.dumps({"protocol": 1, "sha256": paths["installer_sha256"], "cliVersion": "0.1.0-development"}, separators=(",", ":")).encode()
    fixed = {"plugin.json": files["plugin.json"], "mcp.json": mcp, ".cursor-plugin/plugin.json": manifest, NAMESPACE + "metadata.json": metadata}
    require(type(value["files"]) is dict and set(value["files"]) == {*fixed, "bin/probe", NAMESPACE + "agentplugins"})
    for name, row in value["files"].items():
        obj(row, "bytes sha256 executable")
        require(type(row["bytes"]) is int and row["bytes"] > 0 and type(row["executable"]) is bool and row["executable"] is (name in ("bin/probe", NAMESPACE + "agentplugins")))
        if name in fixed:
            require(row["bytes"] == len(fixed[name]) and row["sha256"] == sha(fixed[name]))
        else:
            require(row["sha256"] == paths["probe_sha256" if name == "bin/probe" else "installer_sha256"])
    return value["digest"]


def installation(value, paths, previous, row, command):
    scope, version = value["scope"], value["version"]
    files, nonce, revision = authored(paths, scope, version)
    require(value["nonce"] == nonce and value["revision"] == revision)
    state_raw = stream(value["state"])
    state = load(state_raw)
    obj(state, "schema_version installations", "transaction_receipts")
    require(type(state["schema_version"]) is int and state["schema_version"] == 5 and type(state["installations"]) is list and len(state["installations"]) == 1)
    i = state["installations"][0]
    obj(i, "installation_id declared_name source package origin_mode operation_group_id data_receipts clients created_at updated_at", "data_retained")
    iid = i["installation_id"]
    require(type(iid) is str and re.fullmatch(r"[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}", iid) and i["declared_name"] == "native-proof" and i["origin_mode"] == "direct")
    physical = "native-proof-" + sha(iid.encode())[:12]
    active = paths[scope] + "/.uap/managed/clients/cursor/" + physical
    data_id = "data_" + sha((iid + "\0" + physical + "\0project").encode())[:24]
    binding_id = "client_" + sha((iid + "\0cursor\0project\0" + active).encode())[:24]
    require(type(i["data_receipts"]) is dict and set(i["data_receipts"]) == {data_id})
    data = i["data_receipts"][data_id]
    obj(data, "data_receipt_id physical_backend_id scope locator ownership_digest state created_at updated_at")
    locator = paths[scope] + "/.uap/plugin-data/" + physical
    for key, expected in {"data_receipt_id": data_id, "physical_backend_id": physical, "scope": "project", "locator": locator, "ownership_digest": "sha256:" + sha(("agentplugins-plugin-data-v1\0" + data_id + "\0" + locator).encode()), "state": "owned"}.items():
        require(data[key] == expected)
    start, end = stamp(command["started_at"]), stamp(command["ended_at"])
    for key in ("created_at", "updated_at"):
        require(stamp(i[key]) <= end and stamp(data[key]) <= end)
    if previous is None:
        require(start <= stamp(i["created_at"]) and start <= stamp(data["created_at"]))
    else:
        require(iid == previous["installation"]["installation_id"] and same(data, previous["data"]) and i["created_at"] == previous["installation"]["created_at"])
    src = paths["source_" + scope]
    expected_source = {"source_binding_id": "src_" + sha((src + "\0\0").encode())[:24], "requested_source": src, "canonical_source": src, "resolved_revision": "", "tree_digest": value["source_digest"]}
    expected_package = {"loader_kind": "agent_plugins", "format_id": "agent-plugins/1.0.0", "schema_uri": SCHEMA, "declared_name": "native-proof", "version": version, "manifest_digest": "sha256:" + sha(files["plugin.json"]), "inventory": {"mcp_present": True, "mcp_enabled": True, "mcp_servers": ["uapcursora"]}}
    require(same(i["source"], expected_source) and same(i["package"], expected_package))
    selected = {**value, "installation": i, "data": data, "active": active, "binding_id": binding_id, "state_raw": state_raw}
    obj(value["data"], "receipt owner caller")
    require(same(value["data"]["receipt"], data) and same(load(stream(value["data"]["owner"])), data)
            and stream(value["data"]["caller"]) == ("caller-data-" + scope + "-" + paths["nonce"] + "\n").encode())
    config = load(stream(value["native_config"]))
    removed = value["removed"]
    require(type(removed) is bool and same(config, {"mcpServers": {} if removed else {"uapcursora": server(paths, selected)}}))
    if removed:
        require(previous is not None and i["data_retained"] is True and i["clients"] == {} and len(state["transaction_receipts"]) == 1)
        previous_binding = previous["binding"]
        receipt(state["transaction_receipts"][0], selected, i["operation_group_id"], len(previous_binding["receipts"]) + 1, previous_binding["native_objects"][0]["managed_digest"], None)
        selected["binding"] = previous_binding
    else:
        require("data_retained" not in i and "transaction_receipts" not in state and type(i["clients"]) is dict and set(i["clients"]) == {binding_id})
        b = i["clients"][binding_id]
        obj(b, "client_binding_id client_id scope target_locator physical_artifact_id materialization activation authentication policy verification package_revision data_receipt_id affected_surfaces native_profile_root native_objects receipts updated_at")
        package_digest = payload(value["payload"], paths, selected, files)
        native_digest = "sha256:" + sha(b"agentplugins-native-mcp-v1\0mcpServers\0uapcursora\0" + compact(server(paths, selected)))
        expected = {"client_binding_id": binding_id, "client_id": "cursor", "scope": "project", "target_locator": active, "physical_artifact_id": physical, "materialization": "materialized", "activation": "prepared", "authentication": "not_checked", "policy": "allowed", "verification": "installation_verified", "package_revision": {"version": version, "tree_digest": value["source_digest"], "manifest_digest": expected_package["manifest_digest"]}, "data_receipt_id": data_id, "affected_surfaces": ["cursor"], "native_profile_root": paths[scope] + "/.cursor"}
        require(all(same(b.get(key), item) for key, item in expected.items()))
        objects = [{"object_id": "package:cursor:" + physical, "kind": "managed_package_directory", "logical_name": "native-proof", "path": active, "managed_digest": package_digest, "protection_class": "managed"}, {"object_id": "cursor_project_mcp_server:uapcursora", "kind": "cursor_project_mcp_server", "logical_name": "uapcursora", "path": paths[scope] + "/.cursor/mcp.json", "managed_digest": native_digest, "protection_class": "managed"}]
        require(same(b["native_objects"], objects))
        old_receipts = previous["binding"]["receipts"] if previous else []
        if row in CHANGED:
            require(type(b["receipts"]) is list and len(b["receipts"]) == len(old_receipts) + 1 and same(b["receipts"][:-1], old_receipts) and start <= stamp(b["updated_at"]) <= end)
            before = previous["binding"]["native_objects"][0]["managed_digest"] if previous else ""
            receipt(b["receipts"][-1], selected, i["operation_group_id"], len(b["receipts"]), before, package_digest)
        else:
            require(previous and state_raw == previous["state_raw"])
        selected["binding"] = b
    if row == "repeat-remove-a":
        require(previous and state_raw == previous["state_raw"])
    return selected


def validate(rows, result, commands):
    paths = result["cycle_layout"]
    obj(paths, "root home a b outside source_a source_b nonce installer client git installer_sha256 probe_sha256")
    root = canonical(paths["root"])
    expected_paths = {"home": root + "/home", "a": root + "/project", "b": root + "/project-B", "outside": root + "/outside", "source_a": root + "/package", "source_b": root + "/package-B"}
    require(all(paths[key] == value for key, value in expected_paths.items()) and type(paths["nonce"]) is str and re.fullmatch(r"[0-9a-f]{32}", paths["nonce"]))
    for key in ("installer", "client", "git"):
        canonical(paths[key])
    require(len({paths[key] for key in ("installer", "client", "git")}) == 3 and all(re.fullmatch(r"[0-9a-f]{64}", paths[key]) for key in ("installer_sha256", "probe_sha256")))
    paths = {**paths, "events": root + "/events.jsonl"}
    proofs, latest, sources, approval_ids = {}, {}, {}, {"a": [], "b": []}
    commands = {row["stage"]: row["value"] for row in commands}
    for row in rows:
        stage, value = row["stage"], row["value"]
        if stage.startswith("source-"):
            obj(value, "scope source version revision nonce digest manifest mcp probe_sha256")
            scope, version = value["scope"], value["version"]
            require(stage == "source-" + scope + "-" + version and (scope, version) in (("a", "1.0.0"), ("b", "1.0.0"), ("a", "1.1.0")))
            files, nonce, revision = authored(paths, scope, version)
            require(value["source"] == paths["source_" + scope] and value["nonce"] == nonce and value["revision"] == revision and stream(value["manifest"]) == files["plugin.json"] and stream(value["mcp"]) == files["mcp.json"] and value["probe_sha256"] == paths["probe_sha256"])
            sources[(scope, version)] = digest(value["digest"])
        elif stage == "original-controls":
            obj(value, "user_config user_body foreign_user approval_controls")
            controls = []
            for scope in ("a", "b", "outside"):
                key = re.sub(r"[^a-zA-Z0-9]+", "-", paths[scope]).strip("-")
                controls += [paths["home"] + "/.cursor/projects/" + key + "/" + name for name in ("mcp-approvals.json", "mcp-disabled.json")]
            require(value["user_config"] == paths["home"] + "/.cursor/mcp.json" and stream(value["user_body"]) == b'{"mcpServers":{}}\n' and value["foreign_user"] == paths["home"] + "/foreign-owned.txt" and same(value["approval_controls"], controls))
        elif stage.startswith("approve-"):
            obj(value, "scope path body fingerprints probe_bytes probe_sha256")
            scope = "b" if stage == "approve-b-one" else "a"
            path, identifier = approval(paths, scope, latest[scope])
            approval_ids[scope].append(identifier)
            require(identifier not in approval_ids[scope][:-1] and value["scope"] == scope and value["path"] == path and same(value["fingerprints"], approval_ids[scope]) and stream(value["body"]) == json.dumps(approval_ids[scope], indent=2).encode())
            proofs[stage] = value
        elif stage == "missing-native-a":
            obj(value, "scope path before managed_package_unchanged")
            require(value["scope"] == "a" and value["path"] == paths["a"] + "/.cursor/mcp.json" and value["managed_package_unchanged"] is True and same(load(stream(value["before"])), {"mcpServers": {"uapcursora": server(paths, latest["a"])}}))
        else:
            obj(value, "scope version nonce revision source_digest removed state native_config data", "payload")
            scope = value["scope"]
            expected_scope = "b" if stage in ("add-b", "remove-b") else "a"
            expected_version = "1.1.0" if stage in ("update-a", "missing-repair-a", "remove-a", "repeat-remove-a") else "1.0.0"
            require(scope == expected_scope and value["version"] == expected_version and value["source_digest"] == sources[(scope, value["version"])] and value["removed"] is (stage in ("remove-a", "repeat-remove-a", "remove-b")) and ("payload" in value) is (not value["removed"]))
            prior = latest.get(scope)
            selected = installation(value, paths, prior, stage, commands[stage])
            latest[scope], proofs[stage] = selected, selected
    require(latest["a"]["removed"] and latest["b"]["removed"] and latest["a"]["installation"]["installation_id"] != latest["b"]["installation"]["installation_id"] and latest["a"]["data"]["data_receipt_id"] != latest["b"]["data"]["data_receipt_id"])
    require(approval_ids["a"] and len(approval_ids["a"]) == 2 and len(approval_ids["b"]) == 1)
    require(sources[("a", "1.0.0")] != sources[("a", "1.1.0")])
    return paths, proofs


def public_result(raw, stage, selected):
    row = load(raw)
    obj(row, "schema_version command result data")
    verb = "repair" if "repair" in stage else "update" if "update" in stage else "remove" if "remove" in stage else "add"
    require(type(row["schema_version"]) is int and row["schema_version"] == 1 and row["command"] == verb and row["result"] == "success")
    data, i = row["data"], selected["installation"]
    if stage == "repeat-add-a":
        output, group = data, ""
    else:
        obj(data, "operation_id batch status succeeded failed plugin dry_run targets", "version source tree_digest manifest_digest security action_required environment plugin_data_preserved data_retained retained_data retained_data_action")
        require(same(data["batch"], True) and same(data["succeeded"], 1) and same(data["failed"], 0) and data["plugin"] == "native-proof" and data["dry_run"] is False and data["status"] == ("data_retained" if verb == "remove" else "completed"))
        require(type(data["targets"]) is list and len(data["targets"]) == 1)
        target = data["targets"][0]
        obj(target, "target status output", "selected next_action")
        require(target["target"] == "cursor" and target["status"] == "external_completed" and (verb != "update" or target["selected"] is True))
        output, group = target["output"], data["operation_id"]
        require(type(group) is str and re.fullmatch(r"op-[0-9a-f]{32}", group) and output["operation_id"] == group and (stage not in CHANGED or group == i["operation_group_id"]))
        if verb == "add":
            require(same(data["environment"], {"reason": "no_save_requested", "saved": False}))
        if verb == "remove":
            require(data["data_retained"] is True and data["plugin_data_preserved"] is True and same(data["retained_data"], [{key: selected["data"][key] for key in ("data_receipt_id", "physical_backend_id", "scope", "state")}]))
    obj(output, "plugin version source tree_digest manifest_digest dry_run result", "next_action security" + (" operation_id" if group else ""))
    expected = {"plugin": "native-proof", "version": selected["version"], "source": "local" if verb in ("repair", "remove") else "direct local source", "tree_digest": selected["source_digest"], "manifest_digest": i["package"]["manifest_digest"], "dry_run": False}
    require(all(same(output[key], value) for key, value in expected.items()))
    def identities(value):
        if type(value) is dict:
            checks = {"installation_id": i["installation_id"], "tree_digest": selected["source_digest"], "manifest_digest": i["package"]["manifest_digest"], "client_id": "cursor", "target": "cursor", "scope": "project", "physical_artifact_id": selected["data"]["physical_backend_id"], "mutated": stage in CHANGED, "dry_run": False}
            for key, item in value.items():
                if key in checks:
                    require(same(item, checks[key]))
                if key == "no_change":
                    require(type(item) is bool and item is (stage == "unchanged-update-a"))
                identities(item)
        elif type(value) is list:
            for item in value:
                identities(item)
    identities(data)
    outcome = output["result"]
    require(outcome["installation_id"] == i["installation_id"] and outcome["requires_confirmation"] is False and outcome["mutated"] is (stage in CHANGED))
    if verb == "remove":
        obj(outcome, "installation_id plugin client_id requires_confirmation mutated deactivation affected_surfaces group_phase")
        require(outcome["plugin"] == "native-proof" and outcome["client_id"] == "cursor" and outcome["affected_surfaces"] == ["cursor"] and outcome["group_phase"] == "external_completed")
        require(same(outcome["deactivation"], {"activation": "not_required", "artifact_removal_allowed": True, "external_removal_complete": True}))
    else:
        obj(outcome, "installation_id plan activation requires_confirmation mutated", "group_phase no_change" if group else "")
        if stage == "unchanged-update-a":
            require(outcome["no_change"] is True)
        plan, activation = outcome["plan"], outcome["activation"]
        obj(plan, "client_id scope status package_mode activation authentication policy verification physical_artifact_id components", "user_actions warnings")
        obj(activation, "activation authentication policy verification", "user_actions")
        require(all(plan[key] == value for key, value in {"client_id": "cursor", "scope": "project", "status": "ready", "package_mode": "native", "activation": "prepared", "authentication": "not_checked", "policy": "allowed", "verification": "package_validated", "physical_artifact_id": selected["data"]["physical_backend_id"]}.items()))
        require(same(plan["components"], [{"kind": "mcp_server", "name": "uapcursora", "support": "native"}]) and all(activation[key] == value for key, value in {"activation": "prepared", "authentication": "not_checked", "policy": "allowed", "verification": "installation_verified"}.items()))
    return group

"""Assertions for the public, unmanaged Grok Project repair lifecycle.

These helpers author only the existing disposable trust/foreign fixtures. The
ordinary installer owns every enabled-member insertion and removal.
"""
import base64
import calendar
import copy
from datetime import datetime
import hashlib
import json
import re
import time

from grok_project_protocol_client import document, read_owned, require, selected_installation

FOREIGN_ENABLED = b'enabled = ["uap-foreign-enabled"]'


def config_path(journey):
    return journey.home / ".grok/config.toml"


def config_bytes(journey):
    return read_owned(config_path(journey), journey.root)


def native_id(active, name):
    return "project/" + hashlib.sha256(str(active).encode()).hexdigest()[:8] + "/" + name


def enabled_receipt(binding):
    objects = binding.get("native_objects", [])
    require(isinstance(objects, list) and all(isinstance(item, dict) for item in objects),
            "owned native object inventory differs")
    matches = [item for item in objects if item.get("kind") == "grok_enabled_plugin"]
    require(len(matches) == 1 and not binding.get("native_activation_attempt")
            and not binding.get("pending_native_intent"), "exact settled enabled receipt missing")
    return matches[0]


def expect_retained_receipt(journey, binding):
    require(enabled_receipt(binding) == journey.enabled_receipt,
            "package lifecycle replaced exact enabled membership ownership")


def timestamp_ns(value):
    match = re.fullmatch(r"([0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2})(?:\.([0-9]{0,8}[1-9]))?Z", value) if type(value) is str else None
    require(match is not None, "ordinary repair timestamp is not canonical UTC")
    seconds = calendar.timegm(datetime.strptime(match[1], "%Y-%m-%dT%H:%M:%S").timetuple())
    return seconds * 1_000_000_000 + int((match[2] or "").ljust(9, "0"))


def repair_group(response, installation, binding):
    require(type(response) is dict and type(response.get("schema_version")) is int and response["schema_version"] == 1
            and response.get("command") == "repair" and response.get("result") == "success", "ordinary repair envelope differs")
    batch = response.get("data")
    require(type(batch) is dict and batch.get("batch") is True and batch.get("status") == "completed"
            and type(batch.get("succeeded")) is int and batch["succeeded"] == 1
            and type(batch.get("failed")) is int and batch["failed"] == 0
            and batch.get("dry_run") is False and batch.get("plugin") == installation["declared_name"], "ordinary repair batch differs")
    group, targets = batch.get("operation_id"), batch.get("targets")
    require(type(group) is str and re.fullmatch(r"op-[0-9a-f]{32}", group)
            and type(targets) is list and len(targets) == 1 and type(targets[0]) is dict
            and targets[0].get("target") == "grok" and targets[0].get("status") == "external_completed", "ordinary repair group or target differs")
    output = targets[0].get("output")
    require(type(output) is dict and output.get("operation_id") == group and output.get("dry_run") is False
            and output.get("plugin") == installation["declared_name"] and output.get("source") == "local"
            and output.get("version") == installation["package"]["version"]
            and output.get("tree_digest") == installation["source"]["tree_digest"]
            and output.get("manifest_digest") == installation["package"]["manifest_digest"], "ordinary repair source or revision differs")
    result = output.get("result")
    require(type(result) is dict and result.get("installation_id") == installation["installation_id"]
            and result.get("mutated") is True and result.get("requires_confirmation") is False
            and result.get("group_phase") == "external_completed", "ordinary repair mutation differs")
    plan, activation = result.get("plan"), result.get("activation")
    lifecycle = dict(activation="prepared", authentication="not_checked", policy="allowed", verification="package_validated")
    require(type(plan) is dict and plan.get("client_id") == "grok" and plan.get("scope") == "project"
            and plan.get("physical_artifact_id") == binding["physical_artifact_id"]
            and plan.get("status") == "ready" and plan.get("package_mode") == "native"
            and type(activation) is dict and all(plan.get(key) == value and activation.get(key) == value for key, value in lifecycle.items()),
            "ordinary repair lifecycle differs")
    return group


def ordinary_repair_state(before, after, installation, binding, active, response, window):
    group = repair_group(response, installation, binding)
    clients = installation["clients"]
    key = binding["client_binding_id"]
    require(set(clients) == {key} and clients[key] == binding and not binding.get("native_activation_attempt")
            and not binding.get("pending_native_intent"), "ordinary repair prior binding is unsettled")
    objects, history = binding.get("native_objects"), binding.get("receipts")
    require(type(objects) is list and all(type(item) is dict and item.get("kind") != "grok_enabled_plugin" for item in objects)
            and type(history) is list and history and all(type(row) is dict and type(row.get("sequence")) is int
            and 0 < row["sequence"] < 1 << 31 for row in history), "ordinary repair prior ownership differs")
    managed = [item for item in objects if item.get("kind") == "managed_package_directory"]
    require(len(managed) == 1 and type(managed[0].get("managed_digest")) is str
            and re.fullmatch(r"sha256:[0-9a-f]{64}", managed[0]["managed_digest"]), "ordinary repair package digest differs")
    digest = managed[0]["managed_digest"]
    require(managed[0] == dict(object_id="package:grok:" + binding["physical_artifact_id"], kind="managed_package_directory",
            logical_name=installation["declared_name"], path=str(active), managed_digest=digest, protection_class="managed")
            and history[-1].get("after_digest") == digest and history[-1].get("phase") == "committed"
            and group != installation.get("operation_group_id")
            and all(row.get("operation_group_id") != group for row in history), "ordinary repair package or group authority differs")
    require(type(before) is dict and type(after) is dict and type(before.get("installations")) is list,
            "ordinary repair state shape differs")
    indices = [index for index, item in enumerate(before["installations"]) if item == installation]
    require(len(indices) == 1 and type(after.get("installations")) is list and len(after["installations"]) == len(before["installations"]),
            "ordinary repair selected state differs")
    current = after["installations"][indices[0]]
    require(type(current) is dict and type(current.get("clients")) is dict and type(current["clients"].get(key)) is dict,
            "ordinary repair selected binding missing")
    stamp = current.get("updated_at")
    observed = timestamp_ns(stamp)
    require(type(window) is tuple and len(window) == 2 and all(type(value) is int for value in window)
            and 0 <= window[0] <= observed <= window[1]
            and observed >= max(timestamp_ns(installation.get("updated_at")), timestamp_ns(binding.get("updated_at")))
            and current["clients"][key].get("updated_at") == stamp, "ordinary repair timestamp authority differs")
    operation = group + "-001"
    suffix = hashlib.sha256(operation.encode()).hexdigest()[:16]
    receipt = dict(operation_id=operation, operation_group_id=group, sequence=max(row["sequence"] for row in history) + 1,
        mutation_type="directory_swap", client_binding_id=key, active_path=str(active),
        staging_path=str(active.parent.parent / (".agentplugins-staging-" + suffix)),
        backup_path=str(active.parent / (".agentplugins-backup-" + suffix)), before_digest=digest, after_digest=digest, phase="committed")
    expected = copy.deepcopy(before)
    selected = expected["installations"][indices[0]]
    selected["operation_group_id"], selected["updated_at"] = group, stamp
    selected["clients"][key]["updated_at"] = stamp
    selected["clients"][key]["receipts"].append(receipt)
    require(json.dumps(after, sort_keys=True) == json.dumps(expected, sort_keys=True),
            "ordinary repair changed state beyond its exact committed directory delta")
    return dict(operation_id=group, receipt=receipt, updated_at=stamp, started_at_unix_ns=window[0], finished_at_unix_ns=window[1])


def preview_enablement(journey, installation):
    from grok_project_protocol_e2e import snapshot
    config = config_bytes(journey)
    current, binding, active, data = selected_installation(journey.a, installation["declared_name"])
    require(current == installation, "ordinary repair baseline selection differs")
    state_path = journey.a / ".uap/state-v2.json"
    before = read_owned(state_path, journey.root)
    roots = (active, data, journey.source, journey.b)
    preserved = [snapshot(root) for root in roots]
    original_events = read_owned(journey.log, journey.root) if journey.log.exists() else None
    def unchanged_native_and_files():
        journey.check()
        require(config_bytes(journey) == config and [snapshot(root) for root in roots] == preserved,
                "repair changed native config, package, data, source or Project B")
        current_events = read_owned(journey.log, journey.root) if journey.log.exists() else None
        require(current_events == original_events, "repair preview executed the MCP witness")
    started = time.time_ns()
    try:
        response = journey.cli("repair-no-enable-consent", "repair", installation["installation_id"], extra=("--yes",))
    except Exception:
        # An unsuccessful public repair has no accepted commit receipt. Refuse
        # if it nevertheless touched native files, the witness, or saved state.
        unchanged_native_and_files()
        require(read_owned(state_path, journey.root) == before,
                "failed ordinary repair changed saved ownership")
        raise
    finished = time.time_ns()
    after = read_owned(state_path, journey.root)
    proof = ordinary_repair_state(document(before), document(after), installation, binding, active, response, (started, finished))
    unchanged_native_and_files()
    journey.evidence["ordinary_repair_observation"] = {
        "partial": True,
        "no_consent_config_unchanged": True,
        "no_consent_exact_directory_repair": proof,
        "post_repair_state_base64": base64.b64encode(after).decode(),
        "post_repair_state_bytes": len(after),
        "before_state_sha256": hashlib.sha256(before).hexdigest(),
        "after_state_sha256": hashlib.sha256(after).hexdigest(),
        "mcp_witness_execution_observed": False,
        "internal_acp_zero_proved_by_this_capture": False,
    }
    try:
        journey.cli("repair-enable-dry-run", "repair", installation["installation_id"],
                    extra=("--confirm-plugin-enable", "--dry-run", "--yes"))
    finally:
        # A native security refusal still needs full negative no-effect proof.
        unchanged_native_and_files()
        require(read_owned(state_path, journey.root) == after, "repair preview changed saved ownership")
    journey.evidence["enablement_previews"] = {
        "no_consent_config_unchanged": True,
        "no_consent_exact_directory_repair": proof,
        "post_repair_state": document(after),
        "before_state_sha256": hashlib.sha256(before).hexdigest(),
        "after_state_sha256": hashlib.sha256(after).hexdigest(),
        "dry_run_config_and_state_unchanged": True,
        "mcp_witness_execution_observed": False,
        "internal_acp_zero_proved_by_this_capture": False,
    }


def enable_owned(journey, installation, active, name):
    before = config_bytes(journey)
    require(before.count(FOREIGN_ENABLED) == 1, "authored enablement baseline differs")
    full_id = native_id(active, name)
    after = before.replace(FOREIGN_ENABLED,
        FOREIGN_ENABLED[:-1] + b", " + json.dumps(full_id).encode() + b"]", 1)
    journey.cli("repair-enable-owned-full-id", "repair", installation["installation_id"],
                extra=("--confirm-plugin-enable", "--yes"), config_after=after)
    current, binding, path, _ = selected_installation(journey.a, name)
    require(current["installation_id"] == installation["installation_id"] and path == active,
            "enablement selected a different installation")
    expected = {"object_id": "grok-enabled:" + hashlib.sha256(str(journey.home / ".grok").encode()).hexdigest()
                + ":" + full_id, "kind": "grok_enabled_plugin", "logical_name": name,
                "path": str(active), "before_digest": "sha256:" + hashlib.sha256(before).hexdigest(),
                "managed_digest": "sha256:" + hashlib.sha256(after).hexdigest(),
                "protection_class": "native_config_member"}
    require(enabled_receipt(binding) == expected, "public repair did not own the exact full native ID")
    journey.enabled_receipt = expected
    journey.cli("repair-repeat-owned-full-id", "repair", installation["installation_id"],
                extra=("--confirm-plugin-enable", "--yes"))
    repeated, repeated_binding, repeated_path, _ = selected_installation(journey.a, name)
    require(repeated["installation_id"] == installation["installation_id"] and repeated_path == active
            and config_bytes(journey) == after, "repeat changed config or selected identity")
    expect_retained_receipt(journey, repeated_binding)
    journey.evidence["owned_enablement"] = {"public_command": "repair", "scope": "project",
        "saved_dependency": False, "environment_authority": None, "native_full_id": full_id,
        "receipt": expected, "repeat_retained_receipt": True, "activation": "prepared"}


def remove_owned(journey, installation, active, name):
    from grok_project_protocol_e2e import digest, write
    _, binding, current_path, _ = selected_installation(journey.a, name)
    require(current_path == active, "removal selected a different artifact")
    expect_retained_receipt(journey, binding)
    before = config_bytes(journey)
    full_id = native_id(active, name)
    token = json.dumps(full_id).encode()
    require(before.count(token) == 1 and before.count(b"[plugins]\n") == 1
            and before.count(token + b"]") == 1, "owned removal fixture membership differs")
    # A later explicit fixture edit adds foreign data after the recorded receipt.
    # The product must remove its current full ID without restoring old bytes.
    foreign = b'[uap_fixture_foreign]\nkeep = "later-user-edit"\n'
    changed = before.replace(token + b"]", token + b', "uap-foreign-later"]', 1)
    changed = changed.replace(b"[plugins]\n", foreign + b"[plugins]\n", 1)
    write(config_path(journey), changed.decode())
    journey.protected[config_path(journey)] = digest(config_path(journey))
    expected = changed.replace(token + b",", b"", 1)
    journey.cli("remove-owned-full-id", "remove", installation["installation_id"], config_after=expected)
    journey.evidence["owned_enablement_removal"] = {
        "removed_native_full_id": full_id, "foreign_later_bytes_preserved": True,
        "before_sha256": hashlib.sha256(changed).hexdigest(),
        "after_sha256": hashlib.sha256(expected).hexdigest()}

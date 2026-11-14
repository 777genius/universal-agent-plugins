"""Assertions for the public, unmanaged Grok Project repair lifecycle.

These helpers author only the existing disposable trust/foreign fixtures. The
ordinary installer owns every enabled-member insertion and removal.
"""
import hashlib
import json

from grok_project_protocol_client import read_owned, require, selected_installation

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


def preview_enablement(journey, installation):
    before = config_bytes(journey)
    state_path = journey.a / ".uap/state-v2.json"
    state = read_owned(state_path, journey.root)
    original_events = read_owned(journey.log, journey.root) if journey.log.exists() else None
    for label, extra in (("repair-no-enable-consent", ("--yes",)),
                         ("repair-enable-dry-run", ("--confirm-plugin-enable", "--dry-run", "--yes"))):
        journey.cli(label, "repair", installation["installation_id"], extra=extra)
        require(config_bytes(journey) == before and read_owned(state_path, journey.root) == state,
                "repair preview changed native config or saved ownership")
        current_events = read_owned(journey.log, journey.root) if journey.log.exists() else None
        require(current_events == original_events, "repair preview executed the MCP witness")
    journey.evidence["enablement_previews"] = {
        "no_consent_config_and_state_unchanged": True,
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

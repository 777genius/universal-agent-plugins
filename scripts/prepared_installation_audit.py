"""Exact Cursor/Grok preparation evidence; never native activation qualification.

Cursor's Project Skill discovery needs a signed-in session, and its User
plugin adapter reports manual activation. These opt-in installation lanes
keep the native audit's original results and count unchanged. Grok's native
bundle preparation is likewise separate from native discovery or execution.
"""

import hashlib
import re
from pathlib import PurePosixPath


ACCEPTANCES = {
    "cursor-prepared-installation": "plugin",
    "cursor-prepared-skills-installation": "skill",
    "grok-prepared-skills-installation": "skill",
}
ROUTES = {mode: ("grok" if mode.startswith("grok-") else "cursor", kind)
          for mode, kind in ACCEPTANCES.items()}
GROK_VERSION = "grok 1.0.70 (f50904eaee6c)"
VERSIONS = {
    "plugin": "3.30.14\n5f54c8173e5552412beb40702adac16275677fa0\nx64",
    "skill": "2026.10.22-27dcf5f",
}
COMMANDS = ("install", "list", "repeat_add", "repeat_list", "cleanup", "post_remove_list")


def grok_skill_identity(fixture, installation_id):
    """Mirror discovery, SourceKey, synthetic manifest and physical ID formulas.

    The existing source verifier remains authoritative for filesystem, lock
    and revision ownership. These derivations additionally bind its public
    candidate, synthetic package and physical bundle to the selected fixture.
    """
    if (type(fixture) is not dict
            or any(type(fixture.get(key)) is not str or not fixture[key]
                   for key in ("repo", "ref", "name", "folder"))
            or re.fullmatch(r"[a-f0-9]{40}", fixture["ref"]) is None
            or type(installation_id) is not str
            or re.fullmatch(r"[a-f0-9]{8}(?:-[a-f0-9]{4}){3}-[a-f0-9]{12}", installation_id) is None):
        return None
    path = "skills/" + fixture["folder"]
    candidate = "sha256:" + hashlib.sha256(
        "\0".join((fixture["repo"], fixture["ref"], path)).encode()).hexdigest()
    source_key = "github-skill:" + fixture["repo"].lower() + ":" + fixture["name"] + ":" + path
    name = "skill-" + hashlib.sha256(source_key.encode()).hexdigest()[:56]
    physical = name[:51].rstrip(".-") + "-" + hashlib.sha256(installation_id.encode()).hexdigest()[:12]
    return candidate, name, physical


def observe(kind, identity, responses, selected_before, selected_after, target="cursor", skill_fixture=None):
    """Inspect complete decoded public envelopes, never bounded stdout tails.

    The caller supplies independently source-verified selections from its
    existing Cursor closure matcher or Grok native bundle verifier. Every
    inventory binding must retain its scope; removal covers the full closure.
    """
    result = {"status": "unqualified"}
    if ((target, kind) not in ROUTES.values() or type(identity) is not str
            or not identity or type(responses) is not dict):
        return result
    if target == "grok" and (type(skill_fixture) is not dict or skill_fixture.get("name") != identity):
        return result
    scope = "project" if kind == "skill" else "user"
    expected_names = ({"marketing-kit", "notion", "hubspot-crm", "playwright"}
                      if kind == "plugin" and identity == "marketing-kit" else {identity})
    for phase in COMMANDS:
        document = responses.get(phase)
        if (type(document) is not dict or not exact(document.get("schema_version"), 1)
                or document.get("result") != "success"):
            return result
    inventories = []
    for phase, selected in (("list", selected_before), ("repeat_list", selected_after)):
        document = responses[phase]
        rows = document.get("data", {}).get("installations") if type(document.get("data")) is dict else None
        if (document.get("command") != "list" or type(rows) is not list
                or len(rows) != len(expected_names)
                or type(selected) is not list or len(selected) != len(rows)
                or any(node not in rows for node in selected)):
            return result
        ids, names = set(), set()
        for node in rows:
            if (type(node) is not dict or type(node.get("installation_id")) is not str
                    or not node["installation_id"] or node["installation_id"] in ids
                    or type(node.get("clients")) is not list or len(node["clients"]) != 1
                    or node.get("mixed_version") is not False):
                return result
            client = node["clients"][0]
            if (type(client) is not dict or client.get("client_id") != target
                    or client.get("scope") != scope):
                return result
            # Preserve exact public state, including pending native work.
            state = {key: client.get(key) for key in (
                "materialization", "activation", "authentication", "policy", "verification")}
            if not exact(state, expected_state(kind, target)):
                return result
            if target == "grok":
                derived = grok_skill_identity(skill_fixture, node["installation_id"])
                revision = client.get("package_revision")
                if (derived is None or node.get("name") != derived[1]
                        or node.get("source") != skill_fixture["repo"] + "/skills/" + skill_fixture["folder"]
                        or type(revision) is not dict or revision.get("resolved_revision") != skill_fixture["ref"]):
                    return result
            ids.add(node["installation_id"])
            if kind == "plugin":
                if type(node.get("name")) is not str or node["name"] in names:
                    return result
                names.add(node["name"])
        if kind == "plugin" and names != expected_names:
            return result
        if ({node["installation_id"] for node in selected} != ids
                or (kind == "plugin" and selected[0].get("name") != identity)):
            return result
        inventories.append(ids)
    if (inventories[0] != inventories[1]
            or selected_before[0]["installation_id"] != selected_after[0]["installation_id"]):
        return result
    bundle_metadata = None
    for phase, repeated in (("install", False), ("repeat_add", True)):
        document = responses[phase]
        if kind == "skill":
            items, actions = document.get("items"), document.get("next_actions")
            if (document.get("command") != "skills add" or document.get("status") != "succeeded"
                    or document.get("scope") != scope or document.get("scope_selection_reason") != "explicit"
                    or document.get("readiness") != "needs_action"
                    or document.get("repairInstructions") != []
                    or type(items) is not list or len(items) != 1 or type(items[0]) is not dict
                    or items[0].get("alias") != identity or items[0].get("targetId") != target
                    or items[0].get("status") != ("unchanged" if repeated else "succeeded")
                    or type(actions) is not list or len(actions) != 1 or type(actions[0]) is not dict
                    or actions[0].get("code") != "verify_native"
                    or actions[0].get("actor") != "native_client"):
                return result
            if target == "grok":
                derived = grok_skill_identity(skill_fixture, selected_before[0]["installation_id"])
                candidate, destination = items[0].get("candidateId"), items[0].get("destinationPath")
                if (derived is None or candidate != derived[0] or type(destination) is not str):
                    return result
                bundle = PurePosixPath(destination)
                if (not bundle.is_absolute() or str(bundle) != destination
                        or any(part in (".", "..") for part in bundle.parts)
                        or bundle.parts[-3:-1] != (".grok", "plugins")
                        or bundle.name != derived[2]):
                    return result
                metadata = (candidate, destination)
                if repeated and metadata != bundle_metadata:
                    return result
                bundle_metadata = metadata
        else:
            data = document.get("data")
            if (document.get("command") != "add" or type(data) is not dict
                    or not scoped_operation(data, "add", scope)):
                return result
            environment = data.get("environment")
            actions, outcomes = data.get("actions"), data.get("results")
            if (type(environment) is not dict or environment.get("scope") != scope
                    or environment.get("scope_selection_reason") != "explicit"
                    or environment.get("selected_targets") != [target]
                    or type(actions) is not list or len(actions) != len(expected_names)
                    or any(type(action) is not dict or action.get("client_id") != target
                           or type(action.get("alias")) is not str
                           or action.get("action") != ("unchanged" if repeated else "install")
                           for action in actions)
                    or {action.get("alias") for action in actions} != expected_names
                    or (outcomes != [] if repeated else not completed_ids(outcomes, "install", inventories[0], target))):
                return result
            if repeated and (any(type(action.get("installation_id")) is not str for action in actions)
                             or {action["installation_id"] for action in actions} != inventories[0]):
                return result
    removal = responses["cleanup"]
    if (not scoped_operation(removal, "remove", scope)
            or not completed_ids(removal.get("results"), "remove", inventories[0], target)):
        return result
    actions = removal.get("actions")
    if (type(actions) is not list or len(actions) != len(inventories[0])
            or any(type(action) is not dict or action.get("client_id") != target
                   or type(action.get("installation_id")) is not str
                   or action.get("action") != "remove" for action in actions)
            or {action.get("installation_id") for action in actions} != inventories[0]):
        return result
    after = responses["post_remove_list"]
    if (after.get("command") != "list" or type(after.get("data")) is not dict
            or after["data"].get("installations") != []):
        return result
    return {"status": "verified", "scope": scope,
            "selected_installation_id": selected_before[0]["installation_id"],
            "removed_installation_ids": sorted(inventories[0]),
            **({"skill_path": ".grok/plugins/" + PurePosixPath(bundle_metadata[1]).name
                              + "/skills/" + identity + "/SKILL.md"} if target == "grok" else {})}


def scoped_operation(document, operation, scope):
    return (type(document) is dict and exact(document.get("schema_version"), 1)
            and document.get("command") == "agentplugins " + operation
            and document.get("result") == "success" and document.get("outcome") == "completed"
            and document.get("scope") == scope and document.get("scope_selection_reason") == "explicit")


def completed_ids(rows, operation, ids, target="cursor"):
    return (type(rows) is list and len(rows) == len(ids)
            and all(type(row) is dict and row.get("client_id") == target
                    and row.get("operation") == operation and row.get("outcome") == "completed"
                    and type(row.get("installation_id")) is str for row in rows)
            and {row["installation_id"] for row in rows} == ids)


def exact(value, expected):
    """Keep booleans, integers and missing fields distinct in evidence."""
    if type(value) is not type(expected):
        return False
    if type(expected) is dict:
        return value.keys() == expected.keys() and all(
            exact(value[key], item) for key, item in expected.items())
    if type(expected) is list:
        return len(value) == len(expected) and all(
            exact(left, right) for left, right in zip(value, expected))
    return value == expected


def expected_state(kind, target="cursor"):
    if target == "grok":
        return {"materialization": "materialized", "policy": "allowed",
                "activation": "prepared", "authentication": "not_checked",
                "verification": "package_validated"}
    return {"materialization": "materialized", "policy": "allowed",
             "activation": "prepared" if kind == "skill" else "manual_activation_required",
             "authentication": "not_required" if kind == "skill" else "not_checked",
             "verification": "installation_verified" if kind == "skill" else "package_validated"}


def public_states(row, kind, target="cursor"):
    state = expected_state(kind, target)
    expected = {}
    for phase in ("after-add", "repeat-add", "after-remove"):
        active = phase != "after-remove"
        expected[phase] = {"status": "observed", "selected_installations": int(active),
                           "selected_bindings": int(active), "invalid_rows": 0,
                           "states": [state] if active else [], "truncated": False}
    return exact(row.get("public_target_states"), expected)


def lifecycle(row, target="cursor"):
    if (any(row.get(key) is not True for key in (
            "listed_identity", "repeat_verified", "repeat_native_unchanged",
            "repeat_lock_unchanged", "removal_verified", "post_list_schema_valid",
            "sentinels_preserved"))
            or any(row.get(key) is not False for key in (
                "repeat_native_runtime_only", "failed_install_registered",
                "failed_install_residue", "post_remove_identity_registered"))
            or row.get("native_residuals") != []
            or row.get("post_removal_leftovers") != []):
        return False
    for phase in COMMANDS:
        command = row.get(phase)
        if (type(command) is not dict or type(command.get("exit_code")) is not int
                or command["exit_code"] != 0 or command.get("timeout") is not False):
            return False
    ids = row.get("matched_installation_ids")
    proof = row.get(target + "_preparation_envelopes")
    removed = proof.get("removed_installation_ids") if type(proof) is dict else None
    expected_count = 4 if row.get("kind") == "plugin" and row.get("identity") == "marketing-kit" else 1
    return (type(ids) is list and len(ids) == 1 and type(ids[0]) is str and bool(ids[0])
            and row.get("repeat_installation_ids") == ids
            and type(proof) is dict and proof.get("status") == "verified"
            and proof.get("scope") == ("project" if row.get("kind") == "skill" else "user")
            and proof.get("selected_installation_id") == ids[0]
            and (target != "grok" or proof.get("skill_path") == row.get("skill_path"))
            and type(removed) is list and len(removed) == expected_count
            and all(type(value) is str and bool(value) for value in removed)
            and len(set(removed)) == expected_count and ids[0] in removed)


def skill_preparation(row, item, target="cursor"):
    grok = target == "grok"
    path = row.get("skill_path")
    path_matches = (type(path) is str and re.fullmatch(
        r"\.grok/plugins/skill-[a-f0-9]{45}-[a-f0-9]{12}/skills/"
        + re.escape(item["name"]) + r"/SKILL\.md", path) is not None
        if grok else path == f'.cursor/skills/{item["name"]}/SKILL.md')
    return (row.get("status") == ("prepared-lifecycle-passed" if grok else "incomplete-native-proof")
            and exact(row.get("public_skill_ready"), {"after-add": False, "repeat-add": False})
            and path_matches
            and row.get("prepared_binding") is grok
            and row.get("prepared_components_only") is False
            and type(row.get("native_file_delta")) is int
            and (row["native_file_delta"] > 0 if grok else row["native_file_delta"] == 0)
            and row.get("native_identity_files") == []
            and row.get("native_component_candidates") == []
            and row.get("signed_release_observation") == []
            and row.get("post_skill_tree_delta") == []
            and "post_removal_skill" in row and row["post_removal_skill"] is None
            and row.get("post_lock_pinned") is False
            and (not grok or row.get("retained_passive_native_files") == [])
            and "repeat_project_unchanged" in row
            and (row["repeat_project_unchanged"] is True if grok else row["repeat_project_unchanged"] is None)
            and all(row.get(key) is True for key in (
                "lock_written", "lock_contains_identity", "lock_exact_source_pinned", "skill_bytes_match")))


def plugin_preparation(row, item, fixtures):
    if (row.get("status") != "prepared-lifecycle-passed"
            or row.get("prepared_binding") is not True
            or type(row.get("prepared_components_only")) is not bool
            or "public_skill_ready" not in row or row["public_skill_ready"] is not None
            or type(row.get("native_file_delta")) is not int or row["native_file_delta"] <= 0
            or type(row.get("native_identity_files")) is not list
            or not row["native_identity_files"]
            or any(type(path) is not str or not path for path in row["native_identity_files"])):
        return False
    if item.startswith("github:"):
        # do_case's matching_installations already verifies the immutable Git
        # selector. The exact fixture and successful matching remain required.
        return row.get("signed_release_observation") == []
    pin = fixtures.get("signed_release_pins", {}).get(item)
    if type(pin) is not dict:
        return False
    expected = {"source": pin["repository"].strip("/") + "/" + pin["path"].strip("/"),
                "recorded_distribution": pin["distribution"], "recorded_revision": pin["revision"],
                "recorded_release_sequence": pin["sequence"], "resolved_revision": pin["revision"],
                "release_sequence": pin["sequence"], "tree_digest": pin["tree_digest"],
                "manifest_digest": pin["manifest_digest"]}
    return exact(row.get("signed_release_observation"), [expected])


def summary(report, fixtures, acceptance):
    """Require all 30 source-bound lifecycles, leaving native PASS at zero."""
    result = {"installation_passed": 0, "installation_acceptance_passed": False,
              "qualification_scope": "owned Cursor preparation lifecycle",
              "native_discovery_qualified": False, "native_activation_qualified": False,
              "native_mcp_qualified": False, "model_execution_qualified": False,
              "authenticated_execution_qualified": False}
    route = ROUTES.get(acceptance)
    if route is None:
        return result
    target, kind = route
    result["qualification_scope"] = "owned " + ("Grok" if target == "grok" else "Cursor") + " preparation lifecycle"
    sources = fixtures.get("skills" if kind == "skill" else "plugin_aliases")
    rows = report.get("results")
    if ((report.get("target"), report.get("kind"), report.get("platform"), report.get("architecture"))
            != (target, kind, "linux", "x86_64")
            or not exact(report.get("client_version"),
                         {"status": "reported", "value": GROK_VERSION if target == "grok" else VERSIONS[kind], "exit_code": 0})
            or not exact(report.get("cli_version"),
                         {"status": "reported", "value": "agentplugins 0.1.115", "exit_code": 0})
            or any(not exact(report.get(key), 30) for key in ("fixture_count", "cases_attempted"))
            or report.get("source_selection_complete") is not True
            or report.get("full_coverage") is not True
            or not exact(report.get("passed"), 0)
            or not exact(report.get("verified_negative_coverage"), 0)
            or not exact(report.get("prepared_lifecycle_passed"), 30 if kind == "plugin" or target == "grok" else 0)
            or type(rows) is not list or len(rows) != 30
            or type(sources) is not list or len(sources) != 30):
        return result
    for index, (row, item) in enumerate(zip(rows, sources)):
        if kind == "skill":
            if type(item) is not dict or type(item.get("source")) is not str or type(item.get("name")) is not str:
                return result
            source, identity = item["source"], item["name"]
        else:
            if type(item) is not str:
                return result
            source, identity = item, fixtures["plugin_declared_names"].get(item, item)
        if (type(row) is not dict or not exact(row.get("index"), index)
                or (row.get("target"), row.get("kind")) != (target, kind)
                or row.get("source") != source or row.get("identity") != identity
                or not lifecycle(row, target) or not public_states(row, kind, target)
                or not (skill_preparation(row, item, target) if kind == "skill"
                        else plugin_preparation(row, item, fixtures))):
            return result
        result["installation_passed"] += 1
    result["installation_acceptance_passed"] = result["installation_passed"] == 30
    return result

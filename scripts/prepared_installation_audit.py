"""Exact Cursor/Grok preparation evidence; never native activation qualification.

Cursor's Project Skill discovery needs a signed-in session, and its User
plugin adapter reports manual activation. These opt-in installation lanes
keep the native audit's original results and count unchanged. Grok's native
bundle preparation is likewise separate from native discovery or execution.
"""

import hashlib
import copy
import json
import os
import re
import stat
import sys
from pathlib import Path, PurePosixPath


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
KIRO_ACCEPTANCES = {"kiro-installation": "plugin", "kiro-skills-installation": "skill"}
KIRO_ROUTES = {mode: ("kiro", kind) for mode, kind in KIRO_ACCEPTANCES.items()}
KIRO_ACTIVE = {22: "sre-extension", 23: "itential-builder", 25: "claude-scientific-writer",
               26: "elements-of-style", 28: "uizze-ui-slop", 29: "recipe-skills"}


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


def observe(kind, identity, responses, selected_before, selected_after, target="cursor", skill_fixture=None,
            kiro_active=False):
    """Inspect complete decoded public envelopes, never bounded stdout tails.

    The caller supplies independently source-verified selections from its
    existing Cursor closure matcher or Grok native bundle verifier. Every
    inventory binding must retain its scope; removal covers the full closure.
    """
    result = {"status": "unqualified"}
    if ((target, kind) not in (*ROUTES.values(), *KIRO_ROUTES.values()) or type(identity) is not str
            or not identity or type(responses) is not dict):
        return result
    if target in ("grok", "kiro") and kind == "skill" and (type(skill_fixture) is not dict or skill_fixture.get("name") != identity):
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
            expected = expected_state(kind, target)
            if target == "kiro" and kiro_active and kind == "plugin":
                expected = {**expected, "activation": "active", "verification": "installation_verified"}
            if not exact(state, expected):
                return result
            if target in ("grok", "kiro") and kind == "skill":
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
            if target in ("grok", "kiro"):
                derived = grok_skill_identity(skill_fixture, selected_before[0]["installation_id"])
                candidate, destination = items[0].get("candidateId"), items[0].get("destinationPath")
                if (derived is None or candidate != derived[0] or type(destination) is not str):
                    return result
                bundle = PurePosixPath(destination)
                if (not bundle.is_absolute() or str(bundle) != destination
                        or any(part in (".", "..") for part in bundle.parts)
                        or (bundle.parts[-3:-1] != (".grok", "plugins") if target == "grok"
                            else bundle.parts[-5:-1] != (".uap", "managed", "clients", "kiro"))
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
            if target == "kiro" and not repeated and any(
                    "installation_id" in action and action["installation_id"] != selected_before[0]["installation_id"]
                    for action in actions):
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
                              + "/skills/" + identity + "/SKILL.md"} if target == "grok" else
               {"skill_path": ".kiro/skills/" + identity + "/SKILL.md"} if target == "kiro" and kind == "skill" else {})}


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
    if target == "kiro":
        return {"materialization": "materialized", "policy": "allowed", "activation": "prepared",
                "authentication": "not_checked", "verification": "installation_verified" if kind == "skill" else "package_validated"}
    if target == "grok":
        return {"materialization": "materialized", "policy": "allowed",
                "activation": "prepared", "authentication": "not_checked",
                "verification": "package_validated"}
    return {"materialization": "materialized", "policy": "allowed",
             "activation": "prepared" if kind == "skill" else "manual_activation_required",
             "authentication": "not_required" if kind == "skill" else "not_checked",
             "verification": "installation_verified" if kind == "skill" else "package_validated"}


def public_states(row, kind, target="cursor", kiro_active=False):
    state = expected_state(kind, target)
    if target == "kiro" and kiro_active and kind == "plugin":
        state = {**state, "activation": "active", "verification": "installation_verified"}
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
        if grok else path == f'.{target}/skills/{item["name"]}/SKILL.md')
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


def kiro_require(condition):
    if not condition:
        raise ValueError("Kiro installation evidence is unresolved")


def kiro_digest(value):
    body = json.dumps(value, sort_keys=True, separators=(",", ":"), allow_nan=False).encode()
    kiro_require(len(body) <= 4 << 20)
    return hashlib.sha256(body).hexdigest()


def kiro_unique_pairs(pairs):
    result = {}
    for key, value in pairs:
        kiro_require(key not in result)
        result[key] = value
    return result


def kiro_nonfinite(value):
    raise ValueError("Kiro shared configuration has a non-finite number")


class KiroInstallationAudit:
    """Hold complete HOME and workspace-native images through all public calls.

    The explicit foreign MCP fixture starts with a real shared document.
    Kiro's atomic MCP writer may replace only that User document's inode and
    mtime; its exact original bytes, mode and owner must return after removal.
    All other files, directories and unknown cache leaves remain protected.
    """

    def __init__(self, cli, env, project, case_root, audit, kind):
        kiro_require(sys.platform == "darwin" and kind in ("plugin", "skill")
                     and type(cli) is str and Path(cli).is_absolute())
        self.cli, self.audit, self.kind = cli, audit, kind
        self.case, self.project = Path(case_root), Path(project)
        self.home, self.data = self.case / "home", self.case / "uap-data"
        kiro_require(self.case.resolve(strict=True) == self.case
                     and self.project == self.case / "project"
                     and env.get("HOME") == env.get("USERPROFILE") == str(self.home)
                     and env.get("AGENTPLUGINS_HOME") == str(self.data)
                     and all(env.get(key) == str(self.home / relative) for key, relative in (
                         ("XDG_CONFIG_HOME", ".config"), ("XDG_CACHE_HOME", ".cache"),
                         ("XDG_DATA_HOME", ".local/share"), ("XDG_STATE_HOME", ".local/state"))))
        for base in (self.home / ".kiro", self.project / ".kiro"):
            for relative in ("settings", "skills/foreign-neighbor"):
                path = base / relative
                kiro_require(self.audit["path_inside_case"](path, self.case))
                path.mkdir(parents=True, exist_ok=True)
            foreign = base / "skills/foreign-neighbor/SKILL.md"
            if not foreign.exists():
                foreign.write_text("---\nname: foreign-neighbor\ndescription: unrelated\n---\n")
            with (base / "settings/mcp.json").open("xb") as stream:
                stream.write(b'{"mcpServers":{"uap-foreign-neighbor":{"command":"/unmanaged/never-executed"}}}\n')
        self.roots = (self.home, self.project / ".kiro")
        self.anchors = {path: self.directory(path) for path in
                        (self.case, self.home, self.project, self.data,
                         *(base / relative for base in (self.home / ".kiro", self.project / ".kiro")
                           for relative in (".", "settings", "skills", "skills/foreign-neighbor")))}
        self.mcp = self.home / ".kiro/settings/mcp.json"
        if kind == "plugin":
            # BeginExactFile retains this empty coordination file after unlock.
            # Seed it before custody, then hold it like every other initial file.
            with self.mcp.with_name(self.mcp.name + ".agentplugins.lock").open("xb") as stream:
                os.fchmod(stream.fileno(), 0o600)
        info = self.mcp.lstat()
        self.mcp_owner = (info.st_mode, info.st_uid, info.st_gid, info.st_dev, info.st_nlink)
        self.observations = []
        self.mcp_images, self.skill_blobs = [], []
        self.deliveries = []
        self.skill_name = None
        self.protected_files = {}
        self.initial_paths = set()
        self.mcp_fd = os.open(self.mcp, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK)
        try:
            self.original_stat = os.fstat(self.mcp_fd)
            self.original_bytes = self.held_bytes(self.mcp_fd)
            for root in self.roots:
                for path in (root, *root.rglob("*")):
                    kiro_require(self.audit["path_inside_case"](path, self.case))
                    self.initial_paths.add(path)
                    if path.is_dir():
                        self.anchors[path] = self.directory(path)
                    elif path != self.mcp:
                        kiro_require(len(self.protected_files) < 128)
                        fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK)
                        try:
                            info, body = os.fstat(fd), self.held_bytes(fd)
                        except BaseException:
                            os.close(fd)
                            raise
                        self.protected_files[path] = (fd, info, body)
            self.initial = self.image()
        except BaseException:
            self.close()
            raise

    def close(self):
        for fd, _, _ in self.protected_files.values():
            os.close(fd)
        self.protected_files.clear()
        if self.mcp_fd is not None:
            os.close(self.mcp_fd)
            self.mcp_fd = None

    @staticmethod
    def file_identity(info):
        return (info.st_mode, info.st_dev, info.st_ino, info.st_uid, info.st_gid,
                info.st_size, info.st_mtime_ns)

    @staticmethod
    def held_bytes(fd):
        before = os.fstat(fd)
        kiro_require(stat.S_ISREG(before.st_mode) and before.st_nlink in (0, 1) and before.st_size <= 1 << 20)
        body = os.pread(fd, (1 << 20) + 1, 0)
        after = os.fstat(fd)
        kiro_require(len(body) == before.st_size and KiroInstallationAudit.file_identity(after)
                     == KiroInstallationAudit.file_identity(before) and after.st_nlink == before.st_nlink)
        return body

    @staticmethod
    def directory(path):
        info = path.lstat()
        kiro_require(stat.S_ISDIR(info.st_mode) and path.resolve(strict=True) == path)
        return info.st_mode, info.st_dev, info.st_ino, info.st_uid, info.st_gid

    def guard(self):
        kiro_require(all(self.directory(path) == identity for path, identity in self.anchors.items()))
        info = self.mcp.lstat()
        kiro_require(stat.S_ISREG(info.st_mode) and self.mcp.resolve(strict=True) == self.mcp
                     and (info.st_mode, info.st_uid, info.st_gid, info.st_dev, info.st_nlink) == self.mcp_owner)
        kiro_require(self.mcp_fd is not None and self.file_identity(os.fstat(self.mcp_fd)) == self.file_identity(self.original_stat)
                     and self.held_bytes(self.mcp_fd) == self.original_bytes)
        for path, (fd, expected, body) in self.protected_files.items():
            current, held = path.lstat(), os.fstat(fd)
            kiro_require(self.file_identity(current) == self.file_identity(held) == self.file_identity(expected)
                         and current.st_nlink == held.st_nlink == expected.st_nlink == 1
                         and self.held_bytes(fd) == body)
        kiro_require(all(self.audit["path_inside_case"](root, self.case) for root in self.roots))

    def image(self):
        self.guard()
        image = self.audit["scoped_snapshot"](self.roots)
        kiro_require(type(image) is dict
                     and not any(key.startswith(("link:", "root-link:")) for key in image)
                     and not any(value in ("unreadable", "symlink", "non-regular") for value in image.values()))
        kiro_digest(image)
        fd = os.open(self.mcp, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK)
        try:
            info = os.fstat(fd)
            body = self.held_bytes(fd)
            kiro_require(self.file_identity(self.mcp.lstat()) == self.file_identity(info) and hashlib.sha256(body).hexdigest()
                         == image.get("home/.kiro/settings/mcp.json"))
            parsed = json.loads(body, object_pairs_hook=kiro_unique_pairs, parse_constant=kiro_nonfinite)
            original = json.loads(self.original_bytes)
            kiro_require(type(parsed) is dict and type(parsed.get("mcpServers")) is dict
                         and parsed.keys() == original.keys()
                         and all(exact(parsed[key], original[key]) for key in original if key != "mcpServers")
                         and parsed["mcpServers"].get("uap-foreign-neighbor")
                         == {"command": "/unmanaged/never-executed"})
            self.current_mcp = parsed
        finally:
            os.close(fd)
        self.guard()
        return image

    def read_bytes(self, path):
        kiro_require(path.is_absolute() and self.audit["path_inside_case"](path, self.case))
        fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK)
        try:
            info = os.fstat(fd)
            kiro_require(info.st_nlink == 1)
            body = self.held_bytes(fd)
            kiro_require(self.file_identity(path.lstat()) == self.file_identity(info))
            return body
        finally:
            os.close(fd)

    def read_document(self, path):
        return json.loads(self.read_bytes(path), object_pairs_hook=kiro_unique_pairs, parse_constant=kiro_nonfinite)

    def tree_contents(self, root):
        self.directory(root)
        before = self.audit["scoped_snapshot"]((root,))
        result = {}
        for path in sorted(root.rglob("*")):
            kiro_require(len(result) < 16384 and self.audit["path_inside_case"](path, self.case))
            info = path.lstat()
            relative = path.relative_to(root).as_posix()
            if stat.S_ISDIR(info.st_mode):
                result[relative] = "directory"
            else:
                body = self.read_bytes(path)
                result[relative] = (hashlib.sha256(body).hexdigest(), bool(info.st_mode & 0o111))
        kiro_require(before == self.audit["scoped_snapshot"]((root,)))
        return result

    def installed_delivery(self):
        """Read existing source-bound ownership and already-rendered projection."""
        state_root = self.project / ".uap" if self.kind == "skill" else self.data
        state = self.read_document(state_root / "state-v2.json")
        kiro_require(type(state) is dict and exact(state.get("schema_version"), 5)
                     and type(state.get("installations")) is list and len(state["installations"]) == 1)
        record = state["installations"][0]
        kiro_require(type(record) is dict and type(record.get("clients")) is dict and len(record["clients"]) == 1)
        binding_key, binding = next(iter(record["clients"].items()))
        kiro_require(type(binding) is dict and binding.get("client_binding_id") == binding_key
                     and binding.get("client_id") == "kiro" and binding.get("scope") == ("project" if self.kind == "skill" else "user"))
        physical, locator = binding.get("physical_artifact_id"), binding.get("target_locator")
        kiro_require(type(physical) is str and re.fullmatch(r"[a-z0-9][a-z0-9.-]{0,63}", physical) is not None
                     and locator == str(state_root / "managed/clients/kiro" / physical))
        active = Path(locator)
        self.directory(active)
        objects = binding.get("native_objects")
        kiro_require(type(objects) is list and bool(objects) and all(type(item) is dict for item in objects))
        owned, objects_seen, expected_mcp, native_specs = [], set(), {}, set()
        selected_root = self.project / ".kiro" if self.kind == "skill" else self.home / ".kiro"
        packages = [item for item in objects if item.get("kind") == "managed_package_directory"]
        kiro_require(len(packages) == 1 and packages[0].get("path") == locator
                     and packages[0].get("object_id") == "package:kiro:" + physical
                     and packages[0].get("protection_class") == "managed")
        for item in objects:
            if item.get("kind") == "managed_package_directory":
                continue
            kind, name = item.get("kind"), item.get("logical_name")
            kiro_require(kind in ("kiro_global_skill_directory", "kiro_global_mcp_server")
                         and type(name) is str and bool(name) and (kind, name) not in objects_seen
                         and item.get("protection_class") == "managed" and item.get("user_modified", False) is False
                         and type(item.get("managed_digest")) is str and re.fullmatch(r"sha256:[0-9a-f]{64}", item["managed_digest"]) is not None)
            objects_seen.add((kind, name))
            if kind == "kiro_global_mcp_server":
                kiro_require(item.get("object_id") == "kiro-mcp:" + name
                             and item.get("path") == str(selected_root / "settings/mcp.json"))
                projection = self.read_document(active / "mcp.json")
                servers = projection.get("mcpServers") if type(projection) is dict else None
                kiro_require(type(servers) is dict and type(servers.get(name)) is dict)
                expected_mcp[name] = servers[name]
                native_specs.add(("mcp_server", name))
            else:
                relative = item.get("source_relative")
                kiro_require(type(relative) is str and relative and not Path(relative).is_absolute()
                             and all(part not in (".", "..") for part in relative.split("/"))
                             and item.get("object_id") == "kiro-skill:" + name
                             and item.get("path") == str(selected_root / "skills" / name)
                             and re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9_.-]*", name) is not None)
                native = selected_root / "skills" / name
                kiro_require(self.tree_contents(native) == self.tree_contents(active / relative))
                owned.append(native)
                native_specs.add(("skill", name))
        # The receipt narrows the only newly observed paths. A temporary
        # unrelated cache or sibling is not part of the owned installation.
        for root in self.roots:
            for path in root.rglob("*"):
                kiro_require(path in self.initial_paths or any(path == base or path.is_relative_to(base) for base in owned))
        expected_shared = json.loads(self.original_bytes)
        expected_shared["mcpServers"].update(expected_mcp if self.kind == "plugin" else {})
        kiro_require(exact(self.current_mcp, expected_shared))
        return {"record": record, "binding": binding, "native_specs": sorted(native_specs),
                "projection_sha256": kiro_digest(expected_mcp)}

    def comparable(self, image):
        result = dict(image)
        if self.kind == "plugin":
            key = "metadata:0:.kiro/settings/mcp.json"
            value = result.get(key)
            kiro_require(type(value) is tuple and len(value) == 4)
            result[key] = value[:2]
        return result

    def observe(self, argv, outcome):
        kiro_require(type(argv) is list and argv and argv[0] == self.cli
                     and all(type(arg) is str and "\0" not in arg for arg in argv)
                     and len(self.observations) < 6 and type(outcome) is dict)
        kiro_digest(outcome.get("_json"))
        image = self.image()
        mutable = {"home/.kiro/settings/mcp.json", "metadata:0:.kiro/settings/mcp.json"} if self.kind == "plugin" else set()
        kiro_require(all(key in mutable or image.get(key) == value for key, value in self.initial.items()))
        blob = None
        if self.kind == "skill":
            if self.skill_name is None:
                kiro_require(len(argv) > 5 and argv[1:3] == ["skills", "add"] and argv[4] == "--skill"
                             and re.fullmatch(r"[a-z0-9]+(?:-[a-z0-9]+)*", argv[5]) is not None)
                self.skill_name = argv[5]
            path = self.project / ".kiro/skills" / self.skill_name / "SKILL.md"
            if path.exists() or path.is_symlink():
                kiro_require(self.audit["path_inside_case"](path, self.case))
                fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK)
                try:
                    info = os.fstat(fd)
                    kiro_require(info.st_nlink == 1)
                    body = self.held_bytes(fd)
                    kiro_require(self.file_identity(path.lstat()) == self.file_identity(info)
                                 and hashlib.sha256(body).hexdigest()
                                 == image.get(".kiro/skills/" + self.skill_name + "/SKILL.md"))
                    blob = hashlib.sha1(b"blob " + str(len(body)).encode() + b"\0" + body).hexdigest()
                finally:
                    os.close(fd)
        self.skill_blobs.append(blob)
        self.mcp_images.append(copy.deepcopy(self.current_mcp))
        positive = (self.observations[0][1].get("exit_code") == 0 if self.observations else outcome.get("exit_code") == 0)
        if positive and len(self.observations) < 4:
            try:
                self.deliveries.append(self.installed_delivery())
            except OSError as error:
                raise ValueError("Kiro source-bound native projection is unavailable") from error
        elif positive:
            kiro_require(bool(self.deliveries))
            active = Path(self.deliveries[0]["binding"]["target_locator"])
            kiro_require(self.audit["path_inside_case"](active, self.case)
                         and not active.exists() and not active.is_symlink())
            state_path = (self.project / ".uap" if self.kind == "skill" else self.data) / "state-v2.json"
            if state_path.exists() or state_path.is_symlink():
                state = self.read_document(state_path)
                kiro_require(type(state) is dict and type(state.get("installations")) is list
                             and all(type(record) is dict and type(record.get("clients")) is dict
                                     and not any(type(binding) is not dict or binding.get("client_id") == "kiro"
                                                 for binding in record["clients"].values())
                                     for record in state["installations"]))
        self.observations.append((list(argv), copy.deepcopy(outcome), image))

    def finish(self, row, item, index, fixtures):
        kiro_require(type(row) is dict and type(fixtures) is dict and type(index) is int and 0 <= index < 30)
        sources = fixtures["skills" if self.kind == "skill" else "plugin_aliases"]
        kiro_require(type(sources) is list and len(sources) == 30 and item == sources[index])
        identity = item["name"] if self.kind == "skill" else fixtures["plugin_declared_names"].get(item, item)
        source = item["source"] if self.kind == "skill" else item
        kiro_require((row.get("index"), row.get("identity"), row.get("source"), row.get("target"), row.get("kind"))
                     == (index, identity, source, "kiro", self.kind))
        final = self.image()
        kiro_require(bool(self.observations) and final == self.observations[-1][2])
        scope = (["--scope", "project", "--project-root", str(self.project)] if self.kind == "skill"
                 else ["--scope", "user"])
        add = ([self.cli, "skills", "add", source, "--skill", identity, "--target", "kiro", *scope,
                "--no-ui", "--yes", "--format", "json"] if self.kind == "skill" else
               [self.cli, "add", source, "--target", "kiro", *scope, "--format", "json"])
        listed = [self.cli, "list", "--target", "kiro", *scope, "--format", "json"]
        negative = self.kind == "plugin" and index == 13
        if negative:
            kiro_require(source == identity == "marketing-kit" and len(self.observations) == 2
                         and [x[0] for x in self.observations] == [add, listed]
                         and all(image == self.initial for _, _, image in self.observations))
            refused, inventory = (x[1] for x in self.observations)
            diagnostic = ('agentplugins: no eligible directory release for "marketing-kit": '
                          '777genius/marketing-kit: release does not support complete target set; missing kiro\n')
            kiro_require(type(refused.get("exit_code")) is int and refused["exit_code"] == 1
                         and refused.get("timeout") is False and refused.get("stdout_full") == ""
                         and refused.get("stderr_full") == diagnostic and refused.get("_json") is None
                         and inventory.get("exit_code") == 0 and inventory.get("timeout") is False
                         and inventory.get("stderr_full") == ""
                         and exact(inventory.get("_json"), {"schema_version": 1, "command": "list",
                             "result": "success", "data": {"installations": []}})
                         and row.get("status") == "documented-or-source-unsupported"
                         and row.get("sentinels_preserved") is True and row.get("native_residuals") == []
                         and all(row.get(key) is False for key in ("listed_identity", "failed_install_registered",
                             "failed_install_residue", "post_remove_identity_registered")))
            public = None
        else:
            kiro_require(len(self.observations) == 6)
            ids = row.get("matched_installation_ids")
            kiro_require(type(ids) is list and len(ids) == 1)
            selector = ids[0]
            expected = [add, listed, add, listed,
                        [self.cli, "remove", selector, "--target", "kiro", *scope, "--format", "json"], listed]
            kiro_require([x[0] for x in self.observations] == expected)
            kiro_require(all(type(out.get("exit_code")) is int and out["exit_code"] == 0
                             and out.get("timeout") is False and out.get("stderr_full") == ""
                             for _, out, _ in self.observations))
            documents = {phase: observation[1].get("_json") for phase, observation in zip(COMMANDS, self.observations)}
            selected = [self.audit["matching_installations"](documents[phase], identity, "kiro", self.kind,
                        item, fixtures["signed_release_pins"], kiro_prepared=True) for phase in ("list", "repeat_list")]
            kiro_require(selected[0] == selected[1])
            proof = observe(self.kind, identity, documents, *selected, target="kiro",
                            skill_fixture=item if self.kind == "skill" else None,
                            kiro_active=self.kind == "plugin" and index in KIRO_ACTIVE)
            row["kiro_preparation_envelopes"] = proof
            kiro_require(lifecycle(row, "kiro") and proof.get("status") == "verified")
            kiro_require(len(self.deliveries) == 4 and bool(selected[0]))
            public = selected[0][0]
            public_client = public["clients"][0]
            expected_repo = (item["repo"] if self.kind == "skill" else
                             source[len("github:"):].rsplit("@", 1)[0] if source.startswith("github:") else
                             fixtures["signed_release_pins"][source]["repository"])
            expected_subpath = ("skills/" + item["folder"] if self.kind == "skill" else
                                "" if source.startswith("github:") else fixtures["signed_release_pins"][source]["path"])
            # Only Project Kiro binds a private native profile. User ownership
            # still requires exact native-object paths under the held HOME.
            for delivery in self.deliveries:
                record, binding = delivery["record"], delivery["binding"]
                origin, package = record.get("source"), record.get("package")
                revision = public_client["package_revision"]
                physical = public["name"][:51].rstrip(".-") + "-" + hashlib.sha256(public["installation_id"].encode()).hexdigest()[:12]
                expected_active = (self.project / ".uap" if self.kind == "skill" else self.data) / "managed/clients/kiro" / physical
                kiro_require(record.get("installation_id") == public["installation_id"]
                             and record.get("declared_name") == public["name"]
                             and type(origin) is dict and type(package) is dict
                             and type(origin.get("repository")) is str and origin["repository"].lower() == expected_repo.lower()
                             and origin.get("package_subpath", "") == expected_subpath
                             and origin.get("resolved_revision") == revision.get("resolved_revision")
                             and origin.get("tree_digest") == revision.get("tree_digest")
                             and package.get("manifest_digest") == revision.get("manifest_digest")
                             and type(binding.get("package_revision")) is dict
                             and all(exact(binding["package_revision"].get(key), value) for key, value in revision.items())
                             and all(exact(binding.get(key), public_client.get(key)) for key in
                                     ("materialization", "activation", "authentication", "policy", "verification"))
                             and binding.get("native_profile_root", "")
                             == (str(self.project / ".kiro") if self.kind == "skill" else ""))
                kiro_require(binding.get("physical_artifact_id") == physical and binding.get("target_locator") == str(expected_active))
                if self.kind == "skill":
                    kiro_require(all(document["items"][0].get("destinationPath") == str(expected_active)
                                     for document in (documents["install"], documents["repeat_add"])))
            first = self.deliveries[0]
            kiro_require(all(exact(delivery["record"].get("source"), first["record"].get("source"))
                             and exact(delivery["record"].get("package"), first["record"].get("package"))
                             and all(exact(delivery["binding"].get(key), first["binding"].get(key)) for key in
                                     ("client_binding_id", "physical_artifact_id", "target_locator", "native_profile_root", "native_objects"))
                             for delivery in self.deliveries))
            kiro_require(all(delivery["native_specs"] == self.deliveries[0]["native_specs"]
                             and delivery["projection_sha256"] == self.deliveries[0]["projection_sha256"]
                             for delivery in self.deliveries))
            if self.kind == "skill":
                kiro_require(skill_preparation(row, item, "kiro") and proof["skill_path"] == row.get("skill_path"))
                kiro_require(self.skill_blobs == [item["skill_md_git_blob_sha1"]] * 4 + [None, None])
                kiro_require(self.deliveries[0]["native_specs"] == [("skill", identity)])
                destination = Path(documents["install"]["items"][0]["destinationPath"])
                kiro_require(destination.parent == self.project / ".uap/managed/clients/kiro")
            else:
                # Owned preparation is separate from the default native verdict.
                kiro_require(row.get("status") == ("installation-lifecycle-observed" if index in KIRO_ACTIVE
                                                  else "incomplete-native-proof")
                             and bool(selected[0]) and selected[0][0].get("name") == identity)
                revision = selected[0][0]["clients"][0]["package_revision"]
                components = documents["install"]["data"]["actions"][0].get("component_support")
                kiro_require(type(components) is list and bool(components)
                             and all(type(component) is dict and component.get("support") in ("native", "unsupported")
                                     for component in components))
                kiro_require(sorted((component.get("kind"), component.get("name")) for component in components
                                    if component.get("support") == "native") == self.deliveries[0]["native_specs"])
                mcp_names = {component.get("name") for component in components
                             if component.get("kind") == "mcp_server" and component.get("support") == "native"}
                kiro_require(all(type(name) is str and bool(name) for name in mcp_names)
                             and all(type(image.get("mcpServers")) is dict
                                     and set(image["mcpServers"]) == {"uap-foreign-neighbor"} | mcp_names
                                     for image in self.mcp_images[:4]))
                for phase in ("install", "repeat_add"):
                    action = documents[phase]["data"]["actions"][0]
                    kiro_require(action.get("revision") == revision.get("resolved_revision")
                                 and action.get("package_digest") == revision.get("tree_digest"))
                    if phase == "repeat_add":
                        kiro_require(action.get("from_revision") == revision.get("resolved_revision")
                                     and action.get("from_digest") == revision.get("tree_digest"))
            frames = [x[2] for x in self.observations]
            kiro_require(frames[0] == frames[1] == frames[2] == frames[3]
                         and self.comparable(frames[0]) != self.comparable(self.initial)
                         and self.comparable(frames[4]) == self.comparable(frames[5]) == self.comparable(self.initial))
            public = {key: selected[0][0]["clients"][0][key] for key in
                      ("materialization", "activation", "authentication", "policy", "verification")}
        return {"status": "verified-installation-negative" if negative else "verified-installation-lifecycle",
                "scope": "project" if self.kind == "skill" else "user", "fixture_index": index,
                "source": source, "identity": identity, "public_state": public,
                "native_roots": ["HOME (complete)", "project/.kiro (complete)"],
                "original_native_preserved": True, "native_runtime_qualified": False,
                "writer_closure_claimed": False,
                **({"upstream_skill_blob_sha1": item["skill_md_git_blob_sha1"]} if self.kind == "skill" else {}),
                "initial_image": {"sha256": kiro_digest(self.initial), "entries": len(self.initial)},
                "observations": [{"argv": argv, "json_sha256": kiro_digest(out.get("_json")),
                                  "image": {"sha256": kiro_digest(image), "entries": len(image)}}
                                 for argv, out, image in self.observations]}


def kiro_summary(report, fixtures, acceptance):
    result = {"installation_passed": 0, "installation_verified_negative_coverage": 0,
              "installation_acceptance_passed": False, "qualification_scope": "owned Kiro installation lifecycle",
              **{key: False for key in ("native_discovery_qualified", "native_activation_qualified",
                  "native_mcp_qualified", "model_execution_qualified", "authenticated_execution_qualified",
                  "native_runtime_qualified", "tools_call_qualified")}}
    route = KIRO_ROUTES.get(acceptance)
    if route is None:
        return result
    _, kind = route
    sources = fixtures.get("skills" if kind == "skill" else "plugin_aliases")
    rows = report.get("results")
    if ((report.get("target"), report.get("kind"), report.get("platform"), report.get("architecture"))
            != ("kiro", kind, "darwin", "arm64")
            or not exact(report.get("client_version"), {"status": "reported", "value": "kiro-cli 3.0.0", "exit_code": 0})
            or not exact(report.get("cli_version"), {"status": "reported", "value": "agentplugins 0.1.115", "exit_code": 0})
            or any(not exact(report.get(key), 30) for key in ("fixture_count", "cases_attempted"))
            or report.get("source_selection_complete") is not True or report.get("full_coverage") is not True
            or not exact(report.get("passed"), 0) or not exact(report.get("verified_negative_coverage"), 0)
            or not exact(report.get("prepared_lifecycle_passed"), 23 if kind == "plugin" else 0)
            or type(rows) is not list or len(rows) != 30 or type(sources) is not list or len(sources) != 30):
        return result
    for index, (row, item) in enumerate(zip(rows, sources)):
        source = item["source"] if kind == "skill" else item
        identity = item["name"] if kind == "skill" else fixtures["plugin_declared_names"].get(item, item)
        proof = row.get("kiro_installation_custody") if type(row) is dict else None
        negative = kind == "plugin" and index == 13
        if (type(row) is not dict or not exact(row.get("index"), index)
                or (row.get("target"), row.get("kind"), row.get("source"), row.get("identity")) != ("kiro", kind, source, identity)
                or type(proof) is not dict or proof.get("status") != ("verified-installation-negative" if negative else "verified-installation-lifecycle")
                or (proof.get("fixture_index"), proof.get("source"), proof.get("identity")) != (index, source, identity)
                or proof.get("original_native_preserved") is not True or proof.get("native_runtime_qualified") is not False
                or proof.get("writer_closure_claimed") is not False
                or proof.get("scope") != ("project" if kind == "skill" else "user")
                or proof.get("native_roots") != ["HOME (complete)", "project/.kiro (complete)"]
                or type(proof.get("observations")) is not list or len(proof["observations"]) != (2 if negative else 6)):
            return result
        if negative:
            if (row.get("status") != "documented-or-source-unsupported" or source != "marketing-kit" or proof.get("public_state") is not None
                    or row.get("sentinels_preserved") is not True or row.get("native_residuals") != []
                    or type(row.get("native_file_delta")) is not int or row["native_file_delta"] != 0
                    or any(row.get(key) is not False for key in ("listed_identity", "failed_install_registered",
                        "failed_install_residue", "post_remove_identity_registered", "repeat_verified", "removal_verified"))
                    or any(row.get(key) != [] for key in ("native_identity_files", "native_component_candidates",
                        "matched_installation_ids", "repeat_installation_ids", "post_removal_leftovers"))
                    or any(key not in row or row[key] is not None for key in ("repeat_add", "repeat_list", "cleanup", "post_remove_list"))):
                return result
            result["installation_verified_negative_coverage"] += 1
            continue
        state = expected_state(kind, "kiro")
        if kind == "plugin" and index in KIRO_ACTIVE:
            state = {**state, "activation": "active", "verification": "installation_verified"}
        if (not exact(proof.get("public_state"), state) or not lifecycle(row, "kiro")
                or not public_states(row, kind, "kiro", kiro_active=index in KIRO_ACTIVE)
                or (kind == "skill" and not skill_preparation(row, item, "kiro"))
                or (kind == "skill" and proof.get("upstream_skill_blob_sha1") != item.get("skill_md_git_blob_sha1"))
                or (kind == "plugin" and row.get("status") != ("installation-lifecycle-observed" if index in KIRO_ACTIVE else "incomplete-native-proof"))):
            return result
        result["installation_passed"] += 1
    result["installation_acceptance_passed"] = (result["installation_passed"], result["installation_verified_negative_coverage"]) == ((29, 1) if kind == "plugin" else (30, 0))
    return result

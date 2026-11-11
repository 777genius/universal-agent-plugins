#!/usr/bin/env python3
"""Actual public npm 0.1.102 + signed Marketing Kit Project lifecycle, no mocks."""
import argparse
import hashlib
import json
import os
import re
from pathlib import Path
import runpy
import shutil
import stat
import tempfile
from time import sleep
import traceback

ROOT = Path(__file__).resolve().parents[1]
audit = runpy.run_path(str(ROOT / "scripts/live-install-audit.py"))
isolated_env = audit["isolated_client_env"]
invoke = audit["call"]
REQUIRED_DISTRIBUTIONS = audit["MARKETING_KIT_DISTRIBUTIONS"]
EXPECTED = set(REQUIRED_DISTRIBUTIONS.values())
# Versions declared by the immutable, published Marketing Kit uap.toml.
# Edge aliases alone cannot prove an unchanged locked selector.
REQUIRED_CHILD_SELECTORS = {
    "notion": {"kind": "version", "value": "0.1.0"},
    "hubspot-crm": {"kind": "version", "value": "0.1.0"},
    "playwright": {"kind": "version", "value": "0.0.80"},
}


# Exact unchanged signed45 bytes, not a policy synthesized for this test.
# Source: Registry ledger commit 3509baaac8e4f7ee3d565440f8c0d0bd1f928fba,
# registry/schemas/1/snapshots/00000000000000000045.json.
FIXED_REFUSAL_LEDGER = "3509baaac8e4f7ee3d565440f8c0d0bd1f928fba"
FIXED_REFUSAL_ORIGIN = (
    "https://raw.githubusercontent.com/777genius/universal-agent-plugins-registry/"
    + FIXED_REFUSAL_LEDGER + "/registry/schemas/1/")
FIXED_REFUSAL_POLICY = {
    "snapshot_sequence": 45,
    "snapshot_digest": "sha256:2aba9b0d97556ac0756dff5f974aa7377914e0d8d1d9b735b46c5bc804b81f50",
}
FIXED_SIGNED_CLOSURE = audit["MARKETING_KIT_SIGNED_CLOSURE"]


# Published npm 0.1.102 dependency operations include result=success AND
# outcome/actions/results. Require exact action evidence; process exit 0 or
# result=success alone is not sufficient.
UPDATE_ALIASES = set(REQUIRED_DISTRIBUTIONS)
# OpenCode and Cursor discover additional native compatibility Skill roots.
# These are read-only E2E audit paths, never installer authorization.
COMPAT_SKILL_ROOTS = {
    "opencode": (".agents", ".claude"),
    "cursor": (".agents", ".claude", ".codex"),
    "gemini": (".agents",),
    "copilot": (".agents",),
}
# Copilot CLI also discovers .claude/skills in repositories, but not as
# a documented personal Skills path.
PROJECT_ONLY_COMPAT_SKILL_ROOTS = {"copilot": (".claude",)}
# Immutable 0.1.102 policy and the scanner used by this Ubuntu E2E workflow;
# also independently pinned by project_native_windsurf_cycle.py.
REFUSAL_CACHE_POLICY = {
    "id": "agent-plugin-install", "version": 2,
    "digest": "sha256:9cf869e299d847d7078aeca01f5a182fcdb0144bbf513c83290459991c79e037",
}
REFUSAL_SCANNER_PATH = "security/lintai/0.1.3/linux-amd64/lintai"
REFUSAL_SCANNER_SHA256 = "b6723451f155eb49e84acb34d1f06de929bddfcff6238cd4e38cdc6519396fba"
LEGACY_ACTIONS = {
    "update": {"unchanged"},
    "repair": {"repair"},
    "install": {"install", "repair", "unchanged"},
    "add": {"install", "repair", "unchanged"},
    "remove": {"remove", "unchanged"},
}


def valid_legacy_dependency_result(envelope, operation, target=None, scope=None):
    """Validate real published dependency receipts for the selected client."""
    if (operation not in LEGACY_ACTIONS
            or (scope is not None and envelope.get("scope") != scope)
            or envelope.get("result") != "success"
            or envelope.get("outcome") != "completed"):
        return False
    actions, results = envelope.get("actions"), envelope.get("results")
    if (not isinstance(actions, list) or not actions
            or not isinstance(results, list)):
        return False
    if any(not isinstance(action, dict)
           or not isinstance(action.get("alias"), str) or not action["alias"]
           or not isinstance(action.get("client_id"), str) or not action["client_id"]
           or (target is not None and action["client_id"] != target)
           or action.get("action") not in LEGACY_ACTIONS[operation]
           for action in actions):
        return False
    aliases = [action["alias"] for action in actions]
    if len(set(aliases)) != len(aliases):
        return False
    if operation in ("update", "install", "add") and set(aliases) != UPDATE_ALIASES:
        return False
    if operation == "repair" and aliases != ["marketing-kit"]:
        return False
    if operation == "remove" and (
            "marketing-kit" not in aliases or not set(aliases) <= UPDATE_ALIASES):
        return False
    active = [action for action in actions if action["action"] != "unchanged"]
    if (len(results) != len(active)
            or operation in ("repair", "install", "remove") and not active):
        return False
    for action, result in zip(active, results):
        if (not isinstance(result, dict)
                or result.get("outcome") not in ("completed", "unchanged")
                or result.get("operation") != action["action"]
                or result.get("client_id") != action["client_id"]
                # Active receipts must identify their durable installation.
                # For a cold add the planned action may not have an ID yet.
                or not isinstance(result.get("installation_id"), str)
                or not result["installation_id"].strip()
                or (action.get("installation_id")
                    and result["installation_id"] != action["installation_id"])):
            return False
    # A full dependency closure needs distinct durable receipts for its
    # independent aliases, not repeated evidence from one installation.
    if len({row["installation_id"] for row in results}) != len(results):
        return False
    return True


def verify_applied_receipt_inventory(envelope, observed):
    """Bind applied and unchanged actions to actual selected-scope inventory."""
    data = envelope.get("data", envelope)
    for action in data["actions"]:
        if action["action"] != "unchanged":
            continue
        distribution = REQUIRED_DISTRIBUTIONS[action["alias"]]
        saved_id = action.get("installation_id")
        if (not isinstance(saved_id, str) or not saved_id.strip()
                or observed.get(distribution) != saved_id):
            raise AssertionError(
                f"unchanged action does not match installed {distribution} identity")
    active = [action for action in data["actions"] if action["action"] != "unchanged"]
    for action, receipt in zip(active, data["results"]):
        distribution = REQUIRED_DISTRIBUTIONS[action["alias"]]
        if observed.get(distribution) != receipt["installation_id"]:
            raise AssertionError(
                f"applied receipt does not match installed {distribution} identity")


def verified(result, stage, operation=None, target=None, scope=None):
    envelope = result.get("_json")
    if (result.get("exit_code") != 0 or result.get("timeout")
            or not isinstance(envelope, dict)):
        raise AssertionError(f"{stage}: CLI did not succeed with JSON: {result}")
    # The pinned 0.1.102 dependency commands must report explicit actions
    # and results even when they also emit result=success.
    if operation == "add":
        # Published 'add' wraps actual scoped dependency receipts in data.
        # An outer generic success cannot certify a missing/partial closure.
        nested = envelope.get("data", envelope)
        # A signed add must be bound to an explicitly selected client/scope;
        # otherwise an unrelated successful dependency report can qualify it.
        if (isinstance(target, str) and bool(target)
                and scope in ("user", "project")
                and envelope.get("result") == "success"
                # A nested success must not mask an explicit outer refusal.
                and envelope.get("outcome") in (None, "completed")
                and isinstance(nested, dict)
                and valid_legacy_dependency_result(nested, "add", target, scope)):
            return envelope
    elif operation in LEGACY_ACTIONS:
        if valid_legacy_dependency_result(envelope, operation, target, scope):
            return envelope
    elif (envelope.get("result") == "success"
          and envelope.get("outcome") in (None, "completed")):
        return envelope
    raise AssertionError(f"{stage}: CLI did not succeed with JSON: {result}")


def require_exact_published_version(result):
    """Reject a dev/newer/wrong binary even if its output contains the pinned version."""
    stdout = result.get("stdout_tail")
    if (result.get("exit_code") != 0 or result.get("timeout")
            or not isinstance(stdout, str)
            or stdout.strip() != "agentplugins 0.1.102"):
        raise AssertionError("not exact published agentplugins 0.1.102")


def verify_pinned_launcher(cli, env, project):
    """Retry only failed private-cache acquisition, never accept the wrong release."""
    for attempt in range(3):
        result = invoke([cli, "version"], env, project, 25)
        if result.get("exit_code") == 0 and not result.get("timeout"):
            require_exact_published_version(result)
            return
        if attempt < 2:
            sleep((attempt + 1) * 5)
    require_exact_published_version(result)


def inspect_signed_closure(cli, env, project, target, expected_policy=None):
    """Use released CLI trust/expiry checks; compare its public info identities."""
    policy = None
    root_targets = None
    for distribution, pinned in FIXED_SIGNED_CLOSURE.items():
        result = invoke([cli, "info", distribution, "--target", target,
                         "--scope", "user", "--format", "json"], env, project, 35)
        if type(result.get("exit_code")) is not int or result.get("timeout") is not False:
            raise AssertionError("signed Directory info process result invalid")
        envelope = verified(result, "signed Directory info")
        data = envelope.get("data")
        if envelope.get("command") != "info" or not isinstance(data, dict):
            raise AssertionError("signed Directory info envelope invalid")
        sequence, digest = data.get("snapshot_sequence"), data.get("snapshot_digest")
        if (type(sequence) is not int or sequence <= 0 or not isinstance(digest, str)
                or not re.fullmatch(r"sha256:[0-9a-f]{64}", digest)):
            raise AssertionError("signed Directory policy identity invalid")
        identity = {"snapshot_sequence": sequence, "snapshot_digest": digest}
        if ((expected_policy is not None and identity != expected_policy)
                or (policy is not None and identity != policy)):
            raise AssertionError("signed Directory policy identity changed")
        release = data.get("selected_release")
        if (data.get("selected_distribution") != distribution
                or not isinstance(release, dict) or release.get("id") != distribution
                or release.get("release_status") != "active"
                or type(release.get("release_sequence")) is not int
                or any(release.get(field) != value for field, value in pinned.items())):
            raise AssertionError("signed Directory immutable source identity changed")
        if target == "vscode":
            targets = release.get("target_compatibility")
            if (not isinstance(targets, list) or any(not isinstance(item, dict) for item in targets)
                    or [{key: item.get(key) for key in ("client", "scopes", "delivery")}
                        for item in targets if item.get("client") == "vscode"]
                    != [{"client": "vscode", "scopes": ["user"], "delivery": "prepared"}]):
                raise AssertionError("signed Directory VS Code preparation metadata changed")
        if distribution == REQUIRED_DISTRIBUTIONS["marketing-kit"]:
            targets = release.get("target_compatibility")
            if (not isinstance(targets, list) or not targets
                    or any(not isinstance(item, dict) or not isinstance(item.get("client"), str)
                           or not item["client"] or item.get("scopes") != ["user"]
                           or not isinstance(item.get("delivery"), str) or not item["delivery"]
                           for item in targets)
                    or len({item["client"] for item in targets}) != len(targets)):
                raise AssertionError("signed Directory root packaging metadata invalid")
            root_targets = {item["client"]: {"scopes": item["scopes"], "delivery": item["delivery"]}
                            for item in targets}
            for client, delivery in (("copilot", "managed"), ("vscode", "prepared")):
                if client in root_targets and root_targets[client]["delivery"] != delivery:
                    raise AssertionError("signed Directory root packaging contract changed")
        policy = identity
    return {**policy, "root_target_compatibility": root_targets,
            "closure": {key: dict(value) for key, value in FIXED_SIGNED_CLOSURE.items()}}


def observe_production_policy(cli, env, project, target, expected=None, guards=()):
    """Observe production defaults without inheriting or selecting another origin."""
    before = refusal_profile_snapshot(env, project, {}, directory_only=True)
    try:
        if "AGENTPLUGINS_DIRECTORY_ORIGIN" in env:
            raise AssertionError("positive lifecycle must use production default origin")
        identity = None if expected is None else {
            key: expected[key] for key in ("snapshot_sequence", "snapshot_digest")}
        policy = {"source_mode": "production_default",
                  **inspect_signed_closure(cli, env, project, target, identity)}
        if expected is not None and policy != expected:
            raise AssertionError("production policy observation changed")
        return policy
    finally:
        try:
            check_refusal_profile_unchanged(env, project, before)
        finally:
            for guard in guards:
                guard()


def inventory(cli, env, project, target, scope="project"):
    if scope not in ("user", "project"):
        raise ValueError(f"unsupported scope: {scope}")
    scope_args = ["--scope", scope]
    if scope == "project":
        scope_args.extend(["--project-root", str(project)])
    result = invoke([cli, "list", "--target", target, *scope_args,
                     "--format", "json"], env, project, 35)
    data = verified(result, "list").get("data")
    if not isinstance(data, dict) or not isinstance(data.get("installations"), list):
        raise AssertionError(f"list returned invalid installations: {data}")
    return data["installations"]


def identities(items, target, scope="project"):
    """Bind each selected installed record to the reviewed signed release."""
    result = {}
    for row in items:
        if not isinstance(row, dict):
            raise AssertionError("invalid installation record")
        directory = row.get("directory") or {}
        if not isinstance(directory, dict):
            raise AssertionError("invalid signed Directory record")
        dist = directory.get("recorded_distribution")
        if not dist:
            # Recorded signed-v1 identity uses recorded_distribution, not distribution_id.
            raise AssertionError(f"installation lacks signed Directory identity: {row.get('name')}; fields={sorted(directory)}")
        pin = FIXED_SIGNED_CLOSURE.get(dist) if isinstance(dist, str) else None
        if (pin is None
                or row.get("source") != pin["repository"].strip("/") + "/" + pin["package_path"].strip("/")
                or directory.get("recorded_revision") != pin["resolved_revision"]
                or type(directory.get("recorded_release_sequence")) is not int
                or directory["recorded_release_sequence"] != pin["release_sequence"]):
            raise AssertionError(f"{dist}: installation differs from fixed signed source identity")
        bindings = row.get("clients") or []
        if isinstance(bindings, dict):
            bindings = list(bindings.values())
        scope_bindings = [b for b in bindings if isinstance(b, dict)
                          and b.get("client_id") == target and b.get("scope") == scope]
        if not scope_bindings:
            raise AssertionError(f"{dist}: missing {scope} binding for {target}")
        if target == "copilot" and scope == "user":
            if len(bindings) != 1 or len(scope_bindings) != 1:
                raise AssertionError(f"{dist}: User installation must have one logical Copilot owner")
            if scope_bindings[0].get("affected_surfaces") != ["copilot", "vscode"]:
                raise AssertionError(f"{dist}: Copilot User shared physical surfaces changed")
        if not all(audit["recorded_signed_release_matches"](
                directory, b.get("package_revision")) for b in scope_bindings):
            raise AssertionError(f"{dist}: {scope} binding lacks exact signed release provenance")
        for binding in scope_bindings:
            revision = binding["package_revision"]
            if (type(revision.get("release_sequence")) is not int
                    or revision.get("distribution_id") != dist
                    or any(revision.get(key) != pin[field] for key, field in (
                        ("version", "package_version"), ("release_sequence", "release_sequence"),
                        ("resolved_revision", "resolved_revision"),
                        ("tree_digest", "tree_digest"), ("manifest_digest", "manifest_digest")))):
                raise AssertionError(f"{dist}: {scope} binding differs from fixed signed release")
            # Signed provenance alone cannot certify successful delivery. The
            # Cursor User adapter deliberately prepares a package for manual Editor
            # activation; that is not native discovery or MCP execution evidence.
            if binding.get("materialization") != "materialized":
                raise AssertionError(f"{dist}: {scope} binding lacks materialized delivery")
            if (target == "cursor" and scope == "user"
                    and binding.get("verification") == "package_validated"
                    and binding.get("activation") == "manual_activation_required"
                    and binding.get("policy") == "allowed"):
                continue
            if binding.get("verification") != "installation_verified":
                raise AssertionError(f"{dist}: {scope} binding lacks installed verification")
        if dist in result:
            raise AssertionError(f"duplicate distribution: {dist}")
        installation_id = row.get("installation_id")
        if not isinstance(installation_id, str) or not installation_id.strip():
            raise AssertionError(f"{dist}: missing installation ID")
        if installation_id in result.values():
            raise AssertionError(f"duplicate installation ID: {installation_id}")
        result[dist] = installation_id
    return result


def lock_ids(project, target=None, *, expected_sources=None):
    lock = json.loads((project / "uap.lock").read_text(encoding="utf-8"))
    packages = lock.get("packages")
    if not isinstance(packages, list):
        raise AssertionError("locked project has no package entries")
    keys = []
    for item in packages:
        if not isinstance(item, dict) or not isinstance(item.get("source_key"), str) or not item["source_key"]:
            raise AssertionError("invalid signed lock package source")
        keys.append(item["source_key"])
    if len(keys) != len(set(keys)):
        raise AssertionError("duplicate signed lock package source")
    if target is not None:
        # The source-key set by itself cannot prove mandatory child edges.
        # Check the recorded signed coordinates and selected-client closure.
        by_alias = {}
        for item in packages:
            alias, source = item.get("alias"), item.get("source")
            registry = source.get("registry") if isinstance(source, dict) else None
            if (alias not in REQUIRED_DISTRIBUTIONS or alias in by_alias
                    or not isinstance(registry, dict)
                    or registry.get("distribution") != REQUIRED_DISTRIBUTIONS[alias]
                    or item["source_key"] != "directory:" + REQUIRED_DISTRIBUTIONS[alias]
                    or not isinstance(item.get("targets"), list)
                    or target not in item["targets"]
                    or any(not isinstance(item.get(field), str)
                           or not re.fullmatch(r"sha256:[0-9a-f]{64}", item[field])
                           for field in ("tree_digest", "manifest_digest"))):
                raise AssertionError("invalid selected-client signed lock pin")
            if expected_sources is not None:
                pinned = expected_sources[REQUIRED_DISTRIBUTIONS[alias]]
                if any(item[field] != pinned[field] for field in ("tree_digest", "manifest_digest")):
                    raise AssertionError("locked package differs from observed immutable source")
            by_alias[alias] = item
        if set(by_alias) != set(REQUIRED_DISTRIBUTIONS):
            raise AssertionError("mandatory signed lock aliases incomplete")
        children = {alias: set() for alias in by_alias}
        for alias, item in by_alias.items():
            required_by = item.get("required_by", [])
            if not isinstance(required_by, list):
                raise AssertionError("invalid signed lock dependency edges")
            for edge in required_by:
                if not isinstance(edge, dict) or edge.get("alias") not in by_alias:
                    raise AssertionError("unknown signed lock dependency consumer")
                if (edge["alias"] == "marketing-kit"
                        and edge.get("selector") != REQUIRED_CHILD_SELECTORS.get(alias)):
                    raise AssertionError("signed Marketing Kit dependency selector changed")
                if alias in children[edge["alias"]]:
                    raise AssertionError("duplicate signed lock dependency edge")
                children[edge["alias"]].add(alias)
        # Marketing Kit's signed uap.toml directly requires all three MCP
        # children. Mere reachability allows a reparented child to masquerade
        # as the unchanged four-node closure during cold lock restoration.
        if children["marketing-kit"] != set(by_alias) - {"marketing-kit"}:
            raise AssertionError("signed Marketing Kit direct required dependencies changed")
        # These three released MCP children have no dependency declarations.
        # Extra child-to-child edges misstate the signed four-package closure.
        if any(children[alias] for alias in children if alias != "marketing-kit"):
            raise AssertionError("unexpected signed Marketing Kit child dependency edge")
        visited, visiting = set(), set()

        def visit(alias):
            if alias in visiting:
                raise AssertionError("cyclic signed lock dependency closure")
            if alias in visited:
                return
            visiting.add(alias)
            for child in children[alias]:
                visit(child)
            visiting.remove(alias)
            visited.add(alias)

        visit("marketing-kit")
        if visited != set(by_alias):
            raise AssertionError("signed Marketing Kit lock closure disconnected")
    return set(keys)


def corrupt_locked_child_digest(lock_file, expected_identity=None):
    """Mutate one child digest on a verified file descriptor, never through a swapped path."""
    try:
        before = lock_file.lstat()
        if not stat.S_ISREG(before.st_mode) or before.st_nlink != 1:
            raise AssertionError("invalid negative-test lock path")
        flags = (os.O_RDWR | getattr(os, "O_NOFOLLOW", 0)
                 | getattr(os, "O_NONBLOCK", 0)
                 | getattr(os, "O_CLOEXEC", 0))
        descriptor = os.open(lock_file, flags)
    except OSError as error:
        raise AssertionError("invalid negative-test lock path") from error
    with os.fdopen(descriptor, "r+b") as stream:
        opened = os.fstat(stream.fileno())
        try:
            current = lock_file.lstat()
        except OSError as error:
            raise AssertionError("invalid negative-test lock path") from error
        identity = (opened.st_dev, opened.st_ino)
        if (not stat.S_ISREG(before.st_mode)
                or not stat.S_ISREG(current.st_mode)
                or not stat.S_ISREG(opened.st_mode)
                or opened.st_nlink != 1 or current.st_nlink != 1
                or identity != (before.st_dev, before.st_ino)
                or identity != (current.st_dev, current.st_ino)
                or (expected_identity is not None
                    and identity != (expected_identity.st_dev, expected_identity.st_ino))):
            raise AssertionError("invalid negative-test lock path")
        document = json.loads(stream.read().decode("utf-8"))
        packages = document.get("packages")
        if not isinstance(packages, list):
            raise AssertionError("invalid negative-test lock fixture")
        matches = [item for item in packages
                   if isinstance(item, dict) and item.get("alias") == "notion"]
        if len(matches) != 1:
            raise AssertionError("missing unique Notion child for negative test")
        original = matches[0].get("tree_digest")
        if not isinstance(original, str) or not re.fullmatch(r"sha256:[0-9a-f]{64}", original):
            raise AssertionError("invalid signed child digest in negative test")
        replacement = "1" if original[7] == "0" else "0"
        corrupted = "sha256:" + replacement + original[8:]
        matches[0]["tree_digest"] = corrupted
        stream.seek(0)
        stream.write((json.dumps(document, ensure_ascii=False) + "\n").encode("utf-8"))
        stream.truncate()
        return original, corrupted


def verify_corrupt_lock_refusal(cli, env, project, target, scope, guards):
    """Require genuine signed-lock refusal before either scoped cold restore."""
    if scope not in ("user", "project"):
        raise AssertionError("invalid cold restore scope")
    lock_parent = project if scope == "project" else Path(env["AGENTPLUGINS_HOME"])
    lock_file = lock_parent / "uap.lock"
    # A preceding native client may replace the lock with a foreign link or
    # special file. Inspect the pathname before opening or reading any bytes.
    try:
        original_identity = lock_file.lstat()
        if not stat.S_ISREG(original_identity.st_mode) or original_identity.st_nlink != 1:
            raise AssertionError("invalid original signed lock path")
        flags = (os.O_RDONLY | getattr(os, "O_NOFOLLOW", 0)
                 | getattr(os, "O_NONBLOCK", 0)
                 | getattr(os, "O_CLOEXEC", 0))
        descriptor = os.open(lock_file, flags)
        with os.fdopen(descriptor, "rb") as stream:
            opened = os.fstat(stream.fileno())
            current = lock_file.lstat()
            if (not stat.S_ISREG(opened.st_mode) or opened.st_nlink != 1
                    or not stat.S_ISREG(current.st_mode) or current.st_nlink != 1
                    or (opened.st_dev, opened.st_ino)
                    != (original_identity.st_dev, original_identity.st_ino)
                    or (current.st_dev, current.st_ino)
                    != (original_identity.st_dev, original_identity.st_ino)):
                raise AssertionError("invalid original signed lock path")
            original = stream.read()
    except OSError as error:
        raise AssertionError("invalid original signed lock path") from error
    original_digest, corrupted_digest = corrupt_locked_child_digest(lock_file, original_identity)
    scope_args = ["--scope", scope]
    if scope == "project":
        scope_args.extend(["--project-root", str(project)])
    try:
        before = refusal_profile_snapshot(env, project, json.loads(original))
        refused = invoke([cli, "install", "--locked", "--yes",
                          *scope_args, "--format", "json"], env, project, 180)
        if refused.get("exit_code") in (None, 0) or refused.get("timeout"):
            raise AssertionError("corrupt signed lock did not fail closed")
        reason = (str(refused.get("stderr_tail") or "") + " " +
                  str(refused.get("_json") or "")).lower()
        # Released 0.1.102 verifies the acquired immutable Git tree against
        # the lock digest *before* comparing it to the signed Directory. Bind
        # both digests to this fixture so a remote/lock timeout or another
        # package's error cannot masquerade as an integrity refusal.
        acquisition_refusal = (
            f"acquired package tree digest {original_digest} "
            f"does not match expected {corrupted_digest}"
        )
        signed_refusal = (
            "verify notion pinned manifest" in reason
            and "signed directory publication differs from exact lock" in reason
        )
        if acquisition_refusal not in reason and not signed_refusal:
            raise AssertionError("corrupt child digest did not receive signed-pin refusal")
        check_refusal_profile_unchanged(env, project, before)
        after_refusal = profile_snapshot(env, project)
        if identities(inventory(cli, env, project, target, scope), target, scope):
            raise AssertionError("corrupt locked restore installed a dependency")
        check_profile_unchanged(env, project, after_refusal,
                                f"corrupt {scope} locked restore inventory")
        return refused["exit_code"]
    finally:
        try:
            # Open first, inspect the *opened inode*, and only then write.
            # A check-then-write of the pathname would follow a hostile symlink.
            try:
                # Do not even open a swapped foreign/special-file pathname.
                # O_NOFOLLOW and O_NONBLOCK also protect the lstat/open race.
                current = lock_file.lstat()
                if (not stat.S_ISREG(current.st_mode)
                        or current.st_nlink != 1
                        or (current.st_dev, current.st_ino)
                        != (original_identity.st_dev, original_identity.st_ino)):
                    raise AssertionError("corrupt signed lock path replaced during restore")
                flags = (os.O_RDWR | getattr(os, "O_NOFOLLOW", 0)
                         | getattr(os, "O_NONBLOCK", 0)
                         | getattr(os, "O_CLOEXEC", 0))
                descriptor = os.open(lock_file, flags)
                with os.fdopen(descriptor, "r+b") as stream:
                    identity = os.fstat(stream.fileno())
                    # fstat alone accepts a symlink to the same moved inode.
                    # The *pathname* must still name the original regular file.
                    current = lock_file.lstat()
                    if (not stat.S_ISREG(current.st_mode)
                            or current.st_nlink != 1
                            or (current.st_dev, current.st_ino)
                            != (original_identity.st_dev, original_identity.st_ino)
                            or not stat.S_ISREG(identity.st_mode)
                            or identity.st_nlink != 1
                            or (identity.st_dev, identity.st_ino)
                            != (original_identity.st_dev, original_identity.st_ino)):
                        raise AssertionError("corrupt signed lock path replaced during restore")
                    stream.seek(0)
                    stream.write(original)
                    stream.truncate()
            except OSError as error:
                raise AssertionError("corrupt signed lock path unsafe during restore") from error
        finally:
            for guard in guards:
                guard()


def run_case(cli, target, root, name, portable=None):
    case = root / name
    # B and C are created *after* untrusted package execution in A. Refuse a
    # pre-planted directory or symlink rather than following it during mkdir.
    if (not root.is_dir() or root.is_symlink()
            or case.exists() or case.is_symlink()):
        raise AssertionError("isolated profile root already occupied or unsafe")
    case.mkdir()  # no parents/exist_ok: another writer cannot silently win
    if not audit["path_inside_case"](case, root):
        raise AssertionError("isolated profile root escaped disposable case")
    home, data, project = case / "home", case / "data", case / "project"
    for directory in (home, data, project):
        directory.mkdir(mode=0o700)
        if not audit["path_inside_case"](directory, case):
            raise AssertionError("isolated profile directory escaped disposable case")
    # Reuse the existing published audit's native-profile root inventory.
    foreign_skill = "---\nname: unmanaged-uap-foreign-skill\ndescription: keep unchanged\n---\n"
    native_foreign = []

    def add_foreign_skill(skill):
        skill.parent.mkdir(parents=True, exist_ok=True)
        skill.write_text(foreign_skill, encoding="utf-8")
        native_foreign.append((skill, foreign_skill))
        # Skills may contain references/assets beyond SKILL.md. Preserve those
        # foreign bytes and directory entries through both scope lifecycles.
        reference = skill.parent / "references" / "unmanaged-context.md"
        reference.parent.mkdir(parents=True, exist_ok=True)
        reference.write_text("unmanaged reference\n", encoding="utf-8")
        native_foreign.append((reference, "unmanaged reference\n"))

    for relative in audit["NATIVE_ROOTS"][target]:
        native_root = home / relative
        native_root.mkdir(parents=True, exist_ok=True)
        marker = native_root / "unmanaged-native-do-not-change.txt"
        marker.write_text("unmanaged native settings\n", encoding="utf-8")
        native_foreign.append((marker, "unmanaged native settings\n"))
        # Pin a genuine unmanaged User SKILL.md as well: protecting a text
        # marker alone could miss native User Skill deletion during removal.
        # Codex also discovers ~/.agents/skills; protecting only its first
        # native root (~/.codex) would miss that independent User Skill tree.
        if (relative == audit["NATIVE_ROOTS"][target][0]
                or (target == "codex" and relative == ".agents")):
            user_skill = native_root / "skills" / "unmanaged-uap-foreign-skill" / "SKILL.md"
            add_foreign_skill(user_skill)
    # Protect compatible User Skills outside the primary client config root.
    # Cursor also discovers ~/.agents, ~/.claude and ~/.codex Skills.
    for relative in COMPAT_SKILL_ROOTS.get(target, ()):
        skill = home / relative / "skills" / "unmanaged-uap-foreign-skill" / "SKILL.md"
        add_foreign_skill(skill)
    # Exercise independent Project Skills too. Copilot's .github/skills lies
    # outside its native MCP roots, as do compatibility Skills.
    project_skill_roots = list(audit["PROJECT_SKILL_ROOTS"].get(target, ()))
    project_skill_roots.extend(root + "/skills" for root in COMPAT_SKILL_ROOTS.get(target, ()))
    project_skill_roots.extend(root + "/skills" for root in PROJECT_ONLY_COMPAT_SKILL_ROOTS.get(target, ()))
    for relative in project_skill_roots:
        skill = project / relative / "unmanaged-uap-foreign-skill" / "SKILL.md"
        add_foreign_skill(skill)
    # The fixture must never read or modify the operator's home or native config.
    user_foreign = home / "foreign-user-do-not-change.txt"
    user_foreign.write_text("unmanaged user data\n", encoding="utf-8")
    project_foreign = project / "foreign-project-do-not-change.txt"
    project_foreign.write_text("unmanaged project data\n", encoding="utf-8")
    env = isolated_env(home, case, data)
    # Reuse the live-audit's prebound Git executable, hook isolation and
    # disposable-root guard. A PATH-selected 'git' after acquiring untrusted
    # sources could be a shadow executable, not the original host prerequisite.
    audit["initialize_project_skill_git_root"](project, env, case)
    if portable is not None:
        for filename in ("uap.toml", "uap.lock"):
            shutil.copyfile(portable / filename, project / filename)
    return env, project, user_foreign, project_foreign, native_foreign


def pin_foreign(entries, case_root):
    """Pin unmanaged file identities and the directories containing them."""
    pinned = []
    skill_dirs = {path.parent for path, _ in entries if path.name == "SKILL.md"}
    for path, content in entries:
        if (not audit["path_inside_case"](path, case_root)
                or path.is_symlink() or not path.is_file()
                or path.read_bytes() != content.encode("utf-8")):
            raise AssertionError(f"invalid foreign fixture: {path.name}")
        identity = path.lstat()
        ancestors = []
        parent = path.parent
        while parent != case_root:
            directory = parent.lstat()
            if not stat.S_ISDIR(directory.st_mode):
                raise AssertionError(f"invalid foreign fixture ancestor: {parent.name}")
            ancestors.append((parent, (directory.st_mode, directory.st_dev, directory.st_ino)))
            parent = parent.parent
        # Pin child names at every recorded foreign-Skill depth, including
        # references/assets; an untracked nested addition is also a mutation.
        skill_children = (tuple(sorted(child.name for child in path.parent.iterdir()))
                          if any(skill_dir in path.parents for skill_dir in skill_dirs)
                          else None)
        pinned.append((path, content, (identity.st_mode, identity.st_dev, identity.st_ino, identity.st_mtime_ns),
                       tuple(ancestors), skill_children))
    return pinned


def check_foreign(entries, case_root):
    """Reject foreign file or ancestor replacement, metadata changes and links."""
    for path, content, expected_identity, ancestors, skill_children in entries:
        # A changed ancestor can conceal a moved original SKILL.md with the
        # same file inode and bytes. Native scope parents are not disposable.
        if not audit["path_inside_case"](path, case_root):
            raise AssertionError(f"foreign content modified: {path.name}")
        try:
            for parent, expected_parent in ancestors:
                directory = parent.lstat()
                if (not stat.S_ISDIR(directory.st_mode)
                        or (directory.st_mode, directory.st_dev, directory.st_ino)
                        != expected_parent):
                    raise AssertionError(f"foreign content modified: {path.name}")
            if (skill_children is not None
                    and tuple(sorted(child.name for child in path.parent.iterdir()))
                    != skill_children):
                raise AssertionError(f"foreign content modified: {path.name}")
            identity = path.lstat()
            current_identity = (identity.st_mode, identity.st_dev, identity.st_ino, identity.st_mtime_ns)
            data = path.read_bytes() if path.is_file() else None
        except OSError as error:
            raise AssertionError(f"foreign content modified: {path.name}") from error
        if current_identity != expected_identity or data != content.encode("utf-8"):
            raise AssertionError(f"foreign content modified: {path.name}")


def check_project_a_unchanged(observed, expected, current_lock, expected_lock, stage):
    """Fail if Project B's lifecycle rekeys or rewrites Project A."""
    if observed != expected:
        raise AssertionError(f"{stage}: Project A installation identities changed")
    if current_lock != expected_lock:
        raise AssertionError(f"{stage}: Project A signed lock changed")


def check_noop_update(observed, expected, current_lock, expected_lock, stage):
    """Require no-op update to preserve signed pins and installation IDs."""
    if observed != expected:
        raise AssertionError(f"{stage}: no-op update changed installation identities")
    if current_lock != expected_lock:
        raise AssertionError(f"{stage}: no-op update changed signed lock")

def scoped_snapshot(roots):
    """Use the shared byte/metadata snapshot authority for every protected root."""
    return audit["scoped_snapshot"](roots, snapshotter=audit["snapshot"])



def refusal_profile_snapshot(env, project, lock, *, directory_only=False):
    """Protect own durable/native state while deriving finite cold-cache names."""
    data = Path(env["AGENTPLUGINS_HOME"])
    if not audit["path_inside_case"](data, data.parent) or not data.parent.is_dir():
        raise AssertionError("corrupt restore cache root unsafe")
    parent = data.parent.lstat()
    subjects = {}
    for pin in lock.get("packages", []):
        subject = {name: pin.get(name) for name in ("tree_digest", "manifest_digest")}
        if not all(isinstance(value, str) and re.fullmatch(r"sha256:[0-9a-f]{64}", value)
                   for value in subject.values()):
            continue
        key = hashlib.sha256("\0".join((
            subject["tree_digest"], subject["manifest_digest"], "lintai", "0.1.3",
            REFUSAL_CACHE_POLICY["id"], "2", REFUSAL_CACHE_POLICY["digest"],
        )).encode()).hexdigest()
        subjects[f"security/assessments/{key}.json"] = subject
    return {
        "protected": scoped_snapshot([Path(env["HOME"]), project]),
        "data": scoped_snapshot([data]),
        "parent": (parent.st_mode, parent.st_dev, parent.st_ino),
        "subjects": subjects,
        "directory_only": directory_only,
    }


def check_refusal_profile_unchanged(env, project, before):
    """Only this refused command may add its own exact released cache files."""
    data = Path(env["AGENTPLUGINS_HOME"])
    current = refusal_profile_snapshot(env, project, {})
    if current["protected"] != before["protected"] or current["parent"] != before["parent"]:
        raise AssertionError("corrupt restore changed protected native/Project state")
    previous, observed = before["data"], current["data"]
    created_root = (previous.get("root-kind:0") == "absent"
                    and observed.get("root-kind:0") == "directory")
    for key, value in previous.items():
        if created_root and key == "root-kind:0":
            continue
        if observed.get(key) != value:
            raise AssertionError("corrupt restore changed existing data entry")
    files = {"directory-v1-cache.json", "directory-v1-cache.json.lock"}
    if not before.get("directory_only"):
        files.update((REFUSAL_SCANNER_PATH, *before["subjects"]))
    directories = {parent.as_posix() for name in files
                   for parent in Path(name).parents if parent != Path(".")}
    allowed = set()
    if created_root:
        entry = data.lstat()
        if not stat.S_ISDIR(entry.st_mode) or stat.S_IMODE(entry.st_mode) != 0o700:
            raise AssertionError("corrupt restore created unsafe data root")
        allowed.update(("root-kind:0", "metadata:0:."))
    for key in observed.keys() - previous.keys():
        if not key.startswith("metadata:0:") or key == "metadata:0:.":
            continue
        relative = key.removeprefix("metadata:0:")
        path = data / relative
        entry = path.lstat()
        if stat.S_ISDIR(entry.st_mode) and relative in directories:
            if stat.S_IMODE(entry.st_mode) != 0o700:
                raise AssertionError("corrupt restore created unsafe cache directory")
            allowed.update((key, f"directory:0:{relative}"))
        elif (stat.S_ISREG(entry.st_mode) and entry.st_nlink == 1
              and relative in files):
            mode = 0o700 if relative == REFUSAL_SCANNER_PATH else 0o600
            if stat.S_IMODE(entry.st_mode) != mode:
                raise AssertionError("corrupt restore created unsafe cache file")
            digest_key = f"{data.name}/{Path(relative)}"
            if relative == REFUSAL_SCANNER_PATH:
                if observed.get(digest_key) != REFUSAL_SCANNER_SHA256:
                    raise AssertionError("corrupt restore created unpinned scanner")
            elif relative in before["subjects"]:
                if entry.st_size > 1 << 20:
                    raise AssertionError("corrupt restore created oversized assessment")
                assessment = json.loads(path.read_text(encoding="utf-8"))
                if (not isinstance(assessment, dict)
                        or assessment.get("subject") != before["subjects"][relative]
                        or assessment.get("scanner") != {"id": "lintai", "version": "0.1.3"}
                        or assessment.get("policy") != REFUSAL_CACHE_POLICY
                        or assessment.get("schema_version") != 1):
                    raise AssertionError("corrupt restore created unrelated assessment")
            elif relative.endswith(".lock") and entry.st_size != 0:
                raise AssertionError("corrupt restore created invalid Directory cache lock")
            allowed.update((key, digest_key))
        else:
            raise AssertionError("corrupt restore created unexpected cache/data entry")
    if (observed.keys() - previous.keys()) - allowed:
        raise AssertionError("corrupt restore created unexpected data state")


def profile_snapshot(env, project):
    """Hash all files and directory entries in HOME, UAP data and Project."""
    return scoped_snapshot([Path(env["HOME"]), Path(env["AGENTPLUGINS_HOME"]), project])


def settled_profile_snapshot(env, project, *, attempts=12, interval=5):
    """Bounded *own-profile* cache/scanner settling before peer operations.

    The released installer can finish its own security cache writes after the
    command exits. Require three identical full-state snapshots before the
    first peer operation. Never rebaseline after a peer command: its writes
    must still fail the unchanged-profile guard.
    """
    previous = None
    identical = 0
    for attempt in range(attempts):
        observed = profile_snapshot(env, project)
        identical = identical + 1 if observed == previous else 1
        if identical >= 3:
            return observed
        previous = observed
        if attempt + 1 < attempts:
            sleep(interval)
    raise AssertionError("own profile did not quiesce before peer lifecycle")


def check_profile_unchanged(env, project, expected, stage):
    """A peer restore/remove must not alter the originating profile or Project."""
    observed = profile_snapshot(env, project)
    if observed != expected:
        # Report only bounded, sandbox-relative key names; never export values,
        # file content, absolute paths, or untrusted diagnostics. A changed
        # snapshot must remain a failure, not a permitted side effect.
        differing = sorted(key for key in (observed.keys() | expected.keys())
                           if observed.get(key) != expected.get(key))
        names = [re.sub(r"[^A-Za-z0-9_./:-]", "?", key)[:120]
                 for key in differing[:12]]
        raise AssertionError(
            f"{stage}: original isolated profile changed "
            f"(entries={len(differing)}, keys={names})")


def check_user_project_unchanged(project, expected, stage):
    """User-scoped commands must not change the working Project's files."""
    if scoped_snapshot([project]) != expected:
        raise AssertionError(f"{stage}: User lifecycle modified Project A")


def check_cold_restore(restored, original, original_lock, restored_lock,
                       original_manifest, restored_manifest, stage):
    """Require exact portable agreement, signed closure and independent IDs."""
    if set(restored) != EXPECTED:
        raise AssertionError(f"{stage}: mandatory signed closure incomplete")
    if set(restored.values()) & set(original.values()):
        raise AssertionError(f"{stage}: reused installation IDs across profiles")
    if restored_lock != original_lock:
        raise AssertionError(f"{stage}: cold restore modified pinned lock")
    if restored_manifest != original_manifest:
        raise AssertionError(f"{stage}: cold restore modified portable manifest")


def check_scope_id_isolation(selected, other_scopes, stage):
    """Refuse receipt ID reuse between independently installed User/Project scopes."""
    prior = {value for identities in other_scopes for value in identities.values()}
    if set(selected.values()) & prior:
        raise AssertionError(f"{stage}: reused installation IDs across scopes")


def copilot_user_c_settings_shape(env, case_root):
    """Read only bounded, redacted Copilot setting counts; never export native config."""
    # The fresh C profile is independent of A/B and already sandboxed.
    return {identity: audit["copilot_settings_sanitized_shape"](
        Path(env["HOME"]), case_root.resolve(strict=True), identity)
        for identity in sorted(UPDATE_ALIASES)}



copilot_json_document = audit["copilot_json_document"]


def copilot_owned_json(path, root):
    body, status = audit["copilot_settings_read"](path, root.resolve(strict=True))
    if status == "absent":
        return None, None
    if status is not None:
        raise AssertionError("Copilot owned JSON path unavailable or unsafe")
    return copilot_json_document(body), "sha256:" + hashlib.sha256(body.encode()).hexdigest()


def copilot_signed_user_versions(cli, env, project, policy):
    """Bind all four native identities to signed state and committed receipts."""
    current_vscode_boundary_kind(policy)
    rows = inventory(cli, env, project, "copilot", "user")
    public = identities(rows, "copilot", "user")
    if set(public) != EXPECTED:
        raise AssertionError("Copilot User public signed closure incomplete")
    for row in rows:
        bindings = row["clients"]
        if isinstance(bindings, dict):
            bindings = list(bindings.values())
        pin = FIXED_SIGNED_CLOSURE[row["directory"]["recorded_distribution"]]
        if bindings[0]["package_revision"].get("version") != pin["package_version"]:
            raise AssertionError("Copilot User public signed version changed")
    state, _ = copilot_owned_json(Path(env["AGENTPLUGINS_HOME"]) / "state-v2.json", project.parent)
    if not isinstance(state, dict) or not isinstance(state.get("installations"), list):
        raise AssertionError("Copilot User owned state unavailable")
    owners, expected_receipts, physical_ids, binding_ids = {}, set(), set(), set()
    for installation in state["installations"]:
        if not isinstance(installation, dict) or not isinstance(installation.get("clients"), dict):
            raise AssertionError("Copilot User state installation malformed")
        if not installation["clients"]:
            continue  # Previously removed Project data has no active binding.
        directory, source, package = (installation.get(key, {}) for key in ("directory", "source", "package"))
        distribution = directory.get("distribution_id")
        alias = next((name for name, item in REQUIRED_DISTRIBUTIONS.items() if item == distribution), None)
        if alias is None or alias in owners or installation.get("installation_id") != public.get(distribution):
            raise AssertionError("Copilot User state identity differs from public inventory")
        pin = FIXED_SIGNED_CLOSURE[distribution]
        if (installation.get("origin_mode") != "directory"
                or type(directory.get("desired_release_sequence")) is not int
                or directory["desired_release_sequence"] != pin["release_sequence"]
                or type(directory.get("snapshot_sequence")) is not int
                or directory["snapshot_sequence"] != policy["snapshot_sequence"]
                or directory.get("snapshot_digest") != policy["snapshot_digest"]
                or package.get("version") != pin["package_version"]
                or package.get("manifest_digest") != pin["manifest_digest"]
                or any(source.get(key) != pin[field] for key, field in (
                    ("repository", "repository"), ("package_subpath", "package_path"),
                    ("resolved_revision", "resolved_revision"), ("tree_digest", "tree_digest")))):
            raise AssertionError("Copilot User owned immutable source changed")
        bindings = installation["clients"]
        if len(bindings) != 1:
            raise AssertionError("Copilot User must retain one logical owner")
        binding_id, binding = next(iter(bindings.items()))
        if not isinstance(binding, dict):
            raise AssertionError("Copilot User physical binding malformed")
        physical = binding.get("physical_artifact_id")
        target = Path(binding.get("target_locator", ""))
        revision = binding.get("package_revision", {})
        if (not isinstance(binding_id, str) or not binding_id or binding_id in binding_ids
                or binding.get("client_binding_id") != binding_id
                or binding.get("client_id") != "copilot" or binding.get("scope") != "user"
                or binding.get("materialization") != "materialized" or binding.get("verification") != "installation_verified"
                or binding.get("affected_surfaces") != ["copilot", "vscode"]
                or not isinstance(physical, str) or not physical.strip() or physical != physical.strip() or physical in physical_ids
                or physical in (".", "..") or Path(physical).name != physical
                or not target.is_absolute() or not target.is_dir()
                or not audit["path_inside_case"](target, project.parent.resolve(strict=True))
                or not isinstance(revision, dict)
                or type(revision.get("release_sequence")) is not int
                or revision.get("distribution_id") != distribution
                or revision.get("release_sequence") != pin["release_sequence"]
                or any(revision.get(key) != pin[field] for key, field in (
                    ("version", "package_version"), ("resolved_revision", "resolved_revision"),
                    ("tree_digest", "tree_digest"), ("manifest_digest", "manifest_digest")))):
            raise AssertionError("Copilot User physical binding is unqualified")
        receipts = binding.get("receipts")
        if (not isinstance(receipts, list) or not receipts or any(not isinstance(item, dict)
                or item.get("client_binding_id") != binding_id or item.get("phase") != "committed"
                or item.get("active_path") != str(target)
                or not isinstance(item.get("operation_id"), str) or not item["operation_id"] for item in receipts)):
            raise AssertionError("Copilot User committed physical receipts missing")
        if len({item["operation_id"] for item in receipts}) != len(receipts):
            raise AssertionError("Copilot User physical receipt replay detected")
        marketplace = "agentplugins-" + hashlib.sha256(physical.strip().encode()).hexdigest()[:12]
        manifest, _ = copilot_owned_json(target / ".github/plugin/marketplace.json", project.parent)
        if (not isinstance(manifest, dict) or manifest.get("name") != marketplace
                or not isinstance(manifest.get("plugins"), list) or len(manifest["plugins"]) != 1
                or any(manifest["plugins"][0].get(key) != value for key, value in
                       (("name", alias), ("version", pin["package_version"]), ("source", ".")))):
            raise AssertionError("Copilot native marketplace differs from owned source")
        owners[alias] = {"version": pin["package_version"], "marketplace": marketplace,
                         "target_locator": str(target), "physical_artifact_id": physical}
        physical_ids.add(physical)
        binding_ids.add(binding_id)
        expected_receipts.add((alias, "copilot", public[distribution], binding_id,
                               "directory:" + distribution, pin["tree_digest"]))
    receipts = state.get("environment_receipts")
    if (set(owners) != UPDATE_ALIASES or not isinstance(receipts, list) or len(receipts) != 4
            or any(not isinstance(item, dict) or item.get("phase") != "committed"
                   or item.get("environment_id") != "user-default"
                   or not isinstance(item.get("receipt_id"), str) or not item["receipt_id"] for item in receipts)
            or len({item["receipt_id"] for item in receipts}) != 4
            or {tuple(item.get(key) for key in ("alias", "client_id", "installation_id", "client_binding_id", "source_key", "package_digest"))
                for item in receipts} != expected_receipts):
        raise AssertionError("Copilot User environment receipts differ from signed closure")
    return owners


COPILOT_BUILTIN_JSON = audit["COPILOT_BUILTIN_JSON"]
COPILOT_BUILTIN_TEXT = audit["COPILOT_BUILTIN_TEXT"]
COPILOT_EMPTY_TEXT = audit["COPILOT_EMPTY_TEXT"]
copilot_native_entries_state = audit["copilot_native_entries_state"]
copilot_native_text_state = audit["copilot_native_text_state"]


def copilot_native_scope_state(env, project, owners):
    """Always obtain complete JSON and text from the same isolated real CLI."""
    home = Path(env["HOME"])
    profile = home / ".copilot"
    executable = audit["AGENT_BINARIES"].get("copilot")
    if (not executable or not Path(executable).is_absolute() or not profile.is_dir()
            or not audit["path_inside_case"](profile, project.parent.resolve(strict=True))):
        raise AssertionError("Copilot native inventory prerequisites unqualified")
    before = profile_snapshot(env, project)
    try:
        responses = []
        for suffix in (["--json"], []):
            result = audit["call"]([executable, "plugin", "list", *suffix],
                {**env, "COPILOT_HOME": str(profile)}, project, 30, capture_full=True)
            if (type(result.get("exit_code")) is not int or result["exit_code"] != 0
                    or result.get("timeout") is not False or result.get("stderr_full") != ""
                    or not isinstance(result.get("stdout_full"), str)
                    or len(result["stdout_full"].encode("utf-8")) > 1 << 20):
                raise AssertionError("Copilot complete native listing did not succeed")
            responses.append(result["stdout_full"])
        state = copilot_native_entries_state(copilot_json_document(responses[0]), owners)
        copilot_native_text_state(responses[1], owners)
        if owners:
            settings, _ = copilot_owned_json(home / ".copilot/settings.json", project.parent)
            enabled = settings.get("enabledPlugins") if isinstance(settings, dict) else None
            qualified = {name + "@" + owner["marketplace"] for name, owner in owners.items()}
            if (not isinstance(enabled, dict) or set(enabled) != qualified
                    or any(value is not True for value in enabled.values())):
                raise AssertionError("Copilot native active settings differ from physical owners")
        return {**state, "profile": str(home.resolve(strict=True)), "owners": owners}
    finally:
        check_copilot_read_unchanged(env, project, before, "Copilot native read-only query", 2)


def check_copilot_read_unchanged(env, project, before, stage, log_limit):
    """Share the exact bounded native-read comparison with the adjunct audit."""
    audit["check_copilot_snapshot_unchanged"](
        Path(env["HOME"]), project.parent.resolve(strict=True), before,
        profile_snapshot(env, project), stage, log_limit)



def copilot_disabled_closure_only(shapes, native_before, native_after, env=None, case_root=None):
    """Bind the narrow BUG-045 predicate to actual profile and exact file bytes."""
    if (env is None or case_root is None or not isinstance(native_before, dict)
            or not isinstance(native_after, dict) or native_before.get("status") != "installed"
            or native_before.get("managed_count") != 4 or native_after.get("status") != "absent"
            or native_after.get("managed_count") != 0 or not isinstance(shapes, dict)
            or set(shapes) != {"before", "after_restore", "after_remove"}
            or any(not isinstance(shapes[phase], dict) or set(shapes[phase]) != UPDATE_ALIASES for phase in shapes)):
        return None
    home = str(Path(env["HOME"]).resolve(strict=True))
    owners = native_before.get("owners", {})
    if (native_before.get("profile") != home or native_after.get("profile") != home
            or set(owners) != UPDATE_ALIASES or native_after.get("owners") != {}):
        return None
    document, digest = copilot_owned_json(Path(env["HOME"]) / ".copilot/settings.json", case_root)
    expected = {name + "@" + owners[name]["marketplace"]: False for name in owners}
    if not copilot_disabled_document_matches(document, expected):
        return None
    if not all(audit["copilot_disabled_preference_only"](
            shapes["before"][name], shapes["after_restore"][name], shapes["after_remove"][name],
            native_before, native_after, [".copilot/settings.json"]) for name in UPDATE_ALIASES):
        return None
    return {"profile": home, "settings_digest": digest, "qualified_ids": sorted(expected)}


def copilot_disabled_document_matches(document, expected):
    """Only the two genuinely observed disabled preference documents qualify."""
    if not isinstance(document, dict) or set(document) not in (
            {"enabledPlugins"}, {"enabledPlugins", "extraKnownMarketplaces"}):
        return False
    if "extraKnownMarketplaces" in document and (
            type(document["extraKnownMarketplaces"]) is not dict or document["extraKnownMarketplaces"]):
        return False
    return (document.get("enabledPlugins") == expected
            and isinstance(document["enabledPlugins"], dict)
            and all(value is False for value in document["enabledPlugins"].values()))


def copilot_disabled_proof_matches(proof, home, case_root):
    if (not isinstance(proof, dict) or proof.get("profile") != str(home.resolve(strict=True))
            or not isinstance(proof.get("qualified_ids"), list) or len(proof["qualified_ids"]) != 4
            or any(not isinstance(item, str) or not re.fullmatch(r"[A-Za-z0-9._-]+@agentplugins-[0-9a-f]{12}", item)
                   for item in proof["qualified_ids"])
            or {item.split("@", 1)[0] for item in proof["qualified_ids"] if isinstance(item, str)} != UPDATE_ALIASES):
        return False
    document, digest = copilot_owned_json(home / ".copilot/settings.json", case_root)
    return (digest == proof.get("settings_digest")
            and copilot_disabled_document_matches(document, dict.fromkeys(proof["qualified_ids"], False)))


def copilot_native_public_state(state):
    """Keep ownership and physical paths private; export prior aggregate fields."""
    return {"status": state["status"], "managed_count": state["managed_count"]}


def copilot_project_mcp_residuals(project, identities, stage, case_root):
    """Inspect Copilot's two exact Project MCP locations by server member name."""
    def reject_constant(_):
        raise ValueError("invalid JSON constant")

    leftovers = set()
    for relative in (".github/mcp.json", ".mcp.json"):
        path = project / relative
        if not audit["path_inside_case"](path, project):
            raise AssertionError(f"{stage}: unsafe Copilot Project MCP config")
        try:
            identity = path.lstat()
        except FileNotFoundError:
            continue
        except OSError as error:
            raise AssertionError(f"{stage}: unreadable Copilot Project MCP config") from error
        if (not stat.S_ISREG(identity.st_mode) or identity.st_nlink != 1
                or identity.st_size > 1 << 20):
            raise AssertionError(f"{stage}: invalid Copilot Project MCP config")
        try:
            body, status = audit["copilot_settings_read"](path, case_root)
            if status is not None or not isinstance(body, str):
                raise ValueError("unsafe or disappeared MCP config")
            current = path.lstat()
            if (not stat.S_ISREG(current.st_mode) or current.st_nlink != 1
                    or (current.st_dev, current.st_ino) != (identity.st_dev, identity.st_ino)):
                raise AssertionError(f"{stage}: replaced Copilot Project MCP config")
            document = json.loads(body, object_pairs_hook=audit["_unique_json_object"],
                                  parse_constant=reject_constant)
            if (not isinstance(document, dict)
                    or document and not isinstance(document.get("mcpServers"), dict)):
                raise ValueError("expected nested MCP server object")
            servers = document.get("mcpServers", {})
            if any(not name or not isinstance(server, dict) for name, server in servers.items()):
                raise ValueError("invalid MCP server member")
        except (OSError, UnicodeError, ValueError) as error:
            raise AssertionError(f"{stage}: malformed Copilot Project MCP config") from error
        for name in servers:
            if (name.casefold() in identities
                    or any(re.fullmatch(re.escape(alias) + r"@agentplugins-[0-9a-f]{12}",
                                        name, re.IGNORECASE) for alias in identities)):
                leftovers.add(f"{relative}: MCP server {name}")
    return leftovers


def check_removed_native_components(target, home, case_root, stage, project_scope=False,
                                    copilot_disabled_proof=None):
    """Refuse native MCP and Skill remnants after an empty scoped inventory."""
    # isolated_client_env() canonicalizes HOME. On macOS TemporaryDirectory
    # can return /var/... while canonical HOME is /private/var/... . Accept
    # either spelling of the *same disposable root*, without resolving a
    # candidate path: that would hide a symlink inserted by a native client.
    if not case_root.is_dir() or case_root.is_symlink():
        raise AssertionError(f"{stage}: native HOME left disposable case")
    try:
        canonical_root = case_root.resolve(strict=True)
    except (OSError, RuntimeError, ValueError) as error:
        raise AssertionError(f"{stage}: native HOME left disposable case") from error

    def inside_case(candidate):
        return (audit["path_inside_case"](candidate, case_root)
                or audit["path_inside_case"](candidate, canonical_root))

    if not inside_case(home):
        raise AssertionError(f"{stage}: native HOME left disposable case")
    # Copilot Project Skills are under .github/skills, not its MCP config
    # roots (.copilot/.config/copilot). Reuse the audit's authoritative paths.
    relative_roots = (*audit["NATIVE_ROOTS"][target],
                      *audit["PROJECT_SKILL_ROOTS"].get(target, ()),
                      *COMPAT_SKILL_ROOTS.get(target, ()))
    if project_scope:
        relative_roots += PROJECT_ONLY_COMPAT_SKILL_ROOTS.get(target, ())
    roots = [home / relative for relative in relative_roots]
    if any(not inside_case(root) for root in roots):
        raise AssertionError(f"{stage}: unsafe native client root")
    if not home.is_dir():
        raise AssertionError(f"{stage}: invalid native client HOME")
    # A regular file at an expected native root (or intermediate .config)
    # is not absence of registration. The shared snapshot skips file roots.
    for root in roots:
        candidate = root
        while candidate != home:
            try:
                mode = candidate.lstat().st_mode
            except (FileNotFoundError, NotADirectoryError):
                pass  # Missing native paths are valid after removal.
            except OSError as error:
                raise AssertionError(f"{stage}: native root metadata unreadable") from error
            else:
                if not stat.S_ISDIR(mode):
                    raise AssertionError(f"{stage}: invalid native client root")
            candidate = candidate.parent
    observed = audit["snapshot"](roots)
    if any(value == "unreadable" for value in observed.values()):
        raise AssertionError(f"{stage}: native cleanup snapshot unreadable")
    leftovers = set()
    # The signed playwright-bridge distribution's native manifest alias is
    # "playwright"; checking its distribution slug alone misses a leftover.
    identities = UPDATE_ALIASES | {
        distribution.rsplit("/", 1)[-1] for distribution in EXPECTED
    }
    if target == "copilot" and project_scope:
        leftovers.update(copilot_project_mcp_residuals(home, identities, stage, case_root))
    for identity in sorted(identities):
        leftovers.update(audit["native_identity_residuals"](
            roots, identity, observed, target))
    # The shared native snapshot records files and links, but not empty
    # directories. An emptied owned Skill/plugin directory is still residue.
    names = {name.casefold() for name in identities}

    def refuse_walk_error(error):
        raise AssertionError(f"{stage}: native cleanup walk unreadable") from error

    for root in roots:
        if not root.is_dir():
            continue
        for base, directories, _ in os.walk(root, followlinks=False,
                                            onerror=refuse_walk_error):
            for name in directories:
                relative = (Path(base) / name).relative_to(root).as_posix()
                if audit["passive_native_path"](target, relative):
                    continue
                if (name.casefold() in names
                        or any(re.fullmatch(
                            re.escape(alias) + r"@agentplugins-[0-9a-f]{12}",
                            name, re.IGNORECASE) for alias in identities)):
                    leftovers.add(f"{root.name}/{relative}: directory")
    if leftovers:
        # BUG-045: vendor-disabled preferences are not active registrations,
        # but only a separate, exact native CLI absence + four-key proof can
        # permit this single retained file. All other roots remain strict.
        if (target == "copilot" and not project_scope and copilot_disabled_proof_matches(copilot_disabled_proof, home, case_root)
                and leftovers == {".copilot/settings.json"}):
            return
        raise AssertionError(f"{stage}: native Marketing Kit remnants: {sorted(leftovers)}")


def bind_pinned_launcher_cache(env, cache, sandbox):
    """Share only the SHA-checked released CLI binary, not native client state."""
    if (not cache.is_dir() or cache.is_symlink()
            or not audit["path_inside_case"](cache, sandbox)):
        raise AssertionError("released CLI cache must be a real in-sandbox directory")
    env["AGENTPLUGINS_CACHE_DIR"] = str(cache)


def execute(cli, env, project, target, label, *args):
    # Published dependency JSON includes the resolved scope. A wrong-scope
    # success receipt cannot qualify the caller's explicit User/Project request.
    if args.count("--scope") != 1:
        raise AssertionError(f"{label}: missing or ambiguous explicit scope")
    selected = args.index("--scope") + 1
    if selected >= len(args) or args[selected] not in ("user", "project"):
        raise AssertionError(f"{label}: invalid explicit scope")
    result = invoke([cli, *args, "--format", "json"], env, project, 180)
    envelope = verified(result, label, args[0], target, args[selected])
    if args[0] in ("add", "update", "install", "repair"):
        # Active results and retained no-op action IDs both belong to the
        # selected client/scope. Cold planned installs may omit action IDs.
        current = identities(
            inventory(cli, env, project, target, args[selected]),
            target, args[selected])
        verify_applied_receipt_inventory(envelope, current)
    return {"stage": label, "exit_code": result["exit_code"], "result": "passed"}


def guarded_native_attestation(attest, checks):
    """Validate peer scopes even when a native CLI fails before UAP remove."""
    try:
        return attest()
    finally:
        for check in checks:
            check()


def codex_user_native_remove_attestation(env, project):
    """Only attest after real native HubSpot removal from this disposable profile."""
    case_root = project.parent
    home = case_root / "home"
    if (env.get("HOME") != str(home)
            or env.get("CODEX_HOME") != str(home / ".codex")
            or not audit["path_inside_case"](home, case_root)
            or not audit["path_inside_case"](project, case_root)):
        raise AssertionError("Codex native removal requires the exact disposable profile")
    native = audit["uninstall_native_codex_plugin"](
        "hubspot-crm", home, case_root, env, project)
    if native.get("completed") is not True:
        raise AssertionError(
            "Codex native HubSpot uninstall unverified: " +
            str(native.get("reason") or "native removal and absence not confirmed"))
    return ("--external-uninstalled",)


def verify_vscode_user_policy_refusal(cli, env, project, guards):
    """Explicit VS Code still needs its own signed grant, before acquisition."""
    before = refusal_profile_snapshot(env, project, {}, directory_only=True)
    inventory_before = None
    try:
        refused = invoke([cli, "add", "marketing-kit", "--target", "vscode",
                          "--scope", "user", "--no-save", "--yes", "--format", "json"],
                         env, project, 180)
        # Frozen VS Code uses its documented direct --no-save route.
        expected = ('Resolving and validating one Agent Plugin package for every selected target...\n'
                    'agentplugins: no eligible directory release for "marketing-kit": '
                    '777genius/marketing-kit: release does not support complete '
                    'target set; missing vscode')
        code = refused.get("exit_code")
        # call() preserves the last 1800 raw characters without stripping.
        # This shorter exact diagnostic and empty stdout cannot be a truncated
        # tail: any truncated nonempty output has length 1800.
        if (type(code) is not int or code <= 0 or refused.get("timeout") is not False
                or refused.get("stderr_tail") not in (expected, expected + "\n",
                    expected.replace("\n", "\r\n") + "\r\n")
                or refused.get("_json") is not None
                or refused.get("stdout_tail") != ""):
            raise AssertionError("VS Code User add did not receive exact missing-vscode policy refusal")
        # The genuine CLI authenticates the Directory before this exact signed
        # eligibility refusal. A cold read may persist only its cache and empty
        # lock; scanners/assessments are unnecessary before package acquisition.
        check_refusal_profile_unchanged(env, project, before)
        inventory_before = profile_snapshot(env, project)
        for target in ("copilot", "vscode"):
            if inventory(cli, env, project, target, "user"):
                raise AssertionError("VS Code User refusal retained an installation")
        return {"stage": "Explicit VS Code User add refused by signed policy",
                "target": "vscode", "scope": "user", "contract": "negative_policy",
                "exit_code": code, "result": "expected_refusal",
                "reason": "missing_vscode_grant"}
    finally:
        try:
            check_refusal_profile_unchanged(env, project, before)
            if inventory_before is not None:
                check_profile_unchanged(env, project, inventory_before,
                                        "VS Code User refusal inventory")
        finally:
            for guard in guards:
                guard()


def verify_vscode_user_saved_refusal(cli, env, project, guards):
    """Refuse VS Code saved management independently of signed prepared grants."""
    before = refusal_profile_snapshot(env, project, {}, directory_only=True)
    inventory_before = None
    try:
        refused = invoke([cli, "add", "marketing-kit", "--target", "vscode",
                          "--scope", "user", "--yes", "--format", "json"],
                         env, project, 180)
        # Prepared VS Code may be signed, but is not a supported saved target.
        expected = ('agentplugins: target vscode cannot be saved; retry '
                    'agentplugins add marketing-kit --target vscode --no-save')
        code = refused.get("exit_code")
        # call() preserves the last 1800 raw characters without stripping.
        # This shorter exact diagnostic and empty stdout cannot be a truncated
        # tail: any truncated nonempty output has length 1800.
        if (type(code) is not int or code != 1 or refused.get("timeout") is not False
                or refused.get("stderr_tail") not in (expected, expected + "\n",
                    expected.replace("\n", "\r\n") + "\r\n")
                or refused.get("_json") is not None
                or refused.get("stdout_tail") != ""):
            raise AssertionError("VS Code User add did not receive exact saved-target refusal")
        # Saved-target preflight refuses before Directory acquisition. Previously
        # authenticated cache files must stay unchanged; the refused request
        # must not create native, portable, or durable state.
        check_refusal_profile_unchanged(env, project, before)
        inventory_before = profile_snapshot(env, project)
        for target in ("copilot", "vscode"):
            if inventory(cli, env, project, target, "user"):
                raise AssertionError("VS Code User refusal retained an installation")
        return {"stage": "Explicit VS Code User saved add refused",
                "target": "vscode", "scope": "user", "contract": "negative_saved_target",
                "exit_code": code, "result": "expected_refusal",
                "reason": "unsupported_saved_target"}
    finally:
        try:
            check_refusal_profile_unchanged(env, project, before)
            if inventory_before is not None:
                check_profile_unchanged(env, project, inventory_before,
                                        "VS Code User refusal inventory")
        finally:
            for guard in guards:
                guard()


def current_vscode_boundary_kind(policy):
    """Select an oracle from authenticated policy facts, never date or sequence."""
    managed = {client: {"scopes": ["user"], "delivery": "managed"}
               for client in ("codex", "cursor", "copilot", "claude", "gemini", "opencode")}
    if (not isinstance(policy, dict) or policy.get("source_mode") != "production_default"
            or type(policy.get("snapshot_sequence")) is not int or policy["snapshot_sequence"] <= 0
            or not isinstance(policy.get("snapshot_digest"), str)
            or not re.fullmatch(r"sha256:[0-9a-f]{64}", policy["snapshot_digest"])
            or policy.get("closure") != FIXED_SIGNED_CLOSURE):
        raise AssertionError("current VS Code boundary requires authenticated positive policy")
    targets = policy.get("root_target_compatibility")
    if targets == managed:
        return "signed_grant_refusal"
    if targets == {**managed, "vscode": {"scopes": ["user"], "delivery": "prepared"}}:
        return "saved_management_capability"
    raise AssertionError("current VS Code boundary has unqualified signed target shape")


def verify_current_vscode_user_boundary(cli, env, project, expected_policy, guards):
    """Reauthenticate current policy in a fresh profile before its mandatory oracle."""
    before = refusal_profile_snapshot(env, project, {}, directory_only=True)
    warmed = None
    try:
        kind = current_vscode_boundary_kind(expected_policy)
        if "AGENTPLUGINS_DIRECTORY_ORIGIN" in env or any(Path(env["AGENTPLUGINS_HOME"]).iterdir()):
            raise AssertionError("current boundary requires fresh production profile")
        verify_pinned_launcher(cli, env, project)
        policy = observe_production_policy(cli, env, project,
            "vscode" if kind == "saved_management_capability" else "codex", expected_policy)
        if current_vscode_boundary_kind(policy) != kind:
            raise AssertionError("current VS Code boundary policy changed")
        check_refusal_profile_unchanged(env, project, before)
        warmed = profile_snapshot(env, project)
        for guard in guards:
            guard()
        verify = (verify_vscode_user_saved_refusal if kind == "saved_management_capability"
                  else verify_vscode_user_policy_refusal)
        refused = verify(cli, env, project, ())
        if type(refused.get("exit_code")) is not int or refused["exit_code"] != 1:
            raise AssertionError("current VS Code boundary requires exact integer exit1")
        check_profile_unchanged(env, project, warmed, "current VS Code boundary refusal")
        return {"stage": "Current-production VS Code User boundary: " + kind,
                "target": "vscode", "scope": "user", "contract": kind,
                "exit_code": refused["exit_code"], "result": "expected_refusal", "reason": refused["reason"],
                "policy": policy, "native_editor_runtime": "not_qualified"}
    finally:
        try:
            check_refusal_profile_unchanged(env, project, before)
            if warmed is not None:
                check_profile_unchanged(env, project, warmed, "current boundary final inventory")
        finally:
            for guard in guards:
                guard()


def verify_fixed_vscode_user_policy_refusal(cli, env, project, guards):
    """Authenticate unchanged45 in a fresh profile, then freeze its warmed state."""
    before = refusal_profile_snapshot(env, project, {}, directory_only=True)
    warmed = None
    try:
        if env.get("AGENTPLUGINS_DIRECTORY_ORIGIN") != FIXED_REFUSAL_ORIGIN:
            raise AssertionError("fixed45 refusal origin differs")
        if any(Path(env["AGENTPLUGINS_HOME"]).iterdir()):
            raise AssertionError("fixed45 refusal requires a fresh empty data profile")
        verify_pinned_launcher(cli, env, project)
        policy = inspect_signed_closure(cli, env, project, "codex", FIXED_REFUSAL_POLICY)
        check_refusal_profile_unchanged(env, project, before)
        warmed = profile_snapshot(env, project)
        for guard in guards:
            guard()
        step = verify_vscode_user_policy_refusal(cli, env, project, ())
        step["policy"] = {"source_mode": "fixed_immutable", "ledger_commit": FIXED_REFUSAL_LEDGER,
                          "origin": FIXED_REFUSAL_ORIGIN, **policy}
        return step
    finally:
        try:
            check_refusal_profile_unchanged(env, project, before)
            if warmed is not None:
                check_profile_unchanged(env, project, warmed, "fixed45 warmed refusal")
        finally:
            for guard in guards:
                guard()


def verify_final_production_observation(cli, env, project, target, expected_policy, guards):
    """Close policy continuity with ordinary catalog info in never-installed E."""
    try:
        if any(Path(env["AGENTPLUGINS_HOME"]).iterdir()):
            raise AssertionError("final policy observation requires never-installed profile E")
        boundary = None
        if target == "copilot":
            boundary = verify_current_vscode_user_boundary(
                cli, env, project, expected_policy, guards)
            policy = boundary["policy"]
        else:
            verify_pinned_launcher(cli, env, project)
            policy = observe_production_policy(cli, env, project, target, expected_policy)
        if policy != expected_policy:
            raise AssertionError("final production policy observation changed")
        return {"status": "passed", "source_profile": "E", "profile_usage": "never_installed",
                "policy": policy}, boundary
    finally:
        failures = []
        for guard in guards:
            try:
                guard()
            except Exception as error:
                failures.append(error)
        if failures:
            raise failures[0]


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--target", required=True, choices=["codex", "cursor", "claude", "gemini", "opencode", "copilot"])
    parser.add_argument("--report", required=True)
    parser.add_argument("--cli", default="agentplugins")
    options = parser.parse_args()
    report_path = Path(options.report)
    report_path.parent.mkdir(parents=True, exist_ok=True)
    report = {"cli_version": "0.1.102", "target": options.target, "source": "real signed marketing-kit", "steps": [], "status": "failed",
              "qualification_scope": "user_and_project", "two_scope_lifecycle_qualified": False,
              "positive_policy": None, "logical_owner_evidence": None,
              "final_policy_observation": {"status": "not_run", "source_profile": "E",
                                           "profile_usage": "never_installed"},
              "current_vscode_user_boundary": {
                  "expectation": "authenticated_current_policy_boundary",
                  "status": "not_run" if options.target == "copilot" else "not_applicable"},
              "scope_results": {"project": {"expectation": "positive_lifecycle", "status": "not_run"},
                                "user": {"expectation": "positive_lifecycle", "status": "not_run"}},
              "historical_policy_results": {"signed45": {
                  "status": "separate_point_in_time_audit", "required_for_current_qualification": False}}}
    try:
        with tempfile.TemporaryDirectory(prefix="uap-real-marketing-kit-") as location:
            root = Path(location)
            # The npm 0.1.102 launcher validates the SHA-256 on every cache hit.
            # Keep one private executable cache for A/B: cold restore is still
            # independent in lock, receipts, clients and native HOME/profile.
            launcher_cache = root / "released-cli-cache"
            launcher_cache.mkdir(mode=0o700)
            env, project_a, foreign_a, project_foreign_a, native_foreign_a = run_case(options.cli, options.target, root, "A")
            bind_pinned_launcher_cache(env, launcher_cache, root)
            foreign_a_files = pin_foreign([(foreign_a, "unmanaged user data\n"),
                                           (project_foreign_a, "unmanaged project data\n"),
                                           *native_foreign_a], root)
            foreign_files = list(foreign_a_files)
            project_before_user = None
            protected_b = None
            protected_c_project = None
            protected_c_profile = None
            verify_pinned_launcher(options.cli, env, project_a)
            report["positive_policy"] = observe_production_policy(
                options.cli, env, project_a, options.target,
                guards=(lambda: check_foreign(foreign_files, root),))
            def run(e, p, label, *args):
                try:
                    report["steps"].append(execute(options.cli, e, p, options.target, label, *args))
                finally:
                    # Also protect foreign state when a lifecycle command refuses/fails.
                    check_foreign(foreign_files, root)
                    if project_before_user is not None:
                        check_user_project_unchanged(project_a, project_before_user, label)
                    if protected_b is not None:
                        check_profile_unchanged(env_b, project_b, protected_b, label)
                    if protected_c_project is not None:
                        check_user_project_unchanged(project_c, protected_c_project, label)
                    if protected_c_profile is not None:
                        check_profile_unchanged(env_c, project_c, protected_c_profile, label)
            def copilot_read(e, p, action, native_commands=0):
                own = settled_profile_snapshot(e, p)
                peers = [(other_env, other_project, profile_snapshot(other_env, other_project))
                         for other_env, other_project in ((env, project_a), (env_b, project_b), (env_c, project_c))
                         if other_env is not e]
                try:
                    return action()
                finally:
                    checks = [lambda: check_copilot_read_unchanged(e, p, own, "Copilot native proof read", native_commands),
                              lambda: check_foreign(foreign_files, root),
                              lambda: check_user_project_unchanged(project_a, project_before_user, "Copilot native proof Project")]
                    checks.extend(lambda pe=pe, pp=pp, before=before: check_profile_unchanged(
                        pe, pp, before, "Copilot native proof peer") for pe, pp, before in peers)
                    failures = []
                    for check in checks:
                        try:
                            check()
                        except (AssertionError, OSError) as error:
                            failures.append(error)
                    if failures:
                        raise failures[0]

            scoped = ("--scope", "project", "--project-root", str(project_a))
            run(env, project_a, "add", "add", "marketing-kit", "--target", options.target, *scoped)
            first = identities(inventory(options.cli, env, project_a, options.target), options.target)
            if set(first) != EXPECTED:
                raise AssertionError(f"mandatory Marketing Kit closure incomplete: {sorted(first)}")
            for name in ("uap.toml", "uap.lock"):
                if not (project_a / name).is_file():
                    raise AssertionError(f"missing Project agreement: {name}")
            expected_pins = {"directory:" + item for item in EXPECTED}
            project_pins = lock_ids(project_a, options.target, expected_sources=FIXED_SIGNED_CLOSURE)
            if project_pins != expected_pins:
                raise AssertionError(f"wrong locked signed source set: {sorted(project_pins)}")
            portable_lock = (project_a / "uap.lock").read_bytes()
            run(env, project_a, "repeat add", "add", "marketing-kit", "--target", options.target, *scoped)
            if identities(inventory(options.cli, env, project_a, options.target), options.target) != first:
                raise AssertionError("repeat add changed installation identities")
            if (project_a / "uap.lock").read_bytes() != portable_lock:
                raise AssertionError("repeat add changed the immutable lock")
            # Real update command, not a mocked planner; unchanged upstream pins
            # are still an observable no-update lifecycle.
            run(env, project_a, "update", "update", "marketing-kit", *scoped)
            identities_after_update = identities(inventory(options.cli, env, project_a, options.target), options.target)
            if set(identities_after_update) != EXPECTED:
                raise AssertionError("update lost a mandatory dependency")
            check_noop_update(identities_after_update, first,
                              (project_a / "uap.lock").read_bytes(), portable_lock,
                              "Project update")
            # Real repair on intact signed pins: this is not drift-recovery proof.
            # A repair may refresh owned receipts, but not the locked closure.
            pinned_after_update = (project_a / "uap.lock").read_bytes()
            manifest_after_update = (project_a / "uap.toml").read_bytes()
            run(env, project_a, "repair intact", "repair", "marketing-kit",
                "--target", options.target, *scoped)
            if identities(inventory(options.cli, env, project_a, options.target), options.target) != identities_after_update:
                raise AssertionError("repair changed installation identities")
            if (project_a / "uap.lock").read_bytes() != pinned_after_update:
                raise AssertionError("repair changed the signed lock")
            native_a_before_b = settled_profile_snapshot(env, project_a)
            env_b, project_b, foreign_b, project_foreign_b, native_foreign_b = run_case(options.cli, options.target, root, "B", project_a)
            bind_pinned_launcher_cache(env_b, launcher_cache, root)
            foreign_b_files = pin_foreign([(foreign_b, "unmanaged user data\n"),
                                           (project_foreign_b, "unmanaged project data\n"),
                                           *native_foreign_b], root)
            foreign_files.extend(foreign_b_files)
            # A new disposable B fixture cannot change an already-settled A.
            # Check before B's first CLI call to distinguish fixture drift from
            # a genuine cross-profile mutation by B's locked restore.
            check_profile_unchanged(env, project_a, native_a_before_b,
                                    "Project B fixture preparation")
            observe_production_policy(options.cli, env_b, project_b, options.target,
                report["positive_policy"], (
                    lambda: check_foreign(foreign_files, root),
                    lambda: check_profile_unchanged(env, project_a, native_a_before_b, "Project B policy"),
                ))
            scoped_b = ("--scope", "project", "--project-root", str(project_b))
            # The genuine published CLI must refuse a corrupted child pin
            # without modifying native or durable state in cold Project B.
            negative_exit = verify_corrupt_lock_refusal(
                options.cli, env_b, project_b, options.target, "project", (
                    lambda: check_foreign(foreign_files, root),
                    lambda: check_profile_unchanged(
                        env, project_a, native_a_before_b, "corrupt Project B restore"),
                ))
            report["steps"].append({
                "stage": "corrupt Project signed lock refused before install",
                "exit_code": negative_exit, "result": "passed"})
            run(env_b, project_b, "cold locked restore", "install", "--locked", "--yes", *scoped_b)
            check_profile_unchanged(env, project_a, native_a_before_b, "cold restore B")
            restored = identities(inventory(options.cli, env_b, project_b, options.target), options.target)
            check_cold_restore(restored, identities_after_update, pinned_after_update,
                               (project_b / "uap.lock").read_bytes(),
                               manifest_after_update, (project_b / "uap.toml").read_bytes(),
                               "Project B")
            if (project_a / "uap.toml").read_bytes() != manifest_after_update:
                raise AssertionError("Project B restore rewrote Project A portable manifest")
            check_project_a_unchanged(
                identities(inventory(options.cli, env, project_a, options.target), options.target),
                identities_after_update, (project_a / "uap.lock").read_bytes(),
                pinned_after_update, "cold restore B")
            native_a_before_b = profile_snapshot(env, project_a)
            run(env_b, project_b, "remove B", "remove", "marketing-kit", *scoped_b)
            check_profile_unchanged(env, project_a, native_a_before_b, "remove B")
            if identities(inventory(options.cli, env_b, project_b, options.target), options.target):
                raise AssertionError("Project B retains root or mandatory children after remove")
            # Empty signed inventory must also imply native absence in both
            # the isolated client HOME and the Project's own client settings.
            check_removed_native_components(options.target, Path(env_b["HOME"]),
                                            root, "Project B native HOME remove")
            check_removed_native_components(options.target, project_b,
                                            root, "Project B native Project remove", project_scope=True)
            check_project_a_unchanged(
                identities(inventory(options.cli, env, project_a, options.target), options.target),
                identities_after_update, (project_a / "uap.lock").read_bytes(),
                pinned_after_update, "remove B")
            if (project_a / "uap.toml").read_bytes() != manifest_after_update:
                raise AssertionError("Project B remove rewrote Project A portable manifest")
            # A's remaining lifecycle and User C must not mutate the removed B profile.
            protected_b = settled_profile_snapshot(env_b, project_b)
            run(env, project_a, "remove A", "remove", "marketing-kit", *scoped)
            if identities(inventory(options.cli, env, project_a, options.target), options.target):
                raise AssertionError("Project A retains root or mandatory children after remove")
            check_removed_native_components(options.target, Path(env["HOME"]),
                                            root, "Project A native HOME remove")
            check_removed_native_components(options.target, project_a,
                                            root, "Project A native Project remove", project_scope=True)
            for e in (env, env_b):
                if (Path(e["AGENTPLUGINS_HOME"]) / "uap.toml").exists():
                    raise AssertionError("Project installation unexpectedly wrote User manifest")
            check_foreign(foreign_files, root)
            check_profile_unchanged(env_b, project_b, protected_b, "Project final inventory")
            report["scope_results"]["project"]["status"] = "passed"
            # Check the same signed closure through a real, separate User lifecycle.
            # Every User command must preserve all of Project A, not just the
            # portable agreement or known foreign sentinels.
            project_before_user = scoped_snapshot([project_a])
            project_files = {
                name: (project_a / name).read_bytes() if (project_a / name).exists() else None
                for name in ("uap.toml", "uap.lock")
            }
            copilot_a_shapes = ({"before": copilot_user_c_settings_shape(env, root)}
                                if options.target == "copilot" else None)
            user_scoped = ("--scope", "user")
            def user_inventory():
                return identities(inventory(options.cli, env, project_a, options.target, "user"),
                                  options.target, "user")
            run(env, project_a, "User add", "add", "marketing-kit",
                "--target", options.target, *user_scoped)
            user_first = user_inventory()
            if set(user_first) != EXPECTED:
                raise AssertionError("User add lost mandatory signed Marketing Kit closure")
            check_scope_id_isolation(user_first, (first, restored), "User add")
            user_portable = Path(env["AGENTPLUGINS_HOME"])
            for name in ("uap.toml", "uap.lock"):
                if not (user_portable / name).is_file():
                    raise AssertionError(f"User add did not persist portable {name}")
            if lock_ids(user_portable, options.target, expected_sources=FIXED_SIGNED_CLOSURE) != expected_pins:
                raise AssertionError("User add did not pin complete signed closure")
            user_initial_lock = (user_portable / "uap.lock").read_bytes()
            if identities(inventory(options.cli, env, project_a, options.target), options.target):
                raise AssertionError("User add recreated removed Project installations")
            run(env, project_a, "User repeat add", "add", "marketing-kit",
                "--target", options.target, *user_scoped)
            if user_inventory() != user_first:
                raise AssertionError("User repeat add changed installation identities")
            if (user_portable / "uap.lock").read_bytes() != user_initial_lock:
                raise AssertionError("User repeat add modified signed lock")
            run(env, project_a, "User update", "update", "marketing-kit", *user_scoped)
            user_after_update = user_inventory()
            if set(user_after_update) != EXPECTED:
                raise AssertionError("User update lost mandatory signed closure")
            if lock_ids(user_portable, options.target, expected_sources=FIXED_SIGNED_CLOSURE) != expected_pins:
                raise AssertionError("User update changed mandatory signed pins")
            check_noop_update(user_after_update, user_first,
                              (user_portable / "uap.lock").read_bytes(), user_initial_lock,
                              "User update")
            user_lock_after_update = (user_portable / "uap.lock").read_bytes()
            user_manifest_after_update = (user_portable / "uap.toml").read_bytes()
            run(env, project_a, "User repair intact", "repair", "marketing-kit",
                "--target", options.target, *user_scoped)
            if user_inventory() != user_after_update:
                raise AssertionError("User repair changed installation identities")
            if (user_portable / "uap.lock").read_bytes() != user_lock_after_update:
                raise AssertionError("User repair modified signed lock")
            if copilot_a_shapes is not None:
                copilot_a_shapes["after_restore"] = copilot_user_c_settings_shape(env, root)

            # Project B has previously installed packages; a true cold User
            # restore needs a third HOME with no prior installs or native state.
            env_c, project_c, foreign_c, project_foreign_c, native_foreign_c = (
                run_case(options.cli, options.target, root, "C"))
            bind_pinned_launcher_cache(env_c, launcher_cache, root)
            # User operations must not mutate their own working Project C.
            protected_c_project = scoped_snapshot([project_c])
            foreign_files.extend(pin_foreign([(foreign_c, "unmanaged user data\n"),
                                              (project_foreign_c, "unmanaged project data\n"),
                                              *native_foreign_c], root))
            # Read-only forensic baseline: native Copilot may retain disabled
            # settings after uninstall. Never treat a residue as a PASS here.
            copilot_c_shapes = ({"before": copilot_user_c_settings_shape(env_c, root)}
                                if options.target == "copilot" else None)
            # Transfer only portable manifest and signed lock, not User A's
            # receipts, native config, store, or installation IDs.
            user_portable_c = Path(env_c["AGENTPLUGINS_HOME"])
            user_portable_c.mkdir(parents=True, exist_ok=True)
            for name in ("uap.toml", "uap.lock"):
                shutil.copyfile(user_portable / name, user_portable_c / name)
            user_a_native_before_c = settled_profile_snapshot(env, project_a)
            observe_production_policy(options.cli, env_c, project_c, options.target,
                report["positive_policy"], (
                    lambda: check_foreign(foreign_files, root),
                    lambda: check_profile_unchanged(env, project_a, user_a_native_before_c, "User C policy"),
                    lambda: check_profile_unchanged(env_b, project_b, protected_b, "User C policy"),
                    lambda: check_user_project_unchanged(project_c, protected_c_project, "User C policy"),
                ))
            negative_exit = verify_corrupt_lock_refusal(
                options.cli, env_c, project_c, options.target, "user", (
                    lambda: check_foreign(foreign_files, root),
                    lambda: check_profile_unchanged(
                        env, project_a, user_a_native_before_c, "corrupt User C restore"),
                    lambda: check_profile_unchanged(
                        env_b, project_b, protected_b, "corrupt User C restore"),
                    lambda: check_user_project_unchanged(
                        project_c, protected_c_project, "corrupt User C restore"),
                ))
            report["steps"].append({
                "stage": "corrupt User signed lock refused before install",
                "exit_code": negative_exit, "result": "passed"})
            run(env_c, project_c, "User cold locked restore C",
                "install", "--locked", "--yes", *user_scoped)
            check_profile_unchanged(env, project_a, user_a_native_before_c,
                                   "User cold restore C")
            restored_user_c = identities(
                inventory(options.cli, env_c, project_c, options.target, "user"),
                options.target, "user")
            check_cold_restore(restored_user_c, user_after_update, user_lock_after_update,
                               (user_portable_c / "uap.lock").read_bytes(),
                               user_manifest_after_update, (user_portable_c / "uap.toml").read_bytes(),
                               "User C")
            if copilot_c_shapes is not None:
                copilot_c_shapes["after_restore"] = copilot_user_c_settings_shape(env_c, root)
            check_scope_id_isolation(restored_user_c, (first, restored),
                                     "User C cold restore")
            if (user_portable / "uap.toml").read_bytes() != user_manifest_after_update:
                raise AssertionError("User C restore rewrote User A portable manifest")
            if user_inventory() != user_after_update:
                raise AssertionError("User C restore mutated User A installations")
            if (user_portable / "uap.lock").read_bytes() != user_lock_after_update:
                raise AssertionError("User C restore rewrote User A signed lock")
            # Pin A *before* running C's external native uninstall. Taking the
            # snapshot after that command would conceal a cross-profile write.
            user_a_native_before_c = profile_snapshot(env, project_a)
            remove_c_attestation = ()
            if options.target == "codex":
                remove_c_attestation = guarded_native_attestation(
                    lambda: codex_user_native_remove_attestation(env_c, project_c),
                    (
                        lambda: check_foreign(foreign_files, root),
                        lambda: check_profile_unchanged(
                            env, project_a, user_a_native_before_c, "User C native uninstall"),
                        lambda: check_profile_unchanged(
                            env_b, project_b, protected_b, "User C native uninstall"),
                        lambda: check_user_project_unchanged(
                            project_c, protected_c_project, "User C native uninstall"),
                    ))
                report["steps"].append({
                    "stage": "User C native HubSpot uninstall",
                    "exit_code": 0, "result": "passed",
                })
            owners_c = (copilot_read(env_c, project_c, lambda: copilot_signed_user_versions(
                options.cli, env_c, project_c, report["positive_policy"])) if copilot_c_shapes is not None else None)
            native_c_before = (copilot_read(env_c, project_c,
                lambda: copilot_native_scope_state(env_c, project_c, owners_c), 2)
                if copilot_c_shapes is not None else None)
            run(env_c, project_c, "User remove C", "remove", "marketing-kit",
                *user_scoped, *remove_c_attestation)
            check_profile_unchanged(env, project_a, user_a_native_before_c,
                                   "User remove C")
            if identities(inventory(options.cli, env_c, project_c, options.target, "user"),
                          options.target, "user"):
                raise AssertionError("User C remove retained required closure")
            copilot_c_disabled = False
            if copilot_c_shapes is not None:
                copilot_c_shapes["after_remove"] = copilot_user_c_settings_shape(env_c, root)
                native_c_after = copilot_read(env_c, project_c, lambda: copilot_native_scope_state(env_c, project_c, {}), 2)
                copilot_c_disabled = copilot_disabled_closure_only(
                    copilot_c_shapes, native_c_before, native_c_after, env_c, root)
                report["copilot_user_c_settings_shape"] = copilot_c_shapes
                report["copilot_user_c_native_state"] = {
                    "before": copilot_native_public_state(native_c_before),
                    "after": copilot_native_public_state(native_c_after),
                    "disabled_only": bool(copilot_c_disabled)}
            check_removed_native_components(options.target, Path(env_c["HOME"]),
                                            root, "User C remove",
                                            copilot_disabled_proof=copilot_c_disabled)
            if user_inventory() != user_after_update:
                raise AssertionError("User C remove mutated User A installations")
            if (user_portable / "uap.lock").read_bytes() != user_lock_after_update:
                raise AssertionError("User C remove rewrote User A signed lock")
            if (user_portable / "uap.toml").read_bytes() != user_manifest_after_update:
                raise AssertionError("User C remove rewrote User A portable manifest")
            # Once User C is removed, User A's remaining native cleanup must
            # not recreate receipts or settings in the independent C profile.
            protected_c_profile = settled_profile_snapshot(env_c, project_c)
            # Codex user plugin dependencies require genuine external native removal.
            # Reuse the audited, pinned Codex CLI path and exact native absence oracle.
            user_remove_attestation = ()
            if options.target == "codex":
                user_remove_attestation = guarded_native_attestation(
                    lambda: codex_user_native_remove_attestation(env, project_a),
                    (
                        lambda: check_foreign(foreign_files, root),
                        lambda: check_user_project_unchanged(
                            project_a, project_before_user, "User A native uninstall"),
                        lambda: check_profile_unchanged(
                            env_b, project_b, protected_b, "User A native uninstall"),
                        lambda: check_user_project_unchanged(
                            project_c, protected_c_project, "User A native uninstall"),
                        lambda: check_profile_unchanged(
                            env_c, project_c, protected_c_profile, "User A native uninstall"),
                    ))
                report["steps"].append({
                    "stage": "User native HubSpot uninstall",
                    "exit_code": 0, "result": "passed",
                })
            owners_a = (copilot_read(env, project_a, lambda: copilot_signed_user_versions(
                options.cli, env, project_a, report["positive_policy"])) if copilot_a_shapes is not None else None)
            native_a_before = (copilot_read(env, project_a,
                lambda: copilot_native_scope_state(env, project_a, owners_a), 2)
                if copilot_a_shapes is not None else None)
            run(env, project_a, "User remove", "remove", "marketing-kit",
                *user_scoped, *user_remove_attestation)
            if user_inventory():
                raise AssertionError("User removal retained Marketing Kit or mandatory children")
            copilot_a_disabled = False
            if copilot_a_shapes is not None:
                copilot_a_shapes["after_remove"] = copilot_user_c_settings_shape(env, root)
                native_a_after = copilot_read(env, project_a, lambda: copilot_native_scope_state(env, project_a, {}), 2)
                copilot_a_disabled = copilot_disabled_closure_only(
                    copilot_a_shapes, native_a_before, native_a_after, env, root)
                report["copilot_user_a_native_state"] = {
                    "before": copilot_native_public_state(native_a_before),
                    "after": copilot_native_public_state(native_a_after),
                    "disabled_only": bool(copilot_a_disabled)}
                report["copilot_user_a_settings_shape"] = copilot_a_shapes
            check_removed_native_components(options.target, Path(env["HOME"]),
                                            root, "User A remove",
                                            copilot_disabled_proof=copilot_a_disabled)
            for name, expected_bytes in project_files.items():
                path = project_a / name
                current_bytes = path.read_bytes() if path.exists() else None
                if current_bytes != expected_bytes:
                    raise AssertionError(f"User lifecycle modified Project artifact {name}")
            # The last inventory/read assertions are outside run()'s finally.
            check_foreign(foreign_files, root)
            check_user_project_unchanged(project_a, project_before_user, "User final inventory")
            check_profile_unchanged(env_b, project_b, protected_b, "User final inventory")
            check_user_project_unchanged(project_c, protected_c_project, "User final inventory")
            check_profile_unchanged(env_c, project_c, protected_c_profile,
                                    "User final inventory")
            # Released info resolves retained removed records before catalog
            # selectors. Keep A/B/C intact and reobserve the same policy in a
            # never-installed E, rather than reinterpret their installed schema.
            protected_a = settled_profile_snapshot(env, project_a)
            env_e, project_e, foreign_e, project_foreign_e, native_foreign_e = (
                run_case(options.cli, options.target, root, "E"))
            bind_pinned_launcher_cache(env_e, launcher_cache, root)
            foreign_files.extend(pin_foreign([(foreign_e, "unmanaged user data\n"),
                (project_foreign_e, "unmanaged project data\n"), *native_foreign_e], root))
            guards = (
                lambda: check_foreign(foreign_files, root),
                lambda: check_profile_unchanged(env, project_a, protected_a, "final production policy"),
                lambda: check_profile_unchanged(env_b, project_b, protected_b, "final production policy"),
                lambda: check_profile_unchanged(env_c, project_c, protected_c_profile, "final production policy"),
            )
            for guard in guards:
                guard()
            observation, boundary = verify_final_production_observation(
                options.cli, env_e, project_e, options.target, report["positive_policy"], guards)
            report["final_policy_observation"] = observation
            if options.target == "copilot":
                # Never change a production profile's origin or downgrade its cache.
                # E is a fresh current-policy case, using the same exact CLI.
                report["logical_owner_evidence"] = {
                    "scope": "user", "logical_owner": "copilot", "packages": sorted(EXPECTED),
                    "affected_surfaces": ["copilot", "vscode"], "separate_vscode_binding": False,
                    "native_editor_runtime": "not_qualified"}
                report["steps"].append(boundary)
                report["current_vscode_user_boundary"].update(
                    status="refused_as_expected", contract=boundary["contract"],
                    reason=boundary["reason"], policy=boundary["policy"])
            report["scope_results"]["user"]["status"] = "passed"
        # TemporaryDirectory cleanup is part of qualification; exceptions from
        # its __exit__ must never leave a successful two-scope result behind.
        report["two_scope_lifecycle_qualified"] = True
        report["status"] = "passed"
    except Exception as error:
        report["status"] = "failed"
        report["failure"] = str(error)[:2500]
        traceback.print_exc()
    finally:
        report_path.write_text(json.dumps(report, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
        print(json.dumps(report, indent=2, ensure_ascii=False))
    return 0 if report["status"] == "passed" else 1


if __name__ == "__main__":
    raise SystemExit(main())

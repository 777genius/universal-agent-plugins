#!/usr/bin/env python3
"""Actual public npm 0.1.102 + signed Marketing Kit Project lifecycle, no mocks."""
import argparse
import json
import os
import re
from pathlib import Path
import runpy
import shutil
import stat
import subprocess
import tempfile
from time import sleep
import traceback

ROOT = Path(__file__).resolve().parents[1]
audit = runpy.run_path(str(ROOT / "scripts/live-install-audit.py"))
isolated_env = audit["isolated_client_env"]
invoke = audit["call"]
EXPECTED = {
    "777genius/marketing-kit",
    "777genius/notion",
    "777genius/hubspot-crm",
    "777genius/playwright-bridge",
}


# Published npm 0.1.102 dependency operations include result=success AND
# outcome/actions/results. Require exact action evidence; process exit 0 or
# result=success alone is not sufficient.
UPDATE_ALIASES = {"marketing-kit", "notion", "hubspot-crm", "playwright"}
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
LEGACY_ACTIONS = {
    "update": {"unchanged"},
    "repair": {"repair"},
    "install": {"install", "repair", "unchanged"},
    "remove": {"remove", "unchanged"},
}


def valid_legacy_dependency_result(envelope, operation, target=None):
    """Validate real published dependency receipts for the selected client."""
    if (operation not in LEGACY_ACTIONS
            or ("result" in envelope and envelope["result"] != "success")
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
    if operation in ("update", "install") and set(aliases) != UPDATE_ALIASES:
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
                or (action.get("installation_id")
                    and result.get("installation_id") != action["installation_id"])):
            return False
    return True


def verified(result, stage, operation=None, target=None):
    envelope = result.get("_json")
    if (result.get("exit_code") != 0 or result.get("timeout")
            or not isinstance(envelope, dict)):
        raise AssertionError(f"{stage}: CLI did not succeed with JSON: {result}")
    # The pinned 0.1.102 dependency commands must report explicit actions
    # and results even when they also emit result=success.
    if operation in LEGACY_ACTIONS:
        if valid_legacy_dependency_result(envelope, operation, target):
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
    result = {}
    for row in items:
        if not isinstance(row, dict):
            raise AssertionError("invalid installation record")
        directory = row.get("directory") or {}
        dist = directory.get("recorded_distribution")
        if not dist:
            # Recorded signed-v1 identity uses recorded_distribution, not distribution_id.
            raise AssertionError(f"installation lacks signed Directory identity: {row.get('name')}; fields={sorted(directory)}")
        bindings = row.get("clients") or []
        if isinstance(bindings, dict):
            bindings = list(bindings.values())
        scope_bindings = [b for b in bindings if isinstance(b, dict)
                          and b.get("client_id") == target and b.get("scope") == scope]
        if not scope_bindings:
            raise AssertionError(f"{dist}: missing {scope} binding for {target}")
        if not all(audit["recorded_signed_release_matches"](
                directory, b.get("package_revision")) for b in scope_bindings):
            raise AssertionError(f"{dist}: {scope} binding lacks exact signed release provenance")
        # Signed provenance alone cannot certify successful delivery. The
        # Cursor User adapter deliberately prepares a package for manual Editor
        # activation; that is not native discovery or MCP execution evidence.
        for binding in scope_bindings:
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


def lock_ids(project):
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
    return set(keys)


def run_case(cli, target, root, name, portable=None):
    case = root / name
    home, data, project = case / "home", case / "data", case / "project"
    for directory in (home, data, project):
        directory.mkdir(parents=True)
    # Reuse the existing published audit's native-profile root inventory.
    foreign_skill = "---\nname: unmanaged-uap-foreign-skill\ndescription: keep unchanged\n---\n"
    native_foreign = []
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
            user_skill.parent.mkdir(parents=True, exist_ok=True)
            user_skill.write_text(foreign_skill, encoding="utf-8")
            native_foreign.append((user_skill, foreign_skill))
    # Protect compatible User Skills outside the primary client config root.
    # Cursor also discovers ~/.agents, ~/.claude and ~/.codex Skills.
    for relative in COMPAT_SKILL_ROOTS.get(target, ()):
        skill = home / relative / "skills" / "unmanaged-uap-foreign-skill" / "SKILL.md"
        skill.parent.mkdir(parents=True, exist_ok=True)
        skill.write_text(foreign_skill, encoding="utf-8")
        native_foreign.append((skill, foreign_skill))
    # Exercise independent Project Skills too. Copilot's .github/skills lies
    # outside its native MCP roots, as do compatibility Skills.
    project_skill_roots = list(audit["PROJECT_SKILL_ROOTS"].get(target, ()))
    project_skill_roots.extend(root + "/skills" for root in COMPAT_SKILL_ROOTS.get(target, ()))
    project_skill_roots.extend(root + "/skills" for root in PROJECT_ONLY_COMPAT_SKILL_ROOTS.get(target, ()))
    for relative in project_skill_roots:
        skill = project / relative / "unmanaged-uap-foreign-skill" / "SKILL.md"
        skill.parent.mkdir(parents=True, exist_ok=True)
        skill.write_text(foreign_skill, encoding="utf-8")
        native_foreign.append((skill, foreign_skill))
    # The fixture must never read or modify the operator's home or native config.
    user_foreign = home / "foreign-user-do-not-change.txt"
    user_foreign.write_text("unmanaged user data\n", encoding="utf-8")
    project_foreign = project / "foreign-project-do-not-change.txt"
    project_foreign.write_text("unmanaged project data\n", encoding="utf-8")
    env = isolated_env(home, case, data)
    subprocess.run(["git", "init", "-q", str(project)], check=True, env=env, capture_output=True)
    if portable is not None:
        for filename in ("uap.toml", "uap.lock"):
            shutil.copyfile(portable / filename, project / filename)
    return env, project, user_foreign, project_foreign, native_foreign


def pin_foreign(entries, case_root):
    """Pin unmanaged file identities and the directories containing them."""
    pinned = []
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
        pinned.append((path, content, (identity.st_mode, identity.st_dev, identity.st_ino),
                       tuple(ancestors)))
    return pinned


def check_foreign(entries, case_root):
    """Reject foreign file or ancestor replacement, metadata changes and links."""
    for path, content, expected_identity, ancestors in entries:
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
            identity = path.lstat()
            current_identity = (identity.st_mode, identity.st_dev, identity.st_ino)
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
    """Record protected file and directory identities without following links."""
    roots = tuple(Path(root) for root in roots)
    state = audit["snapshot"](roots)
    # The shared audit records failed file reads and walk errors as
    # 'unreadable'. Identical failures before/after must not prove isolation.
    if any(digest == "unreadable" for digest in state.values()):
        raise AssertionError("protected scope snapshot unreadable: native entry")

    def record_metadata(index, path, relative):
        """Preserve directory and file inode identity, not only permissions."""
        try:
            entry = path.lstat()
        except OSError as error:
            raise AssertionError("protected scope snapshot unreadable: metadata") from error
        state[f"metadata:{index}:{relative}"] = (
            entry.st_mode, entry.st_dev, entry.st_ino
        )

    def refuse_walk_error(error):
        raise AssertionError(f"protected scope snapshot unreadable: {error}") from error

    for index, root in enumerate(roots):
        if root.is_symlink():
            state[f"root-link:{index}"] = os.readlink(root)
            record_metadata(index, root, ".")
            continue
        # An empty protected profile must not disappear or turn into a file
        # unnoticed: audit.snapshot records children, not root existence.
        if not root.is_dir():
            state[f"root-kind:{index}"] = "file" if root.exists() else "absent"
            continue
        state[f"root-kind:{index}"] = "directory"
        record_metadata(index, root, ".")
        for directory, dirs, files in os.walk(root, followlinks=False, onerror=refuse_walk_error):
            for name in dirs:
                path = Path(directory) / name
                relative = path.relative_to(root).as_posix()
                if path.is_symlink():
                    state[f"link:{index}:{relative}"] = os.readlink(path)
                    record_metadata(index, path, relative)
                else:
                    state[f"directory:{index}:{relative}"] = "present"
                    record_metadata(index, path, relative)
            for name in files:
                path = Path(directory) / name
                if path.is_symlink():
                    relative = path.relative_to(root).as_posix()
                    state[f"link:{index}:{relative}"] = os.readlink(path)
                    record_metadata(index, path, relative)
                else:
                    record_metadata(index, path, path.relative_to(root).as_posix())
    return state


def profile_snapshot(env, project):
    """Hash all files and directory entries in HOME, UAP data and Project."""
    return scoped_snapshot([Path(env["HOME"]), Path(env["AGENTPLUGINS_HOME"]), project])


def check_profile_unchanged(env, project, expected, stage):
    """A peer restore/remove must not alter the originating profile or Project."""
    if profile_snapshot(env, project) != expected:
        raise AssertionError(f"{stage}: original isolated profile changed")


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


def check_removed_native_components(target, home, case_root, stage, project_scope=False):
    """Refuse native MCP and Skill remnants after an empty scoped inventory."""
    if not audit["path_inside_case"](home, case_root):
        raise AssertionError(f"{stage}: native HOME left disposable case")
    # Copilot Project Skills are under .github/skills, not its MCP config
    # roots (.copilot/.config/copilot). Reuse the audit's authoritative paths.
    relative_roots = (*audit["NATIVE_ROOTS"][target],
                      *audit["PROJECT_SKILL_ROOTS"].get(target, ()),
                      *COMPAT_SKILL_ROOTS.get(target, ()))
    if project_scope:
        relative_roots += PROJECT_ONLY_COMPAT_SKILL_ROOTS.get(target, ())
    roots = [home / relative for relative in relative_roots]
    if any(not audit["path_inside_case"](root, case_root) for root in roots):
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
        raise AssertionError(f"{stage}: native Marketing Kit remnants: {sorted(leftovers)}")


def bind_pinned_launcher_cache(env, cache, sandbox):
    """Share only the SHA-checked released CLI binary, not native client state."""
    if (not cache.is_dir() or cache.is_symlink()
            or not audit["path_inside_case"](cache, sandbox)):
        raise AssertionError("released CLI cache must be a real in-sandbox directory")
    env["AGENTPLUGINS_CACHE_DIR"] = str(cache)


def execute(cli, env, project, target, label, *args):
    result = invoke([cli, *args, "--format", "json"], env, project, 180)
    verified(result, label, args[0], target)
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


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--target", required=True, choices=["codex", "cursor", "claude", "gemini", "opencode", "copilot"])
    parser.add_argument("--report", required=True)
    parser.add_argument("--cli", default="agentplugins")
    options = parser.parse_args()
    report_path = Path(options.report)
    report_path.parent.mkdir(parents=True, exist_ok=True)
    report = {"cli_version": "0.1.102", "target": options.target, "source": "real signed marketing-kit", "steps": [], "status": "failed"}
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
            scoped = ("--scope", "project", "--project-root", str(project_a))
            run(env, project_a, "add", "add", "marketing-kit", "--target", options.target, *scoped)
            first = identities(inventory(options.cli, env, project_a, options.target), options.target)
            if set(first) != EXPECTED:
                raise AssertionError(f"mandatory Marketing Kit closure incomplete: {sorted(first)}")
            for name in ("uap.toml", "uap.lock"):
                if not (project_a / name).is_file():
                    raise AssertionError(f"missing Project agreement: {name}")
            expected_pins = {"directory:" + item for item in EXPECTED}
            if lock_ids(project_a) != expected_pins:
                raise AssertionError(f"wrong locked signed source set: {sorted(lock_ids(project_a))}")
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
            native_a_before_b = profile_snapshot(env, project_a)
            env_b, project_b, foreign_b, project_foreign_b, native_foreign_b = run_case(options.cli, options.target, root, "B", project_a)
            bind_pinned_launcher_cache(env_b, launcher_cache, root)
            foreign_b_files = pin_foreign([(foreign_b, "unmanaged user data\n"),
                                           (project_foreign_b, "unmanaged project data\n"),
                                           *native_foreign_b], root)
            foreign_files.extend(foreign_b_files)
            scoped_b = ("--scope", "project", "--project-root", str(project_b))
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
            protected_b = profile_snapshot(env_b, project_b)
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
            # Check the same signed closure through a real, separate User lifecycle.
            # Every User command must preserve all of Project A, not just the
            # portable agreement or known foreign sentinels.
            project_before_user = scoped_snapshot([project_a])
            project_files = {
                name: (project_a / name).read_bytes() if (project_a / name).exists() else None
                for name in ("uap.toml", "uap.lock")
            }
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
            if lock_ids(user_portable) != expected_pins:
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
            if lock_ids(user_portable) != expected_pins:
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
            # Transfer only portable manifest and signed lock, not User A's
            # receipts, native config, store, or installation IDs.
            user_portable_c = Path(env_c["AGENTPLUGINS_HOME"])
            user_portable_c.mkdir(parents=True, exist_ok=True)
            for name in ("uap.toml", "uap.lock"):
                shutil.copyfile(user_portable / name, user_portable_c / name)
            user_a_native_before_c = profile_snapshot(env, project_a)
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
            run(env_c, project_c, "User remove C", "remove", "marketing-kit",
                *user_scoped, *remove_c_attestation)
            check_profile_unchanged(env, project_a, user_a_native_before_c,
                                   "User remove C")
            if identities(inventory(options.cli, env_c, project_c, options.target, "user"),
                          options.target, "user"):
                raise AssertionError("User C remove retained required closure")
            check_removed_native_components(options.target, Path(env_c["HOME"]),
                                            root, "User C remove")
            if user_inventory() != user_after_update:
                raise AssertionError("User C remove mutated User A installations")
            if (user_portable / "uap.lock").read_bytes() != user_lock_after_update:
                raise AssertionError("User C remove rewrote User A signed lock")
            if (user_portable / "uap.toml").read_bytes() != user_manifest_after_update:
                raise AssertionError("User C remove rewrote User A portable manifest")
            # Once User C is removed, User A's remaining native cleanup must
            # not recreate receipts or settings in the independent C profile.
            protected_c_profile = profile_snapshot(env_c, project_c)
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
            run(env, project_a, "User remove", "remove", "marketing-kit",
                *user_scoped, *user_remove_attestation)
            if user_inventory():
                raise AssertionError("User removal retained Marketing Kit or mandatory children")
            check_removed_native_components(options.target, Path(env["HOME"]),
                                            root, "User A remove")
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
            report["status"] = "passed"
    except Exception as error:
        report["failure"] = str(error)[:2500]
        traceback.print_exc()
    finally:
        report_path.write_text(json.dumps(report, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
        print(json.dumps(report, indent=2, ensure_ascii=False))
    return 0 if report["status"] == "passed" else 1


if __name__ == "__main__":
    raise SystemExit(main())

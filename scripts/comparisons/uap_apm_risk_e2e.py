#!/usr/bin/env python3
"""Isolated, evidence-only risky lifecycle comparison; never touches actual profiles.

Three workflow groups execute in parallel. This script reports real PASS/FAIL,
BLOCKED, UNSUPPORTED or NOT_RUN per product and scenario, not a leaderboard.
"""
from __future__ import annotations

import argparse
import base64
import hashlib
import json
import os
from pathlib import Path
import re
import secrets
import shutil
import signal
import stat
import subprocess
import tempfile
import time
import tomllib

import uap_apm_skills_e2e as baseline

REPO = baseline.REPOSITORY
SKILL = baseline.SKILL_PATH
SKILL_SECOND = "tests/fixtures/uap_apm/skills/benchmark-secondary"
PARENT = "tests/fixtures/uap_apm/risk_graph/parent"
CHILD = "tests/fixtures/uap_apm/risk_graph/child"
PARENT_V1 = "25bc7b7cc56f97761eaf76b00448e0c4ec4ee244"
PARENT_V2 = "73c417b0c845aebc5452e8d9c57d25d4eb2ae246"
CHILD_V1 = "ab81308e402e3ac64c3e1840088770d98a1b6245"
CHILD_V2 = "fb96f0f4d1ef62c2ea3dfc11c1880380fe3737b2"
SOURCE = "https://github.com/777genius/universal-agent-plugins.git"
GROUPS = {
    "cold-profile": ("R1_missing_receipts", "R2_claude_profiles"),
    "graph-removal": ("R3_transitive_update", "R6_git_pull_removal"),
    "failure-secrets": ("R4_interrupted_operation", "R5_mcp_env_secret"),
}
MARKER = "UAP_APM_SECRET_CANARY"


def h(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def fixture(commit: str, subpath: str) -> bytes:
    return subprocess.check_output(["git", "show", f"{commit}:{subpath}"])


def git(*args: str, cwd: Path, env: dict[str, str]) -> str:
    clean = dict(env)
    clean.update({"GIT_CONFIG_NOSYSTEM": "1", "GIT_TERMINAL_PROMPT": "0"})
    p = subprocess.run(["git", "-c", "user.name=777genius",
                        "-c", "user.email=iliyazelenkog@gmail.com", *args],
                       cwd=cwd, env=clean, stdin=subprocess.DEVNULL,
                       stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                       timeout=30, check=True)
    return p.stdout.decode("utf-8", "replace").strip()


def observed_file(path: Path, boundary: Path, limit: int = 2 * 1024 * 1024) -> bytes | None:
    """A missing, nonregular, oversize or symlinked file never counts as content."""
    try:
        rel = path.relative_to(boundary)
        if not rel.parts or ".." in rel.parts:
            return None
        cursor = boundary
        if cursor.is_symlink():
            return None
        for part in rel.parts:
            cursor = cursor / part
            if cursor.is_symlink():
                return None
        if not stat.S_ISREG(path.lstat().st_mode):
            return None
        fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK)
        with os.fdopen(fd, "rb") as source:
            state = os.fstat(source.fileno())
            if not stat.S_ISREG(state.st_mode) or state.st_size > limit:
                return None
            result = source.read(limit + 1)
        return result if len(result) <= limit else None
    except (ValueError, OSError):
        return None


def skill_presence(path: Path, boundary: Path) -> str:
    """The removed Skill is absent, present, or unobservable (never PASS)."""
    try:
        rel = path.relative_to(boundary)
        if not rel.parts or ".." in rel.parts or boundary.is_symlink():
            return "unknown"
        cursor = boundary
        for component in rel.parts[:-1]:
            cursor = cursor / component
            if cursor.is_symlink():
                return "unknown"
            try:
                if not stat.S_ISDIR(cursor.lstat().st_mode):
                    return "unknown"
            except FileNotFoundError:
                return "absent"
        leaf = cursor / rel.parts[-1]
        if leaf.is_symlink():
            return "unknown"
        leaf.lstat()
    except FileNotFoundError:
        return "absent"
    except (OSError, ValueError):
        return "unknown"
    return "present"


def status(eligible: bool, command: dict | None, invariant: bool, reason: str,
           *, protected: bool = True, provenance: bool = True) -> tuple[str, str]:
    if not eligible:
        return "BLOCKED", "documented prerequisite or supported route not established: " + reason
    if not protected:
        return "FAIL", "a protected or foreign file changed: " + reason
    if command is None or not observed_command(command):
        return "BLOCKED", "process outcome incomplete: " + reason
    if command["exit_code"] != 0:
        return "BLOCKED" if invariant else "FAIL", "fully observed command refused: " + reason
    if not invariant or not provenance:
        return "FAIL", reason
    return "PASS", reason


def receipt(product: str, case: str, label: str, **observed) -> dict:
    return {"product": product, "scenario": case, "status": label,
            "native_model_execution": "NOT_RUN", **observed}


def run(product: str, cli: str, project: Path, root: Path, env: dict[str, str],
        *, replay: bool = False, commit: str, target: str = "codex") -> dict:
    if target == "codex":
        argv = baseline.tool_args(product, cli, project, commit, replay=replay)
    elif product == "uap":
        if replay:
            argv = [cli, "install", "--locked", "--scope", "project",
                    "--project-root", str(project), "--yes", "--format", "json"]
        else:
            argv = [cli, "skills", "add", f"{REPO}@{commit}//{SKILL}",
                    "--target", target, "--scope", "project",
                    "--project-root", str(project), "--no-ui", "--yes", "--format", "json"]
    else:
        argv = [cli, "install", *(["--frozen"] if replay else []), "--target", target]
    return baseline.run_process(argv, project, env, root)


def prepare_project(product: str, project: Path, commit: str) -> None:
    if product == "apm":
        baseline.prepare_apm_manifest(project, commit)


def clean_snapshot(project: Path, files: tuple[str, ...]) -> dict[str, str | None]:
    return {name: (h(content) if (content := observed_file(project / name, project)) is not None else None)
            for name in files}


def file_identity(path: Path, boundary: Path) -> tuple | None:
    """Observed bytes plus POSIX identity/permissions, not only content hashes."""
    data = observed_file(path, boundary)
    if data is None:
        return None
    try:
        st = path.lstat()
        return (h(data), st.st_mode, st.st_uid, st.st_gid, st.st_ino,
                st.st_mtime_ns, st.st_ctime_ns)
    except OSError:
        return None


def apm_claude_skill_kind(portable: dict[str, bytes], commit: str) -> bool:
    """Only APM's native Claude Skill/bundle path tests the reported #3391 bug."""
    try:
        import yaml
        lock = yaml.safe_load(portable["apm.lock.yaml"])
        deps = lock.get("dependencies", [])
        return any(
            isinstance(row, dict)
            and row.get("repo_url") == REPO
            and row.get("virtual_path") == SKILL
            and row.get("resolved_commit") == commit
            and row.get("package_type") in ("claude_skill", "skill_bundle")
            for row in deps
        )
    except (KeyError, TypeError, ValueError, ImportError, AttributeError):
        return False


def lock_validation_refused(command: dict, lock_name: str) -> bool:
    """No-op, unrelated failure or process crash cannot validate lock authority."""
    if not observed_command(command):
        return False
    code = command.get("exit_code")
    if not isinstance(code, int) or isinstance(code, bool) or code <= 0:
        # Negative codes mean a terminating POSIX signal, not a normal refusal.
        return False
    expected = (lock_name.lower(), "lockfile")
    # Parsing/success wording is not rejection evidence. Require a
    # diagnostic of a failed lock validation, not an ambiguous stem.
    error_tokens = ("invalid", "malformed", "parse error", "syntax error",
                    "corrupt", "rejected", "failed to parse", "cannot parse",
                    "unexpected token")
    nonfailure = ("parsed successfully", "parse succeeded", "no error",
                  "without error", "validation passed")
    for line in command.get("output_tail", "").lower().splitlines():
        # Filenames/paths (including the fixture's parent directory) must not
        # donate error vocabulary to a diagnostic for a different failure.
        diagnostic_text = re.sub(r"[^\s]*[/\\][^\s]*", "<path>", line)
        if (not any(word in diagnostic_text for word in nonfailure)
                and any(key in diagnostic_text for key in expected)
                and any(word in diagnostic_text for word in error_tokens)):
            return True
    return False


def case_cold(product: str, cli: str, root: Path, commit: str, expected: dict[str, bytes]) -> dict:
    """Cold clone containing already committed Claude Skill, without receipts.

    This directly covers the previously untested APM claude_skill/skill_bundle
    failure shape (#3391), rather than re-labeling #617's passing Codex case.
    """
    env = baseline.isolated_env(root)
    project, foreign = baseline.make_project(root)
    if product == "apm":
        (project / "apm.yml").write_text(
            "name: cold-claude-skills\nversion: 1.0.0\ntargets:\n  - claude\n"
            "dependencies:\n  apm:\n"
            f"    - {REPO}/{SKILL}#{commit}\n"
        )
    initial = run(product, cli, project, root, env, commit=commit, target="claude")
    admitted_skills = profile_scan(project / ".claude", expected)
    portable, capture_error = baseline.captured_portable(project, product)
    provenance = bool(portable) and capture_error is None and (
        baseline.portable_identity(portable, commit, product)
            ["exact_source_commit_recorded_in_lock"])
    specific_apm_kind = (product != "apm" or
                         apm_claude_skill_kind(portable, commit))
    foreign_initial = baseline.safe_same_bytes(
        project / ".agents/skills/foreign-neighbor/SKILL.md", foreign, project)
    if not (complete_command(initial) and len(admitted_skills) == 1 and
            provenance and specific_apm_kind and foreign_initial):
        return receipt(product, "R1_missing_receipts", "BLOCKED",
                       stage="initial_claude_skill_admission", initial=initial,
                       observed_claude_skill_count=len(admitted_skills),
                       source_locked=provenance,
                       apm_claude_skill_package_kind=specific_apm_kind,
                       foreign_initial_intact=foreign_initial,
                       reason="full Claude Skill bytes and exact native source/lock not admitted")
    # The full measured destination is authoritative, not a guessed client
    # directory; changes in physical naming during restore are detected.
    physical = next(iter(admitted_skills))
    owned = tuple(f".claude/{physical}/{name}" for name in baseline.EXPECTED_FILES)
    fresh_root = root / "cold"
    fresh_env = baseline.isolated_env(fresh_root)
    committed, neighbor = baseline.make_project(fresh_root)
    for name, data in portable.items():
        (committed / name).write_bytes(data)
    for name in owned:
        dest = committed / name
        dest.parent.mkdir(parents=True, exist_ok=True)
        payload = name.removeprefix(f".claude/{physical}/")
        dest.write_bytes(expected[payload])
    git("init", "-q", cwd=committed, env=fresh_env)
    git("add", "-A", cwd=committed, env=fresh_env)
    git("commit", "-qm", "pinned portable inputs and committed Claude Skill", cwd=committed, env=fresh_env)
    teammate = fresh_root / "clean-clone"
    git("clone", "-q", str(committed), str(teammate), cwd=fresh_root, env=fresh_env)
    # A second, disjoint clone with a corrupted lock proves that the selected
    # frozen command actually consults its portable authority. No-op managers
    # cannot qualify merely because the fixture was already on disk.
    negative_root = fresh_root / "lock-authority-control"
    negative_env = baseline.isolated_env(negative_root)
    negative_clone = fresh_root / "lock-authority-clone"
    git("clone", "-q", str(committed), str(negative_clone), cwd=fresh_root, env=fresh_env)
    lock_name = "uap.lock" if product == "uap" else "apm.lock.yaml"
    # Bytes carry no diagnostic words; they must be diagnosed by the CLI.
    (negative_clone / lock_name).write_bytes(b"\x00\x01\x02\x03\n")
    negative_result = run(product, cli, negative_clone, negative_root, negative_env,
                          commit=commit, target="claude", replay=True)
    corrupt_lock_rejected = lock_validation_refused(negative_result, lock_name)
    before = clean_snapshot(teammate, owned)
    claude_before = profile_snapshot(teammate / ".claude")
    portable_meta_before = {name: file_identity(teammate / name, teammate)
                            for name in portable}
    before_identity = tuple(file_identity(teammate / name, teammate) for name in owned)
    portable_before, before_error = baseline.captured_portable(teammate, product)
    foreign_path = teammate / ".agents/skills/foreign-neighbor/SKILL.md"
    foreign_before = file_identity(foreign_path, teammate)
    project_before = project_file_snapshot(teammate)
    actual = run(product, cli, teammate, fresh_root, fresh_env,
                 commit=commit, target="claude", replay=True)
    after = clean_snapshot(teammate, owned)
    after_identity = tuple(file_identity(teammate / name, teammate) for name in owned)
    claude_after = profile_snapshot(teammate / ".claude")
    portable_meta_after = {name: file_identity(teammate / name, teammate)
                           for name in portable}
    portable_after, after_error = baseline.captured_portable(teammate, product)
    project_after = project_file_snapshot(teammate)
    preserved = (foreign_before is not None
                 and file_identity(foreign_path, teammate) == foreign_before
                 and baseline.safe_same_bytes(foreign_path, neighbor, teammate))
    identical = (all(before.values()) and before == after and preserved
                 and all(item is not None for item in before_identity)
                 and before_identity == after_identity
                 and profile_scan(teammate / ".claude", expected) == admitted_skills)
    lock_intact = (before_error is None and after_error is None and
                   portable_before == portable_after and
                   all(value is not None for value in portable_meta_before.values()) and
                   portable_meta_before == portable_meta_after)
    outcome, why = status(
        True, actual, identical and lock_intact,
        "cold-cloned committed Claude Skill and portable lock must stay unchanged",
        protected=identical,
    )
    extra_profile_mutation = (not claude_before["complete"]
                              or not claude_after["complete"]
                              or claude_before != claude_after)
    if outcome == "PASS" and not (project_before["complete"] and project_after["complete"]):
        outcome, why = "BLOCKED", "whole-project observer incomplete; manager-owned state excluded"
    elif outcome == "PASS" and project_before != project_after:
        outcome, why = "FAIL", "unaccounted project file or metadata changed during cold replay"
    elif outcome == "PASS" and not corrupt_lock_rejected:
        outcome, why = "BLOCKED", "corrupted locked replay was not independently refused"
    elif outcome == "PASS" and extra_profile_mutation:
        outcome, why = "BLOCKED", "unaccounted Claude files or metadata changed during cold replay"
    return receipt(product, "R1_missing_receipts", outcome, reason=why,
                   scenario_type="preservation of committed Claude Skill without receipts; not proof of restoring deleted files",
                   apm_claude_skill_package_kind=specific_apm_kind,
                   observed_physical_skill_path=physical,
                   initial_portable_sha256={k: h(v) for k, v in portable.items()},
                   before=before, after=after, lock_identical=lock_intact,
                   corrupted_lock_refused=corrupt_lock_rejected,
                   corrupted_lock_command=negative_result,
                   claude_profile_unchanged=not extra_profile_mutation,
                   nonmanaged_project_unchanged=project_before == project_after,
                   nonmanaged_project_observed=(project_before["complete"]
                                                and project_after["complete"]),
                   command=actual, foreign_unchanged=preserved)


def observed_command(command: dict) -> bool:
    """Receipt is complete even if the tool deliberately refused to apply."""
    return (command.get("exit_code") is not None
            and command.get("timed_out") is False
            and not command.get("cleanup_error")
            and command.get("capture_complete") is True)


def complete_command(command: dict) -> bool:
    """Successful, completely observed process; zero exit alone is not proof."""
    return observed_command(command) and command.get("exit_code") == 0


def profile_scan(root: Path, expected: dict[str, bytes]) -> dict[str, dict]:
    """Admit only complete, ordinary Skill + reference bytes in one directory."""
    folder = root / "skills"
    result: dict[str, dict] = {}
    if not folder.is_dir() or folder.is_symlink():
        return result
    try:
        for subdir in sorted(folder.iterdir()):
            if subdir.is_symlink() or not subdir.is_dir():
                continue
            skill = observed_file(subdir / "SKILL.md", root)
            reference = observed_file(subdir / "references/fixture.txt", root)
            if skill == expected["SKILL.md"] and reference == expected["references/fixture.txt"]:
                result[str(subdir.relative_to(root))] = {
                    "skill_sha256": h(skill),
                    "reference_sha256": h(reference),
                }
            if len(result) > 32:
                return {}
    except OSError:
        return {}
    return result


def profile_snapshot(root: Path, *, max_entries: int = 2048) -> dict:
    """Observe profile content, modes and identities, not just byte payloads."""
    entries: list[str] = []
    unknown: list[str] = []
    if root.is_symlink() or not root.is_dir():
        return {"sha256": None, "count": 0, "complete": False}
    def stamp(path: Path) -> str:
        value = path.lstat()
        return ":".join(str(v) for v in (
            value.st_mode, value.st_uid, value.st_gid, value.st_ino,
            value.st_mtime_ns, value.st_ctime_ns))
    try:
        entries.append("ROOT:" + stamp(root))
    except OSError:
        return {"sha256": None, "count": 0, "complete": False}
    for current, dirs, files in os.walk(
            root, followlinks=False,
            onerror=lambda error: unknown.append("unlistable")):
        here = Path(current)
        for dirname in sorted(dirs[:]):
            child = here / dirname
            if child.is_symlink():
                unknown.append("symlink_directory")
                dirs.remove(dirname)
            else:
                try:
                    entries.append("D:" + str(child.relative_to(root)) + ":" + stamp(child))
                except OSError:
                    unknown.append("unreadable_directory")
        for filename in sorted(files):
            child = here / filename
            data = observed_file(child, root)
            if data is None:
                unknown.append("unreadable")
            else:
                try:
                    entries.append("F:" + str(child.relative_to(root)) + ":" +
                                   h(data) + ":" + stamp(child))
                except OSError:
                    unknown.append("unreadable_file_metadata")
        if len(entries) > max_entries:
            unknown.append("too_many_entries")
            break
    return {"sha256": h("\n".join(sorted(entries)).encode("utf-8")),
            "count": len(entries), "complete": not unknown}


def project_file_snapshot(project: Path, *, max_entries: int = 4096) -> dict:
    """Hash observed user files outside known manager-only state directories.

    Root/directory timestamps are intentionally not treated as independent
    user files: creating a documented manager-state directory changes parent
    metadata without altering an agent-readable project file. Every regular
    non-manager file is checked for byte, inode, owner, mode and timestamp drift.
    """
    ignored = {".git", ".uap", ".apm-local", "apm_modules"}
    observed: list[str] = []
    problems: list[str] = []
    if project.is_symlink() or not project.is_dir():
        return {"sha256": None, "count": 0, "complete": False}
    for current, dirs, files in os.walk(
            project, followlinks=False,
            onerror=lambda err: problems.append("unlistable_directory")):
        here = Path(current)
        if here == project:
            dirs[:] = [name for name in dirs if name not in ignored]
        for name in sorted(dirs[:]):
            path = here / name
            if path.is_symlink():
                dirs.remove(name)
                problems.append("symlink_directory")
            else:
                observed.append("D:" + str(path.relative_to(project)))
        for name in sorted(files):
            path = here / name
            content = observed_file(path, project)
            if content is None:
                problems.append("unreadable_or_nonregular_file")
                continue
            try:
                info = path.lstat()
                observed.append(":".join(str(value) for value in (
                    "F", path.relative_to(project), h(content),
                    info.st_mode, info.st_uid, info.st_gid, info.st_ino,
                    info.st_mtime_ns, info.st_ctime_ns)))
            except OSError:
                problems.append("metadata_error")
        if len(observed) > max_entries:
            problems.append("too_many_entries")
            break
    return {"sha256": h("\n".join(sorted(observed)).encode("utf-8")),
            "count": len(observed), "complete": not problems,
            "ignored_owned_state_roots": sorted(ignored)}


def saved_profile_source_matches(product: str, env: dict[str, str], commit: str) -> bool:
    """Source identity comes from the manager's *user scope lock*, not file names."""
    managed = (Path(env["AGENTPLUGINS_HOME"]) if product == "uap"
               else Path(env["HOME"]) / ".apm")
    portable, err = baseline.captured_portable(managed, product)
    return (err is None and bool(portable) and
            baseline.portable_identity(portable, commit, product)
                ["exact_source_commit_recorded_in_lock"])


def observe_native_claude(root: Path) -> dict | None:
    """Independently hash and version-probe the actual PATH-selected client."""
    executable = shutil.which("claude")
    if not executable:
        return None
    try:
        native = Path(executable).resolve(strict=True)
        st = native.lstat()
        if not stat.S_ISREG(st.st_mode) or not 0 < st.st_size <= 512 * 1024 * 1024:
            return None
        digest = hashlib.sha256()
        with native.open("rb") as stream:
            for chunk in iter(lambda: stream.read(1024 * 1024), b""):
                digest.update(chunk)
        observed_hash = digest.hexdigest()
        if observed_hash != "26d020351e8112f4006790f3cfce43b4c9df0c1bb1d0e542364d64151b81d5ba":
            return None
        probe_root = root / "claude-proof"
        proof_env = baseline.isolated_env(probe_root)
        # Execute the exact canonical file whose bytes were hash-verified.
        probe = baseline.run_process([str(native), "--version"],
                                     probe_root, proof_env, root)
        output = probe.get("output_tail", "").strip()
        if not complete_command(probe) or re.search(
                r"(?<![0-9])2\.1\.263(?![0-9])", output) is None:
            return None
        return {"native_sha256": observed_hash, "observed_version": output}
    except (OSError, ValueError):
        return None


def case_profiles(product: str, cli: str, root: Path, commit: str,
                  expected: dict[str, bytes], *, claude_available: bool,
                  claude_version: str | None = None,
                  claude_native_sha256: str | None = None) -> dict:
    verified = (claude_available and bool(claude_version)
                and re.search(r"(?<![0-9])2\.1\.263(?![0-9])", claude_version) is not None
                and claude_native_sha256 ==
                    "26d020351e8112f4006790f3cfce43b4c9df0c1bb1d0e542364d64151b81d5ba")
    if not verified:
        return receipt(product, "R2_claude_profiles", "BLOCKED",
                       reason="pinned real Claude version and native digest unavailable")
    native_proof = observe_native_claude(root)
    if native_proof is None or native_proof["observed_version"] != claude_version.strip():
        return receipt(product, "R2_claude_profiles", "BLOCKED",
                       reason="actual selected Claude native executable/version not verified")
    env = baseline.isolated_env(root)
    home = Path(env["HOME"])
    a, b = home / ".claude", home / "profiles" / "b"
    for dest in (a, b):
        sentinel = dest / "skills/foreign-neighbor/SKILL.md"
        sentinel.parent.mkdir(parents=True, exist_ok=True)
        sentinel.write_bytes(b"FOREIGN PROFILE DATA\n")
    project = root / "project"
    project.mkdir()
    apm_source = f"{REPO}/{SKILL}#{commit}"
    before_a, before_b = profile_snapshot(a), profile_snapshot(b)
    foreign_before = {p: file_identity(
        p / "skills/foreign-neighbor/SKILL.md", home) for p in (a, b)}
    results: list[dict] = []
    for index, selected in enumerate((a, b, a)):
        active = dict(env, CLAUDE_CONFIG_DIR=str(selected))
        if product == "uap":
            argv = [cli, "skills", "add", f"{REPO}@{commit}//{SKILL}",
                    "--target", "claude", "--scope", "user", "--no-ui",
                    "--yes", "--format", "json"]
        elif index == 0:
            argv = [cli, "install", apm_source, "--global", "--target", "claude"]
        else:
            argv = [cli, "install", "--global", "--target", "claude"]
        command = baseline.run_process(argv, project, active, root)
        results.append({
            "phase": index, "selected": "A" if selected == a else "B",
            "cmd": command, "a": profile_scan(a, expected),
            "b": profile_scan(b, expected),
            "a_snapshot": profile_snapshot(a),
            "b_snapshot": profile_snapshot(b),
            "saved_source_matches": saved_profile_source_matches(product, env, commit),
            "foreign": all(
                identity is not None
                and identity[0] == h(b"FOREIGN PROFILE DATA\n")
                and file_identity(p / "skills/foreign-neighbor/SKILL.md", home)
                    == identity
                for p, identity in foreign_before.items()),
        })
    first, second, third = results
    complete = all(complete_command(item["cmd"]) for item in results)
    intact = all(item["foreign"] for item in results)
    snapshots_complete = all(
        item["a_snapshot"]["complete"] and item["b_snapshot"]["complete"]
        for item in results) and before_a["complete"] and before_b["complete"]
    # The unselected native profile must be byte-identical throughout each
    # different selected-profile operation. A never authorized writes into B.
    no_cross_profile_change = (
        first["b_snapshot"] == before_b
        and second["a_snapshot"] == first["a_snapshot"]
        and third["b_snapshot"] == second["b_snapshot"])
    exact_delivery = (
        len(first["a"]) == 1 and not first["b"]
        and len(second["a"]) == 1 and len(second["b"]) == 1
        and len(third["a"]) == 1 and len(third["b"]) == 1)
    pinned_source = all(item["saved_source_matches"] for item in results)
    if not intact:
        verdict, why = "FAIL", "foreign profile sentinel was modified"
    elif not snapshots_complete:
        verdict, why = "BLOCKED", "native profile observation incomplete"
    elif not no_cross_profile_change:
        verdict, why = "FAIL", "operation wrote into the unselected Claude profile"
    elif not complete:
        verdict, why = "BLOCKED", "Claude install had incomplete, failed or timed-out process evidence"
    elif not exact_delivery:
        verdict, why = "FAIL", "selected Claude profiles lack exact Skill and reference bytes"
    elif not pinned_source:
        verdict, why = "BLOCKED", "saved user-scope source pin could not be verified"
    else:
        verdict, why = "PASS", "source-pinned Skill and both isolated profiles preserved"
    return receipt(product, "R2_claude_profiles", verdict, reason=why,
                   phases=results, verified_claude_version=claude_version,
                   verified_claude_native_sha256=claude_native_sha256,
                   independently_verified_claude=native_proof,
                   profile_snapshots_complete=snapshots_complete,
                   unselected_profiles_unchanged=no_cross_profile_change,
                   saved_source_matches=pinned_source,
                   selected_uninstall="NOT_RUN (needs separate qualified ownership)") 


def root_graph(product: str, project: Path, parent_commit: str) -> None:
    if product == "uap":
        (project / "uap.toml").write_text(
            "schema_version = 1\n\n[dependencies.parent]\n"
            f'git = "{SOURCE}"\npath = "{PARENT}"\ncommit = "{parent_commit}"\n'
            '\n[targets]\ncodex = ["parent"]\n')
    else:
        (project / "apm.yml").write_text(
            "name: risk-graph-consumer\nversion: 1.0.0\ntargets: [codex]\n"
            "dependencies:\n  apm:\n"
            f"    - {REPO}/{PARENT}#{parent_commit}\n")


def graph_command(product: str, cli: str, project: Path,
                  *, updated: bool = False, lock: bool = False) -> list[str]:
    if product == "uap":
        command=[cli, "deps", "lock" if lock else "sync",
                 "--scope","project","--project-root",str(project)]
        if not lock:
            command += ["--yes","--format","json"]
        return command
    return [cli, "install", "--target", "codex", *(["--update"] if updated else [])]


def graph_lock_matches(project: Path, product: str,
                       parent_commit: str, child_commit: str) -> bool:
    """Require the exact two-node graph and actual parent-child provenance.

    One valid child entry is not enough: stale/duplicate pins or a missing
    dependency edge make the test unable to claim a correct full update.
    """
    try:
        if product == "uap":
            raw = observed_file(project / "uap.lock", project)
            if raw is None:
                return False
            lock = json.loads(raw)
            nodes = lock.get("packages", [])
            if not isinstance(nodes, list) or len(nodes) != 2:
                return False
            def source(node: dict) -> tuple[str, str, str]:
                git = node.get("source", {}).get("git", {})
                return (git.get("repository") or git.get("url"),
                        git.get("path"), git.get("commit"))
            if any(not isinstance(node, dict) for node in nodes):
                return False
            parent = [n for n in nodes if source(n)[1] == PARENT]
            child = [n for n in nodes if source(n)[1] == CHILD]
            if len(parent) != 1 or len(child) != 1:
                return False
            p, c = parent[0], child[0]
            source_aliases = {REPO, SOURCE, f"https://github.com/{REPO}"}
            if (source(p) not in {(r, PARENT, parent_commit) for r in source_aliases}
                    or source(c) not in {(r, CHILD, child_commit) for r in source_aliases}):
                return False
            # Recorded coordinates alone are insufficient. UAP replays the
            # requested selector, so a stale v1 intent paired with v2 bytes
            # must never qualify as an exact graph update.
            for node, expected in ((p, parent_commit), (c, child_commit)):
                selector = node.get("source", {}).get("git", {}).get("requested")
                if selector != {"kind": "commit", "value": expected}:
                    return False
            parent_alias = p.get("alias")
            child_alias = c.get("alias")
            if (not isinstance(parent_alias, str) or not parent_alias
                    or not isinstance(child_alias, str) or not child_alias
                    or parent_alias == child_alias
                    or p.get("required_by")):
                # Parent must be a direct root. A reverse edge is an invalid
                # two-node cycle, not an admissible source closure.
                return False
            edges = c.get("required_by")
            if not isinstance(edges, list) or len(edges) != 1:
                return False
            edge = edges[0]
            selector = edge.get("selector", {}) if isinstance(edge, dict) else {}
            return (edge.get("alias") == parent_alias
                    and selector.get("kind") == "commit"
                    and selector.get("value") == child_commit)
        import yaml
        raw = observed_file(project / "apm.lock.yaml", project)
        if raw is None:
            return False
        lock = yaml.safe_load(raw)
        nodes = lock.get("dependencies", [])
        if not isinstance(nodes, list) or len(nodes) != 2 or any(
                not isinstance(n, dict) for n in nodes):
            return False
        parent = [n for n in nodes if n.get("virtual_path") == PARENT]
        child = [n for n in nodes if n.get("virtual_path") == CHILD]
        if len(parent) != 1 or len(child) != 1:
            return False
        p, c = parent[0], child[0]
        repos = {REPO, SOURCE, f"https://github.com/{REPO}"}
        return (p.get("repo_url") in repos and c.get("repo_url") in repos
                and p.get("resolved_commit") == parent_commit
                and c.get("resolved_commit") == child_commit
                and c.get("resolved_by") == p.get("repo_url")
                and not p.get("resolved_by"))
    except (OSError, ValueError, TypeError, KeyError, AttributeError, ImportError):
        return False


def case_graph(product: str, cli: str, root: Path) -> dict:
    env=baseline.isolated_env(root)
    project,_=baseline.make_project(root)
    root_graph(product,project,PARENT_V1)
    initial=baseline.run_process(graph_command(product,cli,project),project,env,root)
    old=fixture(CHILD_V1,CHILD+"/skills/risk-child/SKILL.md")
    new=fixture(CHILD_V2,CHILD+"/skills/risk-child/SKILL.md")
    old_skill=project / ".agents/skills/risk-child/SKILL.md"
    parent_skill=project / ".agents/skills/risk-parent/SKILL.md"
    parent_v1=fixture(PARENT_V1,PARENT+"/skills/risk-parent/SKILL.md")
    parent_v2=fixture(PARENT_V2,PARENT+"/skills/risk-parent/SKILL.md")
    first=(observed_file(old_skill,project)==old
           and observed_file(parent_skill,project)==parent_v1
           and graph_lock_matches(project,product,PARENT_V1,CHILD_V1))
    if not complete_command(initial) or not first:
        return receipt(product,"R3_transitive_update","BLOCKED",stage="initial_graph_admission",
                       reason="two-package transitive graph did not reach expected native source state",
                       initial=initial,child_v1_seen=first)
    root_graph(product,project,PARENT_V2)
    prelock=None
    if product=="uap":
        prelock=baseline.run_process(graph_command(product,cli,project,lock=True),project,env,root)
        if not complete_command(prelock):
            return receipt(product,"R3_transitive_update","BLOCKED",
                           stage="pin_refresh",initial=initial,pin_refresh=prelock,
                           reason="UAP initial source pin refresh incomplete or refused")
    foreign_path = project / ".agents/skills/foreign-neighbor/SKILL.md"
    foreign_before = file_identity(foreign_path, project)
    updated=baseline.run_process(graph_command(product,cli,project,updated=True),project,env,root)
    actual=observed_file(old_skill,project)
    graph_ok=graph_lock_matches(project,product,PARENT_V2,CHILD_V2)
    foreign_ok=(foreign_before is not None
                and foreign_before[0] == h(b"FOREIGN USER CONTENT: preserve without edits\n")
                and file_identity(foreign_path, project) == foreign_before)
    did=(actual==new and graph_ok
         and observed_file(parent_skill,project)==parent_v2)
    result,reason=status(True,updated,did,"parent v2 must resolve child B once and deploy child B",
                         protected=foreign_ok)
    return receipt(product,"R3_transitive_update",result,reason=reason,initial=initial,
                   pin_refresh=prelock,command=updated,foreign_unchanged=foreign_ok,
                   original_sha256=h(old),expected_sha256=h(new),
                   actual_sha256=h(actual) if actual is not None else None,
                   parent_v2_payload_matches=observed_file(parent_skill,project)==parent_v2,
                   complete_parent_child_graph_locked=graph_ok)


def case_interrupted(product: str, cli: str, root: Path, commit: str) -> dict:
    """No probabilistic SIGKILL as alleged post-commit evidence: fail closed."""
    env=baseline.isolated_env(root)
    project,original=baseline.make_project(root)
    prepare_project(product,project,commit)
    argv=baseline.tool_args(product,cli,project,commit)
    # Preflight is a *separate* real negative control, not a forged crash proof.
    # A safe isolated error is introduced via a malformed existing lock.
    file=project / ("uap.lock" if product=="uap" else "apm.lock.yaml")
    file.write_text("{invalid: unresolved: [\n")
    before=clean_snapshot(project,(str(file.relative_to(project)),
                        ".agents/skills/foreign-neighbor/SKILL.md"))
    control=baseline.run_process(argv,project,env,root)
    after=clean_snapshot(project,(str(file.relative_to(project)),
                        ".agents/skills/foreign-neighbor/SKILL.md"))
    foreign_ok=baseline.safe_same_bytes(
        project/".agents/skills/foreign-neighbor/SKILL.md",original,project)
    if not foreign_ok or before!=after:
        label="FAIL";reason="negative preflight unexpectedly touched foreign content/lock"
    elif not observed_command(control):
        label="BLOCKED";reason="preflight control outcome incomplete; no negative verification"
    elif control["exit_code"] == 0:
        label="FAIL";reason="invalid pre-existing lock was silently accepted"
    elif not lock_validation_refused(control, file.name):
        label="BLOCKED";reason="preflight refusal cause unverified; post-write interruption NOT_RUN"
    else:
        label="BLOCKED";reason="verified preflight refusal; deterministic post-write interruption NOT_RUN"
    return receipt(product,"R4_interrupted_operation",label,reason=reason,
                   controlled_preflight=control,preflight_untouched=before==after,
                   deterministic_post_write_barrier=False,crash_recovery="NOT_RUN")


def scan_synthetic_canary(roots: tuple[Path, ...], synthetic: str,
                          *, max_files: int = 8192,
                          max_bytes: int = 64 * 1024 * 1024) -> dict:
    """Bounded named-root survey: a missing proof cannot become a secrecy PASS.

    Never follow symbolic links, include file bodies in output, or scan the
    ambient runner home. The synthetic canary is never serialized.
    """
    inspected, seen_bytes, hits, unknown = 0, 0, [], []
    token = synthetic.encode("utf-8")
    # The experiment checks a bounded set of direct and common textual
    # encodings; it cannot prove the absence of arbitrary transformations.
    variants = tuple(sorted({token, token.upper(), base64.b64encode(token),
                             token.hex().encode("ascii")}))
    path_markers = tuple(value.decode("ascii") for value in variants)
    for root in roots:
        if not root.is_dir() or root.is_symlink():
            unknown.append("root_not_regular")
            continue
        for current, dirs, files in os.walk(
                root, followlinks=False,
                onerror=lambda error: unknown.append("unlistable_directory")):
            here = Path(current)
            if any(value in str(here.relative_to(root)) for value in path_markers):
                hits.append(h(str(here.relative_to(root)).encode("utf-8")))
            if here.is_symlink():
                unknown.append("symlink_directory")
                dirs[:] = []
                continue
            # Unverified symlinked directories are not silently ignored.
            for directory in dirs[:]:
                candidate = here / directory
                if any(value in str(candidate.relative_to(root)) for value in path_markers):
                    hits.append(h(str(candidate.relative_to(root)).encode("utf-8")))
                if candidate.is_symlink():
                    unknown.append("symlink_directory")
                    dirs.remove(directory)
            for name in files:
                file = here / name
                if any(value in str(file.relative_to(root)) for value in path_markers):
                    hits.append(h(str(file.relative_to(root)).encode("utf-8")))
                inspected += 1
                if inspected > max_files:
                    unknown.append("too_many_files")
                    break
                try:
                    state = file.lstat()
                    if not stat.S_ISREG(state.st_mode):
                        unknown.append("nonregular_file")
                        continue
                    if state.st_size > 2 * 1024 * 1024:
                        unknown.append("large_file")
                        continue
                    data = observed_file(file, root)
                    if data is None:
                        unknown.append("unreadable_file")
                        continue
                    seen_bytes += len(data)
                    if seen_bytes > max_bytes:
                        unknown.append("too_many_bytes")
                        break
                    if any(value in data for value in variants):
                        # Only a relative location hash, never data or a private root.
                        hits.append(h(str(file.relative_to(root)).encode("utf-8")))
                except (OSError, ValueError):
                    unknown.append("scan_error")
            if inspected > max_files or seen_bytes > max_bytes:
                break
    return {"inspected_files": inspected, "inspected_bytes": seen_bytes,
            "canary_found": bool(hits), "matching_path_hashes": hits[:16],
            "complete": not unknown, "incomplete_reasons": sorted(set(unknown)),
            "encodings_probed": ["utf8", "uppercase", "standard_base64", "lowercase_hex"],
            "other_transformations": "NOT_RUN"}


def codex_mcp_contract(project: Path) -> str:
    """Require the exact server and Codex's explicit env-forwarding contract."""
    raw = observed_file(project / ".codex/config.toml", project)
    if raw is None:
        return "missing_config"
    try:
        config = tomllib.loads(raw.decode("utf-8"))
        servers = config.get("mcp_servers", {})
        server = servers.get("riskcanary") if isinstance(servers, dict) else None
        if not isinstance(server, dict):
            return "missing_server"
        if server.get("command") != "node":
            return "wrong_command"
        if server.get("args") != ["-e", "process.exit(0)"]:
            return "wrong_arguments"
        forwarded = server.get("env_vars")
        embedded = server.get("env", {})
        if (not isinstance(forwarded, list)
                or forwarded != [MARKER] or
                not all(isinstance(name, str) for name in forwarded)
                or not isinstance(embedded, dict) or embedded):
            return "environment_forwarding_unverified"
        return "match"
    except (ValueError, UnicodeError, AttributeError, TypeError):
        return "invalid_toml"


def case_secret(product: str, cli: str, root: Path) -> dict:
    env=baseline.isolated_env(root)
    project,foreign=baseline.make_project(root)
    name="riskcanary"
    sentinel="risk-" + secrets.token_hex(24)
    placeholder="$"+"{"+MARKER+"}"
    env[MARKER]=sentinel
    command=["node","-e","process.exit(0)"]
    if product=="uap":
        package=project / "package"
        package.mkdir()
        (package/"plugin.json").write_text(json.dumps({
            "name":"risk-mcp", "version":"0.1.0", "mcpServers":"./mcp.json"})+"\n")
        (package/"mcp.json").write_text(json.dumps({"mcpServers":{
            name:{"command":command[0],"args":command[1:],
                  "env":{MARKER:placeholder}}}})+"\n")
        argv=[cli,"add",str(package),"--target","codex","--scope","project",
              "--project-root",str(project),"--yes","--format","json"]
    else:
        (project/"apm.yml").write_text(
            "name: secret-fixture\nversion: 1.0.0\ntargets: [codex]\n"
            "dependencies:\n  mcp:\n    - name: riskcanary\n"
            "      registry: false\n      transport: stdio\n      command: node\n"
            '      args: ["-e", "process.exit(0)"]\n'
            f'      env:\n        {MARKER}: "{placeholder}"\n')
        argv=[cli,"install","--target","codex"]
    cmd=baseline.run_process(argv,project,env,root)
    # Diagnostic stdout is suppressed wholesale for this synthetic-secret
    # case: encoded, transformed and fragmented representations cannot be
    # reliably scrubbed with single-substring replacement.
    cmd["output_tail"] = "<suppressed: synthetic-secret scenario>"
    targets=[".codex/config.toml","uap.toml","uap.lock","apm.yml","apm.lock.yaml"]
    found=[]
    for name_path in targets:
        data=observed_file(project/name_path,project)
        if data is not None:
            found.append({"file":name_path,"synthetic_plaintext_present":sentinel.encode() in data,
                          "literal_placeholder_present":placeholder.encode() in data,
                          "sha256":h(data)})
    # Inspect the complete *named disposable experiment root*, including
    # temporary/cache/managed storage, not only five selected output paths.
    # This does not establish protection from writes outside the sandbox.
    scan = scan_synthetic_canary((root,), sentinel)
    leak=scan["canary_found"] or any(row["synthetic_plaintext_present"] for row in found)
    protected=baseline.safe_same_bytes(project/".agents/skills/foreign-neighbor/SKILL.md",
                                      foreign,project)
    config=next((row for row in found if row["file"]==".codex/config.toml"),None)
    contract=codex_mcp_contract(project)
    if leak or not protected:
        verdict="FAIL";reason="synthetic canary persisted or foreign content was changed"
    elif not scan["complete"]:
        verdict="BLOCKED";reason="bounded project/profile scan incomplete; no secrecy PASS"
    elif cmd.get("timed_out") or cmd.get("cleanup_error"):
        verdict="BLOCKED";reason="MCP project command could not be observed completely"
    elif cmd["exit_code"] != 0:
        verdict="BLOCKED";reason="MCP project route refused, check safe local prerequisite in receipt"
    elif config is None or contract in (
            "missing_config", "missing_server", "wrong_command",
            "wrong_arguments", "invalid_toml"):
        verdict="FAIL";reason="successful install did not materialize the exact requested MCP server"
    elif contract != "match":
        verdict="BLOCKED";reason="requested MCP environment forwarding cannot be verified"
    else:
        # A process may have detached a writer and closed its output pipes.
        # One post-exit scan proves only a point-in-time absence, not
        # containment or that it cannot persist the canary after observation.
        verdict="BLOCKED";reason=("exact MCP config observed and no canary in scanned "
                                   "encodings at exit; detached writers unverified; "
                                   "native tools/call NOT_RUN")
    output = receipt(product,"R5_mcp_env_secret",verdict,reason=reason,
                     command=cmd,config_files=found,filesystem_scan=scan,
                     mcp_config_contract=contract,
                     foreign_unchanged=protected, native_mcp_call="NOT_RUN")
    # Never serialize a synthetic secret to an Actions artifact, including
    # when the observed program happens to put it in a filename or log.
    if sentinel in json.dumps(output, ensure_ascii=False):
        return receipt(product,"R5_mcp_env_secret","BLOCKED",
                       reason="observer could not safely redact recorded data",
                       stage="evidence_redaction_failure", native_mcp_call="NOT_RUN")
    return output


def two_skills_manifest(product: str, project: Path, commit: str,
                        *, include_secondary: bool = True) -> None:
    if product=="uap":
        s=("schema_version = 1\n\n[dependencies.benchmark]\nkind = \"skill\"\n"
           f'git = "{SOURCE}"\nskill = "benchmark-note"\npath = "{SKILL}"\ncommit = "{commit}"\n')
        if include_secondary:
            s+=("\n[dependencies.secondary]\nkind = \"skill\"\n"
                f'git = "{SOURCE}"\nskill = "benchmark-secondary"\n'
                f'path = "{SKILL_SECOND}"\ncommit = "{commit}"\n')
        s+="\n[targets]\ncodex = ["+('\"benchmark\", \"secondary\"' if include_secondary else '"benchmark"')+"]\n"
        (project/"uap.toml").write_text(s)
    else:
        s=("name: removal-consumer\nversion: 1.0.0\ntargets: [codex]\n"
           "dependencies:\n  apm:\n"
           f"    - {REPO}/{SKILL}#{commit}\n")
        if include_secondary:
            s+=f"    - {REPO}/{SKILL_SECOND}#{commit}\n"
        (project/"apm.yml").write_text(s)


def removal_command(product: str, cli: str, project: Path, *, lock: bool=False) -> list[str]:
    if product=="uap":
        argv=[cli,"deps","lock" if lock else "sync","--scope","project",
              "--project-root",str(project)]
        if not lock:
            argv+=["--yes","--format","json"]
        return argv
    return [cli,"install","--target","codex"]


def classify_removed_result(command: dict, stale_skill_state: str,
                            protected_files_intact: bool) -> tuple[str, str]:
    """A failed sync or pre-sync audit cannot make a visible orphan PASS."""
    if not protected_files_intact:
        return "FAIL", "surviving Skill or foreign file unexpectedly modified"
    if not complete_command(command):
        return "BLOCKED", "sync not fully successful; no cleanup PASS can be inferred"
    if stale_skill_state == "present":
        return "FAIL", "removed dependency remains agent-visible after successful sync"
    if stale_skill_state != "absent":
        return "BLOCKED", "removed Skill is unobservable; no cleanup PASS"
    return "PASS", "removed dependency cleaned and surviving Skill intact"


def audit_problem_observed(audit: dict | None) -> bool | None:
    """Only a completed ordinary audit diagnostic is evidence of a problem."""
    if audit is None or not observed_command(audit):
        return None
    code = audit.get("exit_code")
    if not isinstance(code, int) or isinstance(code, bool) or code <= 0:
        return False
    output = audit.get("output_tail", "").lower()
    return (not any(word in output for word in (
                "unknown option", "unknown command", "traceback", "usage:"))
            and any(term in output for term in (
                "drift detected", "orphaned", "policy violation", "lockfile mismatch")))


def case_removed(product: str, cli: str, root: Path, commit: str,
                 expected: dict[str,bytes]) -> dict:
    env=baseline.isolated_env(root)
    author,_=baseline.make_project(root)
    two_skills_manifest(product,author,commit)
    start=baseline.run_process(removal_command(product,cli,author),author,env,root)
    names=("uap.toml","uap.lock") if product=="uap" else ("apm.yml","apm.lock.yaml")
    secondary=author/".agents/skills/benchmark-secondary/SKILL.md"
    second_body=fixture(commit,SKILL_SECOND+"/SKILL.md")
    if (not complete_command(start) or observed_file(secondary,author)!=second_body
        or any(observed_file(author/name,author) is None for name in names)):
        return receipt(product,"R6_git_pull_removal","BLOCKED",
                       stage="two_dependencies_initial_admission",initial=start,
                       reason="two pinned Skills did not form a supported source graph")
    git("init","-q",cwd=author,env=env)
    git("add",*names,cwd=author,env=env)
    git("commit","-qm","two deps",cwd=author,env=env)
    teammate=root/"teammate"
    git("clone","-q",str(author),str(teammate),cwd=root,env=env)
    sibling = teammate / ".agents/skills/foreign-neighbor/SKILL.md"
    sibling.parent.mkdir(parents=True, exist_ok=True)
    sibling.write_bytes(b"FOREIGN USER CONTENT: preserve without edits\n")
    foreign_before = file_identity(sibling, teammate)
    teammate_root=root/"teammate-iso"
    mate_env=baseline.isolated_env(teammate_root)
    mate_first=baseline.run_process(removal_command(product,cli,teammate),teammate,mate_env,root)
    mate_path=teammate/".agents/skills/benchmark-secondary/SKILL.md"
    if not complete_command(mate_first) or observed_file(mate_path,teammate)!=second_body:
        return receipt(product,"R6_git_pull_removal","BLOCKED",
                       stage="teammate_initial_install",initial=mate_first,
                       reason="second user agent installation not admitted")
    two_skills_manifest(product,author,commit,include_secondary=False)
    refresh=None
    if product=="uap":
        refresh=baseline.run_process(removal_command(product,cli,author,lock=True),author,env,root)
    changed=baseline.run_process(removal_command(product,cli,author),author,env,root)
    if not complete_command(changed) or (refresh is not None and
                                         not complete_command(refresh)):
        return receipt(product,"R6_git_pull_removal","BLOCKED",stage="author_removal",
                       refresh=refresh,command=changed,
                       reason="author could not produce valid remaining lock")
    git("add",*names,cwd=author,env=env)
    git("commit","-qm","drop secondary",cwd=author,env=env)
    git("pull","-q","--ff-only",cwd=teammate,env=mate_env)
    postpull=observed_file(mate_path,teammate)
    audit=None
    if product=="apm":
        audit=baseline.run_process([cli,"audit","--ci","--no-policy"],
                                   teammate,mate_env,root)
    applied=baseline.run_process(removal_command(product,cli,teammate),
                                 teammate,mate_env,root)
    final_leaf=skill_presence(mate_path,teammate)
    final_directory=skill_presence(mate_path.parent,teammate)
    # A missing SKILL.md with leftover resources or an empty owned directory
    # is not a fully removed dependency.
    final=("present" if "present" in (final_leaf,final_directory)
           else "unknown" if "unknown" in (final_leaf,final_directory)
           else "absent")
    survivor_paths=(".agents/skills/benchmark-note/SKILL.md",
                    ".agents/skills/benchmark-note/references/fixture.txt")
    survivor_intact=all(
        observed_file(teammate/name,teammate) ==
        expected[name.removeprefix(".agents/skills/benchmark-note/")]
        for name in survivor_paths)
    foreign=observed_file(sibling,teammate)
    preserved=(survivor_intact and
               foreign==b"FOREIGN USER CONTENT: preserve without edits\n"
               and foreign_before is not None
               and foreign_before == file_identity(sibling,teammate))
    # Audit is independent read-only evidence: a pre-sync warning cannot excuse
    # stale agent-visible files or convert an unsuccessful sync to PASS.
    audit_diagnostic = audit_problem_observed(audit)
    verdict,reason = classify_removed_result(applied, final, preserved)
    return receipt(product,"R6_git_pull_removal",verdict,reason=reason,
                   author_action=changed,teammate_install=mate_first,teammate_sync=applied,
                   audit=audit,postpull_before_install_sha256=h(postpull) if postpull else None,
                   # Unknown (symlink, unreadable) is not proven present.
                   stale_skill_still_present={"present": True, "absent": False}.get(final),
                   stale_skill_state=final, stale_directory_state=final_directory,
                   audit_reported_a_problem=audit_diagnostic,
                   survivor_intact=survivor_intact,
                   foreign_unchanged=preserved)


def observe_cli_identity(product: str, executable: Path, root: Path) -> dict:
    """Observe actual binary and version; the separately pinned source tag is not CLI output.

    UAP's bare Go build intentionally reports 0.1.0-development even when its
    verified source came from immutable 0.1.95 tag. Do not rewrite that fact.
    """
    if not executable.is_file() or executable.is_symlink():
        raise ValueError("expected regular executable for " + product)
    if not 0 < executable.stat().st_size <= 128 * 1024 * 1024:
        raise ValueError("executable too large or empty")
    env=baseline.isolated_env(root)
    command=baseline.run_process(
        [str(executable), "version" if product=="uap" else "--version"],
        root,env,root)
    expected=("agentplugins 0.1.0-development" if product=="uap"
              else "Agent Package Manager (APM) CLI version 0.35.0")
    # The parser never silently assumes 0.1.95 from a development build.
    verified=(complete_command(command)
              and expected in [line.strip() for line in
                               command.get("output_tail","").splitlines()])
    return {"entrypoint_sha256":h(executable.read_bytes()),
            "observed_version":command.get("output_tail","")[-512:],
            "probe_exit_code":command.get("exit_code"),
            "probe_capture_complete":command.get("capture_complete"),
            "expected_observed_version":expected,
            "probe_verified":verified,
            "source_build_not_published_binary":product=="uap"}


def main() -> int:
    parser=argparse.ArgumentParser()
    parser.add_argument("--uap",type=Path,required=True)
    parser.add_argument("--apm",type=Path,required=True)
    parser.add_argument("--fixture-commit",required=True)
    parser.add_argument("--uap-source-sha",required=True)
    parser.add_argument("--group",choices=sorted(GROUPS),required=True)
    parser.add_argument("--output",type=Path,required=True)
    parser.add_argument("--claude-verified",action="store_true")
    parser.add_argument("--claude-version",default=None)
    parser.add_argument("--claude-native-sha256",default=None)
    o=parser.parse_args()
    if not all(baseline.PIN.fullmatch(v) for v in
               [o.fixture_commit,o.uap_source_sha,PARENT_V1,PARENT_V2,CHILD_V1,CHILD_V2]):
        parser.error("source pins must be exact immutable 40-character SHAs")
    for name,exe in [("uap",o.uap),("apm",o.apm)]:
        if not exe.is_file() or exe.is_symlink():
            parser.error(f"missing regular {name} executable")
    # CLI output is observed, not inferred from the workflow's requested pins.
    # The workflow separately checks the tag/source and APM wheel digest.
    with tempfile.TemporaryDirectory(prefix="uap-apm-cli-identity-",
                                     dir=os.environ.get("RUNNER_TEMP")) as version_temp:
        identities={}
        for product, executable in (("uap",o.uap),("apm",o.apm)):
            verify_root=Path(version_temp)/product
            verify_root.mkdir()
            identities[product]=observe_cli_identity(product,executable,verify_root)
    if not all(identity["probe_verified"] for identity in identities.values()):
        parser.error("real CLI version probe incomplete or inconsistent")
    cases=[]
    meta={"schema_version":1,"group":o.group,"fixture_commit":o.fixture_commit,
          "uap_release_source_sha":o.uap_source_sha,
          "harness_sha256":h(Path(__file__).read_bytes()),
          "released_source_targets":{"uap":"agentplugins-v0.1.95 source tag",
                                     "apm":"apm-cli==0.35.0 pinned wheel"},
          "observed_cli_identities":identities,
          "limitations":["No production credentials or model calls",
                         "No native agent session / MCP tools/call",
                         "Risk cases with missing prerequisites are BLOCKED, not product FAIL",
                         "No proof of OS mount namespace filesystem confinement"]}
    o.output.mkdir(parents=True,exist_ok=True)
    def save():
        data=dict(meta,cases=cases)
        (o.output/"report.json").write_text(json.dumps(data,indent=2,sort_keys=True)+"\n")
        rows=[f"# UAP / APM risk matrix — {o.group} (evidence, not a winner)",
              "","| Scenario | UAP | APM |","| --- | --- | --- |"]
        for case in GROUPS[o.group]:
            vals=[next((x["status"] for x in cases if x["product"]==product and
                        x["scenario"]==case),"NOT_RUN") for product in ("uap","apm")]
            rows.append(f"| {case} | {vals[0]} | {vals[1]} |")
        rows+=["","Only actual read-back receipts qualify. No vendor OAuth or model runs.",""]
        (o.output/"report.md").write_text("\n".join(rows))
    save()
    expected={name:fixture(o.fixture_commit,SKILL+"/"+name)
              for name in baseline.EXPECTED_FILES}
    with tempfile.TemporaryDirectory(prefix="uap-apm-risks-",dir=os.environ.get("RUNNER_TEMP")) as temp:
        for product,cli in (("uap",str(o.uap)),("apm",str(o.apm))):
            for scenario in GROUPS[o.group]:
                root=Path(temp)/product/scenario
                root.mkdir(parents=True)
                try:
                    if scenario=="R1_missing_receipts":
                        entry=case_cold(product,cli,root,o.fixture_commit,expected)
                    elif scenario=="R2_claude_profiles":
                        entry=case_profiles(product,cli,root,o.fixture_commit,expected,
                                            claude_available=o.claude_verified,
                                            claude_version=o.claude_version,
                                            claude_native_sha256=o.claude_native_sha256)
                    elif scenario=="R3_transitive_update":
                        entry=case_graph(product,cli,root)
                    elif scenario=="R4_interrupted_operation":
                        entry=case_interrupted(product,cli,root,o.fixture_commit)
                    elif scenario=="R5_mcp_env_secret":
                        entry=case_secret(product,cli,root)
                    else:
                        # The second Skill was introduced after the original
                        # #617 fixture SHA. Use a later immutable source commit.
                        entry=case_removed(product,cli,root,PARENT_V2,expected)
                except Exception as exc:
                    entry=receipt(product,scenario,"BLOCKED",
                                  stage="observer_exception",
                                  reason=f"observer failed closed: {type(exc).__name__}")
                cases.append(entry)
                save()
    # Observer exceptions are infrastructure failures; product FAIL/BLOCKED
    # are evidence statuses and must not turn the workflow itself red.
    return 1 if any(row.get("stage")=="observer_exception" for row in cases) else 0


if __name__=="__main__":
    raise SystemExit(main())

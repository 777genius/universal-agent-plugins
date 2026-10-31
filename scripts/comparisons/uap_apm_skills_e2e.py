#!/usr/bin/env python3
"""Evidence-only, opt-in, isolated Agent Skills 1.0 comparison.

This deliberately does not score Agent Plugins whole-package/native loading.
No live agents, network tools inside fixtures, credentials or destructive flags.
"""
from __future__ import annotations

import argparse
from datetime import datetime, timezone
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import signal
import subprocess
import tempfile
import time

REPOSITORY = "777genius/universal-agent-plugins"
SKILL_PATH = "tests/fixtures/uap_apm/skills/benchmark-note"
EXPECTED_FILES = ("SKILL.md", "references/fixture.txt")
PIN = re.compile(r"[0-9a-f]{40}\Z")
TIMEOUT_SECONDS = 150


def sha(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def run_process(argv: list[str], cwd: Path, env: dict[str, str], root: Path) -> dict:
    started = time.monotonic()
    timed_out, cleanup_error, return_code = False, None, None
    log_hash, tail = hashlib.sha256(), b""
    try:
        with tempfile.TemporaryFile() as log:
            proc = subprocess.Popen(
                argv, cwd=cwd, env=env, stdin=subprocess.DEVNULL,
                stdout=log, stderr=subprocess.STDOUT, start_new_session=True,
            )
            try:
                try:
                    return_code = proc.wait(timeout=TIMEOUT_SECONDS)
                except subprocess.TimeoutExpired:
                    timed_out = True
                    try:
                        os.killpg(proc.pid, signal.SIGKILL)
                    except ProcessLookupError:
                        pass
                    try:
                        return_code = proc.wait(timeout=10)
                    except subprocess.TimeoutExpired:
                        cleanup_error = "child did not exit after session-group kill"
            finally:
                try:
                    os.killpg(proc.pid, signal.SIGKILL)
                except ProcessLookupError:
                    pass
                except OSError as exc:
                    cleanup_error = type(exc).__name__
            log.seek(0)
            while chunk := log.read(64 * 1024):
                log_hash.update(chunk)
                tail = (tail + chunk)[-8192:]
    except (OSError, subprocess.SubprocessError) as exc:
        cleanup_error = type(exc).__name__
    text = tail.decode("utf-8", "replace").replace(str(root), "<sandbox>")
    text = re.sub(r"(?i)(token|password|api[_-]?key)\s*[:=]\s*\S+", r"\1=<redacted>", text)
    return {
        "argv": [str(a).replace(str(root), "<sandbox>") for a in argv],
        "exit_code": return_code,
        "timed_out": timed_out,
        "cleanup_error": cleanup_error,
        "duration_ms": round((time.monotonic() - started) * 1000),
        "output_sha256": log_hash.hexdigest(),
        "output_tail": text[-3500:],
    }


def isolated_env(root: Path) -> dict[str, str]:
    home = root / "home"
    paths = {
        "HOME": home, "XDG_CONFIG_HOME": home / ".config",
        "XDG_CACHE_HOME": home / ".cache", "XDG_DATA_HOME": home / ".local" / "share",
        "AGENTPLUGINS_HOME": home / "agentplugins", "TMPDIR": root / "tmp",
        "GIT_CONFIG_GLOBAL": root / "empty-gitconfig",
    }
    for path in paths.values():
        path.mkdir(parents=True, exist_ok=True) if path != paths["GIT_CONFIG_GLOBAL"] else None
    paths["GIT_CONFIG_GLOBAL"].write_text("")
    return {
        "PATH": os.environ["PATH"],
        "LANG": "C.UTF-8", "LC_ALL": "C.UTF-8", "CI": "true",
        "TERM": "dumb", "NO_COLOR": "1",
        "GIT_CONFIG_NOSYSTEM": "1", "GIT_TERMINAL_PROMPT": "0",
        "GIT_CONFIG_GLOBAL": str(paths["GIT_CONFIG_GLOBAL"]),
        "HOME": str(home), "XDG_CONFIG_HOME": str(paths["XDG_CONFIG_HOME"]),
        "XDG_CACHE_HOME": str(paths["XDG_CACHE_HOME"]),
        "XDG_DATA_HOME": str(paths["XDG_DATA_HOME"]),
        "AGENTPLUGINS_HOME": str(paths["AGENTPLUGINS_HOME"]),
        "TMPDIR": str(paths["TMPDIR"]),
    }


def make_project(root: Path) -> tuple[Path, bytes]:
    project = root / "project"
    sentinel = project / ".agents" / "skills" / "foreign-neighbor" / "SKILL.md"
    sentinel.parent.mkdir(parents=True)
    data = b"FOREIGN USER CONTENT: preserve without edits\n"
    sentinel.write_bytes(data)
    return project, data


def safe_same_bytes(path: Path, original: bytes, boundary: Path) -> bool:
    """Read only ordinary files within a non-symlinked disposable project tree."""
    try:
        relative = path.relative_to(boundary)
        if not relative.parts or boundary.is_symlink():
            return False
        current = boundary
        for part in relative.parts:
            current = current / part
            if current.is_symlink():
                return False
        return path.is_file() and path.stat().st_size == len(original) and path.read_bytes() == original
    except (OSError, ValueError):
        return False


def payloads(project: Path, expected: dict[str, bytes]) -> dict:
    # Only Codex's projected Skill counts, never acquired/cache sources.
    skill = project / ".agents" / "skills" / "benchmark-note" / "SKILL.md"
    peer = skill.parent / "references" / "fixture.txt"
    files = {"skills": [], "references": []}
    if safe_same_bytes(skill, expected["SKILL.md"], project):
        files["skills"].append(str(skill.relative_to(project)))
        if safe_same_bytes(peer, expected["references/fixture.txt"], project):
            files["references"].append(str(peer.relative_to(project)))
    return files


def add_result(results: list[dict], product: str, scenario: str, command: dict,
               files: dict, foreign_ok: bool, reason: str | None = None) -> None:
    passed = (not command.get("timed_out") and not command.get("cleanup_error")
              and command.get("exit_code") == 0
              and bool(files["skills"]) and bool(files["references"]) and foreign_ok)
    status = "PASS" if passed else "FAIL"
    if scenario == "S1_foreign_collision":
        # Non-zero + intact foreign content may be a correct refusal or a
        # connection/manifest failure. Inspect evidence before declaring safety.
        if foreign_ok and not command.get("timed_out") and not passed:
            status = "BLOCKED"
        elif not foreign_ok:
            status = "FAIL"
    results.append({
        "product": product, "scenario": scenario,
        "status": status,
        "reason": reason or ("verified projected Skill and preserved foreign files"
                             if passed else "check bounded command log and observed files"),
        "command": command, "observed": files, "foreign_neighbor_unchanged": foreign_ok,
        "native_loaded": "NOT_RUN", "oauth_or_tool_call": "NOT_RUN",
    })


def tool_args(product: str, executable: str, project: Path, commit: str, *, replay: bool = False) -> list[str]:
    if product == "uap":
        if replay:
            return [executable, "install", "--locked", "--scope", "project", "--project-root", str(project), "--yes", "--format", "json"]
        return [executable, "skills", "add", "777genius/universal-agent-plugins",
                "--path", SKILL_PATH, "--ref", commit, "--target", "codex",
                "--scope", "project", "--project-root", str(project),
                "--no-ui", "--yes", "--format", "json"]
    if replay:
        return [executable, "install", "--frozen", "--target", "codex"]
    return [executable, "install", "--target", "codex"]


def prepare_apm_manifest(project: Path, commit: str) -> None:
    # Documented APM v0.35.0 GitHub owner/repository/subfolder#ref source.
    source = f"{REPOSITORY}/{SKILL_PATH}#{commit}"
    (project / "apm.yml").write_text(
        "name: skills-comparison\nversion: 1.0.0\ntargets:\n  - codex\n"
        "dependencies:\n  apm:\n    - " + source + "\n"
    )


def captured_portable(project: Path, product: str) -> dict[str, bytes]:
    names = ("uap.toml", "uap.lock") if product == "uap" else ("apm.yml", "apm.lock.yaml")
    found = {}
    for name in names:
        path = project / name
        try:
            if path.is_symlink() or not path.is_file():
                return {}
            data = path.read_bytes()
            if not data or len(data) > 2 * 1024 * 1024:
                return {}
            found[name] = data
        except OSError:
            return {}
    return found


def stored_portable(portable: dict[str, bytes], output: Path, product: str, phase: str) -> None:
    directory = output / "portable" / product / phase
    directory.mkdir(parents=True, exist_ok=True)
    for name, content in portable.items():
        (directory / name).write_bytes(content)


def portable_identity(portable: dict[str, bytes], commit: str) -> dict:
    lock_name = "uap.lock" if "uap.lock" in portable else "apm.lock.yaml"
    return {
        "sha256": {name: sha(content) for name, content in portable.items()},
        "exact_source_commit_recorded_in_lock": commit.encode("ascii") in portable.get(lock_name, b""),
    }


def evaluate(product: str, executable: str, root: Path, commit: str,
             expected: dict[str, bytes], output: Path, on_progress=None) -> list[dict]:
    results: list[dict] = []

    def saved() -> None:
        if on_progress is not None:
            on_progress(results)

    def skip(scenario: str, reason: str) -> None:
        results.append({"product": product, "scenario": scenario, "status": "NOT_RUN",
                        "reason": reason, "native_loaded": "NOT_RUN",
                        "oauth_or_tool_call": "NOT_RUN"})
        saved()

    env = isolated_env(root)
    project, foreign = make_project(root)
    sentinel = project / ".agents" / "skills" / "foreign-neighbor" / "SKILL.md"
    if product == "apm":
        prepare_apm_manifest(project, commit)
    command = run_process(tool_args(product, executable, project, commit), project, env, root)
    observed = payloads(project, expected)
    preserved = safe_same_bytes(sentinel, foreign, project)
    add_result(results, product, "S1_install", command, observed, preserved)
    saved()

    # Snapshot portable bytes immediately after FIRST install, before replay.
    initial = captured_portable(project, product)
    if initial:
        stored_portable(initial, output, product, "initial")
    admitted = results[-1]["status"] == "PASS"
    if not admitted:
        skip("S1_repeat", "initial delivery did not pass exact projected-byte and safety checks")
        skip("R1_cold_locked_restore", "initial delivery not admitted")
    elif not initial:
        skip("S1_repeat", "no complete portable manifest/lock from the documented initial route")
        skip("R1_cold_locked_restore", "no complete portable manifest/lock from the documented initial route")
    else:
        source_identity = portable_identity(initial, commit)
        if not source_identity["exact_source_commit_recorded_in_lock"]:
            skip("S1_repeat", "source commit absent from recorded lock; provenance needs manual verification")
            skip("R1_cold_locked_restore", "source commit absent from recorded lock; provenance needs manual verification")
        else:
            replay = run_process(tool_args(product, executable, project, commit, replay=True), project, env, root)
            after_replay = captured_portable(project, product)
            add_result(results, product, "S1_repeat", replay, payloads(project, expected),
                       safe_same_bytes(sentinel, foreign, project))
            results[-1]["portable"] = {
                "initial": source_identity, "after": portable_identity(after_replay, commit),
                "byte_identical": initial == after_replay,
            }
            if results[-1]["status"] == "PASS" and after_replay != initial:
                results[-1]["status"] = "FAIL"
                results[-1]["reason"] = "repeat rewrote portable manifest or lock"
            if after_replay:
                stored_portable(after_replay, output, product, "after-repeat")
            saved()

            fresh_root = root / "restore"
            fresh_env = isolated_env(fresh_root)
            fresh, fresh_foreign = make_project(fresh_root)
            for name, content in initial.items():
                (fresh / name).write_bytes(content)
            restore = run_process(tool_args(product, executable, fresh, commit, replay=True),
                                  fresh, fresh_env, fresh_root)
            after_restore = captured_portable(fresh, product)
            add_result(results, product, "R1_cold_locked_restore", restore, payloads(fresh, expected),
                       safe_same_bytes(fresh / ".agents" / "skills" / "foreign-neighbor" / "SKILL.md",
                                       fresh_foreign, fresh))
            results[-1]["portable"] = {
                "initial": source_identity, "after": portable_identity(after_restore, commit),
                "byte_identical": initial == after_restore,
            }
            if results[-1]["status"] == "PASS" and after_restore != initial:
                results[-1]["status"] = "FAIL"
                results[-1]["reason"] = "cold restore rewrote the supplied portable manifest or lock"
            if after_restore:
                stored_portable(after_restore, output, product, "after-cold-restore")
            saved()

    # Independent collision experiment; never overwrite or delete a foreign Skill.
    collision_root = root / "collision"
    collision_env = isolated_env(collision_root)
    conflicting_project, foreign_content = make_project(collision_root)
    conflict_file = conflicting_project / ".agents" / "skills" / "benchmark-note" / "SKILL.md"
    conflict_file.parent.mkdir(parents=True)
    conflict_bytes = b"FOREIGN OWNED COLLISION -- NEVER OVERWRITE\n"
    conflict_file.write_bytes(conflict_bytes)
    if product == "apm":
        prepare_apm_manifest(conflicting_project, commit)
    conflict = run_process(tool_args(product, executable, conflicting_project, commit),
                           conflicting_project, collision_env, collision_root)
    preserved = (
        safe_same_bytes(conflict_file, conflict_bytes, conflicting_project)
        and safe_same_bytes(conflicting_project / ".agents" / "skills" / "foreign-neighbor" / "SKILL.md",
                            foreign_content, conflicting_project)
    )
    add_result(results, product, "S1_foreign_collision", conflict,
               payloads(conflicting_project, expected), preserved)
    if results[-1]["status"] == "BLOCKED":
        results[-1]["reason"] = "foreign bytes preserved, but refusal cause not independently classified"
    saved()
    return results


def write_receipt(output: Path, metadata: dict, cases: list[dict]) -> None:
    output.mkdir(parents=True, exist_ok=True)
    receipt = dict(metadata)
    receipt["cases"] = cases
    (output / "report.json").write_text(json.dumps(receipt, indent=2, sort_keys=True) + "\n")
    rows = ["# UAP vs APM Agent Skills 1.0 E2E (unreviewed observations)",
            "", "| Scenario | UAP | APM |", "| --- | --- | --- |"]
    for name in ("S1_install", "S1_repeat", "R1_cold_locked_restore", "S1_foreign_collision"):
        states = [next((c["status"] for c in cases
                        if c.get("product") == product and c.get("scenario") == name), "NOT_RUN")
                  for product in ("uap", "apm")]
        rows.append(f"| {name} | {states[0]} | {states[1]} |")
    rows.extend(["", "No competitive winner is inferred from this raw receipt.",
                 "Agent Plugins 1.0 whole-package, native activation and vendor OAuth were NOT_RUN.",
                 "Environment variables alone do not prove filesystem confinement.", ""])
    (output / "report.md").write_text("\n".join(rows))


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--uap", type=Path, required=True)
    parser.add_argument("--apm", type=Path, required=True)
    parser.add_argument("--fixture-commit", required=True)
    parser.add_argument("--uap-source-sha", required=True)
    parser.add_argument("--output", type=Path, required=True)
    opts = parser.parse_args()
    if not PIN.fullmatch(opts.fixture_commit) or not PIN.fullmatch(opts.uap_source_sha):
        parser.error("commits must be full immutable Git SHAs")
    expected = {}
    for name in EXPECTED_FILES:
        expected[name] = subprocess.check_output(
            ["git", "show", f"{opts.fixture_commit}:{SKILL_PATH}/{name}"]
        )
    versions = {}
    for name, executable in (("uap", opts.uap), ("apm", opts.apm)):
        if not executable.is_file() or executable.is_symlink():
            parser.error(f"missing ordinary executable {name}: {executable}")
        versions[name] = {"entrypoint_sha256": sha(executable.read_bytes())}
    metadata = {
        "protocol_version": 1,
        "collected_at_utc": datetime.now(timezone.utc).isoformat(),
        "harness_source_commit": subprocess.check_output(
            ["git", "rev-parse", "HEAD"], text=True
        ).strip(),
        "harness_script_sha256": sha(Path(__file__).read_bytes()),
        "fixture_repository": REPOSITORY,
        "fixture_git_commit": opts.fixture_commit,
        "fixture_sha256": {name: sha(data) for name, data in expected.items()},
        "uap_source_commit": opts.uap_source_sha,
        "uap_artifact_kind": "source build from immutable release tag, not the published binary",
        "apm_claimed_version": "0.35.0", "platform": "linux-amd64",
        "executable_identity": versions,
        "limitations": [
            "S1 exact Codex project Skill files only, not native client loading",
            "APM and UAP use distinct documented manager manifests and install routes",
            "No filesystem confinement proof, only independent ephemeral HOME/XDG/project roots",
            "P1/P2 Agent Plugins 1.0, OAuth/tool execution and other clients NOT_RUN",
            "NOT_RUN/BLOCKED are never counted as FAIL or PASS without evidence admission",
        ],
    }
    cases: list[dict] = []
    write_receipt(opts.output, metadata, cases)  # Recoverable partial evidence from start.
    errors = []
    with tempfile.TemporaryDirectory(prefix="uap-apm-", dir=os.environ.get("RUNNER_TEMP")) as directory:
        for name, executable in (("uap", opts.uap), ("apm", opts.apm)):
            root = Path(directory) / name
            root.mkdir()
            def progress(partial: list[dict]) -> None:
                write_receipt(opts.output, metadata, cases + partial)
            try:
                cases.extend(evaluate(name, str(executable), root, opts.fixture_commit,
                                      expected, opts.output, on_progress=progress))
            except Exception as exc:
                # Do not silently treat harness defects or destroyed output as product failures.
                errors.append(name)
                cases.append({
                    "product": name, "scenario": "HARNESS_ERROR", "status": "BLOCKED",
                    "reason": f"benchmark observation error ({type(exc).__name__})",
                    "native_loaded": "NOT_RUN", "oauth_or_tool_call": "NOT_RUN",
                })
            write_receipt(opts.output, metadata, cases)
    if errors:
        print("Experiment incomplete due to harness errors:", ", ".join(errors))
        return 1  # Upload-if-always still retains the partial report.
    return 0  # Green means evidence captured; individual product failures remain visible.




if __name__ == "__main__":
    raise SystemExit(main())

#!/usr/bin/env python3
"""Evidence-only, opt-in, isolated Agent Skills 1.0 comparison.

This deliberately does not score Agent Plugins whole-package/native loading.
No live agents, network tools inside fixtures, credentials or destructive flags.
"""
from __future__ import annotations

import argparse
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
    with tempfile.TemporaryFile() as log:
        proc = subprocess.Popen(
            argv, cwd=cwd, env=env, stdin=subprocess.DEVNULL,
            stdout=log, stderr=subprocess.STDOUT,
            start_new_session=True,
        )
        timed_out = False
        try:
            proc.wait(timeout=TIMEOUT_SECONDS)
        except subprocess.TimeoutExpired:
            timed_out = True
            try:
                os.killpg(proc.pid, signal.SIGKILL)
            except ProcessLookupError:
                pass
            proc.wait(timeout=10)
        # Bound helpers left in the selected process group, even on exit zero.
        try:
            os.killpg(proc.pid, signal.SIGKILL)
        except ProcessLookupError:
            pass
        digest = hashlib.sha256()
        tail = b""
        log.seek(0)
        while chunk := log.read(64 * 1024):
            digest.update(chunk)
            tail = (tail + chunk)[-8192:]
    text = tail.decode("utf-8", "replace").replace(str(root), "<sandbox>")
    text = re.sub(r"(?i)(token|password|api[_-]?key)\s*[:=]\s*\S+", r"\1=<redacted>", text)
    return {
        "argv": [str(a).replace(str(root), "<sandbox>") for a in argv],
        "exit_code": proc.returncode,
        "timed_out": timed_out,
        "duration_ms": round((time.monotonic() - started) * 1000),
        "output_sha256": digest.hexdigest(),
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


def payloads(project: Path, expected: dict[str, bytes]) -> dict:
    # Only Codex's delivered Skill counts. An identical cached/downloaded
    # package under apm_modules/ or .uap/ is not an installed client component.
    skill = project / ".agents" / "skills" / "benchmark-note" / "SKILL.md"
    peer = skill.parent / "references" / "fixture.txt"
    if skill.is_symlink() or not skill.is_file() or skill.read_bytes() != expected["SKILL.md"]:
        return {"skills": [], "references": []}
    files = {"skills": [str(skill.relative_to(project))], "references": []}
    if peer.is_file() and not peer.is_symlink() and peer.read_bytes() == expected["references/fixture.txt"]:
        files["references"].append(str(peer.relative_to(project)))
    return files


def add_result(results: list[dict], product: str, scenario: str, command: dict,
               files: dict, foreign_ok: bool, reason: str | None = None) -> None:
    passed = (not command["timed_out"] and command["exit_code"] == 0
              and bool(files["skills"]) and bool(files["references"]) and foreign_ok)
    status = "PASS" if passed else "FAIL"
    if scenario == "foreign_collision":
        # A nonzero response with unchanged foreign data might be a correct
        # conflict refusal OR a network/preflight failure. No automatic PASS.
        if foreign_ok and not command["timed_out"] and not passed:
            status = "BLOCKED"
        elif not foreign_ok:
            status = "FAIL"
    results.append({
        "product": product, "scenario": scenario,
        "status": status,
        "reason": reason or ("observed required bytes and preservation" if passed else "see bounded command output and observed files"),
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


def evaluate(product: str, executable: str, root: Path, commit: str,
             expected: dict[str, bytes]) -> list[dict]:
    results: list[dict] = []
    env = isolated_env(root)
    project, foreign = make_project(root)
    sentinel = project / ".agents" / "skills" / "foreign-neighbor" / "SKILL.md"
    if product == "apm":
        prepare_apm_manifest(project, commit)
    cmd = run_process(tool_args(product, executable, project, commit), project, env, root)
    files = payloads(project, expected)
    add_result(results, product, "S1_install", cmd, files, sentinel.read_bytes() == foreign)

    if cmd["exit_code"] != 0 or cmd["timed_out"] or not files["skills"] or not files["references"]:
        for name in ("S1_repeat", "R1_cold_locked_restore"):
            results.append({
                "product": product, "scenario": name, "status": "NOT_RUN",
                "reason": "initial delivery not admitted", "native_loaded": "NOT_RUN",
                "oauth_or_tool_call": "NOT_RUN",
            })
    else:
        # Both tools must replay their own recorded lock; repeating a mutable
        # add request for only one tool would not be an equivalent scenario.
        lock_name = "uap.lock" if product == "uap" else "apm.lock.yaml"
        if (project / lock_name).is_file():
            repeat = run_process(tool_args(product, executable, project, commit, replay=True), project, env, root)
            add_result(results, product, "S1_repeat", repeat, payloads(project, expected), sentinel.read_bytes() == foreign)
        else:
            results.append({
                "product": product, "scenario": "S1_repeat", "status": "NOT_RUN",
                "reason": "initial documented source route did not create a saved lock",
                "native_loaded": "NOT_RUN", "oauth_or_tool_call": "NOT_RUN",
            })
        # A cold copy receives ONLY portable intent/lock, not receipts, caches or user config.
        restore_root = root / "restore"
        restore_env = isolated_env(restore_root)
        fresh, fresh_foreign = make_project(restore_root)
        for name in (("uap.toml", "uap.lock") if product == "uap" else ("apm.yml", "apm.lock.yaml")):
            source = project / name
            if not source.is_file():
                results.append({
                    "product": product, "scenario": "R1_cold_locked_restore",
                    "status": "NOT_RUN", "reason": f"no portable {name} produced by initial documented route",
                    "native_loaded": "NOT_RUN", "oauth_or_tool_call": "NOT_RUN",
                })
                break
            shutil.copy2(source, fresh / name)
        else:
            restored = run_process(tool_args(product, executable, fresh, commit, replay=True), fresh, restore_env, restore_root)
            add_result(results, product, "R1_cold_locked_restore", restored, payloads(fresh, expected),
                       (fresh / ".agents" / "skills" / "foreign-neighbor" / "SKILL.md").read_bytes() == fresh_foreign)

    # Separate adversarial environment: a pre-existing foreign Skill with the same name.
    collision_root = root / "collision"
    collision_env = isolated_env(collision_root)
    conflicting_project, existing_foreign = make_project(collision_root)
    conflict_file = conflicting_project / ".agents" / "skills" / "benchmark-note" / "SKILL.md"
    conflict_file.parent.mkdir(parents=True)
    conflict_content = b"FOREIGN OWNED COLLISION -- NEVER OVERWRITE\n"
    conflict_file.write_bytes(conflict_content)
    if product == "apm":
        prepare_apm_manifest(conflicting_project, commit)
    conflict = run_process(tool_args(product, executable, conflicting_project, commit),
                           conflicting_project, collision_env, collision_root)
    preserved = (conflict_file.read_bytes() == conflict_content and
                 (conflicting_project / ".agents" / "skills" / "foreign-neighbor" / "SKILL.md").read_bytes() == existing_foreign)
    add_result(results, product, "S1_foreign_collision", conflict,
               payloads(conflicting_project, expected), preserved,
               "safe refusal or non-destructive outcome required" if preserved else "foreign content overwritten")
    return results


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
    opts.output.mkdir(parents=True, exist_ok=True)
    expected = {}
    for name in EXPECTED_FILES:
        data = subprocess.check_output(["git", "show", f"{opts.fixture_commit}:{SKILL_PATH}/{name}"])
        expected[name] = data
    versions = {}
    for name, executable in (("uap", opts.uap), ("apm", opts.apm)):
        if not executable.is_file():
            parser.error(f"missing executable {name}: {executable}")
        versions[name] = {"executable_sha256": sha(executable.read_bytes())}
    with tempfile.TemporaryDirectory(prefix="uap-apm-", dir=os.environ.get("RUNNER_TEMP")) as directory:
        base = Path(directory)
        records = []
        for name, executable in (("uap", opts.uap), ("apm", opts.apm)):
            root = base / name
            root.mkdir()
            records.extend(evaluate(name, str(executable), root, opts.fixture_commit, expected))
        receipt = {
            "protocol_version": 1,
            "fixture_repository": REPOSITORY,
            "fixture_git_commit": opts.fixture_commit,
            "fixture_sha256": {name: sha(data) for name, data in expected.items()},
            "uap_source_commit": opts.uap_source_sha,
            "apm_claimed_version": "0.35.0", "platform": "linux-amd64",
            "executable_identity": versions, "cases": records,
            "limitations": [
                "S1 package delivery only; no actual Codex/LLM/activation proof",
                "APM and UAP command syntax differ by documented package-manager format",
                "P1/P2 Agent Plugins 1.0, native verification, OAuth/tool calls and additional agents NOT_RUN",
                "NOT_RUN is not FAIL; observed errors need human admission review before comparison claims",
            ],
        }
        (opts.output / "report.json").write_text(json.dumps(receipt, indent=2, sort_keys=True) + "\n")
        rows = ["# First UAP vs APM Agent Skills 1.0 E2E (unreviewed observations)",
                "", "| Scenario | UAP | APM |", "| --- | --- | --- |"]
        for name in ("S1_install", "S1_repeat", "R1_cold_locked_restore", "S1_foreign_collision"):
            statuses = [next((x["status"] for x in records if x["product"] == product and x["scenario"] == name),
                             "NOT_RUN") for product in ("uap", "apm")]
            rows.append(f"| {name} | {statuses[0]} | {statuses[1]} |")
        rows.extend(["", "No winner is inferred until source, native boundaries, and failure causes have been reviewed.",
                     "No Agent Plugins whole-package or authenticated client execution was tested.", ""])
        (opts.output / "report.md").write_text("\n".join(rows))
    return 0  # Observation failures do not silently change test infrastructure into a score gate.


if __name__ == "__main__":
    raise SystemExit(main())

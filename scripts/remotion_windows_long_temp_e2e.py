"""Real pinned Remotion Skill lifecycle on short and default-long Windows TEMP.

Reuses the existing published/native case auditor with a source-built CLI.
No alternate success interpretation, fixture edit or native trust bypass.
"""

import hashlib
import importlib.util
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import tomllib

FIXTURE_SHA256 = "7399eaed15b3eb0981a6be12082dd8825529e896a59ea125b9500a666c21112e"
REMOTION_SHA = "c7cb2fdcacd134fb2ccb9b7a44f9ffa44e467271"

# Codex 0.153.4 supports native sqlite_home/log_dir in config.toml. These
# process-generated stores must not be mistaken for active plugin configuration.
# Both remain inside the same disposable case and get independent validation.
CODEX_DB_NAMES = {
    "state_5.sqlite", "logs_2.sqlite", "goals_1.sqlite",
    "memories_1.sqlite", "queue_1.sqlite", "thread_history_1.sqlite",
}


def prepare_codex_runtime_case(root):
    case = root / "skill-24"
    profile = case / "home" / ".codex"
    sqlite = case / "home" / ".local" / "state" / "codex-sqlite"
    logs = case / "home" / ".local" / "state" / "codex-logs"
    for directory in (profile, sqlite, logs):
        directory.mkdir(parents=True, exist_ok=False)
        if directory.is_symlink() or not directory.resolve(strict=True).is_relative_to(case.resolve(strict=True)):
            raise RuntimeError("Codex runtime store escaped disposable case")
    config = profile / "config.toml"
    original = ("sqlite_home = " + json.dumps(str(sqlite.resolve(strict=True))) +
                "\nlog_dir = " + json.dumps(str(logs.resolve(strict=True))) + "\n").encode()
    with config.open("xb") as stream:
        stream.write(original)
    return case, config, original, sqlite, logs


def verify_codex_runtime_case(prepared, audit):
    case, config, original, sqlite, logs = prepared
    anchor = case.resolve(strict=True)
    project = case / "project"
    if (project.is_symlink() or not project.is_dir()
            or project.resolve(strict=True).parent != anchor):
        raise RuntimeError("Codex trust target escaped disposable case")
    project = project.resolve(strict=True)
    if config.is_symlink() or not config.resolve(strict=True).is_relative_to(anchor):
        raise RuntimeError("native Codex runtime configuration escaped disposable case")
    expected = tomllib.loads(original.decode("utf-8"))
    observed = tomllib.loads(config.read_text(encoding="utf-8"))
    projects = observed.pop("projects", None)
    # Normal confirmed installation must grant this exact project's trust. It
    # remains a user preference after removal; runtime keys and every other
    # configuration value must retain their original meaning.
    if observed != expected or not isinstance(projects, dict) or len(projects) != 1:
        raise RuntimeError("native Codex configuration changed beyond exact project trust")
    trusted_path, trust = next(iter(projects.items()))
    trusted = Path(trusted_path)
    # Reuse the auditor's physical containment for Windows extended-length
    # spellings, then require this exact directory without redirected ancestry.
    if (not audit.codex_marketplace_source_in_case(trusted, case)
            or any(path.is_symlink() or path.is_junction()
                   for path in (trusted, *trusted.parents))
            or not project.samefile(trusted)
            or trust != {"trust_level": "trusted"}):
        raise RuntimeError("native Codex did not preserve exact selected project trust")
    counts = {"sqlite": 0, "logs": 0}

    def refuse_walk_error(error):
        raise RuntimeError("Codex runtime metadata directory is unreadable") from error

    for label, root in (("sqlite", sqlite), ("logs", logs)):
        if root.is_symlink() or not root.is_dir() or not root.resolve(strict=True).is_relative_to(anchor):
            raise RuntimeError("Codex runtime metadata store escaped disposable case")
        for base, dirs, files in os.walk(root, followlinks=False, onerror=refuse_walk_error):
            if label == "sqlite" and dirs:
                raise RuntimeError("Codex SQLite home contains an unexpected subdirectory")
            for name in dirs:
                directory = Path(base) / name
                if directory.is_symlink() or not directory.is_dir() or not directory.resolve(strict=True).is_relative_to(anchor):
                    raise RuntimeError("Codex runtime metadata contains an unsafe directory")
            for name in files:
                file = Path(base) / name
                if file.is_symlink() or not file.is_file() or not file.resolve(strict=True).is_relative_to(anchor):
                    raise RuntimeError("Codex runtime metadata contains an unsafe file")
                # Only six DBs from exact upstream Codex 0.153.4 and their
                # SQLite WAL/SHM/journal companions qualify in this store.
                if label == "sqlite" and not any(name == db or name in (
                        db + "-wal", db + "-shm", db + "-journal") for db in CODEX_DB_NAMES):
                    raise RuntimeError("Codex runtime SQLite store has an unknown member")
                if file.stat().st_size > (256 << 20 if label == "sqlite" else 32 << 20):
                    raise RuntimeError("Codex runtime metadata exceeds per-file audit bound")
                counts[label] += 1
    if counts["sqlite"] == 0:
        raise RuntimeError("pinned Codex did not use its separately guarded SQLite home")
    return counts


def main():
    if os.name != "nt":
        raise SystemExit("genuine Windows runner required")
    repo = Path(__file__).resolve().parent.parent
    fixture_path = repo / "tests/live-install/fixtures.json"
    report = Path(os.environ["RUNNER_TEMP"]) / "remotion-windows-source-longtemp.json"
    # Windows checkout may convert LF to CRLF. Bind the immutable fixture to
    # Git's committed object, then accept only that exact platform conversion
    # in the disposable worktree; never invent or rewrite fixture bytes.
    fixture_bytes = subprocess.check_output(
        ["git", "show", "HEAD:tests/live-install/fixtures.json"], cwd=repo)
    checkout_bytes = fixture_path.read_bytes()
    if hashlib.sha256(fixture_bytes).hexdigest() != FIXTURE_SHA256:
        report.write_text(json.dumps({"stage": "fixture", "reason": "git-blob-digest"}) + "\n")
        raise SystemExit("immutable Git fixture SHA-256 mismatch")
    if checkout_bytes not in (fixture_bytes, fixture_bytes.replace(b"\n", b"\r\n")):
        report.write_text(json.dumps({"stage": "fixture", "reason": "unexpected-worktree-bytes"}) + "\n")
        raise SystemExit("checkout modified immutable fixture beyond Windows EOL conversion")
    fixtures = json.loads(fixture_bytes)
    item = fixtures["skills"][24]
    if item["name"] != "remotion-best-practices" or REMOTION_SHA not in item["source"]:
        raise SystemExit("wrong immutable Remotion skill source")

    module_path = repo / "scripts/live-install-audit.py"
    spec = importlib.util.spec_from_file_location("uap_remotion_audit", module_path)
    audit = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(audit)
    audit.validate_fixtures(fixtures)

    cli = Path(os.environ["UAP_SOURCE_CLI"]).resolve(strict=True)
    if not cli.is_file() or cli.suffix.lower() != ".exe":
        raise SystemExit("source-built Windows CLI executable missing")
    short = Path(tempfile.mkdtemp(prefix="uap-remotion-short-", dir=os.environ["RUNNER_TEMP"]))
    long = Path(tempfile.mkdtemp(prefix="uap-live-30x30-"))
    evidence = {
        "source_checkout_sha": subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=repo, text=True).strip(),
        "fixture_sha256": FIXTURE_SHA256,
        "fixture_worktree_eol": ("unchanged" if checkout_bytes == fixture_bytes else "crlf-only"),
        "remotion_commit": REMOTION_SHA,
        "cli_path_kind": "source-built-windows",
        "results": {},
    }
    failures = []
    try:
        short_length = len(str(short / "skill-24"))
        long_length = len(str(long / "skill-24"))
        if long_length < 65 or long_length <= short_length:
            raise RuntimeError("default Windows TEMP is not longer than runner TEMP")
        for label, root in (("runner-temp", short), ("default-windows-temp", long)):
            try:
                runtime = prepare_codex_runtime_case(root)
                row = audit.do_case(str(cli), "codex", "skill", item, 24, root,
                                    fixtures["plugin_declared_names"], fixtures["signed_release_pins"])
                evidence["results"][label] = {"root_chars": len(str(root / "skill-24")), "case": row}
                runtime_counts = verify_codex_runtime_case(runtime, audit)
                evidence["results"][label]["codex_runtime_stores_checked"] = runtime_counts
                if row.get("status") != "passed":
                    failures.append(f"{label}: native E2E case did not PASS")
            except Exception as error:
                failures.append(f"{label}: {type(error).__name__}")
                evidence["results"].setdefault(label, {"root_chars": len(str(root / "skill-24"))})["error_type"] = type(error).__name__
    finally:
        report.write_text(json.dumps(evidence, indent=2) + "\n", encoding="utf-8")
        for root in (short, long):
            shutil.rmtree(root, ignore_errors=True)
    if failures:
        raise SystemExit("; ".join(failures))
    print("Real Codex Remotion native lifecycle PASS in both short and default-long Windows TEMP")


if __name__ == "__main__":
    main()

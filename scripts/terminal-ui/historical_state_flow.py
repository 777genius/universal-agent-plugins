#!/usr/bin/env python3
"""Generate legacy state with real release binaries, then migrate it."""

import argparse
import hashlib
import io
import json
import os
from pathlib import Path
import shutil
import subprocess
import tarfile

from harness import Fixture, check


REPO = Path(__file__).resolve().parents[2]
RELEASES = (
    ("agentplugins-v0.1.4", "0.1.4", 2),
    ("agentplugins-v0.1.5", "0.1.5", 2),
    ("agentplugins-v0.1.6", "0.1.6", 3),
)


def sha256_bytes(body):
    return hashlib.sha256(body).hexdigest()


def write_json(path, value):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(
        json.dumps(value, indent=2, sort_keys=True) + "\n",
        encoding="utf-8")


def run(argv, cwd, env, evidence, label, expected):
    result = subprocess.run(
        [str(value) for value in argv], cwd=cwd, env=env,
        text=True, capture_output=True, timeout=120)
    (evidence / f"{label}.stdout").write_text(
        result.stdout, encoding="utf-8")
    (evidence / f"{label}.stderr").write_text(
        result.stderr, encoding="utf-8")
    write_json(evidence / f"{label}.command.json", [str(v) for v in argv])
    write_json(evidence / f"{label}.result.json", {
        "exit": result.returncode,
    })
    check(
        result.returncode == expected,
        f"{label}: exit={result.returncode}, expected={expected}; "
        f"stderr={result.stderr!r}")
    return result


def extract_release(tag, target):
    archive = subprocess.check_output(
        ["git", "archive", "--format=tar", tag], cwd=REPO)
    with tarfile.open(fileobj=io.BytesIO(archive), mode="r:") as bundle:
        def safe_filter(member, destination):
            if member.issym() or member.islnk():
                link = Path(member.linkname)
                if link.is_absolute() or ".." in link.parts:
                    return None
            return tarfile.data_filter(member, destination)

        for member in bundle.getmembers():
            parts = Path(member.name).parts
            check(
                not Path(member.name).is_absolute() and ".." not in parts,
                f"{tag}: unsafe archive path {member.name!r}")
        bundle.extractall(target, filter=safe_filter)


def parse_json(stdout, label):
    try:
        value = json.loads(stdout)
    except json.JSONDecodeError as error:
        raise AssertionError(f"{label}: invalid JSON: {error}") from error
    check(value.get("schema_version") == 1, f"{label}: schema changed")
    return value


def build_release(go, tag, version, workspace, evidence):
    source = workspace / "source"
    source.mkdir()
    extract_release(tag, source)
    module = source / "cli/plugin-kit-ai"
    binary = workspace / "agentplugins"
    env = dict(os.environ, GOTOOLCHAIN="local", CGO_ENABLED="0")
    run([
        go, "build", "-trimpath", f"-ldflags=-X=main.version={version}",
        "-o", binary, "./cmd/agentplugins",
    ], module, env, evidence, "build", 0)
    commit = subprocess.check_output(
        ["git", "rev-list", "-n", "1", tag],
        cwd=REPO, text=True).strip()
    return binary, commit


def release_flow(current, go, tag, version, expected_schema, root):
    evidence = root / tag
    evidence.mkdir()
    workspace = root / (tag + "-workspace")
    workspace.mkdir()
    historical, commit = build_release(
        go, tag, version, workspace, evidence)
    fixture = Fixture(workspace / "fixture")
    env = dict(fixture.env)

    created = run([
        historical, "add", str(fixture.package), "--target=cursor",
        "--yes", "--format=json",
    ], fixture.project, env, evidence, "historical-add", 0)
    parse_json(created.stdout, "historical-add")
    state_path = fixture.data / "state-v2.json"
    original = state_path.read_bytes()
    legacy = json.loads(original)
    check(
        legacy.get("schema_version") == expected_schema,
        f"{tag}: wrote schema {legacy.get('schema_version')}, "
        f"expected {expected_schema}")
    check(
        len(legacy.get("installations", [])) == 1,
        f"{tag}: historical add lost installation")

    rejected = run([
        current, "update", "pty-synthetic", "--target=cursor",
        "--format=json",
    ], fixture.project, env, evidence, "current-update-before-migrate", 1)
    diagnostic = (rejected.stdout + rejected.stderr).lower()
    check(
        "migrate-state" in diagnostic or "migration" in diagnostic,
        f"{tag}: current mutation lacked migration diagnostic")
    check(
        state_path.read_bytes() == original,
        f"{tag}: rejected mutation changed legacy state")

    dry = run([
        current, "migrate-state", "--dry-run", "--format=json",
    ], fixture.project, env, evidence, "migrate-dry-run", 0)
    parse_json(dry.stdout, "migrate-dry-run")
    check(
        state_path.read_bytes() == original,
        f"{tag}: dry-run changed legacy state")

    applied = run([
        current, "migrate-state", "--format=json",
    ], fixture.project, env, evidence, "migrate-apply", 0)
    parse_json(applied.stdout, "migrate-apply")
    migrated = json.loads(state_path.read_text(encoding="utf-8"))
    check(
        migrated.get("schema_version") == 4,
        f"{tag}: migration did not write schema 4")
    check(
        len(migrated.get("installations", [])) == 1,
        f"{tag}: migration lost installation")
    backups = list(state_path.parent.glob(
        f"{state_path.name}.schema{expected_schema}.backup-agentplugins-*"))
    check(
        len(backups) == 1 and backups[0].read_bytes() == original,
        f"{tag}: migration backup differs from historical state")

    removed = run([
        current, "remove", "pty-synthetic", "--target=cursor",
        "--external-uninstalled", "--purge-data", "--format=json",
    ], fixture.project, env, evidence, "current-remove", 0)
    parse_json(removed.stdout, "current-remove")
    final = json.loads(state_path.read_text(encoding="utf-8"))
    check(
        final.get("schema_version") == 4 and
        not final.get("installations"),
        f"{tag}: migrated state did not complete remove lifecycle")

    return {
        "tag": tag,
        "commit": commit,
        "version": version,
        "legacy_schema": expected_schema,
        "historical_binary_sha256": sha256_bytes(historical.read_bytes()),
        "historical_state_sha256": sha256_bytes(original),
        "migrated_state_sha256": sha256_bytes(state_path.read_bytes()),
        "status": "pass",
    }


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--binary", type=Path, required=True)
    parser.add_argument("--go", default=shutil.which("go"))
    parser.add_argument("--artifacts", type=Path, required=True)
    args = parser.parse_args()
    current = args.binary.resolve(strict=True)
    check(args.go, "go is required")
    args.artifacts.mkdir(parents=True, exist_ok=False)
    rows = []
    for tag, version, schema in RELEASES:
        row = release_flow(
            current, args.go, tag, version, schema, args.artifacts)
        rows.append(row)
        print(json.dumps(row), flush=True)
    report = {
        "schema_version": 1,
        "current_binary_sha256": sha256_bytes(current.read_bytes()),
        "isolation": "all release builds, client homes and states are disposable",
        "releases": rows,
    }
    write_json(args.artifacts / "report.json", report)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

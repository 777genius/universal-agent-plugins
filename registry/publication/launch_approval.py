#!/usr/bin/env python3
"""Validate the immutable Git launch-approval marker and its ledger lineage."""

from __future__ import annotations

import argparse
import json
import re
import subprocess
from pathlib import Path


SHA_RE = re.compile(r"[0-9a-f]{40}")
GIT = "/usr/bin/git"
EXPECTED_CONTRACT_KEYS = {
    "approval_environment",
    "contract_version",
    "launch_sequence",
    "launch_signing_key_id",
    "ledger_branch",
    "marker_ref",
    "repository",
    "schema_version",
    "sequence_tag_prefix",
}


class InvalidLaunchApproval(ValueError):
    pass


def require(condition: bool, message: str) -> None:
    if not condition:
        raise InvalidLaunchApproval(message)


def git(repo: Path, *args: str) -> str:
    completed = subprocess.run(
        [GIT, "-C", str(repo), *args],
        check=False,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
    )
    if completed.returncode:
        raise InvalidLaunchApproval(
            f"git {' '.join(args)} failed: {completed.stderr.strip()}"
        )
    return completed.stdout.strip()


def load_json_at(repo: Path, commit: str, relative: str) -> dict:
    raw = git(repo, "show", f"{commit}:{relative}")
    try:
        value = json.loads(raw)
    except json.JSONDecodeError as error:
        raise InvalidLaunchApproval(f"{relative} is not valid JSON") from error
    require(isinstance(value, dict), f"{relative} must be an object")
    return value


def load_contract(path: Path) -> dict:
    value = json.loads(path.read_bytes())
    require(isinstance(value, dict), "launch marker contract must be an object")
    require(set(value) == EXPECTED_CONTRACT_KEYS, "launch marker contract fields differ")
    require(value["contract_version"] == 1, "unsupported launch marker contract")
    require(value["launch_sequence"] == 1, "launch marker must bind sequence 1")
    require(value["schema_version"] == 1, "launch marker must bind schema 1")
    require(
        value["marker_ref"] == "refs/tags/directory-publication-schema-1-launch-approved",
        "unexpected launch marker ref",
    )
    require(
        value["sequence_tag_prefix"] == "directory-publication-schema-1-sequence-",
        "unexpected publication sequence tag prefix",
    )
    for key in ("approval_environment", "launch_signing_key_id", "ledger_branch", "repository"):
        require(isinstance(value[key], str) and value[key], f"invalid contract {key}")
    return value


def validate(
    *, repo: Path, contract_path: Path, repository: str, environment: str,
    current_commit: str,
    marker_commit: str | None = None, expected_publication_id: str | None = None,
    expected_snapshot_digest: str | None = None,
    expected_source_commit: str | None = None,
) -> str:
    contract = load_contract(contract_path)
    require(repository == contract["repository"], "repository differs from launch contract")
    require(environment == contract["approval_environment"],
            "environment differs from launch contract")
    require(SHA_RE.fullmatch(current_commit) is not None, "current ledger commit is invalid")
    require(git(repo, "rev-parse", f"{current_commit}^{{commit}}") == current_commit,
            "current ledger commit is not exact")

    if marker_commit is None:
        marker_commit = git(repo, "rev-parse", f"{contract['marker_ref']}^{{commit}}")
    require(SHA_RE.fullmatch(marker_commit) is not None, "launch marker commit is invalid")
    require(git(repo, "rev-parse", f"{marker_commit}^{{commit}}") == marker_commit,
            "launch marker does not target an exact commit")

    sequence_tag = (
        "refs/tags/" + contract["sequence_tag_prefix"]
        + f"{contract['launch_sequence']:020d}"
    )
    signed_commit = git(repo, "rev-parse", f"{sequence_tag}^{{commit}}")
    parents = git(repo, "show", "-s", "--format=%P", marker_commit).split()
    require(parents == [signed_commit], "launch marker target is not the sequence-1 materialization child")
    git(repo, "merge-base", "--is-ancestor", marker_commit, current_commit)
    require(
        git(repo, "diff", "--name-only", signed_commit, marker_commit, "--", "registry") == "",
        "launch marker target changes signed registry bytes",
    )

    ledger_path = "registry/schemas/1/ledger-contract.json"
    launch_ledger_contract = load_json_at(repo, marker_commit, ledger_path)
    current_ledger_contract = load_json_at(repo, current_commit, ledger_path)
    require(launch_ledger_contract == current_ledger_contract,
            "current ledger has a different bootstrap contract")
    require(launch_ledger_contract.get("schema_version") == contract["schema_version"],
            "ledger schema differs from launch contract")
    require(launch_ledger_contract.get("initial_sequence") == contract["launch_sequence"],
            "ledger initial sequence differs from launch contract")
    require(launch_ledger_contract.get("sequence_tag_prefix") == contract["sequence_tag_prefix"],
            "ledger sequence tag prefix differs from launch contract")
    require(SHA_RE.fullmatch(str(launch_ledger_contract.get("seed_commit", ""))) is not None,
            "ledger bootstrap seed is invalid")
    git(repo, "merge-base", "--is-ancestor", launch_ledger_contract["seed_commit"], signed_commit)

    sequence_name = f"{contract['launch_sequence']:020d}"
    latest = load_json_at(repo, marker_commit, "registry/schemas/1/latest.json")
    require(latest.get("sequence") == contract["launch_sequence"],
            "launch marker target is not materialized sequence 1")
    require(latest.get("snapshot_path") == f"snapshots/{sequence_name}.json",
            "launch snapshot path differs")
    require(latest.get("envelope_path") == f"snapshots/{sequence_name}.envelope.json",
            "launch envelope path differs")
    snapshot = load_json_at(repo, marker_commit, f"registry/schemas/1/{latest['snapshot_path']}")
    envelope = load_json_at(repo, marker_commit, f"registry/schemas/1/{latest['envelope_path']}")
    require(snapshot.get("sequence") == contract["launch_sequence"], "launch snapshot sequence differs")
    require(envelope.get("sequence") == contract["launch_sequence"], "launch envelope sequence differs")
    require(envelope.get("key_id") == contract["launch_signing_key_id"],
            "launch signing key differs from approved lineage")
    if expected_publication_id is not None:
        require(snapshot.get("publication_id") == expected_publication_id,
                "launch publication ID differs from ceremony")
    if expected_snapshot_digest is not None:
        require(envelope.get("snapshot_digest") == expected_snapshot_digest,
                "launch snapshot digest differs from ceremony")
    if expected_source_commit is not None:
        require(snapshot.get("source_commit") == expected_source_commit,
                "launch source commit differs from ceremony")
    return marker_commit


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--repo", required=True, type=Path)
    parser.add_argument("--contract", required=True, type=Path)
    parser.add_argument("--repository", required=True)
    parser.add_argument("--environment", required=True)
    parser.add_argument("--current-ledger-commit", required=True)
    parser.add_argument("--marker-commit")
    parser.add_argument("--expected-publication-id")
    parser.add_argument("--expected-snapshot-digest")
    parser.add_argument("--expected-source-commit")
    args = parser.parse_args()
    try:
        marker = validate(
            repo=args.repo,
            contract_path=args.contract,
            repository=args.repository,
            environment=args.environment,
            current_commit=args.current_ledger_commit,
            marker_commit=args.marker_commit,
            expected_publication_id=args.expected_publication_id,
            expected_snapshot_digest=args.expected_snapshot_digest,
            expected_source_commit=args.expected_source_commit,
        )
    except (InvalidLaunchApproval, OSError, json.JSONDecodeError) as error:
        parser.error(str(error))
    print(marker)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

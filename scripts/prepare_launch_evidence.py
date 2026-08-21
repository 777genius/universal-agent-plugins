#!/usr/bin/env python3
"""Resolve official release and an immutable staged Directory identity for one run."""

from __future__ import annotations

import argparse
import json
import os
import tempfile
from pathlib import Path

from run_launch_evidence_e2e import (
    fetch_staged_directory,
    make_challenge,
    read_production_config,
    resolve_github_release,
    resolve_npm_package,
    sha256_file,
)


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--asset-name", required=True, help="one manifest-listed asset used to authenticate the release")
    parser.add_argument("--npm-facade", action="store_true", help="also resolve the exact npm facade matching the release")
    parser.add_argument("--run-root", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--publication-id", required=True)
    parser.add_argument("--publication-sequence", type=int, required=True)
    parser.add_argument("--publication-snapshot-digest", required=True)
    parser.add_argument("--publication-source-commit", required=True)
    parser.add_argument("--publication-ledger-commit", required=True)
    args = parser.parse_args()
    if args.run_root.exists():
        raise ValueError("prepared run root must not exist")
    args.run_root.mkdir(parents=True)
    config = read_production_config()
    catalog_repository = os.environ.get("GITHUB_REPOSITORY")
    if catalog_repository != config["catalog_repository"]:
        raise ValueError("workflow repository does not match checked-in catalog repository")
    cli_repository = config["cli_release_repository"]
    release_tag = config["cli_release_tag"]
    asset, manifest, release_digest = resolve_github_release(
        cli_repository, release_tag, args.run_root / "release" / args.asset_name,
        asset_name=args.asset_name, token=None,
    )
    directory_env, snapshot, directory_digest = fetch_staged_directory(
        args.run_root / "directory", repository=catalog_repository,
        ledger_commit=args.publication_ledger_commit,
        expected_publication_id=args.publication_id,
        expected_sequence=args.publication_sequence,
        expected_snapshot_digest=args.publication_snapshot_digest,
        expected_source_commit=args.publication_source_commit,
    )
    npm_package = None
    if args.npm_facade:
        _, npm_package = resolve_npm_package(
            "universal-agent-plugins", manifest["version"],
            args.run_root / "npm" / f"universal-agent-plugins-{manifest['version']}.tgz",
        )
    challenge = make_challenge(
        os.environ["GITHUB_SHA"], os.environ["GITHUB_RUN_ID"], os.environ["GITHUB_RUN_ATTEMPT"],
        release_digest, directory_digest, args.run_root,
    )
    value = {
        "schema_version": 1, "catalog_repository": catalog_repository,
        "cli_release_repository": cli_repository, "cli_release_tag": release_tag,
        "release_manifest": manifest, "release_manifest_digest": release_digest,
        "release_checksums_digest": sha256_file(args.run_root / "release" / "checksums.txt"),
        "github_release_identity": json.loads((args.run_root / "release" / "github-release-identity.json").read_text()),
        "authenticated_asset": {"name": args.asset_name, "digest": sha256_file(asset)},
        "github_asset_attestation": json.loads((args.run_root / "release" / f"{args.asset_name}.attestation.json").read_text()),
        "directory": {"origin": directory_env["AGENTPLUGINS_DIRECTORY_ORIGIN"], "snapshot": "directory/snapshot.json", "envelope": "directory/envelope.json", "digest": directory_digest, "sequence": snapshot["sequence"], "publication_id": snapshot["publication_id"], "source_commit": snapshot["source_commit"], "ledger_commit": args.publication_ledger_commit},
        "github": {"sha": os.environ["GITHUB_SHA"], "run_id": os.environ["GITHUB_RUN_ID"], "run_attempt": os.environ["GITHUB_RUN_ATTEMPT"]},
        "challenge": challenge,
    }
    if npm_package is not None:
        value["npm_package"] = npm_package
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(value, indent=2, sort_keys=True) + "\n")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

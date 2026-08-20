#!/usr/bin/env python3
"""Validate, sequence, and Ed25519-sign one canonical Directory candidate."""

from __future__ import annotations

import argparse
import base64
import copy
import os
import sys
from datetime import timedelta
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
from directory_publication import (
    CANDIDATE_SCHEMA,
    MAX_CANDIDATE_BYTES,
    MAX_SNAPSHOT_BYTES,
    PublicationError,
    atomic_write,
    candidate_digest,
    canonical_json,
    ed25519_private_key,
    format_timestamp,
    load_ledger_latest,
    load_public_keys,
    parse_json_bytes,
    parse_timestamp,
    read_json,
    read_bytes_bounded,
    require,
    sha256_digest,
    signature_message,
    validate_latest,
    validate_snapshot_semantics,
    validate_with_schema,
)


def public_bytes(private_key) -> bytes:  # type: ignore[no-untyped-def]
    from cryptography.hazmat.primitives import serialization
    return private_key.public_key().public_bytes(
        encoding=serialization.Encoding.Raw,
        format=serialization.PublicFormat.Raw,
    )


def assign_release_publication_times(
    distributions: list[dict[str, object]], previous: dict[str, object] | None, now: str
) -> list[dict[str, object]]:
    """Assign new-release times while preserving signed historical provenance."""
    prior: dict[tuple[str, int], str] = {}
    if previous is not None:
        for distribution in previous["distributions"]:  # type: ignore[index]
            for release in distribution["releases"]:
                prior[(distribution["id"], release["sequence"])] = release["published_at"]
    assigned = copy.deepcopy(distributions)
    for distribution in assigned:
        for release in distribution["releases"]:  # type: ignore[index]
            identity = (distribution["id"], release["sequence"])
            if release["published_at"] is None:
                release["published_at"] = prior.get(identity, now)
            elif identity in prior:
                require(
                    release["published_at"] == prior[identity],
                    f"published release {identity} timestamp changed in candidate",
                )
    return assigned


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--candidate", type=Path, required=True)
    parser.add_argument("--candidate-digest", required=True)
    parser.add_argument("--ledger", type=Path, required=True)
    parser.add_argument("--trusted-keys", type=Path, required=True)
    parser.add_argument("--key-id", required=True)
    parser.add_argument("--now", required=True)
    parser.add_argument("--result", type=Path, required=True)
    args = parser.parse_args()
    try:
        candidate_body = read_bytes_bounded(args.candidate, MAX_CANDIDATE_BYTES)
        candidate = parse_json_bytes(candidate_body, str(args.candidate), max_bytes=4 << 20)
        require(isinstance(candidate, dict), "candidate must be an object")
        validate_with_schema(candidate, CANDIDATE_SCHEMA)
        require(canonical_json(candidate) == candidate_body, "candidate is not canonical JSON")
        require(candidate_digest(candidate_body) == args.candidate_digest, "candidate digest mismatch")
        now = parse_timestamp(args.now, "now")
        trusted = load_public_keys(args.trusted_keys)
        require(args.key_id in trusted, f"active key ID {args.key_id!r} is not trusted")
        encoded_private = os.environ.get("DIRECTORY_ED25519_PRIVATE_KEY")
        require(encoded_private is not None, "DIRECTORY_ED25519_PRIVATE_KEY is not set")
        private_key = ed25519_private_key(encoded_private)
        require(public_bytes(private_key) == trusted[args.key_id], "private key does not match active trusted key ID")

        loaded = load_ledger_latest(args.ledger, trusted)
        previous = loaded[0] if loaded else None
        historical_evidence = loaded[2] if loaded else {}
        distributions = assign_release_publication_times(
            candidate["distributions"], previous, format_timestamp(now)
        )
        if previous is not None and previous["publication_id"] == candidate["publication_id"]:
            require(previous["source_commit"] == candidate["source_commit"], "publication ID was reused for another source commit")
            require(previous["products"] == candidate["products"] and previous["distributions"] == distributions and previous["evidence"] == candidate["evidence"] and previous["revocations"] == candidate["revocations"], "publication ID was reused for different candidate content")
            result = {"reused": True, "sequence": previous["sequence"], "snapshot_digest": sha256_digest(canonical_json(previous))}
            atomic_write(args.result, canonical_json(result))
            print(f"reused sequence {previous['sequence']}")
            return 0

        sequence = 1 if previous is None else previous["sequence"] + 1
        snapshot = {
            "snapshot_schema_version": candidate["snapshot_schema_version"],
            "sequence": sequence,
            "publication_id": candidate["publication_id"],
            "source_commit": candidate["source_commit"],
            "generated_at": format_timestamp(now),
            "expires_at": format_timestamp(now + timedelta(days=candidate["lifetime_days"])),
            "products": candidate["products"],
            "distributions": distributions,
            "evidence": candidate["evidence"],
            "revocations": candidate["revocations"],
        }
        validate_snapshot_semantics(snapshot, previous, historical_evidence)
        snapshot_body = canonical_json(snapshot)
        require(len(snapshot_body) <= MAX_SNAPSHOT_BYTES, "generated snapshot exceeds response size contract")
        snapshot_digest = sha256_digest(snapshot_body)
        signature = private_key.sign(signature_message(snapshot_body))
        envelope = {
            "envelope_schema_version": 1,
            "snapshot_schema_version": 1,
            "sequence": sequence,
            "key_id": args.key_id,
            "algorithm": "Ed25519",
            "signature_domain": "UAP-DIRECTORY-SNAPSHOT-ED25519-V1",
            "snapshot_digest": snapshot_digest,
            "signature": base64.b64encode(signature).decode("ascii"),
        }
        latest = {
            "pointer_schema_version": 1,
            "snapshot_schema_version": 1,
            "sequence": sequence,
            "snapshot_path": f"snapshots/{sequence:020d}.json",
            "envelope_path": f"snapshots/{sequence:020d}.envelope.json",
            "fetch_contract": {
                "https_required": True,
                "same_origin_redirects_only": True,
                "forward_credentials_on_redirect": False,
                "max_redirects": 2,
                "latest_max_bytes": 16384,
                "snapshot_max_bytes": 4194304,
                "envelope_max_bytes": 16384,
                "retry_attempts": 3,
            },
        }
        validate_with_schema(envelope, Path(__file__).resolve().parents[1] / "schemas" / "directory-envelope.schema.json")
        validate_latest(latest)
        feed = args.ledger / "registry" / "schemas" / "1"
        snapshot_path = feed / latest["snapshot_path"]
        envelope_path = feed / latest["envelope_path"]
        require(not snapshot_path.exists() and not envelope_path.exists(), f"sequence {sequence} artifact already exists")
        atomic_write(snapshot_path, snapshot_body)
        atomic_write(envelope_path, canonical_json(envelope))
        atomic_write(feed / "latest.json", canonical_json(latest))
        result = {"reused": False, "sequence": sequence, "snapshot_digest": snapshot_digest}
        atomic_write(args.result, canonical_json(result))
        print(f"published sequence {sequence} {snapshot_digest}")
        return 0
    except (OSError, PublicationError, KeyError, TypeError, ValueError) as error:
        print(f"sign-directory-publication: {error}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())

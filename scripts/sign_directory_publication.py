#!/usr/bin/env python3
"""Validate, sequence, and Ed25519-sign one canonical Directory candidate."""

from __future__ import annotations

import argparse
import base64
import copy
import os
import subprocess
import sys
from datetime import timedelta
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
from directory_publication import (
    MAX_CANDIDATE_BYTES,
    MAX_SNAPSHOT_BYTES,
    LEDGER_CONTRACT_NAME,
    PublicationError,
    atomic_write,
    candidate_digest,
    canonical_json,
    ed25519_private_key,
    ed25519_public_bytes,
    ed25519_sign,
    format_timestamp,
    load_ledger_latest,
    load_public_keys,
    parse_json_bytes,
    parse_timestamp,
    read_json,
    read_bytes_bounded,
    require,
    require_integer_const,
    sha256_digest,
    signature_message,
    validate_envelope_contract,
    validate_directory_records,
    validate_latest,
    validate_snapshot_semantics,
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


def validate_bound_candidate(candidate: dict[str, object]) -> None:
    """Validate the complete signer-facing schema using only the standard library."""
    require(set(candidate) == {
        "candidate_schema_version", "snapshot_schema_version", "publication_id",
        "source_commit", "lifetime_days", "products", "distributions", "evidence",
        "revocations",
    }, "candidate fields are invalid")
    require_integer_const(candidate["candidate_schema_version"], 1, "candidate schema version is invalid")
    require_integer_const(candidate["snapshot_schema_version"], 1, "candidate snapshot schema version is invalid")
    publication_id = candidate["publication_id"]
    require(
        isinstance(publication_id, str) and 1 <= len(publication_id) <= 128
        and publication_id[0].isalnum()
        and all(character.isascii() and (character.isalnum() or character in "._-") for character in publication_id),
        "candidate publication ID is invalid",
    )
    source_commit = candidate["source_commit"]
    require(isinstance(source_commit, str) and len(source_commit) == 40 and all(character in "0123456789abcdef" for character in source_commit), "candidate source commit is invalid")
    lifetime_days = candidate["lifetime_days"]
    require(type(lifetime_days) is int and 1 <= lifetime_days <= 30, "candidate lifetime is invalid")
    for field in ("products", "distributions", "evidence", "revocations"):
        require(isinstance(candidate[field], list), f"candidate {field} must be an array")
    validate_directory_records(candidate, snapshot=False)


def verify_sequence_one_transaction(ledger: Path, seed_commit: str) -> None:
    """Bind an initialization retry to its immutable sequence-1 transaction."""
    git = "/usr/bin/git"
    tag = "refs/tags/directory-publication-schema-1-sequence-00000000000000000001"

    def run(*arguments: str, capture: bool = False) -> str:
        try:
            completed = subprocess.run(
                [git, "-C", str(ledger), *arguments], check=False,
                stdout=subprocess.PIPE if capture else subprocess.DEVNULL,
                stderr=subprocess.PIPE, text=True,
            )
        except OSError as error:
            raise PublicationError(f"cannot inspect sequence-1 git transaction: {error}") from error
        require(completed.returncode == 0, "sequence-1 tag/transaction identity is invalid")
        return completed.stdout.strip() if capture else ""

    require(Path(git).is_file(), f"reviewed signer runtime is missing: {git}")
    tag_commit = run("rev-parse", f"{tag}^{{commit}}", capture=True)
    head_commit = run("rev-parse", "HEAD", capture=True)
    require(len(tag_commit) == 40 and len(head_commit) == 40, "sequence-1 git identity is invalid")
    run("merge-base", "--is-ancestor", tag_commit, head_commit)
    parents = run("show", "-s", "--format=%P", tag_commit, capture=True).split()
    require(parents == [seed_commit], "sequence-1 publication commit is not the exact seed transaction")
    changed = run("diff", "--name-only", seed_commit, tag_commit, capture=True).splitlines()
    require(sorted(changed) == sorted([
        "registry/schemas/1/latest.json",
        "registry/schemas/1/ledger-contract.json",
        "registry/schemas/1/snapshots/00000000000000000001.envelope.json",
        "registry/schemas/1/snapshots/00000000000000000001.json",
    ]), "sequence-1 publication transaction changed unexpected paths")
    run("diff", "--quiet", tag_commit, head_commit, "--", "registry/schemas/1")
    run("diff", "--quiet", "HEAD", "--", "registry/schemas/1")


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--candidate", type=Path, required=True)
    parser.add_argument("--candidate-digest", required=True)
    parser.add_argument("--ledger", type=Path, required=True)
    parser.add_argument("--trusted-keys", type=Path, required=True)
    parser.add_argument("--key-id", required=True)
    parser.add_argument("--now", required=True)
    parser.add_argument("--result", type=Path, required=True)
    parser.add_argument("--initialize-ledger", action="store_true")
    parser.add_argument("--ledger-seed-commit")
    parser.add_argument("--ledger-sequence-floor", type=int)
    args = parser.parse_args()
    try:
        candidate_body = read_bytes_bounded(args.candidate, MAX_CANDIDATE_BYTES)
        candidate = parse_json_bytes(candidate_body, str(args.candidate), max_bytes=4 << 20)
        require(isinstance(candidate, dict), "candidate must be an object")
        validate_bound_candidate(candidate)
        require(canonical_json(candidate) == candidate_body, "candidate is not canonical JSON")
        require(candidate_digest(candidate_body) == args.candidate_digest, "candidate digest mismatch")
        now = parse_timestamp(args.now, "now")
        trusted = load_public_keys(args.trusted_keys)
        require(args.key_id in trusted, f"active key ID {args.key_id!r} is not trusted")
        encoded_private = os.environ.get("DIRECTORY_ED25519_PRIVATE_KEY")
        require(encoded_private is not None, "DIRECTORY_ED25519_PRIVATE_KEY is not set")
        private_key = ed25519_private_key(encoded_private)
        require(ed25519_public_bytes(private_key) == trusted[args.key_id], "private key does not match active trusted key ID")

        loaded = load_ledger_latest(
            args.ledger, trusted, allow_initialization=args.initialize_ledger,
            seed_commit=args.ledger_seed_commit, minimum_sequence=args.ledger_sequence_floor,
            validate_schema=False, require_external_floor=True,
        )
        if args.initialize_ledger and loaded is not None:
            require(args.ledger_seed_commit is not None, "initialization requires an exact seed commit")
            verify_sequence_one_transaction(args.ledger, args.ledger_seed_commit)
        previous = loaded[0] if loaded else None
        historical_evidence = loaded[2] if loaded else {}
        publications = loaded[3] if loaded else {}
        distributions = assign_release_publication_times(
            candidate["distributions"], previous, format_timestamp(now)
        )
        matches = publications.get(candidate["publication_id"], [])
        if matches:
            require(len(matches) == 1, "publication ID occurs more than once in the ledger")
            matched, matched_envelope = matches[0]
            require(previous is matched, "publication ID belongs to a historical publication, not current latest")
            require(matched["source_commit"] == candidate["source_commit"], "publication ID was reused for another source commit")
            require(matched_envelope["key_id"] == args.key_id, "publication ID was reused with another signer key")
            require(
                parse_timestamp(matched["expires_at"], "expires_at") - parse_timestamp(matched["generated_at"], "generated_at")
                == timedelta(days=candidate["lifetime_days"]),
                "publication ID was reused with another lifetime",
            )
            require(matched["products"] == candidate["products"] and matched["distributions"] == distributions and matched["evidence"] == candidate["evidence"] and matched["revocations"] == candidate["revocations"], "publication ID was reused for different candidate content")
            result = {"reused": True, "sequence": matched["sequence"], "snapshot_digest": sha256_digest(canonical_json(matched))}
            atomic_write(args.result, canonical_json(result))
            print(f"reused sequence {matched['sequence']}")
            return 0
        if previous is not None and candidate["publication_id"].isdigit() and previous["publication_id"].isdigit():
            require(int(candidate["publication_id"]) > int(previous["publication_id"]), "publication run ID is older than current latest")
        require(not args.initialize_ledger or previous is None, "initialization rerun publication ID differs from the original sequence 1 publication")

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
        validate_snapshot_semantics(snapshot, previous, historical_evidence, validate_schema=False)
        snapshot_body = canonical_json(snapshot)
        require(len(snapshot_body) <= MAX_SNAPSHOT_BYTES, "generated snapshot exceeds response size contract")
        snapshot_digest = sha256_digest(snapshot_body)
        signature = ed25519_sign(private_key, signature_message(snapshot_body))
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
        validate_envelope_contract(envelope)
        validate_latest(latest, validate_schema=False)
        feed = args.ledger / "registry" / "schemas" / "1"
        if previous is None:
            contract = {
                "contract_version": 1,
                "initial_sequence": 1,
                "schema_version": 1,
                "seed_commit": args.ledger_seed_commit,
                "sequence_tag_prefix": "directory-publication-schema-1-sequence-",
            }
            atomic_write(feed / LEDGER_CONTRACT_NAME, canonical_json(contract))
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

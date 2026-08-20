#!/usr/bin/env python3
"""Verify a client-consumable Directory pointer, envelope, and snapshot."""

from __future__ import annotations

import argparse
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
from directory_publication import (
    MAX_ENVELOPE_BYTES,
    MAX_LATEST_BYTES,
    MAX_SNAPSHOT_BYTES,
    PublicationError,
    canonical_json,
    load_public_keys,
    parse_json_bytes,
    parse_timestamp,
    read_bytes_bounded,
    read_json,
    require,
    validate_latest,
    validate_snapshot_semantics,
    verify_envelope,
)


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--feed", type=Path, required=True, help="registry/schemas/1 directory")
    parser.add_argument("--trusted-keys", type=Path, required=True)
    parser.add_argument("--now", required=True)
    parser.add_argument("--minimum-sequence", type=int, default=1)
    parser.add_argument("--allow-expired-ledger", action="store_true", help="publisher recovery only; never client eligibility")
    args = parser.parse_args()
    try:
        latest_body = read_bytes_bounded(args.feed / "latest.json", MAX_LATEST_BYTES)
        latest = parse_json_bytes(latest_body, "latest pointer", max_bytes=MAX_LATEST_BYTES)
        require(isinstance(latest, dict), "latest pointer must be an object")
        require(canonical_json(latest) == latest_body, "latest pointer is not canonical JSON")
        validate_latest(latest)
        snapshot_body = read_bytes_bounded(
            args.feed / latest["snapshot_path"],
            latest["fetch_contract"]["snapshot_max_bytes"],
        )
        require(len(snapshot_body) <= latest["fetch_contract"]["snapshot_max_bytes"] <= MAX_SNAPSHOT_BYTES, "snapshot response exceeds contract")
        envelope_body = read_bytes_bounded(
            args.feed / latest["envelope_path"],
            latest["fetch_contract"]["envelope_max_bytes"],
        )
        envelope = parse_json_bytes(envelope_body, "signature envelope", max_bytes=MAX_ENVELOPE_BYTES)
        require(isinstance(envelope, dict), "envelope must be an object")
        require(len(envelope_body) <= latest["fetch_contract"]["envelope_max_bytes"], "envelope response exceeds contract")
        require(canonical_json(envelope) == envelope_body, "signature envelope is not canonical JSON")
        verify_envelope(snapshot_body, envelope, load_public_keys(args.trusted_keys))
        snapshot = parse_json_bytes(snapshot_body, "snapshot", max_bytes=MAX_SNAPSHOT_BYTES)
        require(isinstance(snapshot, dict), "snapshot must be an object")
        validate_snapshot_semantics(snapshot)
        require(snapshot["sequence"] == envelope["sequence"] == latest["sequence"], "artifact sequence mismatch")
        require(snapshot["sequence"] >= args.minimum_sequence, f"snapshot sequence {snapshot['sequence']} is below local floor {args.minimum_sequence}")
        now = parse_timestamp(args.now, "now")
        generated = parse_timestamp(snapshot["generated_at"], "generated_at")
        expires = parse_timestamp(snapshot["expires_at"], "expires_at")
        require(now >= generated, "snapshot is not yet valid; check the local clock")
        if not args.allow_expired_ledger:
            require(now < expires, "snapshot is expired; short-name resolution is unavailable")
        print(f"valid directory snapshot sequence {snapshot['sequence']}")
        return 0
    except (OSError, PublicationError, KeyError, TypeError) as error:
        print(f"verify-directory-publication: {error}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())

#!/usr/bin/env python3
"""Observe the exact signed publication at production after deployment."""

from __future__ import annotations

import argparse
import json
import tempfile
import time
from pathlib import Path

from run_launch_evidence_e2e import FULL_SHA, fetch_production_directory


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--publication-id", required=True)
    parser.add_argument("--publication-sequence", type=int, required=True)
    parser.add_argument("--publication-snapshot-digest", required=True)
    parser.add_argument("--publication-source-commit", required=True)
    parser.add_argument("--publication-ledger-commit", required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    if not FULL_SHA.fullmatch(args.publication_ledger_commit):
        raise ValueError("production observation ledger commit is invalid")
    last_error: ValueError | None = None
    for attempt in range(1, 4):
        try:
            with tempfile.TemporaryDirectory(prefix="uap-production-latest-") as tmp:
                environment, snapshot, digest = fetch_production_directory(
                    Path(tmp) / "directory",
                    expected_publication_id=args.publication_id,
                    expected_sequence=args.publication_sequence,
                    expected_snapshot_digest=args.publication_snapshot_digest,
                    expected_source_commit=args.publication_source_commit,
                )
            break
        except ValueError as error:
            last_error = error
            if attempt == 3:
                raise
            time.sleep(5)
    else:  # pragma: no cover - the bounded loop either succeeds or raises
        raise last_error or ValueError("production latest observation failed")
    value = {
        "schema_version": 1,
        "origin": environment["AGENTPLUGINS_DIRECTORY_ORIGIN"],
        "publication_id": snapshot["publication_id"],
        "sequence": snapshot["sequence"],
        "snapshot_digest": digest,
        "source_commit": snapshot["source_commit"],
        "ledger_commit": args.publication_ledger_commit,
    }
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(value, indent=2, sort_keys=True) + "\n")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

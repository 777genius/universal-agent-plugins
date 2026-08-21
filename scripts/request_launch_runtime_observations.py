#!/usr/bin/env python3
"""Request challenge-bound live observations from the protected OAuth/runtime observer."""

from __future__ import annotations

import argparse
import json
import os
from pathlib import Path
from urllib.parse import urlsplit
from urllib.request import Request, urlopen

from launch_observer_signatures import verify_observer_bundle


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--endpoint", required=True)
    parser.add_argument("--context", type=Path, required=True)
    parser.add_argument("--output-directory", type=Path, required=True)
    args = parser.parse_args()
    parsed = urlsplit(args.endpoint)
    if parsed.scheme != "https" or not parsed.hostname or parsed.username or parsed.password or parsed.query or parsed.fragment:
        raise ValueError("protected observer endpoint must be credential-free HTTPS")
    oidc = os.environ.get("ACTIONS_ID_TOKEN")
    if not oidc:
        raise ValueError("GitHub OIDC identity is required for the protected observer")
    context = json.loads(args.context.read_text())
    request_body = json.dumps({
        "schema_version": 1, "purpose": "stable-launch-e2e",
        "catalog_repository": context["catalog_repository"],
        "cli_release_repository": context["cli_release_repository"],
        "cli_release_tag": context["cli_release_tag"],
        "release_manifest_digest": context["release_manifest_digest"],
        "release_checksums_digest": context["release_checksums_digest"],
        "directory_digest": context["directory"]["digest"],
        "github": context["github"], "challenge": context["challenge"],
    }, sort_keys=True, separators=(",", ":")).encode()
    request = Request(args.endpoint, data=request_body, method="POST", headers={
        "Authorization": "Bearer " + oidc, "Content-Type": "application/json",
        "Accept": "application/json", "User-Agent": "uap-stable-launch-evidence/1",
    })
    with urlopen(request, timeout=900) as response:
        body = response.read((8 << 20) + 1)
    if len(body) > 8 << 20:
        raise ValueError("protected observer response exceeds size bound")
    value = json.loads(body)
    public_key = os.environ.get("OBSERVER_ED25519_PUBLIC_KEY", "")
    key_id = os.environ.get("OBSERVER_KEY_ID", "")
    if not public_key or not key_id:
        raise ValueError("an explicit protected observer Ed25519 trust key is required")
    artifacts = verify_observer_bundle(
        value, challenge=context["challenge"]["value"],
        public_key_base64=public_key, expected_key_id=key_id,
    )
    required = set(artifacts)
    args.output_directory.mkdir(parents=True, exist_ok=False)
    (args.output_directory / "observer-bundle.json").write_text(json.dumps(value, indent=2, sort_keys=True) + "\n")
    for name in sorted(required):
        target = args.output_directory / name
        target.write_text(json.dumps(artifacts[name], indent=2, sort_keys=True) + "\n")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

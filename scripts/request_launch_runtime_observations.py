#!/usr/bin/env python3
"""Request challenge-bound live observations from the protected OAuth/runtime observer."""

from __future__ import annotations

import argparse
import json
import os
import time
import urllib.request
from pathlib import Path
from urllib.parse import urlsplit

from launch_observer_signatures import verify_observer_bundle


MAX_RESPONSE_BYTES = 8 << 20
REQUEST_TIMEOUT_SECONDS = 900
RESPONSE_TOTAL_SECONDS = 900


class ObserverTransportError(ValueError):
    """A sanitized failure while contacting the protected observer."""


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):  # type: ignore[no-untyped-def]
        raise ObserverTransportError("protected observer redirects are forbidden")


def validate_endpoint(endpoint: str) -> None:
    try:
        parsed = urlsplit(endpoint)
        port = parsed.port
    except (TypeError, ValueError):
        raise ObserverTransportError(
            "protected observer endpoint must be credential-free HTTPS"
        ) from None
    if (
        parsed.scheme != "https"
        or not parsed.hostname
        or parsed.username
        or parsed.password
        or parsed.query
        or parsed.fragment
        or (port is not None and not 1 <= port <= 65535)
    ):
        raise ObserverTransportError(
            "protected observer endpoint must be credential-free HTTPS"
        )


def request_observer_bundle(
    endpoint: str, request_body: bytes, oidc: str, opener=None  # type: ignore[no-untyped-def]
) -> object:
    validate_endpoint(endpoint)
    opener = opener or urllib.request.build_opener(NoRedirect())
    request = urllib.request.Request(
        endpoint,
        data=request_body,
        method="POST",
        headers={
            "Authorization": "Bearer " + oidc,
            "Content-Type": "application/json",
            "Accept": "application/json",
            "User-Agent": "uap-stable-launch-evidence/1",
        },
    )
    started = time.monotonic()
    try:
        response = opener.open(request, timeout=REQUEST_TIMEOUT_SECONDS)
        with response:
            if response.status != 200 or response.geturl() != endpoint:
                raise ObserverTransportError(
                    "protected observer request failed closed"
                )
            length = response.headers.get("Content-Length")
            if length is not None and (
                not isinstance(length, str)
                or not length.isascii()
                or not length.isdigit()
                or int(length) > MAX_RESPONSE_BYTES
            ):
                raise ObserverTransportError(
                    "protected observer response exceeds size bound"
                )
            chunks = []
            total = 0
            while True:
                if time.monotonic() - started > RESPONSE_TOTAL_SECONDS:
                    raise ObserverTransportError(
                        "protected observer response exceeded time bound"
                    )
                chunk = response.read(min(64 << 10, MAX_RESPONSE_BYTES - total + 1))
                if not chunk:
                    break
                if not isinstance(chunk, bytes):
                    raise ObserverTransportError(
                        "protected observer request failed closed"
                    )
                total += len(chunk)
                if total > MAX_RESPONSE_BYTES:
                    raise ObserverTransportError(
                        "protected observer response exceeds size bound"
                    )
                chunks.append(chunk)
    except ObserverTransportError:
        raise
    except Exception:
        # Do not expose remote error text, URLs, response bodies, or credentials.
        raise ObserverTransportError("protected observer request failed closed") from None
    try:
        return json.loads(b"".join(chunks))
    except (TypeError, ValueError, UnicodeError):
        raise ObserverTransportError(
            "protected observer returned invalid JSON"
        ) from None


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--endpoint", required=True)
    parser.add_argument("--context", type=Path, required=True)
    parser.add_argument("--output-directory", type=Path, required=True)
    args = parser.parse_args()
    validate_endpoint(args.endpoint)
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
    value = request_observer_bundle(args.endpoint, request_body, oidc)
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

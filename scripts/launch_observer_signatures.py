#!/usr/bin/env python3
"""Verify challenge-bound responses from the protected launch observer."""

from __future__ import annotations

import base64
import binascii
import json
import re
from datetime import datetime, timedelta, timezone
from typing import Any


SIGNATURE_DOMAIN = b"UAP-STABLE-LAUNCH-OBSERVER-BUNDLE-V1\0"
MAX_BUNDLE_AGE = timedelta(minutes=30)
FORBIDDEN_KEY = re.compile(r"(?i)(?:token|secret|password|cookie|authorization|oauth[_-]?(?:code|state))")
ABSOLUTE_PATH = re.compile(r"^(?:/(?!/)|[A-Za-z]:\\)")


def canonical_json(value: Any) -> bytes:
    return json.dumps(value, ensure_ascii=False, sort_keys=True, separators=(",", ":")).encode()


def signed_payload(bundle: dict[str, Any]) -> bytes:
    return SIGNATURE_DOMAIN + canonical_json({key: value for key, value in bundle.items() if key != "signature"})


def assert_bundle_redacted(value: Any) -> None:
    if isinstance(value, dict):
        for key, child in value.items():
            if FORBIDDEN_KEY.search(key):
                raise ValueError("protected observer bundle contains a credential-like field")
            assert_bundle_redacted(child)
    elif isinstance(value, list):
        for child in value:
            assert_bundle_redacted(child)
    elif isinstance(value, str):
        if ABSOLUTE_PATH.match(value):
            raise ValueError("protected observer bundle contains an absolute local path")
        if re.search(r"(?i)\bBearer\s+\S+", value) or re.search(r"https://[^/@\s]+:[^/@\s]+@", value):
            raise ValueError("protected observer bundle contains credential material")


def verify_observer_bundle(
    bundle: dict[str, Any], *, challenge: str, public_key_base64: str,
    expected_key_id: str, now: datetime | None = None,
) -> dict[str, Any]:
    """Return signed artifacts after strict Ed25519 and freshness validation."""
    required = {"schema_version", "challenge", "signed_at", "key_id", "artifacts", "signature"}
    if set(bundle) != required or bundle.get("schema_version") != 1:
        raise ValueError("protected observer returned a non-canonical signed bundle")
    if bundle.get("challenge") != challenge:
        raise ValueError("protected observer bundle is not correlated to this challenge")
    if bundle.get("key_id") != expected_key_id or not expected_key_id:
        raise ValueError("protected observer bundle key is not explicitly trusted")
    try:
        signed_at = datetime.fromisoformat(str(bundle["signed_at"]).replace("Z", "+00:00"))
    except (TypeError, ValueError) as error:
        raise ValueError("protected observer bundle timestamp is invalid") from error
    current = now or datetime.now(timezone.utc)
    if signed_at.tzinfo is None or signed_at > current + timedelta(minutes=2) or current - signed_at > MAX_BUNDLE_AGE:
        raise ValueError("protected observer bundle is stale or from the future")
    artifacts = bundle.get("artifacts")
    expected_artifacts = {
        "runtime-attestations.json", "notion-oauth-attestations.json",
        "chatgpt-cloudflare-attestation.json", "consent.json",
    }
    if not isinstance(artifacts, dict) or set(artifacts) != expected_artifacts:
        raise ValueError("protected observer bundle has a non-canonical artifact set")
    assert_bundle_redacted(artifacts)
    try:
        public_key = base64.b64decode(public_key_base64, validate=True)
        signature = base64.b64decode(str(bundle["signature"]), validate=True)
    except (binascii.Error, ValueError) as error:
        raise ValueError("protected observer Ed25519 material is not canonical base64") from error
    if len(public_key) != 32 or len(signature) != 64:
        raise ValueError("protected observer Ed25519 material has an invalid length")
    try:
        from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PublicKey

        Ed25519PublicKey.from_public_bytes(public_key).verify(signature, signed_payload(bundle))
    except ImportError as error:
        raise ValueError("cryptography is required to verify protected observer evidence") from error
    except Exception as error:
        if error.__class__.__module__.startswith("cryptography"):
            raise ValueError("protected observer bundle signature is invalid") from error
        raise
    return artifacts

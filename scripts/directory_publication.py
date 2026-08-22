#!/usr/bin/env python3
"""Security primitives and contracts for static Directory publication.

This module intentionally has no network client.  The preparation process reads
reviewed repository data, while the signer accepts only a bounded canonical
candidate and an already-existing publication ledger.
"""

from __future__ import annotations

import base64
import hashlib
import json
import os
import re
import subprocess
import tempfile
import unicodedata
from datetime import datetime, timezone
from pathlib import Path, PurePosixPath
from typing import Any


ROOT = Path(__file__).resolve().parents[1]
SNAPSHOT_SCHEMA = ROOT / "schemas" / "directory-snapshot.schema.json"
ENVELOPE_SCHEMA = ROOT / "schemas" / "directory-envelope.schema.json"
LATEST_SCHEMA = ROOT / "schemas" / "directory-latest.schema.json"
CANDIDATE_SCHEMA = ROOT / "schemas" / "directory-publication-candidate.schema.json"

SIGNATURE_DOMAIN = b"UAP-DIRECTORY-SNAPSHOT-ED25519-V1\x00"
CANDIDATE_DOMAIN = b"UAP-DIRECTORY-PUBLICATION-CANDIDATE-V1\x00"
DIGEST_RE = re.compile(r"^sha256:[0-9a-f]{64}$")
SHA_RE = re.compile(r"^[0-9a-f]{40}$")
ID_RE = re.compile(r"^[a-z0-9](?:[a-z0-9._-]*[a-z0-9])?$")
DIST_ID_RE = re.compile(r"^[a-z0-9](?:[a-z0-9-]*[a-z0-9])?/[a-z0-9](?:[a-z0-9._-]*[a-z0-9])?$")
MAX_CANDIDATE_BYTES = 4 << 20
MAX_SNAPSHOT_BYTES = 4 << 20
MAX_ENVELOPE_BYTES = 16 << 10
MAX_LATEST_BYTES = 16 << 10
MAX_LEDGER_SNAPSHOTS = 100_000
LEDGER_CONTRACT_NAME = "ledger-contract.json"
OPENSSL = "/usr/bin/openssl"
ED25519_PRIVATE_DER_PREFIX = bytes.fromhex("302e020100300506032b657004220420")
ED25519_PUBLIC_DER_PREFIX = bytes.fromhex("302a300506032b6570032100")


class PublicationError(Exception):
    """A fail-closed publication contract violation."""


def require(condition: bool, message: str) -> None:
    if not condition:
        raise PublicationError(message)


def require_integer_const(value: Any, expected: int, message: str) -> None:
    """Require an exact JSON integer constant (``bool`` is not an integer)."""
    require(type(value) is int and value == expected, message)


def parse_json_bytes(body: bytes, source: str, *, max_bytes: int) -> Any:
    require(len(body) <= max_bytes, f"{source}: exceeds {max_bytes} bytes")

    def object_pairs(pairs: list[tuple[str, Any]]) -> dict[str, Any]:
        result: dict[str, Any] = {}
        normalized: set[str] = set()
        for key, value in pairs:
            require(key not in result, f"{source}: duplicate JSON key {key!r}")
            folded = unicodedata.normalize("NFC", key).casefold()
            require(folded not in normalized, f"{source}: case/Unicode-colliding key {key!r}")
            require(key == unicodedata.normalize("NFC", key), f"{source}: non-NFC key {key!r}")
            normalized.add(folded)
            result[key] = value
        return result

    def reject_float(value: str) -> None:
        raise PublicationError(f"{source}: non-integer JSON number {value!r} is forbidden")

    try:
        return json.loads(
            body.decode("utf-8"),
            object_pairs_hook=object_pairs,
            parse_float=reject_float,
            parse_constant=reject_float,
        )
    except (UnicodeError, json.JSONDecodeError) as error:
        raise PublicationError(f"{source}: invalid UTF-8 JSON: {error}") from error


def read_json(path: Path, *, max_bytes: int = MAX_CANDIDATE_BYTES) -> Any:
    body = read_bytes_bounded(path, max_bytes)
    return parse_json_bytes(body, str(path), max_bytes=max_bytes)


def read_bytes_bounded(path: Path, max_bytes: int) -> bytes:
    try:
        with path.open("rb") as source:
            body = source.read(max_bytes + 1)
    except OSError as error:
        raise PublicationError(f"{path}: cannot read: {error}") from error
    require(len(body) <= max_bytes, f"{path}: exceeds {max_bytes} bytes")
    return body


def canonical_json(value: Any) -> bytes:
    """Return the repository's integer-only canonical JSON profile.

    Inputs are NFC-normalized and floats are forbidden.  A trailing LF is part
    of the canonical bytes and therefore of every digest and signature.
    """

    def walk(item: Any, location: str) -> None:
        if item is None or isinstance(item, (bool, int)):
            return
        if isinstance(item, float):
            raise PublicationError(f"{location}: floating-point values are forbidden")
        if isinstance(item, str):
            require(item == unicodedata.normalize("NFC", item), f"{location}: string must be NFC")
            return
        if isinstance(item, list):
            for index, child in enumerate(item):
                walk(child, f"{location}[{index}]")
            return
        if isinstance(item, dict):
            folded: set[str] = set()
            for key, child in item.items():
                require(isinstance(key, str), f"{location}: object key must be a string")
                require(key == unicodedata.normalize("NFC", key), f"{location}: key must be NFC")
                normalized = key.casefold()
                require(normalized not in folded, f"{location}: case/Unicode-colliding key {key!r}")
                folded.add(normalized)
                walk(child, f"{location}.{key}")
            return
        raise PublicationError(f"{location}: unsupported JSON value {type(item).__name__}")

    walk(value, "$")
    return (json.dumps(value, ensure_ascii=False, allow_nan=False, sort_keys=True, separators=(",", ":")) + "\n").encode("utf-8")


def sha256_digest(body: bytes) -> str:
    return "sha256:" + hashlib.sha256(body).hexdigest()


def candidate_digest(body: bytes) -> str:
    return "sha256:" + hashlib.sha256(CANDIDATE_DOMAIN + len(body).to_bytes(8, "big") + body).hexdigest()


def signature_message(snapshot: bytes) -> bytes:
    return SIGNATURE_DOMAIN + len(snapshot).to_bytes(8, "big") + snapshot


def parse_timestamp(value: str, field: str) -> datetime:
    require(isinstance(value, str) and value.endswith("Z"), f"{field}: must be UTC RFC 3339 with Z")
    try:
        parsed = datetime.fromisoformat(value[:-1] + "+00:00")
    except ValueError as error:
        raise PublicationError(f"{field}: invalid timestamp") from error
    require(parsed.tzinfo is not None and parsed.utcoffset().total_seconds() == 0, f"{field}: must be UTC")
    require(parsed.microsecond == 0, f"{field}: subsecond precision is forbidden")
    return parsed


def format_timestamp(value: datetime) -> str:
    return value.astimezone(timezone.utc).replace(microsecond=0).isoformat().replace("+00:00", "Z")


def validate_with_schema(value: Any, schema_path: Path) -> None:
    try:
        import jsonschema
    except ImportError as error:  # pragma: no cover - workflow installs it
        raise PublicationError("jsonschema is required for publication validation") from error
    schema = read_json(schema_path, max_bytes=1 << 20)
    local_schemas = {}
    for local_path in (ROOT / "schemas").glob("*.schema.json"):
        local = read_json(local_path, max_bytes=1 << 20)
        if isinstance(local, dict) and isinstance(local.get("$id"), str):
            local_schemas[local["$id"]] = local
            local_schemas[local_path.resolve().as_uri()] = local
    resolver = jsonschema.RefResolver.from_schema(schema, store=local_schemas)
    validator = jsonschema.Draft202012Validator(schema, resolver=resolver)
    errors = sorted(validator.iter_errors(value), key=lambda item: list(item.absolute_path))
    if errors:
        error = errors[0]
        where = ".".join(str(part) for part in error.absolute_path) or "$"
        raise PublicationError(f"{schema_path.name}: {where}: {error.message}")


def b64decode_exact(value: str, size: int, field: str) -> bytes:
    require(isinstance(value, str), f"{field}: must be base64")
    try:
        decoded = base64.b64decode(value, validate=True)
    except (ValueError, base64.binascii.Error) as error:
        raise PublicationError(f"{field}: invalid base64") from error
    require(len(decoded) == size, f"{field}: must decode to {size} bytes")
    return decoded


def load_public_keys(path: Path) -> dict[str, bytes]:
    value = read_json(path, max_bytes=64 << 10)
    require(isinstance(value, dict) and set(value) == {"schema_version", "keys"}, "trusted keys: invalid document")
    require_integer_const(value.get("schema_version"), 1, "trusted keys: invalid schema version")
    entries = value.get("keys")
    require(isinstance(entries, list), "trusted keys: keys must be an array")
    result: dict[str, bytes] = {}
    for entry in entries:
        require(isinstance(entry, dict) and set(entry) == {"key_id", "public_key"}, "trusted keys: invalid entry")
        key_id = entry["key_id"]
        require(isinstance(key_id, str) and ID_RE.fullmatch(key_id), "trusted keys: invalid key ID")
        require(key_id not in result, f"trusted keys: duplicate key ID {key_id}")
        result[key_id] = b64decode_exact(entry["public_key"], 32, f"trusted key {key_id}")
    return result


def ed25519_private_key(encoded: str) -> bytes:
    """Decode a raw Ed25519 seed without importing third-party code."""
    return b64decode_exact(encoded, 32, "private signing seed")


def _openssl(arguments: list[str], *, input_body: bytes | None = None) -> bytes:
    require(Path(OPENSSL).is_file(), f"reviewed signer runtime is missing: {OPENSSL}")
    try:
        completed = subprocess.run(
            [OPENSSL, *arguments], input=input_body, stdout=subprocess.PIPE,
            stderr=subprocess.PIPE, check=False,
        )
    except OSError as error:
        raise PublicationError(f"failed to execute reviewed signer runtime: {error}") from error
    require(completed.returncode == 0, "OpenSSL Ed25519 operation failed")
    return completed.stdout


def ed25519_public_bytes(seed: bytes) -> bytes:
    require(len(seed) == 32, "Ed25519 seed must be 32 bytes")
    public_der = _openssl(
        ["pkey", "-inform", "DER", "-pubout", "-outform", "DER"],
        input_body=ED25519_PRIVATE_DER_PREFIX + seed,
    )
    require(public_der.startswith(ED25519_PUBLIC_DER_PREFIX), "OpenSSL returned an invalid Ed25519 public key")
    public_key = public_der[len(ED25519_PUBLIC_DER_PREFIX):]
    require(len(public_key) == 32, "OpenSSL returned an invalid Ed25519 public key")
    return public_key


def ed25519_sign(seed: bytes, message: bytes) -> bytes:
    require(len(seed) == 32, "Ed25519 seed must be 32 bytes")
    with tempfile.TemporaryDirectory(prefix="uap-directory-ed25519-sign-") as temporary:
        private_path = Path(temporary) / "private.der"
        message_path = Path(temporary) / "message.bin"
        private_path.write_bytes(ED25519_PRIVATE_DER_PREFIX + seed)
        private_path.chmod(0o600)
        message_path.write_bytes(message)
        signature = _openssl([
            "pkeyutl", "-sign", "-rawin", "-inkey", str(private_path),
            "-keyform", "DER", "-in", str(message_path),
        ])
        require(len(signature) == 64, "OpenSSL returned an invalid Ed25519 signature")
        return signature


def ed25519_verify(public_key: bytes, message: bytes, signature: bytes) -> None:
    require(len(public_key) == 32, "Ed25519 public key must be 32 bytes")
    with tempfile.TemporaryDirectory(prefix="uap-directory-ed25519-verify-") as temporary:
        public_path = Path(temporary) / "public.der"
        message_path = Path(temporary) / "message.bin"
        signature_path = Path(temporary) / "signature.bin"
        public_path.write_bytes(ED25519_PUBLIC_DER_PREFIX + public_key)
        message_path.write_bytes(message)
        signature_path.write_bytes(signature)
        _openssl([
            "pkeyutl", "-verify", "-pubin", "-inkey", str(public_path),
            "-keyform", "DER", "-rawin", "-in", str(message_path),
            "-sigfile", str(signature_path),
        ])


def validate_envelope_contract(envelope: dict[str, Any]) -> None:
    """Validate the small envelope contract without third-party code."""
    require(set(envelope) == {
        "envelope_schema_version", "snapshot_schema_version", "sequence", "key_id",
        "algorithm", "signature_domain", "snapshot_digest", "signature",
    }, "signature envelope fields are invalid")
    require_integer_const(envelope["envelope_schema_version"], 1, "signature envelope version is invalid")
    require_integer_const(envelope["snapshot_schema_version"], 1, "signature envelope snapshot version is invalid")
    require(type(envelope["sequence"]) is int and envelope["sequence"] >= 1, "signature envelope sequence is invalid")
    require(isinstance(envelope["key_id"], str) and ID_RE.fullmatch(envelope["key_id"]) is not None, "signature envelope key ID is invalid")
    require(envelope["algorithm"] == "Ed25519", "signature envelope algorithm is invalid")
    require(envelope["signature_domain"] == "UAP-DIRECTORY-SNAPSHOT-ED25519-V1", "signature envelope domain is invalid")
    require(isinstance(envelope["snapshot_digest"], str) and DIGEST_RE.fullmatch(envelope["snapshot_digest"]) is not None, "signature envelope digest is invalid")
    b64decode_exact(envelope["signature"], 64, "signature")


SIMPLE_ID_RE = re.compile(r"[a-z0-9]+(?:-[a-z0-9]+)*")
DISTRIBUTION_ID_RE = re.compile(r"[a-z0-9]+(?:-[a-z0-9]+)*/[a-z0-9]+(?:-[a-z0-9]+)*")
EVIDENCE_ID_RE = re.compile(r"[a-z0-9][a-z0-9._/-]*")
REPOSITORY_RE = re.compile(r"[A-Za-z0-9][A-Za-z0-9-]*/[A-Za-z0-9][A-Za-z0-9._-]*")
PACKAGE_PATH_RE = re.compile(r"(?!/)(?!.*(?:^|/)\.\.?/)[A-Za-z0-9._-]+(?:/[A-Za-z0-9._-]+)*")
TIMESTAMP_RE = re.compile(r"[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z")
SEMVER_RE = re.compile(r"[0-9]+\.[0-9]+\.[0-9]+")
CLIENTS = {"codex", "chatgpt", "cursor", "copilot", "vscode", "kiro"}


def _object(value: Any, required: set[str], optional: set[str], label: str) -> dict[str, Any]:
    require(isinstance(value, dict), f"{label} must be an object")
    require(set(value) == required | (set(value) & optional), f"{label} fields are invalid")
    require(required.issubset(value), f"{label} required fields are missing")
    return value


def _string(value: Any, pattern: re.Pattern[str] | None, label: str, minimum: int = 0, maximum: int | None = None) -> None:
    require(isinstance(value, str) and len(value) >= minimum, f"{label} is invalid")
    require(maximum is None or len(value) <= maximum, f"{label} is invalid")
    require(pattern is None or pattern.fullmatch(value) is not None, f"{label} is invalid")


def _array(value: Any, label: str, *, minimum: int = 0, maximum: int | None = None, unique: bool = False) -> list[Any]:
    require(isinstance(value, list) and len(value) >= minimum, f"{label} must be a valid array")
    require(maximum is None or len(value) <= maximum, f"{label} must be a valid array")
    if unique:
        require(all(value.count(item) == 1 for item in value), f"{label} must contain unique items")
    return value


def _positive_integer(value: Any, label: str) -> None:
    require(type(value) is int and value >= 1, f"{label} is invalid")


def _validate_source(value: Any, label: str) -> None:
    source = _object(value, {"repository", "revision", "path"}, set(), label)
    _string(source["repository"], REPOSITORY_RE, f"{label}.repository")
    _string(source["revision"], SHA_RE, f"{label}.revision")
    _string(source["path"], PACKAGE_PATH_RE, f"{label}.path")


def _validate_target(value: Any, label: str) -> None:
    target = _object(value, {"client", "scopes", "delivery", "authentication"}, {"app_binding"}, label)
    require(target["client"] in CLIENTS, f"{label}.client is invalid")
    require(target["scopes"] == ["user"], f"{label}.scopes is invalid")
    require(target["delivery"] in {"managed", "prepared", "manual_activation"}, f"{label}.delivery is invalid")
    require(target["authentication"] in {"not_required", "required", "unknown"}, f"{label}.authentication is invalid")
    require(("app_binding" in target) == (target["client"] == "chatgpt"), f"{label}.app_binding is invalid")
    if "app_binding" in target:
        binding = _object(target["app_binding"], {"app_key", "id", "mcp_server"}, set(), f"{label}.app_binding")
        for field in binding:
            _string(binding[field], None, f"{label}.app_binding.{field}", minimum=1)


def _validate_policy(value: Any, label: str) -> None:
    policy = _object(value, {"release_sequence", "status", "minimum_installer_version", "targets", "current_evidence"}, set(), label)
    _positive_integer(policy["release_sequence"], f"{label}.release_sequence")
    require(policy["status"] in {"active", "superseded", "revoked"}, f"{label}.status is invalid")
    _string(policy["minimum_installer_version"], SEMVER_RE, f"{label}.minimum_installer_version")
    for index, target in enumerate(_array(policy["targets"], f"{label}.targets", minimum=1)):
        _validate_target(target, f"{label}.targets[{index}]")
    for evidence_id in _array(policy["current_evidence"], f"{label}.current_evidence", unique=True):
        _string(evidence_id, EVIDENCE_ID_RE, f"{label}.current_evidence item")


def _validate_release(value: Any, label: str, *, snapshot: bool) -> None:
    required = {"sequence", "package_version", "manifest_name", "agent_plugins_schema", "package_source", "tree_digest_algorithm", "tree_digest", "manifest_digest", "components", "published_at"}
    release = _object(value, required, {"build_provenance"}, label)
    _positive_integer(release["sequence"], f"{label}.sequence")
    _string(release["package_version"], None, f"{label}.package_version")
    _string(release["manifest_name"], SIMPLE_ID_RE, f"{label}.manifest_name")
    require(release["agent_plugins_schema"] == "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json", f"{label}.agent_plugins_schema is invalid")
    _validate_source(release["package_source"], f"{label}.package_source")
    require(release["tree_digest_algorithm"] == "agentplugins-tree-sha256-v1", f"{label}.tree_digest_algorithm is invalid")
    for field in ("tree_digest", "manifest_digest"):
        _string(release[field], DIGEST_RE, f"{label}.{field}")
    components = _array(release["components"], f"{label}.components", minimum=1, unique=True)
    require(all(item in {"extensions", "mcp", "skills"} for item in components), f"{label}.components is invalid")
    published = release["published_at"]
    require((not snapshot and published is None) or isinstance(published, str), f"{label}.published_at is invalid")
    if published is not None:
        _string(published, TIMESTAMP_RE, f"{label}.published_at")
    if "build_provenance" in release:
        provenance = _object(release["build_provenance"], {"upstream_repository", "upstream_revision"}, set(), f"{label}.build_provenance")
        _string(provenance["upstream_repository"], REPOSITORY_RE, f"{label}.build_provenance.upstream_repository")
        _string(provenance["upstream_revision"], SHA_RE, f"{label}.build_provenance.upstream_revision")


def _validate_distribution(value: Any, label: str, *, snapshot: bool) -> None:
    distribution = _object(value, {"schema_version", "id", "product_id", "kind", "status", "packager", "releases", "release_policies"}, set(), label)
    require_integer_const(distribution["schema_version"], 1, f"{label}.schema_version is invalid")
    _string(distribution["id"], DISTRIBUTION_ID_RE, f"{label}.id")
    for field in ("product_id", "packager"):
        _string(distribution[field], SIMPLE_ID_RE, f"{label}.{field}")
    require(distribution["kind"] in {"upstream", "community_bridge", "community"}, f"{label}.kind is invalid")
    require(distribution["status"] in {"active", "suspended"}, f"{label}.status is invalid")
    for index, release in enumerate(_array(distribution["releases"], f"{label}.releases", minimum=1)):
        _validate_release(release, f"{label}.releases[{index}]", snapshot=snapshot)
    for index, policy in enumerate(_array(distribution["release_policies"], f"{label}.release_policies", minimum=1)):
        _validate_policy(policy, f"{label}.release_policies[{index}]")


def _validate_product(value: Any, label: str) -> None:
    product = _object(value, {"schema_version", "id", "display_name", "description", "manifest_name", "aliases", "reserved_aliases", "categories", "minimum_capabilities", "default_distribution", "distributions"}, {"icon"}, label)
    require_integer_const(product["schema_version"], 1, f"{label}.schema_version is invalid")
    for field in ("id", "manifest_name"):
        _string(product[field], SIMPLE_ID_RE, f"{label}.{field}")
    _string(product["display_name"], None, f"{label}.display_name", minimum=1, maximum=100)
    _string(product["description"], None, f"{label}.description", minimum=1, maximum=500)
    for field, maximum in (("aliases", None), ("reserved_aliases", None), ("categories", 20)):
        for item in _array(product[field], f"{label}.{field}", minimum=1, maximum=maximum, unique=True):
            _string(item, SIMPLE_ID_RE, f"{label}.{field} item")
    capabilities = _object(product["minimum_capabilities"], {"skills", "mcp"}, set(), f"{label}.minimum_capabilities")
    require(all(value in {"required", "optional"} for value in capabilities.values()), f"{label}.minimum_capabilities is invalid")
    require("required" in capabilities.values(), f"{label}.minimum_capabilities cannot both be optional")
    _string(product["default_distribution"], DISTRIBUTION_ID_RE, f"{label}.default_distribution")
    for item in _array(product["distributions"], f"{label}.distributions", minimum=1, unique=True):
        _string(item, DISTRIBUTION_ID_RE, f"{label}.distributions item")
    if "icon" in product:
        icon = _object(product["icon"], {"path", "digest"}, set(), f"{label}.icon")
        _string(icon["path"], re.compile(r"assets/plugin-icons/[A-Za-z0-9._-]+"), f"{label}.icon.path")
        _string(icon["digest"], DIGEST_RE, f"{label}.icon.digest")


def _validate_evidence(value: Any, label: str) -> None:
    required = {"schema_version", "id", "product_id", "distribution_id", "release_sequence", "package_tree_digest", "manifest_digest", "source_repository", "source_revision", "source_path", "level", "outcome", "artifact"}
    optional = {"client", "client_version", "installer_version", "os", "architecture", "dependency_identity", "observed_at"}
    evidence = _object(value, required, optional, label)
    require_integer_const(evidence["schema_version"], 1, f"{label}.schema_version is invalid")
    _string(evidence["id"], EVIDENCE_ID_RE, f"{label}.id")
    _string(evidence["product_id"], SIMPLE_ID_RE, f"{label}.product_id")
    _string(evidence["distribution_id"], DISTRIBUTION_ID_RE, f"{label}.distribution_id")
    _positive_integer(evidence["release_sequence"], f"{label}.release_sequence")
    _string(evidence["package_tree_digest"], DIGEST_RE, f"{label}.package_tree_digest")
    _string(evidence["manifest_digest"], DIGEST_RE, f"{label}.manifest_digest")
    _string(evidence["source_repository"], REPOSITORY_RE, f"{label}.source_repository")
    _string(evidence["source_revision"], SHA_RE, f"{label}.source_revision")
    _string(evidence["source_path"], None, f"{label}.source_path", minimum=1)
    source_path = evidence["source_path"]
    require(
        not source_path.startswith("/") and not source_path.endswith("/")
        and "\\" not in source_path and "//" not in source_path
        and not any(part in {"", ".", ".."} for part in PurePosixPath(source_path).parts)
        and not any(character in source_path for character in "?#%\x00"),
        f"{label}.source_path is unsafe",
    )
    require(evidence["level"] in {"schema", "materialization", "discovery", "runtime", "oauth"}, f"{label}.level is invalid")
    require(evidence["outcome"] in {"passed", "failed", "inconclusive", "not_tested", "not_applicable"}, f"{label}.outcome is invalid")
    if "client" in evidence:
        require(evidence["client"] in CLIENTS, f"{label}.client is invalid")
    if evidence["level"] == "schema":
        require("client" not in evidence, f"{label}.client is forbidden for schema evidence")
    else:
        require({"client", "client_version", "installer_version", "os", "architecture", "observed_at"}.issubset(evidence), f"{label} client fields are missing")
    for field in ("client_version", "installer_version", "os", "architecture", "dependency_identity"):
        if field in evidence:
            _string(evidence[field], None, f"{label}.{field}", minimum=1)
    if "observed_at" in evidence:
        _string(evidence["observed_at"], TIMESTAMP_RE, f"{label}.observed_at")
    artifact = _object(evidence["artifact"], {"repository", "revision", "path", "digest"}, set(), f"{label}.artifact")
    _string(artifact["repository"], REPOSITORY_RE, f"{label}.artifact.repository")
    _string(artifact["revision"], SHA_RE, f"{label}.artifact.revision")
    _string(artifact["path"], None, f"{label}.artifact.path", minimum=1)
    artifact_path = artifact["path"]
    require(
        not artifact_path.startswith("/") and "\\" not in artifact_path
        and ".." not in PurePosixPath(artifact_path).parts and "\x00" not in artifact_path,
        f"{label}.artifact.path is unsafe",
    )
    _string(artifact["digest"], DIGEST_RE, f"{label}.artifact.digest")


def validate_directory_records(value: dict[str, Any], *, snapshot: bool) -> None:
    for index, product in enumerate(value["products"]):
        _validate_product(product, f"products[{index}]")
    for index, distribution in enumerate(value["distributions"]):
        _validate_distribution(distribution, f"distributions[{index}]", snapshot=snapshot)
    for index, evidence in enumerate(value["evidence"]):
        _validate_evidence(evidence, f"evidence[{index}]")
    for index, item in enumerate(value["revocations"]):
        revocation = _object(item, {"distribution_id", "release_sequence"}, set(), f"revocations[{index}]")
        _string(revocation["distribution_id"], DISTRIBUTION_ID_RE, f"revocations[{index}].distribution_id")
        _positive_integer(revocation["release_sequence"], f"revocations[{index}].release_sequence")


def verify_envelope(
    snapshot: bytes, envelope: dict[str, Any], trusted_keys: dict[str, bytes], *,
    validate_schema: bool = True,
) -> None:
    if validate_schema:
        validate_with_schema(envelope, ENVELOPE_SCHEMA)
    validate_envelope_contract(envelope)
    require(len(snapshot) <= MAX_SNAPSHOT_BYTES, "snapshot exceeds size limit")
    require(canonical_json(parse_json_bytes(snapshot, "snapshot", max_bytes=MAX_SNAPSHOT_BYTES)) == snapshot, "snapshot is not canonical JSON")
    require(envelope["snapshot_digest"] == sha256_digest(snapshot), "snapshot digest mismatch")
    key_id = envelope["key_id"]
    require(key_id in trusted_keys, f"unknown signing key ID {key_id}")
    signature = b64decode_exact(envelope["signature"], 64, "signature")
    try:
        ed25519_verify(trusted_keys[key_id], signature_message(snapshot), signature)
    except PublicationError as error:
        raise PublicationError("invalid Ed25519 snapshot signature") from error


def release_identity(distribution: dict[str, Any], release: dict[str, Any]) -> tuple[str, int]:
    return distribution["id"], release["sequence"]


def immutable_release(release: dict[str, Any]) -> dict[str, Any]:
    return {key: value for key, value in release.items() if key != "published_at"}


def iter_releases(snapshot: dict[str, Any]):  # type: ignore[no-untyped-def]
    for distribution in snapshot["distributions"]:
        for release in distribution["releases"]:
            yield distribution, release


def distribution_policies(distribution: dict[str, Any]) -> dict[int, dict[str, Any]]:
    return {policy["release_sequence"]: policy for policy in distribution["release_policies"]}


def validate_snapshot_semantics(
    snapshot: dict[str, Any],
    previous: dict[str, Any] | None = None,
    historical_evidence: dict[str, dict[str, Any]] | None = None,
    *, validate_schema: bool = True,
) -> None:
    if validate_schema:
        validate_with_schema(snapshot, SNAPSHOT_SCHEMA)
    require(set(snapshot) == {
        "snapshot_schema_version", "sequence", "publication_id", "source_commit",
        "generated_at", "expires_at", "products", "distributions", "evidence", "revocations",
    }, "snapshot fields are invalid")
    require_integer_const(snapshot["snapshot_schema_version"], 1, "snapshot schema version is invalid")
    require(type(snapshot["sequence"]) is int and snapshot["sequence"] >= 1, "snapshot sequence is invalid")
    require(isinstance(snapshot["publication_id"], str) and re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9._-]{0,127}", snapshot["publication_id"]) is not None, "snapshot publication ID is invalid")
    require(isinstance(snapshot["source_commit"], str) and SHA_RE.fullmatch(snapshot["source_commit"]) is not None, "snapshot source commit is invalid")
    for field in ("products", "distributions", "evidence", "revocations"):
        require(isinstance(snapshot[field], list), f"snapshot {field} must be an array")
    validate_directory_records(snapshot, snapshot=True)
    generated = parse_timestamp(snapshot["generated_at"], "generated_at")
    expires = parse_timestamp(snapshot["expires_at"], "expires_at")
    require(expires > generated, "expires_at must be later than generated_at")
    require(expires - generated <= __import__("datetime").timedelta(days=31), "snapshot lifetime exceeds 31 days")
    product_ids: set[str] = set()
    aliases: dict[str, str] = {}
    distribution_map: dict[str, dict[str, Any]] = {}
    release_map: dict[tuple[str, int], dict[str, Any]] = {}
    policy_map: dict[tuple[str, int], dict[str, Any]] = {}
    evidence_map: dict[str, dict[str, Any]] = {}
    for product in snapshot["products"]:
        require(product["id"] not in product_ids, f"duplicate product {product['id']}")
        product_ids.add(product["id"])
        distribution_ids = set(product["distributions"])
        require(len(distribution_ids) == len(product["distributions"]), f"{product['id']}: duplicate distribution")
        require(product["default_distribution"] in distribution_ids, f"{product['id']}: default distribution missing")
        require(set(product["aliases"]).issubset(product["reserved_aliases"]), f"{product['id']}: active aliases must remain reserved")
        for alias in product["reserved_aliases"]:
            require(alias not in aliases, f"reserved alias {alias} is assigned to multiple products")
            aliases[alias] = product["id"]
    for distribution in snapshot["distributions"]:
        require(distribution["id"] not in distribution_map, f"duplicate distribution {distribution['id']}")
        distribution_map[distribution["id"]] = distribution
        sequences: set[int] = set()
        policies = distribution_policies(distribution)
        require(len(policies) == len(distribution["release_policies"]), f"{distribution['id']}: duplicate release policy")
        for release in distribution["releases"]:
            require(release["sequence"] not in sequences, f"{distribution['id']}: duplicate release sequence")
            sequences.add(release["sequence"])
            identity = release_identity(distribution, release)
            require(identity not in release_map, f"duplicate release identity {identity}")
            release_map[identity] = release
            published = parse_timestamp(release["published_at"], f"{identity}.published_at")
            require(published <= generated, f"{identity}: publication timestamp is in the future")
            require(release["sequence"] in policies, f"{identity}: release policy is missing")
            policy_map[identity] = policies[release["sequence"]]
        require(set(policies) == sequences, f"{distribution['id']}: release policies do not match releases")
    for product in snapshot["products"]:
        for distribution_id in product["distributions"]:
            require(distribution_id in distribution_map, f"{product['id']}: distribution {distribution_id} is missing")
            require(distribution_map[distribution_id]["product_id"] == product["id"], f"{product['id']}: distribution product mismatch")
    selected_ids = {item for policy in policy_map.values() for item in policy["current_evidence"]}
    applicability_by_release: dict[tuple[str, int], set[tuple[Any, ...]]] = {}
    for evidence in snapshot["evidence"]:
        evidence_id = evidence["id"]
        require(evidence_id not in evidence_map, f"duplicate evidence identity {evidence_id}")
        evidence_map[evidence_id] = evidence
        identity = (evidence["distribution_id"], evidence["release_sequence"])
        require(identity in release_map, f"{evidence_id}: evidence release is missing")
        release = release_map[identity]
        source = release["package_source"]
        require(
            evidence["product_id"] == distribution_map[evidence["distribution_id"]]["product_id"]
            and evidence["package_tree_digest"] == release["tree_digest"]
            and evidence["manifest_digest"] == release["manifest_digest"]
            and evidence["source_repository"] == source["repository"]
            and evidence["source_revision"] == source["revision"]
            and evidence["source_path"] == source["path"],
            f"{evidence_id}: evidence source identity does not match release",
        )
        applicability = tuple(evidence.get(field) for field in ("level", "client", "client_version", "installer_version", "dependency_identity", "os", "architecture"))
        seen_applicability = applicability_by_release.setdefault(identity, set())
        require(applicability not in seen_applicability, f"{identity}: multiple current evidence records for one applicability tuple")
        seen_applicability.add(applicability)
        if "observed_at" in evidence:
            require(parse_timestamp(evidence["observed_at"], f"{evidence_id}.observed_at") <= generated, f"{evidence_id}: evidence timestamp is in the future")
    require(set(evidence_map) == selected_ids, "snapshot evidence must exactly match current signed evidence pointers")
    expected_revocations = {
        (identity[0], identity[1]) for identity, policy in policy_map.items() if policy["status"] == "revoked"
    }
    actual_revocations = {(item["distribution_id"], item["release_sequence"]) for item in snapshot["revocations"]}
    require(len(actual_revocations) == len(snapshot["revocations"]), "duplicate revocation")
    require(actual_revocations == expected_revocations, "revocation summary does not match signed release policies")

    if historical_evidence is not None:
        for evidence_id, evidence in evidence_map.items():
            if evidence_id in historical_evidence:
                require(evidence == historical_evidence[evidence_id], f"immutable evidence {evidence_id} changed")
        historical_evidence.update(evidence_map)

    if previous is None:
        return
    require(snapshot["snapshot_schema_version"] == previous["snapshot_schema_version"], "snapshot schema feed changed")
    require(snapshot["sequence"] > previous["sequence"], "snapshot sequence did not increase")
    require(generated > parse_timestamp(previous["generated_at"], "previous.generated_at"), "snapshot generation time did not increase")
    product_map = {product["id"]: product for product in snapshot["products"]}
    previous_products = {product["id"]: product for product in previous["products"]}
    for product_id, old_product in previous_products.items():
        require(product_id in product_map, f"published product {product_id} was removed")
        new_product = product_map[product_id]
        require(new_product["manifest_name"] == old_product["manifest_name"], f"published product {product_id} manifest name changed")
        historical_aliases = set(old_product["aliases"]) | set(old_product["reserved_aliases"])
        require(historical_aliases.issubset(new_product["reserved_aliases"]), f"published product {product_id} reserved alias was removed")
        require(set(old_product["distributions"]).issubset(new_product["distributions"]), f"published product {product_id} distribution was removed")
    previous_distribution_map = {distribution["id"]: distribution for distribution in previous["distributions"]}
    for distribution_id, old_distribution in previous_distribution_map.items():
        require(distribution_id in distribution_map, f"published distribution {distribution_id} was removed")
        new_distribution = distribution_map[distribution_id]
        for field in ("id", "product_id", "kind", "packager"):
            require(new_distribution[field] == old_distribution[field], f"published distribution {distribution_id} {field} changed")
    previous_map = {release_identity(distribution, release): release for distribution, release in iter_releases(previous)}
    previous_policies = {
        (distribution["id"], sequence): policy
        for distribution in previous["distributions"]
        for sequence, policy in distribution_policies(distribution).items()
    }
    previous_evidence: dict[str, dict[str, Any]] = {}
    for evidence in previous["evidence"]:
        old = previous_evidence.setdefault(evidence["id"], evidence)
        require(old == evidence, f"previous snapshot reused evidence identity {evidence['id']} with different fields")
    for evidence_id, evidence in evidence_map.items():
        if evidence_id in previous_evidence:
            require(evidence == previous_evidence[evidence_id], f"immutable evidence {evidence_id} changed")
    for identity, old_release in previous_map.items():
        require(identity in release_map, f"published release {identity} was removed")
        new_release = release_map[identity]
        require(immutable_release(new_release) == immutable_release(old_release), f"published release {identity} immutable fields changed")
        require(new_release["published_at"] == old_release["published_at"], f"published release {identity} timestamp changed")
        old_status = previous_policies[identity]["status"]
        new_status = policy_map[identity]["status"]
        if old_status == "revoked":
            require(new_status == "revoked", f"revoked release {identity} cannot be restored")
    previous_highest: dict[str, int] = {}
    for distribution_id, sequence in previous_map:
        previous_highest[distribution_id] = max(previous_highest.get(distribution_id, 0), sequence)
    for distribution_id, sequence in release_map:
        if (distribution_id, sequence) not in previous_map and distribution_id in previous_highest:
            require(sequence > previous_highest[distribution_id], f"new release {(distribution_id, sequence)} is not above the published sequence floor")


def validate_relative_artifact(path: str, expected: str) -> None:
    require(isinstance(path, str) and path == expected, f"artifact path must be exactly {expected!r}")
    parsed = PurePosixPath(path)
    require(not parsed.is_absolute() and ".." not in parsed.parts and ":" not in path and "\\" not in path, "unsafe artifact path")


def validate_latest(latest: dict[str, Any], *, validate_schema: bool = True) -> None:
    if validate_schema:
        validate_with_schema(latest, LATEST_SCHEMA)
    require(set(latest) == {
        "pointer_schema_version", "snapshot_schema_version", "sequence",
        "snapshot_path", "envelope_path", "fetch_contract",
    }, "latest pointer fields are invalid")
    require_integer_const(latest["pointer_schema_version"], 1, "latest pointer version is invalid")
    require_integer_const(latest["snapshot_schema_version"], 1, "latest pointer snapshot version is invalid")
    require(type(latest["sequence"]) is int and latest["sequence"] >= 1, "latest pointer sequence is invalid")
    sequence = latest["sequence"]
    stem = f"{sequence:020d}"
    validate_relative_artifact(latest["snapshot_path"], f"snapshots/{stem}.json")
    validate_relative_artifact(latest["envelope_path"], f"snapshots/{stem}.envelope.json")
    limits = latest["fetch_contract"]
    require(isinstance(limits, dict) and set(limits) == {
        "https_required", "same_origin_redirects_only", "forward_credentials_on_redirect",
        "max_redirects", "latest_max_bytes", "snapshot_max_bytes", "envelope_max_bytes",
        "retry_attempts",
    }, "latest fetch contract fields are invalid")
    require(limits["https_required"] is True, "HTTPS is required")
    require(limits["same_origin_redirects_only"] is True, "redirects must stay same-origin")
    require(limits["forward_credentials_on_redirect"] is False, "redirect credentials must not be forwarded")
    for field in ("max_redirects", "latest_max_bytes", "snapshot_max_bytes", "envelope_max_bytes", "retry_attempts"):
        require(type(limits[field]) is int and limits[field] >= 0, f"{field} must be a non-negative integer")
    require(limits["latest_max_bytes"] >= 1 and limits["snapshot_max_bytes"] >= 1 and limits["envelope_max_bytes"] >= 1 and limits["retry_attempts"] >= 1, "fetch sizes and retry attempts must be positive")
    require(limits["max_redirects"] <= 2, "redirect limit exceeds implementation maximum")
    require(limits["retry_attempts"] <= 3, "retry limit exceeds implementation maximum")
    require(limits["latest_max_bytes"] <= MAX_LATEST_BYTES, "latest response limit exceeds implementation maximum")
    require(limits["snapshot_max_bytes"] <= MAX_SNAPSHOT_BYTES, "snapshot response limit exceeds implementation maximum")
    require(limits["envelope_max_bytes"] <= MAX_ENVELOPE_BYTES, "envelope response limit exceeds implementation maximum")


def load_ledger_latest(
    ledger_root: Path, trusted_keys: dict[str, bytes], *,
    allow_initialization: bool = False, seed_commit: str | None = None,
    minimum_sequence: int | None = None, validate_schema: bool = True,
    require_external_floor: bool = False,
) -> tuple[dict[str, Any], dict[str, Any], dict[str, dict[str, Any]], dict[str, list[tuple[dict[str, Any], dict[str, Any]]]]] | None:
    feed = ledger_root / "registry" / "schemas" / "1"
    latest_path = feed / "latest.json"
    if not latest_path.exists():
        require(allow_initialization, "publication ledger latest pointer is missing; explicit initialization is required")
        require(seed_commit is not None and SHA_RE.fullmatch(seed_commit) is not None, "initialization requires an exact seed commit")
        require(minimum_sequence is None, "initialization cannot accept an existing sequence floor")
        require(not feed.exists() or not any(feed.iterdir()), "initial publication feed is not empty")
        return None
    require(not allow_initialization or minimum_sequence is None, "initialization cannot accept an existing sequence floor")
    if require_external_floor and not allow_initialization:
        require(seed_commit is not None and SHA_RE.fullmatch(seed_commit) is not None, "normal publication requires the protected seed commit")
        require(type(minimum_sequence) is int and minimum_sequence >= 1, "normal publication requires the immutable tag sequence floor")
    contract_path = feed / LEDGER_CONTRACT_NAME
    contract_body = read_bytes_bounded(contract_path, MAX_LATEST_BYTES)
    contract = parse_json_bytes(contract_body, str(contract_path), max_bytes=MAX_LATEST_BYTES)
    require(
        isinstance(contract, dict) and set(contract) == {
            "contract_version", "initial_sequence", "schema_version", "seed_commit", "sequence_tag_prefix"
        },
        "publication ledger contract marker is invalid",
    )
    require(canonical_json(contract) == contract_body, "publication ledger contract marker is not canonical JSON")
    require(
        {path.name for path in feed.iterdir()} == {
            LEDGER_CONTRACT_NAME, "latest.json", "snapshots",
        },
        "publication ledger feed contains unexpected entries",
    )
    require_integer_const(contract["contract_version"], 1, "publication ledger contract version is unsupported")
    require_integer_const(contract["schema_version"], 1, "publication ledger schema version is unsupported")
    require_integer_const(contract["initial_sequence"], 1, "publication ledger initial sequence marker is invalid")
    require(isinstance(contract["seed_commit"], str) and SHA_RE.fullmatch(contract["seed_commit"]) is not None, "publication ledger seed commit marker is invalid")
    require(contract["sequence_tag_prefix"] == "directory-publication-schema-1-sequence-", "publication ledger tag namespace marker is invalid")
    if seed_commit is not None:
        require(contract["seed_commit"] == seed_commit, "publication ledger seed commit differs from the protected contract")
    latest_body = read_bytes_bounded(latest_path, MAX_LATEST_BYTES)
    latest = parse_json_bytes(latest_body, str(latest_path), max_bytes=MAX_LATEST_BYTES)
    require(isinstance(latest, dict), "latest pointer must be an object")
    require(canonical_json(latest) == latest_body, "latest pointer is not canonical JSON")
    validate_latest(latest, validate_schema=validate_schema)
    highest = latest["sequence"]
    if minimum_sequence is not None:
        require(type(minimum_sequence) is int and minimum_sequence >= 1, "publication ledger sequence floor is invalid")
        require(highest >= minimum_sequence, "publication ledger latest sequence regressed below the immutable tag floor")
    require(highest <= MAX_LEDGER_SNAPSHOTS, "publication ledger exceeds supported history bound")
    snapshots_dir = feed / "snapshots"
    require(snapshots_dir.is_dir(), "publication ledger snapshots directory is missing")
    actual_files = {path.name for path in snapshots_dir.iterdir()}
    expected_files: set[str] = set()
    previous: dict[str, Any] | None = None
    historical_evidence: dict[str, dict[str, Any]] = {}
    publications: dict[str, list[tuple[dict[str, Any], dict[str, Any]]]] = {}
    for sequence in range(1, highest + 1):
        stem = f"{sequence:020d}"
        snapshot_name = f"{stem}.json"
        envelope_name = f"{stem}.envelope.json"
        expected_files.update((snapshot_name, envelope_name))
        snapshot_path = snapshots_dir / snapshot_name
        envelope_path = snapshots_dir / envelope_name
        require(snapshot_path.is_file() and envelope_path.is_file(), f"publication ledger sequence {sequence} is incomplete")
        snapshot_bytes = read_bytes_bounded(snapshot_path, MAX_SNAPSHOT_BYTES)
        envelope_body = read_bytes_bounded(envelope_path, MAX_ENVELOPE_BYTES)
        envelope = parse_json_bytes(envelope_body, str(envelope_path), max_bytes=MAX_ENVELOPE_BYTES)
        require(isinstance(envelope, dict), "signature envelope must be an object")
        require(canonical_json(envelope) == envelope_body, "signature envelope is not canonical JSON")
        verify_envelope(snapshot_bytes, envelope, trusted_keys, validate_schema=validate_schema)
        snapshot = parse_json_bytes(snapshot_bytes, str(snapshot_path), max_bytes=MAX_SNAPSHOT_BYTES)
        require(isinstance(snapshot, dict), "snapshot must be an object")
        validate_snapshot_semantics(snapshot, previous, historical_evidence, validate_schema=validate_schema)
        require(snapshot["sequence"] == sequence == envelope["sequence"], f"publication ledger sequence {sequence} identity mismatch")
        publications.setdefault(snapshot["publication_id"], []).append((snapshot, envelope))
        previous = snapshot
    require(actual_files == expected_files, "publication ledger contains unexpected or non-contiguous historical artifacts")
    require(previous is not None and previous["sequence"] == highest, "publication ledger latest sequence mismatch")
    if allow_initialization:
        require(highest == 1, "initialization rerun requires the original sequence 1 publication")
    return previous, latest, historical_evidence, publications


def atomic_write(path: Path, body: bytes, mode: int = 0o644) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    temporary = path.with_name(f".{path.name}.{os.getpid()}.tmp")
    try:
        with temporary.open("xb") as output:
            output.write(body)
            output.flush()
            os.fsync(output.fileno())
        os.chmod(temporary, mode)
        os.replace(temporary, path)
        descriptor = os.open(path.parent, os.O_RDONLY)
        try:
            os.fsync(descriptor)
        finally:
            os.close(descriptor)
    except Exception:
        temporary.unlink(missing_ok=True)
        raise

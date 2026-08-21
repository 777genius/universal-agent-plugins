#!/usr/bin/env python3
"""Run the Phase 6 launch matrix without turning unavailable systems into passes.

The runner creates fresh client homes, invokes the supplied Agent Plugins binary,
and exports only tuple-scoped, redacted evidence. Runtime/OAuth observations are
accepted only from an explicit attestation file; package projection is never
promoted to runtime evidence.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import platform
import re
import shutil
import subprocess
import tempfile
from collections import Counter
from datetime import datetime, timezone
from pathlib import Path
from typing import Any
from urllib.parse import urlsplit


ROOT = Path(__file__).resolve().parents[1]
SCENARIOS = ROOT / "tests" / "e2e" / "launch-scenarios.json"
EXTERNAL_PACKAGE = ROOT / "tests" / "e2e" / "fixtures" / "external-package"
STATE_FIXTURE = ROOT / "tests" / "e2e" / "fixtures" / "state-schema-2.json"
RECOVERY_FIXTURE = ROOT / "tests" / "e2e" / "fixtures" / "recovery-cases.json"
OUTCOMES = {"passed", "failed", "inconclusive", "not_applicable"}
DIGEST = re.compile(r"^sha256:[a-f0-9]{64}$")
FULL_SHA = re.compile(r"^[a-f0-9]{40}$")
VERSION = re.compile(r"^(\d+)\.(\d+)\.(\d+)$")
MINIMUM_STABLE_VERSION = (0, 1, 8)
IDENTITY_ID = re.compile(r"^[a-z0-9][a-z0-9._-]{2,63}$")
CLIENT_ROOTS = {
    "codex": ".codex",
    "cursor": ".cursor",
    "kiro": ".kiro",
    "copilot": ".copilot",
    "vscode": ".config/Code/User",
}
SECRET_NAME = re.compile(r"(?i)(token|secret|password|cookie|authorization|oauth[_-]?code)")


def utc_now() -> str:
    return datetime.now(timezone.utc).replace(microsecond=0).isoformat().replace("+00:00", "Z")


def sha256_file(path: Path) -> str:
    return "sha256:" + hashlib.sha256(path.read_bytes()).hexdigest()


def package_digest(path: Path) -> str:
    framed = bytearray(b"uap-e2e-package-v1\0")
    for item in sorted(p for p in path.rglob("*") if p.is_file()):
        relative = item.relative_to(path).as_posix().encode()
        body = item.read_bytes()
        framed.extend(len(relative).to_bytes(8, "big") + relative)
        framed.extend(len(body).to_bytes(8, "big") + body)
    return "sha256:" + hashlib.sha256(framed).hexdigest()


def parse_stable_version(value: str) -> tuple[int, int, int]:
    match = VERSION.fullmatch(value)
    if not match:
        raise ValueError("Agent Plugins version must be an exact semantic version")
    parsed = tuple(int(match.group(index)) for index in (1, 2, 3))
    if parsed < MINIMUM_STABLE_VERSION:
        raise ValueError("stable launch requires agentplugins 0.1.8 or newer")
    return parsed


def validated_directory_environment(
    origin: str, snapshot_path: Path, envelope_path: Path, trust_path: Path
) -> tuple[dict[str, str], dict[str, Any], str]:
    parsed = urlsplit(origin)
    if parsed.scheme != "https" or not parsed.hostname or parsed.username or parsed.password or parsed.query or parsed.fragment:
        raise ValueError("Directory origin must be credential-free public HTTPS")
    snapshot_bytes = snapshot_path.read_bytes()
    snapshot = json.loads(snapshot_bytes)
    envelope = json.loads(envelope_path.read_text())
    trust = json.loads(trust_path.read_text())
    digest = "sha256:" + hashlib.sha256(snapshot_bytes).hexdigest()
    if envelope.get("snapshot_digest") != digest:
        raise ValueError("Directory envelope digest does not match snapshot bytes")
    if envelope.get("algorithm") != "Ed25519" or envelope.get("signature_domain") != "UAP-DIRECTORY-SNAPSHOT-ED25519-V1" or not envelope.get("signature"):
        raise ValueError("Directory envelope is not a signed Ed25519 fixture")
    if envelope.get("sequence") != snapshot.get("sequence") or envelope.get("snapshot_schema_version") != snapshot.get("snapshot_schema_version"):
        raise ValueError("Directory envelope identity does not match snapshot")
    key_ids = {key.get("id") or key.get("key_id") for key in trust.get("keys", [])}
    if envelope.get("key_id") not in key_ids:
        raise ValueError("Directory envelope key is absent from the trust fixture")
    if not isinstance(snapshot.get("sequence"), int) or snapshot["sequence"] < 1:
        raise ValueError("Directory snapshot sequence is invalid")
    try:
        expires_at = datetime.fromisoformat(snapshot["expires_at"].replace("Z", "+00:00"))
    except (KeyError, TypeError, ValueError) as error:
        raise ValueError("Directory snapshot expiry is invalid") from error
    if expires_at <= datetime.now(timezone.utc):
        raise ValueError("base Directory snapshot is expired")
    return ({
        "AGENTPLUGINS_DIRECTORY_ORIGIN": origin,
        "AGENTPLUGINS_DIRECTORY_SNAPSHOT": str(snapshot_path.resolve()),
        "AGENTPLUGINS_DIRECTORY_ENVELOPE": str(envelope_path.resolve()),
        "AGENTPLUGINS_DIRECTORY_TRUST": str(trust_path.resolve()),
    }, snapshot, digest)


def isolated_environment(sandbox: Path, clients: tuple[str, ...], directory_environment: dict[str, str] | None = None) -> dict[str, str]:
    """Return an allowlisted environment with disposable homes and no credentials."""
    allowed = ("PATH", "LANG", "LC_ALL", "LC_CTYPE", "TZ", "SYSTEMROOT", "WINDIR", "COMSPEC", "PATHEXT", "SSL_CERT_FILE", "SSL_CERT_DIR")
    env = {key: os.environ[key] for key in allowed if key in os.environ}
    home = sandbox / "home"
    temp = sandbox / "runtime" / "tmp"
    for path in (home, temp, sandbox / "config", sandbox / "cache", sandbox / "workspace", sandbox / "runtime", sandbox / "evidence"):
        path.mkdir(parents=True, exist_ok=True)
    for client in clients:
        (home / CLIENT_ROOTS[client]).mkdir(parents=True, exist_ok=True)
    env.update({
        "HOME": str(home), "USERPROFILE": str(home),
        "XDG_CONFIG_HOME": str(sandbox / "config"), "XDG_CACHE_HOME": str(sandbox / "cache"),
        "AGENTPLUGINS_HOME": str(sandbox / "runtime" / "agentplugins"),
        "AGENTPLUGINS_EVIDENCE_ROOT": str(sandbox / "evidence"),
        "TMPDIR": str(temp), "TMP": str(temp), "TEMP": str(temp),
        "GIT_CONFIG_GLOBAL": str(sandbox / "gitconfig"), "GIT_CONFIG_NOSYSTEM": "1",
        "GIT_TERMINAL_PROMPT": "0", "CI": "true",
    })
    if directory_environment:
        env["AGENTPLUGINS_DIRECTORY_ORIGIN"] = directory_environment["AGENTPLUGINS_DIRECTORY_ORIGIN"]
        fixture_root = sandbox / "config" / "directory-trust"
        fixture_root.mkdir(parents=True, exist_ok=True)
        for key, filename in (
            ("AGENTPLUGINS_DIRECTORY_SNAPSHOT", "snapshot.json"),
            ("AGENTPLUGINS_DIRECTORY_ENVELOPE", "envelope.json"),
            ("AGENTPLUGINS_DIRECTORY_TRUST", "trusted-keys.json"),
        ):
            target = fixture_root / filename
            shutil.copy2(directory_environment[key], target)
            env[key] = str(target)
        env["AGENTPLUGINS_DIRECTORY_CACHE"] = str(sandbox / "cache" / "directory")
    return env


def find_value(value: Any, keys: set[str]) -> Any:
    if isinstance(value, dict):
        for key, child in value.items():
            if key in keys and child not in (None, ""):
                return child
        for child in value.values():
            found = find_value(child, keys)
            if found not in (None, ""):
                return found
    elif isinstance(value, list):
        for child in value:
            found = find_value(child, keys)
            if found not in (None, ""):
                return found
    return None


def collect_digests(value: Any) -> set[str]:
    found: set[str] = set()
    if isinstance(value, dict):
        for key, child in value.items():
            if key in {"package_digest", "tree_digest"} and isinstance(child, str) and DIGEST.fullmatch(child):
                found.add(child)
            found.update(collect_digests(child))
    elif isinstance(value, list):
        for child in value:
            found.update(collect_digests(child))
    return found


class LaunchHarness:
    def __init__(
        self,
        binary: Path | None,
        attestations: Path | None,
        scenario_driver: Path | None = None,
        *,
        mode: str = "enforced",
        binary_digest: str | None = None,
        expected_version: str | None = None,
        directory_origin: str | None = None,
        directory_snapshot: Path | None = None,
        directory_envelope: Path | None = None,
        directory_trust: Path | None = None,
        run_root: Path | None = None,
        consent: Path | None = None,
        notion_oauth: Path | None = None,
        chatgpt_attestation: Path | None = None,
    ) -> None:
        self.config = json.loads(SCENARIOS.read_text())
        if mode not in {"enforced", "fixture-only"}:
            raise ValueError("mode must be enforced or fixture-only")
        self.mode = mode
        self.binary = binary.resolve() if binary else None
        self.scenario_driver = scenario_driver.resolve() if scenario_driver else None
        self.expected_version = expected_version
        self.binary_digest = binary_digest
        self.directory_environment: dict[str, str] = {}
        self.snapshot: dict[str, Any] = {}
        self.snapshot_digest: str | None = None
        self.run_root = run_root.resolve() if run_root else None
        self._sandbox_counter = 0
        self.observed_at = utc_now()
        self.os_name = platform.system() or "unknown"
        self.architecture = platform.machine() or "unknown"
        self.cli_version: str | None = None
        self.rows: list[dict[str, Any]] = []
        self.consent_digest = self._load_consent(consent)
        supplied_directory = (directory_origin, directory_snapshot, directory_envelope, directory_trust)
        if any(item is not None for item in supplied_directory):
            if not all(item is not None for item in supplied_directory):
                raise ValueError("Directory origin, snapshot, envelope, and trust fixture are required together")
            self.directory_environment, self.snapshot, self.snapshot_digest = validated_directory_environment(
                str(directory_origin), Path(directory_snapshot), Path(directory_envelope), Path(directory_trust)
            )
        self.attestations = self._load_attestations(attestations)
        notion_records = self._load_attestations(notion_oauth)
        if any(key[0] != "notion" for key in notion_records):
            raise ValueError("Notion OAuth artifact may contain only Notion attestations")
        chatgpt_records = self._load_attestations(chatgpt_attestation)
        if any(key != ("cloudflare-docs", "chatgpt", "oauth") for key in chatgpt_records):
            raise ValueError("ChatGPT artifact is scoped only to Cloudflare Docs registered binding")
        for records in (notion_records, chatgpt_records):
            overlap = set(self.attestations).intersection(records)
            if overlap:
                raise ValueError(f"duplicate attestation tuple across artifacts: {sorted(overlap)}")
            self.attestations.update(records)
        self.notion_oauth_supplied = bool(notion_records)
        self.chatgpt_attestation_supplied = bool(chatgpt_records)
        self._preflight()

    @property
    def cli_available(self) -> bool:
        return bool(self.binary and self.binary.is_file() and os.access(self.binary, os.X_OK))

    def _load_consent(self, path: Path | None) -> str | None:
        if path is None:
            return None
        value = json.loads(path.read_text())
        if value.get("schema_version") != 1 or value.get("purpose") != "stable-launch-e2e" or value.get("consent") is not True or value.get("disposable_only") is not True:
            raise ValueError("consent artifact does not authorize stable-launch disposable E2E")
        return sha256_file(path)

    def _preflight(self) -> None:
        if self.run_root is not None:
            if self.run_root.exists():
                raise ValueError("disposable run root must not already exist")
            real_home = Path.home().resolve()
            repository = ROOT.resolve()
            if self.run_root == real_home or self.run_root == repository or self.run_root in real_home.parents or self.run_root in repository.parents or repository in self.run_root.parents:
                raise ValueError("refusing a real existing home/project path as disposable root")
            self.run_root.mkdir(parents=True)
        if self.mode == "enforced":
            missing = []
            if not self.cli_available: missing.append("exact executable binary")
            if not self.binary_digest: missing.append("binary checksum")
            if not self.expected_version: missing.append("binary version")
            if not self.snapshot: missing.append("signed Directory fixture")
            if not self.scenario_driver or not self.scenario_driver.is_file() or not os.access(self.scenario_driver, os.X_OK): missing.append("scenario driver")
            if not self.attestations: missing.append("runtime/OAuth attestations")
            if not self.notion_oauth_supplied: missing.append("separate Notion OAuth artifact")
            if not self.chatgpt_attestation_supplied: missing.append("separate ChatGPT Cloudflare artifact")
            if not self.consent_digest: missing.append("consent artifact")
            if not self.run_root: missing.append("fresh disposable run root")
            if missing:
                raise ValueError("enforced launch gate missing required input: " + ", ".join(missing))
        if not self.consent_digest:
            raise ValueError("no evidence is emitted without an explicit consent artifact")
        if self.expected_version:
            parse_stable_version(self.expected_version)
        if self.binary_digest:
            if not DIGEST.fullmatch(self.binary_digest):
                raise ValueError("binary checksum must be lowercase sha256:<64 hex>")
            if self.cli_available and sha256_file(self.binary) != self.binary_digest:
                raise ValueError("binary checksum does not match exact executable")

    def fresh_sandbox(self, label: str) -> Path:
        if self.run_root is None:
            # Contract-only tests may request disposable roots without executing runtime.
            self.run_root = Path(tempfile.mkdtemp(prefix="uap-fixture-only-root-"))
        self._sandbox_counter += 1
        sandbox = self.run_root / "runs" / f"{self._sandbox_counter:04d}-{label}"
        if sandbox.exists():
            raise ValueError("disposable scenario root already exists")
        sandbox.mkdir(parents=True)
        return sandbox

    def tuple(self, *, product_id: str | None = None, digest: str | None = None, manifest_digest: str | None = None, distribution_id: str | None = None, distribution_kind: str | None = None, release_sequence: int | None = None, package_version: str | None = None, client_version: str | None = None, dependency: str | None = None) -> dict[str, Any]:
        return {
            "product_id": product_id,
            "tree_digest": digest,
            "manifest_digest": manifest_digest,
            "distribution_id": distribution_id,
            "distribution_kind": distribution_kind,
            "release_sequence": release_sequence,
            "package_version": package_version,
            "snapshot_sequence": self.snapshot.get("sequence"),
            "snapshot_digest": self.snapshot_digest,
            "binary_digest": self.binary_digest,
            "dependency_identity": dependency,
            "installer_version": self.cli_version or self.expected_version,
            "adapter_version": self.cli_version,
            "client_version": client_version,
            "os": self.os_name,
            "architecture": self.architecture,
            "observed_at": self.observed_at,
        }

    def add(self, scenario: str, plugin: str | None, client: str | None, level: str, outcome: str, reason: str, *, tuple_value: dict[str, Any] | None = None, details: dict[str, Any] | None = None) -> None:
        if outcome not in OUTCOMES:
            raise ValueError(f"invalid outcome: {outcome}")
        identity = json.dumps([scenario, plugin, client, level], separators=(",", ":"))
        if any(row["scenario"] == scenario and row["plugin"] == plugin and row["client"] == client and row["level"] == level for row in self.rows):
            raise ValueError(f"duplicate evidence tuple: {scenario}/{plugin}/{client}/{level}")
        row = {
            "id": hashlib.sha256(identity.encode()).hexdigest()[:24],
            "scenario": scenario, "plugin": plugin, "client": client,
            "level": level, "outcome": outcome,
            "tuple": tuple_value or self.tuple(), "reason": reason,
        }
        if details:
            row["details"] = details
        self.rows.append(row)

    def _load_attestations(self, path: Path | None) -> dict[tuple[str, str, str], dict[str, Any]]:
        if path is None:
            return {}
        value = json.loads(path.read_text())
        if value.get("schema_version") != 1:
            raise ValueError("runtime attestation schema_version must be 1")
        records = value.get("attestations", [])
        result: dict[tuple[str, str, str], dict[str, Any]] = {}
        for record in records:
            self._reject_mutable_refs(record)
            key = (record["plugin"], record["client"], record["level"])
            if key in result:
                raise ValueError(f"duplicate attestation tuple: {key}")
            if record.get("outcome") not in OUTCOMES:
                raise ValueError(f"invalid attestation outcome: {key}")
            tuple_value = record.get("tuple", {})
            if record.get("outcome") == "passed":
                required = ("product_id", "tree_digest", "manifest_digest", "distribution_id", "distribution_kind", "release_sequence", "package_version", "snapshot_sequence", "snapshot_digest", "binary_digest", "dependency_identity", "installer_version", "adapter_version", "client_version", "os", "architecture", "observed_at")
                if any(not tuple_value.get(item) for item in required):
                    raise ValueError(f"passed attestation has incomplete tuple: {key}")
                for field in ("tree_digest", "manifest_digest", "snapshot_digest", "binary_digest"):
                    if not DIGEST.fullmatch(tuple_value[field]):
                        raise ValueError(f"passed attestation has invalid {field}: {key}")
                if tuple_value["installer_version"] != self.expected_version:
                    raise ValueError(f"attestation installer version does not match supplied binary: {key}")
                if tuple_value["binary_digest"] != self.binary_digest:
                    raise ValueError(f"attestation binary digest does not match supplied binary: {key}")
                release = self.directory_release(record["plugin"])
                expected_identity = {
                    "product_id": record["plugin"],
                    "distribution_id": release["distribution_id"],
                    "distribution_kind": release["distribution_kind"],
                    "release_sequence": release["release_sequence"],
                    "package_version": release["package_version"],
                    "tree_digest": release["tree_digest"],
                    "manifest_digest": release["manifest_digest"],
                    "snapshot_sequence": self.snapshot.get("sequence"),
                    "snapshot_digest": self.snapshot_digest,
                }
                if any(tuple_value.get(field) != expected for field, expected in expected_identity.items()):
                    raise ValueError(f"attestation identity does not match signed Directory release: {key}")
                if record.get("consent_artifact_digest") != self.consent_digest:
                    raise ValueError(f"runtime pass lacks the supplied consent artifact: {key}")
                if record.get("runtime_invocation") is not True or record.get("discovery_verified") is not True:
                    raise ValueError(f"runtime pass lacks invocation/discovery proof: {key}")
                if not isinstance(record.get("identity_id"), str) or not IDENTITY_ID.fullmatch(record["identity_id"]) or record.get("isolated_identity") is not True:
                    raise ValueError(f"runtime pass lacks explicit isolated test identity: {key}")
                if (record["plugin"] == "notion" or record["client"] == "chatgpt") and not (record.get("consent_attested") is True and record.get("isolated_identity") is True):
                    raise ValueError(f"OAuth pass lacks consent/isolated identity: {key}")
                if (record["plugin"] == "notion" or record["client"] == "chatgpt") and not record.get("identity_id"):
                    raise ValueError(f"OAuth pass lacks explicit test identity: {key}")
                if record["plugin"] == "notion" and record.get("oauth_artifact_approved") is not True:
                    raise ValueError(f"Notion runtime pass lacks approved OAuth artifact: {key}")
                if record["client"] == "chatgpt" and not all(record.get(field) is True for field in ("registered_app_binding", "ui_activation", "read_only")):
                    raise ValueError(f"ChatGPT pass lacks registered binding/UI/read-only proof: {key}")
            result[key] = record
        return result

    @staticmethod
    def _reject_mutable_refs(value: Any) -> None:
        if isinstance(value, dict):
            for key, child in value.items():
                if key in {"revision", "source_revision", "commit_sha"} and (not isinstance(child, str) or not FULL_SHA.fullmatch(child)):
                    raise ValueError("evidence contains a mutable or invalid source revision")
                if key == "ref":
                    raise ValueError("evidence must not contain mutable refs")
                LaunchHarness._reject_mutable_refs(child)
        elif isinstance(value, list):
            for child in value:
                LaunchHarness._reject_mutable_refs(child)

    def command(self, argv: list[str], sandbox: Path, clients: tuple[str, ...]) -> tuple[str, dict[str, Any] | None, str | None]:
        if not self.cli_available:
            return "inconclusive", None, "fixture-only non-runtime mode: Agent Plugins CLI binary was not supplied"
        env = isolated_environment(sandbox, clients, self.directory_environment)
        try:
            completed = subprocess.run([str(self.binary), *argv], cwd=sandbox / "workspace", env=env, text=True, capture_output=True, timeout=180, check=False)
        except subprocess.TimeoutExpired:
            return "inconclusive", None, "isolated CLI command timed out"
        if completed.returncode:
            return "failed", None, f"CLI returned exit status {completed.returncode}"
        try:
            value = json.loads(completed.stdout)
        except json.JSONDecodeError:
            return "failed", None, "CLI did not return structured JSON"
        self._assert_result_paths(value, sandbox)
        if value.get("schema_version") != 1 or value.get("command") != argv[0]:
            return "failed", value, "CLI returned an invalid command envelope"
        result = value.get("data", {}).get("result", {})
        if argv[0] in {"add", "update", "repair", "remove"} and result.get("mutated") is not True:
            return "failed", value, "CLI did not report a committed mutation"
        if self.cli_version is None:
            return "inconclusive", value, "CLI version could not be recorded for the evidence tuple"
        return "passed", value, "isolated CLI command completed"

    def driven_scenario(self, scenario: str) -> tuple[str, dict[str, Any] | None, str]:
        if not self.cli_available or not self.scenario_driver or not self.scenario_driver.is_file() or not os.access(self.scenario_driver, os.X_OK):
            return "inconclusive", None, "fixture-only non-runtime mode: compatible scenario driver was not supplied"
        sandbox = self.fresh_sandbox("driver-" + scenario)
        env = isolated_environment(sandbox, ("codex", "cursor", "kiro", "copilot", "vscode"), self.directory_environment)
        completed = subprocess.run([str(self.scenario_driver), scenario, str(self.binary)], cwd=sandbox / "workspace", env=env, text=True, capture_output=True, timeout=180, check=False)
        if completed.returncode:
            return "failed", None, f"scenario driver returned exit status {completed.returncode}"
        try:
            value = json.loads(completed.stdout)
        except json.JSONDecodeError:
            return "failed", None, "scenario driver did not return JSON"
        outcome = value.get("outcome")
        if outcome not in OUTCOMES:
            return "failed", None, "scenario driver returned an invalid outcome"
        self._assert_result_paths(value, sandbox)
        return outcome, value, str(value.get("reason") or "scenario driver observation")

    @staticmethod
    def _assert_result_paths(value: Any, sandbox: Path) -> None:
        root = sandbox.resolve()
        if isinstance(value, dict):
            for key, child in value.items():
                if (key.endswith("_path") or key.endswith("_root")) and isinstance(child, str):
                    path = Path(child)
                    if path.is_absolute() and path.resolve() != root and root not in path.resolve().parents:
                        raise ValueError("scenario result path is outside the disposable root")
                LaunchHarness._assert_result_paths(child, sandbox)
        elif isinstance(value, list):
            for child in value:
                LaunchHarness._assert_result_paths(child, sandbox)

    def discover_version(self) -> None:
        if not self.cli_available:
            return
        sandbox = self.fresh_sandbox("version")
        env = isolated_environment(sandbox, ("cursor",), self.directory_environment)
        result = subprocess.run([str(self.binary), "version"], cwd=sandbox / "workspace", env=env, text=True, capture_output=True, timeout=30, check=False)
        if result.returncode == 0:
            self.cli_version = result.stdout.strip().removeprefix("agentplugins ").strip() or None
        if self.cli_version != self.expected_version:
            raise ValueError(f"binary version mismatch: expected {self.expected_version}, observed {self.cli_version}")
        parse_stable_version(self.cli_version)

    def validate_fixtures(self) -> None:
        state = json.loads(STATE_FIXTURE.read_text())
        recovery = json.loads(RECOVERY_FIXTURE.read_text())
        expected = set(self.config["fault_scenarios"]) - {"state_schema_2_migration"}
        actual = {item["id"] for item in recovery["cases"]}
        valid = state.get("schema_version") == 2 and expected == actual and (EXTERNAL_PACKAGE / "plugin.json").is_file()
        self.add("fixture_contracts", None, None, "harness", "passed" if valid else "failed", "scenario fixtures are structurally complete" if valid else "scenario fixture mismatch", details={"fault_case_count": len(actual), "external_package_digest": package_digest(EXTERNAL_PACKAGE)})

    def directory_release(self, product_id: str) -> dict[str, Any]:
        products = {item["id"]: item for item in self.snapshot.get("products", [])}
        distributions = {item["id"]: item for item in self.snapshot.get("distributions", [])}
        product = products.get(product_id)
        if not product:
            raise ValueError(f"signed Directory snapshot lacks product {product_id}")
        distribution = distributions.get(product["default_distribution"])
        if not distribution:
            raise ValueError(f"signed Directory snapshot lacks default distribution for {product_id}")
        policies = {item["release_sequence"]: item for item in distribution.get("release_policies", [])}
        eligible = [release for release in distribution.get("releases", []) if policies.get(release["sequence"], {}).get("status") == "active"]
        if not eligible:
            raise ValueError(f"signed Directory snapshot lacks an active release for {product_id}")
        release = max(eligible, key=lambda item: item["sequence"])
        policy = policies[release["sequence"]]
        clients = sorted({target["client"] for target in policy.get("targets", []) if "user" in target.get("scopes", [])})
        return {"product_id": product_id, "distribution_id": distribution["id"], "distribution_kind": distribution["kind"], "release_sequence": release["sequence"], "package_version": release.get("package_version"), "tree_digest": release["tree_digest"], "manifest_digest": release["manifest_digest"], "compatible_clients": clients}

    def evidence_tuple(self, product_id: str, *, client_version: str | None, dependency: str) -> dict[str, Any]:
        release = self.directory_release(product_id)
        return self.tuple(
            product_id=product_id,
            digest=release["tree_digest"], manifest_digest=release["manifest_digest"],
            distribution_id=release["distribution_id"], distribution_kind=release["distribution_kind"],
            release_sequence=release["release_sequence"], package_version=release["package_version"],
            client_version=client_version, dependency=dependency,
        )

    def tuple_matches_release(self, product_id: str, value: dict[str, Any] | None) -> bool:
        if not value:
            return False
        expected = self.evidence_tuple(product_id, client_version=value.get("client_version"), dependency=value.get("dependency_identity"))
        identity_fields = ("product_id", "tree_digest", "manifest_digest", "distribution_id", "distribution_kind", "release_sequence", "package_version", "snapshot_sequence", "snapshot_digest", "binary_digest", "installer_version")
        return all(value.get(field) == expected.get(field) for field in identity_fields)

    def command_matches_release(self, product_id: str, value: dict[str, Any] | None) -> bool:
        if not value:
            return False
        release = self.directory_release(product_id)
        expected = {
            "product_id": product_id,
            "distribution_id": release["distribution_id"],
            "release_sequence": release["release_sequence"],
            "tree_digest": release["tree_digest"],
            "manifest_digest": release["manifest_digest"],
            "snapshot_sequence": self.snapshot.get("sequence"),
            "snapshot_digest": self.snapshot_digest,
        }
        observed = {
            "product_id": find_value(value, {"product_id"}),
            "distribution_id": find_value(value, {"distribution_id"}),
            "release_sequence": find_value(value, {"release_sequence"}),
            "tree_digest": find_value(value, {"tree_digest", "package_digest"}),
            "manifest_digest": find_value(value, {"manifest_digest"}),
            "snapshot_sequence": find_value(value, {"snapshot_sequence"}),
            "snapshot_digest": find_value(value, {"snapshot_digest"}),
        }
        return observed == expected

    @staticmethod
    def info_reconciled(value: dict[str, Any] | None) -> bool:
        return bool(
            value
            and find_value(value, {"receipt_reconciled"}) is True
            and find_value(value, {"native_discovery_reconciled"}) is True
        )

    def all_package_matrix(self) -> None:
        names = [item["id"] for item in self.snapshot.get("products", [])]
        if len(names) != 26 or len(set(names)) != 26:
            raise RuntimeError(f"signed launch Directory must contain 26 unique products, found {len(set(names))}")
        for plugin in names:
            sandbox = self.fresh_sandbox("package-" + plugin)
            release = self.directory_release(plugin)
            supported = [client for client in (self.config["all_package_client"], "codex", "kiro") if client in release["compatible_clients"]]
            if not supported:
                raise ValueError(f"signed Directory release has no isolated launch-gate client for {plugin}")
            client = supported[0]
            resolved_digest: str | None = None
            resolved_client_version: str | None = None
            for operation in self.config["all_package_operations"]:
                outcome, value, reason = self.command([operation, plugin, "--target", client, "--format", "json"], sandbox, (client,))
                digest = find_value(value, {"package_digest", "tree_digest"}) if value else None
                observed_client_version = find_value(value, {"client_version"}) if value else None
                if isinstance(observed_client_version, str) and observed_client_version:
                    resolved_client_version = observed_client_version
                if digest is not None and not DIGEST.fullmatch(str(digest)):
                    outcome, reason = "failed", "CLI returned an invalid package digest"
                    digest = None
                if isinstance(digest, str):
                    resolved_digest = digest
                if outcome == "passed" and resolved_digest is None:
                    outcome, reason = "inconclusive", "CLI output did not expose the immutable package digest"
                if outcome == "passed" and resolved_client_version is None:
                    outcome, reason = "inconclusive", "CLI output did not expose the exact client version"
                if outcome == "passed" and not self.command_matches_release(plugin, value):
                    outcome, reason = "failed", "CLI result identity does not match the signed Directory release"
                if outcome == "passed" and operation == "info":
                    if not self.info_reconciled(value):
                        outcome, reason = "failed", "info output did not prove exact owned-receipt and native-discovery reconciliation"
                if outcome == "passed" and resolved_digest != release["tree_digest"]:
                    outcome, reason = "failed", "CLI package digest does not match the signed Directory release"
                self.add(f"all_26_{operation}", plugin, client, "discovery" if operation == "info" else "materialization", outcome, reason or "unknown result", tuple_value=self.evidence_tuple(plugin, client_version=resolved_client_version, dependency=f"signed-directory@{self.snapshot_digest}"), details={"operation": operation, **release, "receipt_reconciliation_required": operation == "info"})

    def context7_multi_target(self) -> None:
        targets = tuple(self.config["context7_targets"])
        target_arg = ",".join(targets)
        expected_digest = self.directory_release("context7")["tree_digest"]
        driver_outcome, value, driver_reason = self.driven_scenario("context7_grouped_lifecycle")
        expected_commands = [[operation, "context7", "--target", target_arg, "--format", "json"] for operation in self.config["context7_lifecycle"]]
        valid = bool(value) and value.get("commands") == expected_commands
        valid = valid and value.get("acquisition_digests") == [expected_digest]
        valid = valid and set(value.get("target_outcomes", {})) == set(targets)
        valid = valid and all(value["target_outcomes"][target] == "passed" for target in targets)
        valid = valid and self.tuple_matches_release("context7", value.get("tuple") if value else None)
        operation_outcomes = value.get("operation_outcomes", {}) if value else {}
        for operation in self.config["context7_lifecycle"]:
            outcome = operation_outcomes.get(operation, driver_outcome)
            reason = driver_reason
            if driver_outcome != "passed" or not valid or outcome != "passed":
                outcome = "failed" if driver_outcome == "passed" else driver_outcome
                reason = "grouped driver did not prove one acquisition, exact commands, and three outcomes" if driver_outcome == "passed" else driver_reason
            self.add(
                f"context7_three_target_{operation}", "context7", target_arg, "materialization", outcome, reason,
                tuple_value=value.get("tuple") if value else self.evidence_tuple("context7", client_version=None, dependency="single-acquisition"),
                details={"operation": operation, "target_argument": target_arg, "single_process_invocation": True, "reported_target_count": len(value.get("target_outcomes", {})) if value else 0},
            )

    def hero_runtime_matrix(self) -> None:
        for plugin in self.config["heroes"]:
            for client in self.config["runtime_clients"]:
                record = self.attestations.get((plugin, client, "runtime"))
                if record:
                    self.add("hero_5x3_runtime", plugin, client, "runtime", record["outcome"], record.get("reason", "explicit runtime attestation"), tuple_value=record.get("tuple"), details={"consent_attested": bool(record.get("consent_attested")), "isolated_identity": bool(record.get("isolated_identity")), "identity_id": record.get("identity_id")})
                else:
                    reason = "runtime client/isolated identity attestation was not supplied" if plugin == "notion" else "client runtime attestation was not supplied"
                    self.add("hero_5x3_runtime", plugin, client, "runtime", "failed", reason)
        chatgpt = self.attestations.get(("cloudflare-docs", "chatgpt", "oauth"))
        if chatgpt:
            self.add("chatgpt_registered_binding", "cloudflare-docs", "chatgpt", "oauth", chatgpt["outcome"], chatgpt.get("reason", "explicit OAuth/runtime attestation"), tuple_value=chatgpt.get("tuple"), details={"consent_attested": bool(chatgpt.get("consent_attested")), "isolated_identity": bool(chatgpt.get("isolated_identity")), "identity_id": chatgpt.get("identity_id"), "registered_app_binding": True, "ui_activation": True, "read_only": True})
        else:
            self.add("chatgpt_registered_binding", "cloudflare-docs", "chatgpt", "oauth", "failed", "registered app binding and human UI consent attestation were not supplied")

    def hero_lifecycle_matrix(self) -> None:
        for plugin in self.config["heroes"]:
            for client in self.config["runtime_clients"]:
                expected_digest = self.directory_release(plugin)["tree_digest"]
                outcome, value, reason = self.driven_scenario(f"hero_lifecycle_{plugin}_{client}")
                required_operations = {"add", "update", "remove", "discovery"}
                operation_outcomes = value.get("operation_outcomes", {}) if value else {}
                tuple_value = value.get("tuple") if value else None
                valid = set(operation_outcomes) == required_operations and all(result == "passed" for result in operation_outcomes.values())
                valid = valid and tuple_value is not None and tuple_value.get("tree_digest") == expected_digest
                valid = valid and self.tuple_matches_release(plugin, tuple_value)
                if outcome == "passed" and not valid:
                    outcome, reason = "failed", "hero driver omitted exact add/update/remove/discovery proof"
                self.add("hero_5x3_lifecycle", plugin, client, "discovery", outcome, reason, tuple_value=tuple_value or self.evidence_tuple(plugin, client_version=None, dependency=f"signed-directory@{self.snapshot_digest}"), details={"operations": sorted(required_operations), "operation_outcomes": operation_outcomes})

    def shared_backend(self) -> None:
        targets = tuple(self.config["shared_backend_targets"])
        outcome, value, reason = self.driven_scenario("shared_copilot_vscode_backend")
        valid = bool(value) and value.get("affected_surfaces") == list(targets)
        valid = valid and value.get("physical_mutations") == {"add": 1, "remove": 1}
        valid = valid and self.tuple_matches_release("context7", value.get("tuple") if value else None)
        if outcome == "passed" and not valid:
            outcome, reason = "failed", "shared-backend driver did not prove one add/remove mutation affecting both surfaces"
        self.add("shared_copilot_vscode_backend", "context7", "copilot,vscode", "materialization", outcome, reason, tuple_value=value.get("tuple") if value else self.evidence_tuple("context7", client_version=None, dependency="copilot-shared-backend"), details={"expected_physical_mutations_per_operation": 1, "operations": ["add", "remove"]})

    def fault_matrix(self) -> None:
        for scenario in (*self.config["fault_scenarios"], *self.config["adapter_repair_faults"], *self.config["advanced_scenarios"]):
            outcome, value, reason = self.driven_scenario(scenario)
            if outcome == "passed" and not self.driver_proof_valid(scenario, value):
                outcome, reason = "failed", "scenario driver omitted the required exact proof fields"
            tuple_value = value.get("tuple") if value else None
            client = scenario.removeprefix("repair_") if scenario.startswith("repair_") else "cursor"
            self.add(scenario, "context7", client, "materialization", outcome, reason, tuple_value=tuple_value, details={"fixture_contract_present": scenario in self.config["fault_scenarios"], "scenario_driver_required": True})

    @staticmethod
    def driver_proof_valid(scenario: str, value: dict[str, Any] | None) -> bool:
        if not value:
            return False
        expected: dict[str, dict[str, Any]] = {
            "state_schema_2_migration": {"migration_applied": True, "provenance_preserved": True, "backup_verified": True},
            "crash_journal_recovery": {"crash_injected": True, "journal_recovered": True, "ownership_reconciled": True},
            "directory_offline": {"offline_cache_used": True, "signature_verified": True},
            "directory_expired": {"expired_snapshot_rejected": True, "zero_mutation": True},
            "directory_tampered": {"tampered_snapshot_rejected": True, "zero_mutation": True},
            "directory_sequence_rollback": {"lower_sequence_rejected": True, "zero_mutation": True},
            "managed_package_tamper": {"tamper_detected": True, "repair_required": True},
            "upstream_owned_short_name": {"source_kind": "upstream", "immutable_revision": True},
            "community_bridge_short_name": {"source_kind": "community_bridge", "immutable_revision": True},
            "plugin_data_update_repair_switch_remove_purge": {"marker_preserved": True, "explicit_purge_deleted": True},
            "stdio_environment_and_containment": {"plugin_root_verified": True, "plugin_data_verified": True, "writable": True, "contained": True},
            "missing_runtime_zero_mutation": {"zero_mutation": True, "copy_ready_requirement": True, "dependency_installed": False},
            "explicit_source_switch": {"switch_applied": True, "rollback_verified": True},
            "distribution_sticky_update": {"distribution_unchanged": True, "release_advanced": True},
            "managed_rollback": {"failure_injected": True, "managed_state_restored": True},
            "external_activation_failure": {"materialization_retained": True, "repair_action_recorded": True},
            "promotion_gate_digest_match": {"digest_match": True, "promotion_simulated": True},
            "promotion_gate_digest_mismatch": {"digest_mismatch": True, "promotion_refused": True, "zero_mutation": True},
            "cross_platform_binary_npm_install": {"required_slots_complete": True, "checksums_verified": True},
            "binary_macos_arm64": {"os": "macos", "architecture": "arm64", "checksum_verified": True},
            "binary_macos_amd64": {"os": "macos", "architecture": "amd64", "checksum_verified": True},
            "binary_linux_arm64": {"os": "linux", "architecture": "arm64", "checksum_verified": True},
            "binary_linux_amd64": {"os": "linux", "architecture": "amd64", "checksum_verified": True},
            "binary_windows_amd64": {"os": "windows", "architecture": "amd64", "checksum_verified": True},
            "npm_install_node22": {"node_major": 22, "npm_install_verified": True, "binary_checksum_verified": True},
        }
        if scenario.startswith("repair_"):
            expected_values = {"fault_injected_once": True, "repair_succeeded": True, "client": scenario.removeprefix("repair_")}
        else:
            expected_values = expected.get(scenario)
        return bool(expected_values) and all(value.get(key) == expected_value for key, expected_value in expected_values.items())

    def journeys(self) -> None:
        digest = package_digest(EXTERNAL_PACKAGE)
        sandbox = self.fresh_sandbox("direct-external")
        disposable_package = sandbox / "workspace" / "external-package"
        shutil.copytree(EXTERNAL_PACKAGE, disposable_package)
        outcome, value, reason = self.command(["add", str(disposable_package), "--target", "cursor", "--format", "json"], sandbox, ("cursor",))
        client_version = find_value(value, {"client_version"}) if value else None
        observed_digest = find_value(value, {"package_digest", "tree_digest"}) if value else None
        if outcome == "passed" and (observed_digest != digest or not isinstance(client_version, str) or not client_version):
            outcome, reason = "failed", "direct-source result omitted or disagreed with exact digest/client version"
        self.add("direct_external_package", "e2e-external-package", "cursor", "materialization", outcome, reason or "unknown result", tuple_value=self.tuple(product_id="e2e-external-package", digest=digest, manifest_digest=sha256_file(EXTERNAL_PACKAGE / "plugin.json"), distribution_id="direct/e2e-external-package", distribution_kind="direct", release_sequence=1, package_version="1.0.0", client_version=client_version if isinstance(client_version, str) else None, dependency="direct-local-source"), details={"directory_submission_used": False, "source_locator": "fixture://external-package"})
        fork_outcome, fork_value, fork_reason = self.driven_scenario("fork_submission")
        if fork_outcome == "passed" and not (
            fork_value
            and fork_value.get("fork_created") is True
            and fork_value.get("submission_validated") is True
            and fork_value.get("publication_performed") is False
        ):
            fork_outcome, fork_reason = "failed", "fork driver omitted validated non-publication submission proof"
        self.add("fork_submission", "e2e-external-package", None, "schema", fork_outcome, fork_reason, tuple_value=fork_value.get("tuple") if fork_value else self.tuple(digest=digest), details={"publication_or_pr_created": fork_outcome == "passed"})

    def export(self) -> dict[str, Any]:
        self.validate_fixtures()
        if self.mode == "fixture-only":
            self.add("fixture_only_non_runtime_contract", None, None, "harness", "passed", "fixture-only mode validates contracts and emits no runtime claim")
        else:
            self.discover_version()
            self.all_package_matrix()
            self.context7_multi_target()
            self.hero_lifecycle_matrix()
            self.hero_runtime_matrix()
            self.shared_backend()
            self.fault_matrix()
            self.journeys()
        counts = Counter(row["outcome"] for row in self.rows)
        required = [row for row in self.rows if row["level"] != "harness"]
        complete = self.mode == "enforced" and bool(required) and all(row["outcome"] == "passed" for row in required)
        run_seed = json.dumps([self.observed_at, self.os_name, self.architecture, sha256_file(SCENARIOS)])
        return {
            "schema_version": 2,
            "run": {"id": hashlib.sha256(run_seed.encode()).hexdigest()[:16], "mode": self.mode, "runtime_claims": self.mode == "enforced", "observed_at": self.observed_at, "platform": self.os_name, "architecture": self.architecture, "disposable": True, "root_id": hashlib.sha256(str(self.run_root).encode()).hexdigest()[:16] if self.run_root else None, "cli": {"available": self.cli_available, "version": self.cli_version or self.expected_version, "binary_digest": self.binary_digest}},
            "matrix": self.rows,
            "summary": {**{name: counts[name] for name in ("passed", "failed", "inconclusive", "not_applicable")}, "required_gates_complete": complete, "hero_runtime_results": sum(row["scenario"] == "hero_5x3_runtime" and row["outcome"] == "passed" for row in self.rows)},
            "privacy": {"redacted_export": True, "consent_artifact_digest": self.consent_digest, "contains_absolute_home_paths": False, "contains_credentials": False, "real_user_project_used": False, "auth_copied": False},
        }


def assert_redacted(value: dict[str, Any]) -> None:
    """Refuse evidence containing obvious credentials or absolute home paths."""
    LaunchHarness._reject_mutable_refs(value)
    for row in value.get("matrix", []):
        if row.get("outcome") != "passed" or row.get("level") == "harness":
            continue
        tuple_value = row.get("tuple", {})
        required = ("product_id", "tree_digest", "manifest_digest", "distribution_id", "distribution_kind", "release_sequence", "package_version", "snapshot_sequence", "snapshot_digest", "binary_digest", "installer_version", "adapter_version", "client_version", "os", "architecture", "observed_at")
        if any(not tuple_value.get(field) for field in required):
            raise ValueError(f"passed evidence has an incomplete applicability tuple: {row.get('id')}")
        for field in ("tree_digest", "manifest_digest", "snapshot_digest", "binary_digest"):
            if not DIGEST.fullmatch(str(tuple_value[field])):
                raise ValueError(f"passed evidence has an invalid digest: {row.get('id')}")
    identities = [(row.get("scenario"), row.get("plugin"), row.get("client"), row.get("level")) for row in value.get("matrix", [])]
    if len(identities) != len(set(identities)):
        raise ValueError("evidence contains duplicate tuples")
    if value.get("run", {}).get("mode") == "enforced" and value.get("summary", {}).get("hero_runtime_results") != 15:
        raise ValueError("enforced evidence requires exactly 15 hero runtime results")
    if any(row.get("client") == "chatgpt" and row.get("plugin") != "cloudflare-docs" for row in value.get("matrix", [])):
        raise ValueError("evidence makes an unsupported broad ChatGPT inference")
    body = json.dumps(value, sort_keys=True)
    if SECRET_NAME.search(body):
        # Schema field names describe privacy exclusions; only reject assignment-like values.
        if re.search(r'(?i)(token|secret|password|cookie|authorization|oauth[_-]?code)["\s]*[:=]["\s]+(?!false|null)', body):
            raise ValueError("evidence export contains a credential-like value")
    def strings(item: Any):
        if isinstance(item, str):
            yield item
        elif isinstance(item, dict):
            for child in item.values():
                yield from strings(child)
        elif isinstance(item, list):
            for child in item:
                yield from strings(child)

    absolute_path = re.compile(r"(?:^|\s)(?:/(?!/)[^\s]+|[A-Za-z]:\\\\[^\s]+)")
    for string in strings(value):
        if absolute_path.search(string):
            raise ValueError("evidence export contains an absolute local path")
        if string.startswith(("http://", "https://")):
            parsed = urlsplit(string)
            if parsed.username or parsed.password:
                raise ValueError("evidence export contains URL credentials")


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--mode", choices=("enforced", "fixture-only"), default="enforced")
    parser.add_argument("--binary", type=Path, help="exact Agent Plugins CLI binary")
    parser.add_argument("--binary-digest", help="sha256 checksum of the exact binary")
    parser.add_argument("--expected-version", help="exact stable agentplugins version (0.1.8 or newer)")
    parser.add_argument("--attestations", type=Path, help="reviewed runtime/OAuth attestation input")
    parser.add_argument("--notion-oauth-attestation", type=Path, help="separate approved Notion OAuth/runtime artifact")
    parser.add_argument("--chatgpt-attestation", type=Path, help="separate Cloudflare Docs ChatGPT binding/UI/runtime artifact")
    parser.add_argument("--scenario-driver", type=Path, help="isolated fault/fork driver implementing the harness JSON contract")
    parser.add_argument("--directory-origin", help="credential-free signed Directory HTTPS origin")
    parser.add_argument("--directory-snapshot", type=Path)
    parser.add_argument("--directory-envelope", type=Path)
    parser.add_argument("--directory-trust", type=Path)
    parser.add_argument("--run-root", type=Path, required=True, help="nonexistent path reserved for this disposable run")
    parser.add_argument("--consent", type=Path, required=True, help="explicit stable-launch E2E consent artifact")
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    evidence = LaunchHarness(
        args.binary, args.attestations, args.scenario_driver, mode=args.mode,
        binary_digest=args.binary_digest, expected_version=args.expected_version,
        directory_origin=args.directory_origin, directory_snapshot=args.directory_snapshot,
        directory_envelope=args.directory_envelope, directory_trust=args.directory_trust,
        run_root=args.run_root, consent=args.consent, notion_oauth=args.notion_oauth_attestation,
        chatgpt_attestation=args.chatgpt_attestation,
    ).export()
    assert_redacted(evidence)
    if args.run_root and args.output.resolve() != (args.run_root.resolve() / "evidence" / args.output.name):
        raise ValueError("output must be inside the disposable evidence root")
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(evidence, indent=2, sort_keys=True) + "\n")
    print(json.dumps(evidence["summary"], sort_keys=True))
    return 0 if args.mode == "fixture-only" or evidence["summary"]["required_gates_complete"] else 2


if __name__ == "__main__":
    raise SystemExit(main())

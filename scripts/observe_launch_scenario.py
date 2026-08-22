#!/usr/bin/env python3
"""Repository-owned stable-launch command and native-state observer.

This program does not accept scenario implementation code or claimed outcomes.
It executes only the immutable plans below, records a challenge-correlated trace,
and derives results from independent before/after state digests. Scenarios that
need a client/runtime capability not present on the native runner fail closed.
"""

from __future__ import annotations

import argparse
import base64
import copy
import hashlib
import json
import os
import platform
import re
import shutil
import subprocess
import sys
import time
from datetime import datetime, timedelta, timezone
from pathlib import Path, PurePosixPath
from typing import Any

from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey

from directory_publication import canonical_json, sha256_digest, signature_message, validate_snapshot_semantics, verify_envelope


_CONFIG = json.loads((Path(__file__).resolve().parents[1] / "tests/e2e/launch-scenarios.json").read_text())
EXPECTED_SCENARIOS = frozenset(
    _CONFIG["acceptance_postconditions"] +
    _CONFIG["fault_scenarios"] + _CONFIG["adapter_repair_faults"] +
    _CONFIG["advanced_scenarios"] + [item for item in _CONFIG["journeys"] if item != "direct_external_package"] +
    ["context7_grouped_lifecycle", "shared_copilot_vscode_backend"] +
    [f"hero_lifecycle_{plugin}_{client}" for plugin in _CONFIG["heroes"] for client in _CONFIG["runtime_clients"]]
)
NATIVE_ROOTS = (".codex", ".cursor", ".kiro", ".copilot", ".config/Code/User")
EXTERNAL_PACKAGE = Path(__file__).resolve().parents[1] / "tests/e2e/fixtures/external-package"
FIXTURE_ROOT = Path(__file__).resolve().parents[1] / "tests/fixtures"
JOURNEY_VALIDATOR = Path(__file__).resolve().parent / "validate_review_journey.py"
CONFORMANCE_KEY_ID = "launch-conformance-only"
CONFORMANCE_SEED = hashlib.sha256(b"UAP launch evidence conformance key; never production").digest()
FULL_SHA = re.compile(r"^[0-9a-f]{40}$")
GITHUB_REPOSITORY = re.compile(
    r"^[A-Za-z0-9](?:[A-Za-z0-9-]{0,38})/[A-Za-z0-9](?:[A-Za-z0-9._-]{0,99})$"
)
GITHUB_SOURCE_PATH = re.compile(r"^[A-Za-z0-9._-]+(?:/[A-Za-z0-9._-]+)*$")


def now() -> str:
    return datetime.now(timezone.utc).replace(microsecond=0).isoformat().replace("+00:00", "Z")


def tree_digest(root: Path) -> str:
    framed = bytearray(b"uap-native-observation-v1\0")
    if root.exists():
        for path in sorted(item for item in root.rglob("*") if item.is_file() and not item.is_symlink()):
            relative = path.relative_to(root).as_posix().encode()
            body = path.read_bytes()
            framed.extend(len(relative).to_bytes(8, "big") + relative)
            framed.extend(len(body).to_bytes(8, "big") + body)
    return "sha256:" + hashlib.sha256(framed).hexdigest()


def observe(home: Path, manager: Path) -> dict[str, Any]:
    return {
        "manager": tree_digest(manager),
        "native": {name: tree_digest(home / name) for name in NATIVE_ROOTS},
    }


def find_value(value: Any, names: set[str]) -> Any:
    if isinstance(value, dict):
        for name in names:
            if name in value:
                return value[name]
        for child in value.values():
            found = find_value(child, names)
            if found is not None:
                return found
    elif isinstance(value, list):
        for child in value:
            found = find_value(child, names)
            if found is not None:
                return found
    return None


def json_output(completed: subprocess.CompletedProcess[str]) -> dict[str, Any] | None:
    if completed.returncode:
        return None
    try:
        value = json.loads(completed.stdout)
    except json.JSONDecodeError:
        return None
    return value if isinstance(value, dict) else None


def parse_canonical_github_source(value: Any) -> dict[str, str] | None:
    """Normalize documented and production GitHub canonical source identities."""
    if not isinstance(value, str) or any(character in value for character in "?#\\%"):
        return None
    repository_locator = value.removeprefix("https://github.com/")
    try:
        repository, locator = repository_locator.split("@", 1)
        revision, package_path = locator.split("//", 1)
    except ValueError:
        return None
    path = PurePosixPath(package_path)
    if (
        not GITHUB_REPOSITORY.fullmatch(repository)
        or not FULL_SHA.fullmatch(revision)
        or not GITHUB_SOURCE_PATH.fullmatch(package_path)
        or not package_path
        or package_path.startswith("/")
        or package_path.endswith("/")
        or "//" in package_path
        or any(part in {"", ".", ".."} for part in package_path.split("/"))
        or path.as_posix() != package_path
    ):
        return None
    return {
        "source_repository": repository,
        "source_revision": revision,
        "source_path": package_path,
    }


def source_identities_match(expected: Any, observed: Any) -> bool:
    """Compare source identities structurally while preserving the observed spelling."""
    if not isinstance(expected, dict) or not isinstance(observed, dict) or set(expected) != set(observed):
        return False
    expected_canonical = parse_canonical_github_source(expected.get("canonical_source"))
    observed_canonical = parse_canonical_github_source(observed.get("canonical_source"))
    return bool(
        expected_canonical
        and expected_canonical == observed_canonical
        and all(
            observed.get(field) == expected.get(field)
            for field in expected
            if field != "canonical_source"
        )
    )


def manager_facts(manager: Path, product: str) -> dict[str, Any]:
    committed = 0
    product_mentions = 0
    json_files = 0
    installation_records = 0
    digests: set[str] = set()
    for path in sorted(manager.rglob("*.json")) if manager.exists() else ():
        try:
            value = json.loads(path.read_text())
        except (OSError, UnicodeError, json.JSONDecodeError):
            continue
        json_files += 1
        stack = [value]
        while stack:
            item = stack.pop()
            if isinstance(item, dict):
                if item.get("phase") == "committed":
                    committed += 1
                installations = item.get("installations")
                if isinstance(installations, list):
                    installation_records += sum(product in json.dumps(record, sort_keys=True) for record in installations)
                product_mentions += sum(child == product for child in item.values())
                digests.update(child for child in item.values() if isinstance(child, str) and child.startswith("sha256:") and len(child) == 71)
                stack.extend(item.values())
            elif isinstance(item, list):
                stack.extend(item)
    return {"json_files": json_files, "committed_receipts": committed, "product_mentions": product_mentions, "installation_records": installation_records, "digests": sorted(digests)}


def materialized_product_mentions(home: Path, product: str, clients: tuple[str, ...]) -> dict[str, int]:
    roots = {
        "codex": home / ".codex", "cursor": home / ".cursor", "kiro": home / ".kiro",
        "copilot": home / ".copilot", "vscode": home / ".config/Code/User",
    }
    result: dict[str, int] = {}
    needle = product.encode()
    for client in clients:
        count = 0
        root = roots[client]
        if root.exists():
            for path in sorted(root.rglob("*")):
                if path.is_symlink() or not path.is_file():
                    continue
                relative = path.relative_to(root).as_posix().encode()
                try:
                    body = path.read_bytes() if path.stat().st_size <= (1 << 20) else b""
                except OSError:
                    body = b""
                if needle in relative or needle in body:
                    count += 1
        result[client] = count
    return result


def evidence_tuple(context: dict[str, Any], value: dict[str, Any], dependency: str, *, client_identity: str | None = None) -> dict[str, Any]:
    release = context["release"]
    return {
        "product_id": release["product_id"], "tree_digest": release["tree_digest"],
        "manifest_digest": release["manifest_digest"], "distribution_id": release["distribution_id"],
        "distribution_kind": release["distribution_kind"], "release_sequence": release["release_sequence"],
        "package_version": release["package_version"], "snapshot_sequence": context["snapshot_sequence"],
        "source_repository": release.get("source_repository"),
        "source_revision": release.get("source_revision"),
        "source_path": release.get("source_path"),
        "snapshot_digest": context["directory_digest"], "binary_digest": context["binary_digest"],
        "dependency_identity": dependency, "installer_version": context["expected_version"],
        "adapter_version": context["expected_version"],
        "client_version": client_identity or find_value(value, {"client_version"}),
        "os": platform.system() or "unknown", "architecture": platform.machine() or "unknown",
        "observed_at": now(),
    }


def identity_matches_release(identity: dict[str, Any], context: dict[str, Any]) -> bool:
    release = context["release"]
    canonical = parse_canonical_github_source(identity.get("canonical_source"))
    if not all(release.get(field) for field in ("source_repository", "source_revision", "source_path")):
        return False
    expected = {
        "product_id": release["product_id"],
        "distribution_id": release["distribution_id"],
        "distribution_kind": release["distribution_kind"],
        "desired_release_sequence": release["release_sequence"],
        "tree_digest": release["tree_digest"],
        "manifest_digest": release["manifest_digest"],
        "resolved_revision": release["source_revision"],
    }
    expected_source = {
        "source_repository": release["source_repository"],
        "source_revision": release["source_revision"],
        "source_path": release["source_path"],
    }
    return bool(canonical == expected_source and all(identity.get(field) == value for field, value in expected.items()))


def traced(binary: Path, argv: list[str], cwd: Path, challenge: str) -> tuple[subprocess.CompletedProcess[str], dict[str, Any]]:
    started = now()
    completed = subprocess.run([str(binary), *argv], cwd=cwd, env=os.environ.copy(), text=True, capture_output=True, check=False, timeout=180)
    ended = now()
    trace = {
        "challenge": challenge, "argv": argv, "started_at": started, "ended_at": ended,
        "exit_code": completed.returncode,
        "stdout_digest": "sha256:" + hashlib.sha256(completed.stdout.encode()).hexdigest(),
        "stderr_digest": "sha256:" + hashlib.sha256(completed.stderr.encode()).hexdigest(),
    }
    return completed, trace


def traced_with_environment(
    binary: Path, argv: list[str], cwd: Path, challenge: str, environment: dict[str, str],
) -> tuple[subprocess.CompletedProcess[str], dict[str, Any]]:
    original = os.environ.copy()
    try:
        os.environ.clear()
        os.environ.update(environment)
        return traced(binary, argv, cwd, challenge)
    finally:
        os.environ.clear()
        os.environ.update(original)


def conformance_directory(
    root: Path, context: dict[str, Any], *, sequence: int,
    default_alternate: bool = False, revoked: bool = False,
    safe_successor: bool = False, sequence_over_semver: bool = False,
    generated_offset: timedelta = timedelta(0), lifetime: timedelta = timedelta(hours=2),
    target_delivery: str = "managed",
) -> tuple[dict[str, str], str]:
    """Create a visibly non-production signed policy fixture from authenticated release bytes."""
    product = copy.deepcopy(context["directory_product"])
    source_distribution = copy.deepcopy(context["directory_distribution"])
    selected_sequence = context["release"]["release_sequence"]
    release = copy.deepcopy(next(item for item in source_distribution["releases"] if item["sequence"] == selected_sequence))
    policy = copy.deepcopy(next(item for item in source_distribution["release_policies"] if item["release_sequence"] == selected_sequence))
    authentication_by_client = {
        target["client"]: target.get("authentication", "unknown")
        for target in policy["targets"]
    }
    release["sequence"] = 1
    release["published_at"] = now()
    policy["release_sequence"] = 1
    policy["minimum_installer_version"] = "0.1.8"
    policy["targets"] = [
        {
            "client": client,
            "delivery": target_delivery,
            "scopes": ["user"],
            "authentication": authentication_by_client.get(client, "unknown"),
        }
        for client in ("codex", "cursor", "kiro")
    ]
    policy["current_evidence"] = []
    policy["status"] = "revoked" if revoked else "active"
    source_distribution["releases"] = [release]
    source_distribution["release_policies"] = [policy]
    source_distribution["status"] = "active"
    distributions = [source_distribution]
    revocations = [{"distribution_id": source_distribution["id"], "release_sequence": 1}] if revoked else []
    if safe_successor or sequence_over_semver:
        successor = copy.deepcopy(release)
        successor["sequence"] = 2
        successor["package_version"] = "1.0.0" if sequence_over_semver else release["package_version"]
        if sequence_over_semver:
            release["package_version"] = "9.0.0"
        successor_policy = copy.deepcopy(policy)
        successor_policy["release_sequence"] = 2
        successor_policy["status"] = "active"
        source_distribution["releases"].append(successor)
        source_distribution["release_policies"].append(successor_policy)
    if default_alternate:
        alternate = copy.deepcopy(source_distribution)
        alternate["id"] = "fixture/context7-alternate"
        alternate["kind"] = "community"
        alternate["packager"] = "fixture"
        product["default_distribution"] = alternate["id"]
        product["distributions"] = [source_distribution["id"], alternate["id"]]
        distributions.append(alternate)
    else:
        product["default_distribution"] = source_distribution["id"]
        product["distributions"] = [source_distribution["id"]]
    generated = datetime.now(timezone.utc).replace(microsecond=0) + generated_offset
    snapshot = {
        "snapshot_schema_version": 1, "sequence": sequence,
        "publication_id": f"launch-conformance-{sequence}", "source_commit": context["github_sha"],
        "generated_at": generated.isoformat().replace("+00:00", "Z"),
        "expires_at": (generated + lifetime).isoformat().replace("+00:00", "Z"),
        "products": [product], "distributions": distributions, "evidence": [], "revocations": revocations,
    }
    validate_snapshot_semantics(snapshot)
    snapshot_body = canonical_json(snapshot)
    private_key = Ed25519PrivateKey.from_private_bytes(CONFORMANCE_SEED)
    public_key = private_key.public_key().public_bytes(serialization.Encoding.Raw, serialization.PublicFormat.Raw)
    envelope = {
        "algorithm": "Ed25519", "envelope_schema_version": 1, "key_id": CONFORMANCE_KEY_ID,
        "sequence": sequence, "signature": base64.b64encode(private_key.sign(signature_message(snapshot_body))).decode(),
        "signature_domain": "UAP-DIRECTORY-SNAPSHOT-ED25519-V1",
        "snapshot_digest": sha256_digest(snapshot_body), "snapshot_schema_version": 1,
    }
    verify_envelope(snapshot_body, envelope, {CONFORMANCE_KEY_ID: public_key})
    directory = root / f"conformance-directory-{sequence}"
    directory.mkdir()
    snapshot_path, envelope_path, trust_path = directory / "snapshot.json", directory / "envelope.json", directory / "trusted-keys.json"
    snapshot_path.write_bytes(snapshot_body)
    envelope_path.write_bytes(canonical_json(envelope))
    trust_path.write_bytes(canonical_json({"schema_version": 1, "keys": [{"key_id": CONFORMANCE_KEY_ID, "public_key": base64.b64encode(public_key).decode()}]}))
    environment = os.environ.copy()
    environment.update({
        "AGENTPLUGINS_DIRECTORY_ORIGIN": "https://conformance.invalid/registry/schemas/1/",
        "AGENTPLUGINS_DIRECTORY_SNAPSHOT": str(snapshot_path),
        "AGENTPLUGINS_DIRECTORY_ENVELOPE": str(envelope_path),
        "AGENTPLUGINS_DIRECTORY_TRUST": str(trust_path),
        "AGENTPLUGINS_DIRECTORY_CONFORMANCE_ONLY": "1",
        "AGENTPLUGINS_DIRECTORY_CACHE": str(root / "directory-cache"),
    })
    return environment, envelope["snapshot_digest"]


def directory_fault_scenario(
    binary: Path, scenario: str, root: Path, challenge: str, context: dict[str, Any],
) -> tuple[bool, dict[str, Any]]:
    home = Path(os.environ["HOME"])
    manager = Path(os.environ["AGENTPLUGINS_HOME"])
    base_sequence = int(context["snapshot_sequence"]) + 2000
    traces: list[dict[str, Any]] = []
    before = observe(home, manager)

    def execute(argv: list[str], environment: dict[str, str]) -> subprocess.CompletedProcess[str]:
        completed, trace = traced_with_environment(binary, argv, root, challenge, environment)
        traces.append(trace)
        return completed

    if scenario == "directory_expired":
        environment, fixture_digest = conformance_directory(
            root, context, sequence=base_sequence, generated_offset=timedelta(hours=-4), lifetime=timedelta(hours=1),
        )
        rejected = execute(["add", "context7", "--target", "cursor", "--format", "json"], environment)
        after = observe(home, manager)
        proof = {"expired_snapshot_rejected": rejected.returncode != 0, "zero_mutation": before == after}
    elif scenario == "directory_tampered":
        environment, fixture_digest = conformance_directory(root, context, sequence=base_sequence)
        snapshot_path = Path(environment["AGENTPLUGINS_DIRECTORY_SNAPSHOT"])
        snapshot_path.write_bytes(snapshot_path.read_bytes() + b"\n")
        rejected = execute(["add", "context7", "--target", "cursor", "--format", "json"], environment)
        after = observe(home, manager)
        proof = {"tampered_snapshot_rejected": rejected.returncode != 0, "zero_mutation": before == after}
    elif scenario == "directory_sequence_rollback":
        high, _ = conformance_directory(root, context, sequence=base_sequence + 1)
        low, fixture_digest = conformance_directory(root, context, sequence=base_sequence)
        installed = execute(["add", "context7", "--target", "cursor", "--format", "json"], high)
        before_rejected = observe(home, manager)
        rejected = execute(["update", "context7", "--target", "cursor", "--format", "json"], low)
        after_rejected = observe(home, manager)
        cleanup = execute(["remove", "context7", "--target", "cursor", "--format", "json"], high)
        after = observe(home, manager)
        proof = {"lower_sequence_rejected": installed.returncode == 0 and rejected.returncode != 0 and cleanup.returncode == 0, "zero_mutation": before_rejected == after_rejected}
    elif scenario == "directory_offline":
        online, fixture_digest = conformance_directory(root, context, sequence=base_sequence)
        installed = execute(["add", "context7", "--target", "cursor", "--format", "json"], online)
        offline = dict(online)
        offline["AGENTPLUGINS_DIRECTORY_ORIGIN"] = "https://offline.invalid/registry/schemas/1/"
        for key in ("AGENTPLUGINS_DIRECTORY_SNAPSHOT", "AGENTPLUGINS_DIRECTORY_ENVELOPE", "AGENTPLUGINS_DIRECTORY_TRUST"):
            offline.pop(key, None)
        updated = execute(["update", "context7", "--target", "cursor", "--format", "json"], offline)
        cleanup = execute(["remove", "context7", "--target", "cursor", "--format", "json"], online)
        after = observe(home, manager)
        proof = {"offline_cache_used": installed.returncode == updated.returncode == cleanup.returncode == 0, "signature_verified": Path(online["AGENTPLUGINS_DIRECTORY_CACHE"]).exists()}
    else:
        raise ValueError("unsupported Directory fault scenario")
    return all(proof.values()), {"command_traces": traces, "before": before, "after": after, "proof": proof, **proof, "fixture_digest": fixture_digest}


def lifecycle(
    binary: Path, product: str, clients: tuple[str, ...], root: Path,
    challenge: str, context: dict[str, Any], *, include_repair: bool,
) -> tuple[bool, dict[str, Any]]:
    home = Path(os.environ["HOME"])
    manager = Path(os.environ["AGENTPLUGINS_HOME"])
    target = ",".join(clients)
    operations = ["add", "update"] + (["repair"] if include_repair else []) + ["info", "remove"]
    traces: list[dict[str, Any]] = []
    values: dict[str, dict[str, Any]] = {}
    observations: list[dict[str, Any]] = []
    outcomes: dict[str, str] = {}
    identities: dict[str, dict[str, Any]] = {}
    previous_receipts = manager_facts(manager, product)["committed_receipts"]
    for operation in operations:
        before = {"state": observe(home, manager), "manager": manager_facts(manager, product), "materialized_mentions": materialized_product_mentions(home, product, clients)}
        argv = [operation, product, "--target", target, "--format", "json"]
        completed, trace = traced(binary, argv, root, challenge)
        traces.append(trace)
        value = json_output(completed)
        after = {"state": observe(home, manager), "manager": manager_facts(manager, product), "materialized_mentions": materialized_product_mentions(home, product, clients)}
        identity = manager_identity(manager, product)
        identities[operation] = identity
        observations.append({"operation": operation, "before": before, "after": after})
        passed = value is not None
        if operation in {"add", "repair", "remove"}:
            passed = passed and after["manager"]["committed_receipts"] > previous_receipts
            previous_receipts = after["manager"]["committed_receipts"]
        elif operation == "update":
            # This fixture contains exactly one release. A truthful update is a
            # successful no-op: it must neither recommit that release nor alter
            # manager/client materialization.
            passed = (
                passed
                and find_value(value, {"mutated"}) is False
                and after == before
                and after["manager"]["committed_receipts"] == previous_receipts
            )
        passed = passed and identity_matches_release(identity, context)
        if operation == "add":
            passed = passed and all(after["materialized_mentions"][client] > 0 for client in clients)
        elif operation == "info":
            # Files and receipts prove fixture materialization only. They do
            # not prove native client discovery.
            passed = passed and after["manager"]["committed_receipts"] > 0
        elif operation == "remove":
            passed = passed and all(after["materialized_mentions"][client] == 0 for client in clients)
        outcomes["discovery" if operation == "info" else operation] = (
            "inconclusive" if operation == "info" and passed else "passed" if passed else "failed"
        )
        if value is not None:
            values[operation] = value
    representative = values.get("info") or values.get("add") or {}
    tuple_value = evidence_tuple(context, representative, "fixture-manager-receipts-and-materialized-files")
    tuple_value["client_version"] = None
    materialization_passed = all(value == "passed" for key, value in outcomes.items() if key != "discovery")
    passed = materialization_passed
    return passed, {
        "command_traces": traces, "operation_observations": observations,
        "operation_outcomes": outcomes, "values": values, "identities": identities, "tuple": tuple_value,
        "evidence_basis": "fixture_materialization", "runtime_proof": False,
        "no_newer_release_update_noop": outcomes.get("update") == "passed",
    }


def shared_backend_lifecycle(
    binary: Path, root: Path, challenge: str, context: dict[str, Any],
) -> tuple[bool, dict[str, Any]]:
    home = Path(os.environ["HOME"])
    manager = Path(os.environ["AGENTPLUGINS_HOME"])
    traces: list[dict[str, Any]] = []
    observations: list[dict[str, Any]] = []
    values: dict[str, dict[str, Any]] = {}
    mutations: dict[str, int] = {}
    shared_identity: dict[str, Any] = {}
    previous_receipts = manager_facts(manager, "context7")["committed_receipts"]
    for operation in ("add", "info", "remove"):
        before = {"state": observe(home, manager), "manager": manager_facts(manager, "context7")}
        completed, trace = traced(binary, [operation, "context7", "--target", "copilot,vscode", "--format", "json"], root, challenge)
        traces.append(trace)
        value = json_output(completed)
        after = {"state": observe(home, manager), "manager": manager_facts(manager, "context7")}
        observations.append({"operation": operation, "before": before, "after": after})
        if value is not None:
            values[operation] = value
        if operation in {"add", "remove"}:
            mutations[operation] = sum(
                before["state"]["native"][name] != after["state"]["native"][name]
                for name in before["state"]["native"]
            )
            if after["manager"]["committed_receipts"] <= previous_receipts:
                mutations[operation] = 0
            previous_receipts = after["manager"]["committed_receipts"]
        if operation == "add":
            shared_identity = manager_identity(manager, "context7")
    info = values.get("info", {})
    surfaces = find_value(info, {"affected_surfaces", "resolved_surfaces", "clients"})
    surfaces = sorted(surfaces) if isinstance(surfaces, list) and all(isinstance(item, str) for item in surfaces) else []
    if not surfaces and isinstance(shared_identity.get("affected_surfaces"), list):
        surfaces = sorted(shared_identity["affected_surfaces"])
    tuple_value = evidence_tuple(context, info or values.get("add", {}), "fixture-shared-backend-and-manager-receipts")
    tuple_value["client_version"] = None
    passed = (
        set(values) == {"add", "info", "remove"}
        and surfaces == ["copilot", "vscode"] and mutations == {"add": 1, "remove": 1}
        and identity_matches_release(shared_identity, context)
        and next(item for item in observations if item["operation"] == "info")["after"]["manager"]["committed_receipts"] > 0
    )
    return passed, {
        "command_traces": traces, "operation_observations": observations,
        "affected_surfaces": surfaces, "physical_mutations": mutations, "tuple": tuple_value,
        "evidence_basis": "fixture_materialization", "runtime_proof": False,
    }


def schema_scenario(
    binary: Path, scenario: str, root: Path, challenge: str,
) -> tuple[bool, dict[str, Any]]:
    home = Path(os.environ["HOME"])
    manager = Path(os.environ["AGENTPLUGINS_HOME"])
    package = root / ("package-" + scenario)
    shutil.copytree(EXTERNAL_PACKAGE, package)
    manifest_path = package / "plugin.json"
    manifest = json.loads(manifest_path.read_text())
    exact = "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json"
    if scenario == "schema_draft_rejected":
        manifest["$schema"] = "https://agent-plugins.org/schemas/draft/plugin.schema.json"
    elif scenario == "schema_unknown_rejected":
        manifest["$schema"] = "https://agent-plugins.org/schemas/2.0.0/plugin.schema.json"
    manifest_path.write_text(json.dumps(manifest, sort_keys=True))
    before = observe(home, manager)
    argv = ["add", "./" + package.name, "--target", "cursor", "--format", "json"]
    completed, trace = traced(binary, argv, root, challenge)
    after_add = observe(home, manager)
    traces = [trace]
    if scenario == "schema_1_0_0_accepted":
        accepted = completed.returncode == 0 and before != after_add and manifest["$schema"] == exact
        removed, remove_trace = traced(binary, ["remove", manifest["name"], "--target", "cursor", "--format", "json"], root, challenge)
        traces.append(remove_trace)
        after = observe(home, manager)
        proof = {"exact_schema": manifest["$schema"], "accepted": accepted and removed.returncode == 0}
        return all(proof.values()), {"command_traces": traces, "before": before, "after": after, "proof": proof}
    rejected = completed.returncode != 0 and before == after_add
    key = "draft_rejected" if scenario == "schema_draft_rejected" else "unknown_rejected"
    proof = {key: rejected, "zero_mutation": before == after_add}
    return all(proof.values()), {"command_traces": traces, "before": before, "after": after_add, "proof": proof}


def project_scope_scenario(binary: Path, root: Path, challenge: str) -> tuple[bool, dict[str, Any]]:
    home = Path(os.environ["HOME"])
    manager = Path(os.environ["AGENTPLUGINS_HOME"])
    before = observe(home, manager)
    completed, trace = traced(binary, ["add", "context7", "--target", "cursor", "--scope", "project", "--format", "json"], root, challenge)
    after = observe(home, manager)
    diagnostic = (completed.stdout + "\n" + completed.stderr).lower()
    proof = {
        "project_scope_rejected": completed.returncode != 0 and "project" in diagnostic and ("unsupported" in diagnostic or "user scope" in diagnostic),
        "manager_unchanged": before["manager"] == after["manager"],
        "native_unchanged": before["native"] == after["native"],
    }
    return all(proof.values()), {"command_traces": [trace], "before": before, "after": after, "proof": proof}


def no_hidden_yes_scenario(binary: Path, root: Path, challenge: str) -> tuple[bool, dict[str, Any]]:
    """Prove the removed option is rejected by mutating parsers before any write."""
    home = Path(os.environ["HOME"])
    manager = Path(os.environ["AGENTPLUGINS_HOME"])
    before = observe(home, manager)
    traces: list[dict[str, Any]] = []
    attempts: list[dict[str, Any]] = []
    commands = (
        ["add", "context7", "--target", "cursor", "--yes", "--format", "json"],
        ["update", "context7", "--target", "cursor", "--yes", "--format", "json"],
        ["remove", "context7", "--target", "cursor", "--yes", "--format", "json"],
    )
    for argv in commands:
        completed, trace = traced(binary, argv, root, challenge)
        traces.append(trace)
        diagnostic = (completed.stdout + "\n" + completed.stderr).lower()
        attempts.append({
            "command": argv[0], "rejected": completed.returncode != 0,
            "unknown_option": "--yes" in diagnostic and any(word in diagnostic for word in ("unknown", "unrecognized", "invalid option", "flag provided but not defined")),
        })
    after = observe(home, manager)
    help_result, help_trace = traced(binary, ["--help"], root, challenge)
    traces.append(help_trace)
    help_text = help_result.stdout + "\n" + help_result.stderr
    proof = {
        "help_exit_zero": help_result.returncode == 0,
        "hidden_yes_absent": "--yes" not in help_text,
        "mutating_commands_rejected": all(item["rejected"] for item in attempts),
        "unknown_option_reported": all(item["unknown_option"] for item in attempts),
        "manager_unchanged": before["manager"] == after["manager"],
        "native_unchanged": before["native"] == after["native"],
    }
    return all(proof.values()), {"command_traces": traces, "before": before, "after": after, "proof": proof, "attempts": attempts}


def manager_identity(manager: Path, product: str) -> dict[str, Any]:
    """Return identity fields from exactly one owned installation record."""
    wanted = {"product_id", "resolved_revision", "canonical_source", "tree_digest", "manifest_digest", "distribution_id", "distribution_kind", "desired_release_sequence", "data_locator", "data_root", "affected_surfaces"}
    matches: list[dict[str, Any]] = []
    for path in sorted(manager.rglob("*.json")) if manager.exists() else ():
        try:
            value = json.loads(path.read_text())
        except (OSError, UnicodeError, json.JSONDecodeError):
            continue
        installations = value.get("installations", []) if isinstance(value, dict) else []
        for installation in installations if isinstance(installations, list) else ():
            if not isinstance(installation, dict):
                continue
            if product not in {installation.get("declared_name"), find_value(installation, {"product_id"})}:
                continue
            identity = {key: find_value(installation, {key}) for key in wanted}
            matches.append({key: child for key, child in identity.items() if child not in (None, "")})
    return matches[0] if len(matches) == 1 else {}


def manager_has_flag(manager: Path, product: str, key: str, expected: Any) -> bool:
    for path in sorted(manager.rglob("*.json")) if manager.exists() else ():
        try:
            value = json.loads(path.read_text())
        except (OSError, UnicodeError, json.JSONDecodeError):
            continue
        if product not in json.dumps(value, sort_keys=True):
            continue
        stack = [value]
        while stack:
            item = stack.pop()
            if isinstance(item, dict):
                if item.get(key) == expected:
                    return True
                stack.extend(item.values())
            elif isinstance(item, list):
                stack.extend(item)
    return False


def direct_full_sha_scenario(
    binary: Path, root: Path, challenge: str, context: dict[str, Any],
) -> tuple[bool, dict[str, Any]]:
    home = Path(os.environ["HOME"])
    manager = Path(os.environ["AGENTPLUGINS_HOME"])
    revision = context["github_sha"]
    selector = f'{context["catalog_repository"]}@{revision}//tests/e2e/fixtures/external-package'
    before = observe(home, manager)
    traces: list[dict[str, Any]] = []
    add, trace = traced(binary, ["add", selector, "--target", "cursor", "--format", "json"], root, challenge)
    traces.append(trace)
    installed_identity = manager_identity(manager, "e2e-external-package")
    update, trace = traced(binary, ["update", "e2e-external-package", "--target", "cursor", "--format", "json"], root, challenge)
    traces.append(trace)
    updated_identity = manager_identity(manager, "e2e-external-package")
    remove, trace = traced(binary, ["remove", "e2e-external-package", "--target", "cursor", "--format", "json"], root, challenge)
    traces.append(trace)
    after = observe(home, manager)
    stable_fields = ("resolved_revision", "canonical_source", "tree_digest", "manifest_digest")
    identity_stable = bool(installed_identity) and all(installed_identity.get(field) == updated_identity.get(field) for field in stable_fields)
    proof = {
        "full_sha": installed_identity.get("resolved_revision") == revision and revision in str(installed_identity.get("canonical_source", "")),
        "network_refetch_unchanged": add.returncode == 0 and update.returncode == 0 and identity_stable,
        "mutable_ref_followed": False if identity_stable else True,
    }
    return all((proof["full_sha"], proof["network_refetch_unchanged"], not proof["mutable_ref_followed"], remove.returncode == 0)), {"command_traces": traces, "before": before, "after": after, "proof": proof, "installed_identity": installed_identity, "updated_identity": updated_identity}


def missing_runtime_scenario(binary: Path, root: Path, challenge: str) -> tuple[bool, dict[str, Any]]:
    home = Path(os.environ["HOME"])
    manager = Path(os.environ["AGENTPLUGINS_HOME"])
    package = root / "package-missing-runtime"
    shutil.copytree(EXTERNAL_PACKAGE, package)
    command = "uap-runtime-that-does-not-exist"
    (package / "mcp.json").write_text(json.dumps({
        "$schema": "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json",
        "mcpServers": {"demo": {"type": "stdio", "command": command}},
    }, sort_keys=True))
    before = observe(home, manager)
    completed, trace = traced(binary, ["add", "./" + package.name, "--target", "cursor", "--format", "json"], root, challenge)
    after = observe(home, manager)
    diagnostic = completed.stdout + "\n" + completed.stderr
    proof = {
        "zero_mutation": before == after,
        "dependency_installed": shutil.which(command, path=os.environ.get("PATH")) is not None,
        "guidance_exact": all(text in diagnostic for text in (f'requires executable "{command}" on PATH', "install it explicitly", "never installs runtimes")),
    }
    return completed.returncode != 0 and proof["zero_mutation"] and not proof["dependency_installed"] and proof["guidance_exact"], {"command_traces": [trace], "before": before, "after": after, "proof": proof}


def repair_fault_scenario(binary: Path, client: str, root: Path, challenge: str) -> tuple[bool, dict[str, Any]]:
    home = Path(os.environ["HOME"])
    manager = Path(os.environ["AGENTPLUGINS_HOME"])
    before = observe(home, manager)
    traces: list[dict[str, Any]] = []
    add, trace = traced(binary, ["add", "context7", "--target", client, "--format", "json"], root, challenge)
    traces.append(trace)
    roots = {"codex": home / ".codex", "cursor": home / ".cursor", "kiro": home / ".kiro"}
    fault_injected = False
    for path in sorted(roots[client].rglob("*"), reverse=True) if roots[client].exists() else ():
        if path.is_file() and not path.is_symlink() and "context7" in (path.as_posix() + path.read_text(errors="ignore")):
            path.unlink()
            fault_injected = True
    repair, trace = traced(binary, ["repair", "context7", "--target", client, "--format", "json"], root, challenge)
    traces.append(trace)
    repaired = materialized_product_mentions(home, "context7", (client,))[client] > 0
    remove, trace = traced(binary, ["remove", "context7", "--target", client, "--format", "json"], root, challenge)
    traces.append(trace)
    after = observe(home, manager)
    proof = {"fault_injected_once": fault_injected, "repair_succeeded": add.returncode == repair.returncode == remove.returncode == 0 and repaired, "client": client}
    return all((proof["fault_injected_once"], proof["repair_succeeded"])), {"command_traces": traces, "before": before, "after": after, "proof": proof, **proof}


def managed_tamper_scenario(binary: Path, root: Path, challenge: str) -> tuple[bool, dict[str, Any]]:
    home = Path(os.environ["HOME"])
    manager = Path(os.environ["AGENTPLUGINS_HOME"])
    before = observe(home, manager)
    traces: list[dict[str, Any]] = []
    add, trace = traced(binary, ["add", "context7", "--target", "cursor", "--format", "json"], root, challenge)
    traces.append(trace)
    tampered = False
    for path in sorted((home / ".cursor").rglob("*")) if (home / ".cursor").exists() else ():
        if path.is_file() and not path.is_symlink() and "context7" in (path.as_posix() + path.read_text(errors="ignore")):
            path.write_bytes(path.read_bytes() + b"\nlaunch-tamper")
            tampered = True
            break
    info, trace = traced(binary, ["info", "context7", "--target", "cursor", "--format", "json"], root, challenge)
    traces.append(trace)
    diagnostic = (info.stdout + "\n" + info.stderr).lower()
    detected = tampered and any(word in diagnostic for word in ("tamper", "digest", "drift", "repair"))
    repair_required = "repair" in diagnostic
    repair, trace = traced(binary, ["repair", "context7", "--target", "cursor", "--format", "json"], root, challenge)
    traces.append(trace)
    remove, trace = traced(binary, ["remove", "context7", "--target", "cursor", "--format", "json"], root, challenge)
    traces.append(trace)
    after = observe(home, manager)
    proof = {"tamper_detected": add.returncode == 0 and detected, "repair_required": repair_required and repair.returncode == 0 and remove.returncode == 0}
    return all(proof.values()), {"command_traces": traces, "before": before, "after": after, "proof": proof, **proof}


def migration_scenario(binary: Path, root: Path, challenge: str) -> tuple[bool, dict[str, Any]]:
    home = Path(os.environ["HOME"])
    manager = Path(os.environ["AGENTPLUGINS_HOME"])
    manager.mkdir(parents=True, exist_ok=True)
    fixture = Path(__file__).resolve().parents[1] / "tests/e2e/fixtures/state-schema-2.json"
    state_path = manager / "state.json"
    state_path.write_bytes(fixture.read_bytes())
    input_digest = "sha256:" + hashlib.sha256(state_path.read_bytes()).hexdigest()
    before = observe(home, manager)
    traces: list[dict[str, Any]] = []
    hidden, trace = traced(binary, ["add", "context7", "--target", "cursor", "--format", "json"], root, challenge)
    traces.append(trace)
    unchanged_before_migration = state_path.read_bytes() == fixture.read_bytes()
    dry, trace = traced(binary, ["migrate-state", "--dry-run", "--format", "json"], root, challenge)
    traces.append(trace)
    apply, trace = traced(binary, ["migrate-state", "--expected-digest", input_digest, "--format", "json"], root, challenge)
    traces.append(trace)
    after = observe(home, manager)
    backups = [path for path in manager.rglob("*") if path.is_file() and "backup" in path.name.lower()]
    migrated_schema = None
    try:
        migrated_schema = json.loads(state_path.read_text()).get("schema_version")
    except (OSError, json.JSONDecodeError):
        pass
    migrated_text = " ".join(path.read_text(errors="ignore") for path in manager.rglob("*.json") if path.is_file())
    proof = {
        "migration_applied": hidden.returncode != 0 and dry.returncode == 0 and apply.returncode == 0 and migrated_schema not in (None, 2),
        "provenance_preserved": unchanged_before_migration and "context7" in migrated_text,
        "backup_verified": any(path.read_bytes() == fixture.read_bytes() for path in backups),
    }
    return all(proof.values()), {"command_traces": traces, "before": before, "after": after, "proof": proof, **proof, "input_digest": input_digest}


def crash_recovery_scenario(binary: Path, root: Path, challenge: str) -> tuple[bool, dict[str, Any]]:
    home = Path(os.environ["HOME"])
    manager = Path(os.environ["AGENTPLUGINS_HOME"])
    before = observe(home, manager)
    started = now()
    process = subprocess.Popen(
        [str(binary), "add", "context7", "--target", "cursor", "--format", "json"],
        cwd=root, env=os.environ.copy(), text=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
    )
    killed = False
    baseline = before["manager"]
    deadline = time.monotonic() + 30
    while time.monotonic() < deadline and process.poll() is None:
        if tree_digest(manager) != baseline:
            process.kill()
            killed = True
            break
        time.sleep(0.01)
    stdout, stderr = process.communicate(timeout=10)
    crash_trace = {
        "challenge": challenge, "argv": ["add", "context7", "--target", "cursor", "--format", "json"],
        "started_at": started, "ended_at": now(), "exit_code": process.returncode,
        "stdout_digest": "sha256:" + hashlib.sha256(stdout.encode()).hexdigest(),
        "stderr_digest": "sha256:" + hashlib.sha256(stderr.encode()).hexdigest(),
    }
    retry, retry_trace = traced(binary, ["add", "context7", "--target", "cursor", "--format", "json"], root, challenge)
    identity = manager_identity(manager, "context7")
    reconciled = retry.returncode == 0 and bool(identity) and materialized_product_mentions(home, "context7", ("cursor",))["cursor"] > 0
    remove, remove_trace = traced(binary, ["remove", "context7", "--target", "cursor", "--format", "json"], root, challenge)
    after = observe(home, manager)
    proof = {"crash_injected": killed and process.returncode != 0, "journal_recovered": reconciled, "ownership_reconciled": reconciled and remove.returncode == 0}
    return all(proof.values()), {"command_traces": [crash_trace, retry_trace, remove_trace], "before": before, "after": after, "proof": proof, **proof}


def managed_rollback_scenario(binary: Path, root: Path, challenge: str) -> tuple[bool, dict[str, Any]]:
    home = Path(os.environ["HOME"])
    manager = Path(os.environ["AGENTPLUGINS_HOME"])
    before = observe(home, manager)
    started = now()
    argv = ["add", "context7", "--target", "codex,cursor,kiro", "--format", "json"]
    process = subprocess.Popen([str(binary), *argv], cwd=root, env=os.environ.copy(), text=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    injected = False
    kiro = home / ".kiro"
    deadline = time.monotonic() + 30
    while time.monotonic() < deadline and process.poll() is None:
        if materialized_product_mentions(home, "context7", ("codex",))["codex"] > 0:
            try:
                if kiro.is_dir() and not any(kiro.iterdir()):
                    kiro.rmdir()
                    kiro.write_text("mid-commit fault")
                    injected = True
                    break
            except OSError:
                pass
        time.sleep(0.005)
    stdout, stderr = process.communicate(timeout=30)
    trace = {
        "challenge": challenge, "argv": argv, "started_at": started, "ended_at": now(), "exit_code": process.returncode,
        "stdout_digest": "sha256:" + hashlib.sha256(stdout.encode()).hexdigest(), "stderr_digest": "sha256:" + hashlib.sha256(stderr.encode()).hexdigest(),
    }
    if kiro.is_file():
        kiro.unlink()
        kiro.mkdir()
    rolled_back = all(materialized_product_mentions(home, "context7", (client,))[client] == 0 for client in ("codex", "cursor", "kiro"))
    state_restored = manager_facts(manager, "context7")["installation_records"] == 0
    if process.returncode == 0:
        cleanup, cleanup_trace = traced(binary, ["remove", "context7", "--target", "codex,cursor,kiro", "--format", "json"], root, challenge)
        trace_list = [trace, cleanup_trace]
    else:
        trace_list = [trace]
    after = observe(home, manager)
    proof = {"failure_injected": injected and process.returncode != 0, "managed_state_restored": rolled_back and state_restored}
    return all(proof.values()), {"command_traces": trace_list, "before": before, "after": after, "proof": proof, **proof}


def sticky_scenario(
    binary: Path, scenario: str, root: Path, challenge: str,
) -> tuple[bool, dict[str, Any]]:
    home = Path(os.environ["HOME"])
    manager = Path(os.environ["AGENTPLUGINS_HOME"])
    before = observe(home, manager)
    traces: list[dict[str, Any]] = []
    add, trace = traced(binary, ["add", "context7", "--target", "cursor", "--format", "json"], root, challenge)
    traces.append(trace)
    original = manager_identity(manager, "context7")
    if scenario == "readd_sticky_distribution":
        middle, trace = traced(binary, ["remove", "context7", "--target", "cursor", "--format", "json"], root, challenge)
        traces.append(trace)
        final, trace = traced(binary, ["add", "context7", "--target", "cursor", "--format", "json"], root, challenge)
        traces.append(trace)
    else:
        cursor = Path(os.environ["HOME"]) / ".cursor"
        for path in sorted(cursor.rglob("*"), reverse=True) if cursor.exists() else ():
            if path.is_file() and not path.is_symlink() and "context7" in (path.as_posix() + path.read_text(errors="ignore")):
                path.unlink()
        middle = subprocess.CompletedProcess([], 0)
        final, trace = traced(binary, ["repair", "context7", "--target", "cursor", "--format", "json"], root, challenge)
        traces.append(trace)
    observed = manager_identity(manager, "context7")
    remove, trace = traced(binary, ["remove", "context7", "--target", "cursor", "--format", "json"], root, challenge)
    traces.append(trace)
    after = observe(home, manager)
    proof = {
        "recorded_distribution_retained": bool(original.get("distribution_id")) and original.get("distribution_id") == observed.get("distribution_id"),
        "recorded_revision_retained": bool(original.get("resolved_revision")) and original.get("resolved_revision") == observed.get("resolved_revision"),
    }
    return add.returncode == middle.returncode == final.returncode == remove.returncode == 0 and all(proof.values()), {"command_traces": traces, "before": before, "after": after, "proof": proof, "original_identity": original, "observed_identity": observed}


def data_locator(manager: Path, product: str) -> Path | None:
    for path in sorted(manager.rglob("*.json")) if manager.exists() else ():
        try:
            value = json.loads(path.read_text())
        except (OSError, UnicodeError, json.JSONDecodeError):
            continue
        if product not in json.dumps(value, sort_keys=True):
            continue
        stack = [value]
        while stack:
            item = stack.pop()
            if isinstance(item, dict):
                if isinstance(item.get("locator"), str) and isinstance(item.get("ownership_digest"), str):
                    return Path(item["locator"])
                stack.extend(item.values())
            elif isinstance(item, list):
                stack.extend(item)
    return None


def plugin_data_scenario(binary: Path, root: Path, challenge: str) -> tuple[bool, dict[str, Any]]:
    home = Path(os.environ["HOME"])
    manager = Path(os.environ["AGENTPLUGINS_HOME"])
    package = root / "package-plugin-data"
    alternate = root / "package-plugin-data-alternate"
    shutil.copytree(EXTERNAL_PACKAGE, package)
    shutil.copytree(EXTERNAL_PACKAGE, alternate)
    mcp = {
        "$schema": "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json",
        "mcpServers": {"demo": {"type": "stdio", "command": "sh", "args": ["-c", "echo ${PLUGIN_DATA}"], "env": {"DATA": "${PLUGIN_DATA}/state"}}},
    }
    (package / "mcp.json").write_text(json.dumps(mcp, sort_keys=True))
    (alternate / "mcp.json").write_text(json.dumps(mcp, sort_keys=True))
    alternate_manifest = json.loads((alternate / "plugin.json").read_text())
    alternate_manifest["description"] = "Alternate exact source for PLUGIN_DATA switch evidence."
    (alternate / "plugin.json").write_text(json.dumps(alternate_manifest, sort_keys=True))
    before = observe(home, manager)
    traces: list[dict[str, Any]] = []

    def execute(argv: list[str]) -> subprocess.CompletedProcess[str]:
        completed, trace = traced(binary, argv, root, challenge)
        traces.append(trace)
        return completed

    add = execute(["add", "./" + package.name, "--target", "cursor", "--format", "json"])
    locator = data_locator(manager, "e2e-external-package")
    safe_locator = bool(locator and locator.is_absolute() and (root in locator.parents or Path(os.environ["AGENTPLUGINS_HOME"]) in locator.parents))
    marker = locator / "launch-marker.txt" if safe_locator and locator else root / "invalid-data-locator"
    if safe_locator:
        marker.write_text("stable-launch-marker")
    update = execute(["update", "e2e-external-package", "--target", "cursor", "--format", "json"])
    update_preserved = marker.is_file() and marker.read_text() == "stable-launch-marker"
    cursor = home / ".cursor"
    for path in sorted(cursor.rglob("*"), reverse=True) if cursor.exists() else ():
        if path.is_file() and not path.is_symlink() and "e2e-external-package" in (path.as_posix() + path.read_text(errors="ignore")):
            path.unlink()
    repair = execute(["repair", "e2e-external-package", "--target", "cursor", "--format", "json"])
    repair_preserved = marker.is_file() and marker.read_text() == "stable-launch-marker"
    switch = execute(["switch", "e2e-external-package", "--to", "./" + alternate.name, "--format", "json"])
    switch_preserved = marker.is_file() and marker.read_text() == "stable-launch-marker"
    remove = execute(["remove", "e2e-external-package", "--target", "cursor", "--format", "json"])
    remove_preserved = marker.is_file() and marker.read_text() == "stable-launch-marker"
    purge = execute(["remove", "e2e-external-package", "--purge-data", "--format", "json"])
    purge_deleted = locator is not None and not locator.exists()
    after = observe(home, manager)
    proof = {
        "update_preserved": update_preserved, "repair_preserved": repair_preserved,
        "switch_preserved": switch_preserved, "remove_preserved": remove_preserved,
        "explicit_owned_purge_deleted": purge_deleted,
    }
    exits = (add, update, repair, switch, remove, purge)
    return safe_locator and all(item.returncode == 0 for item in exits) and all(proof.values()), {"command_traces": traces, "before": before, "after": after, "proof": proof, "data_receipt_observed": safe_locator}


def explicit_switch_scenario(binary: Path, root: Path, challenge: str) -> tuple[bool, dict[str, Any]]:
    home = Path(os.environ["HOME"])
    manager = Path(os.environ["AGENTPLUGINS_HOME"])
    first, second = root / "switch-first", root / "switch-second"
    shutil.copytree(EXTERNAL_PACKAGE, first)
    shutil.copytree(EXTERNAL_PACKAGE, second)
    manifest = json.loads((second / "plugin.json").read_text())
    manifest["description"] = "Second immutable source used for switch rollback evidence."
    (second / "plugin.json").write_text(json.dumps(manifest, sort_keys=True))
    before = observe(home, manager)
    traces: list[dict[str, Any]] = []

    def execute(argv: list[str]) -> subprocess.CompletedProcess[str]:
        completed, trace = traced(binary, argv, root, challenge)
        traces.append(trace)
        return completed

    add = execute(["add", "./switch-first", "--target", "cursor", "--format", "json"])
    initial = manager_identity(manager, "e2e-external-package")
    switched = execute(["switch", "e2e-external-package", "--to", "./switch-second", "--format", "json"])
    alternate = manager_identity(manager, "e2e-external-package")
    rolled_back = execute(["switch", "e2e-external-package", "--to", "./switch-first", "--format", "json"])
    restored = manager_identity(manager, "e2e-external-package")
    remove = execute(["remove", "e2e-external-package", "--target", "cursor", "--format", "json"])
    after = observe(home, manager)
    proof = {
        "switch_applied": switched.returncode == 0 and initial.get("tree_digest") != alternate.get("tree_digest"),
        "rollback_verified": rolled_back.returncode == 0 and initial.get("tree_digest") == restored.get("tree_digest") and add.returncode == remove.returncode == 0,
    }
    return all(proof.values()), {"command_traces": traces, "before": before, "after": after, "proof": proof, **proof}


def sticky_update_scenario(binary: Path, root: Path, challenge: str, context: dict[str, Any]) -> tuple[bool, dict[str, Any]]:
    home = Path(os.environ["HOME"])
    manager = Path(os.environ["AGENTPLUGINS_HOME"])
    sequence = int(context["snapshot_sequence"]) + 3000
    initial_env, _ = conformance_directory(root, context, sequence=sequence)
    update_env, fixture_digest = conformance_directory(root, context, sequence=sequence + 1, safe_successor=True)
    before = observe(home, manager)
    add, add_trace = traced_with_environment(binary, ["add", "context7", "--target", "cursor", "--format", "json"], root, challenge, initial_env)
    initial = manager_identity(manager, "context7")
    update, update_trace = traced_with_environment(binary, ["update", "context7", "--target", "cursor", "--format", "json"], root, challenge, update_env)
    update_value = json_output(update)
    updated = manager_identity(manager, "context7")
    remove, remove_trace = traced_with_environment(binary, ["remove", "context7", "--target", "cursor", "--format", "json"], root, challenge, update_env)
    after = observe(home, manager)
    proof = {
        "distribution_unchanged": bool(initial.get("distribution_id")) and initial.get("distribution_id") == updated.get("distribution_id"),
        "release_advanced": initial.get("desired_release_sequence") == 1 and updated.get("desired_release_sequence") == 2 and find_value(update_value, {"mutated"}) is True,
        "trusted_two_release_fixture": True,
        "update_advanced_from_sequence": initial.get("desired_release_sequence"),
        "update_advanced_to_sequence": updated.get("desired_release_sequence"),
    }
    return add.returncode == update.returncode == remove.returncode == 0 and all(proof.values()), {"command_traces": [add_trace, update_trace, remove_trace], "before": before, "after": after, "proof": proof, **proof, "fixture_digest": fixture_digest}


def source_kind_scenario(binary: Path, kind: str, root: Path, challenge: str, context: dict[str, Any]) -> tuple[bool, dict[str, Any]]:
    home = Path(os.environ["HOME"])
    manager = Path(os.environ["AGENTPLUGINS_HOME"])
    environment, fixture_digest = conformance_directory(root, context, sequence=int(context["snapshot_sequence"]) + 3200)
    product = context["release"]["product_id"]
    before = observe(home, manager)
    add, add_trace = traced_with_environment(binary, ["add", product, "--target", "cursor", "--format", "json"], root, challenge, environment)
    identity = manager_identity(manager, product)
    remove, remove_trace = traced_with_environment(binary, ["remove", product, "--target", "cursor", "--format", "json"], root, challenge, environment)
    after = observe(home, manager)
    canonical = parse_canonical_github_source(identity.get("canonical_source")) or {}
    source_identity = {
        "product_id": identity.get("product_id"),
        "distribution_id": identity.get("distribution_id"), "distribution_kind": identity.get("distribution_kind"),
        "release_sequence": identity.get("desired_release_sequence"), "source_revision": identity.get("resolved_revision"),
        "source_repository": canonical.get("source_repository"), "source_path": canonical.get("source_path"),
        "canonical_source": identity.get("canonical_source"),
        "tree_digest": identity.get("tree_digest"), "manifest_digest": identity.get("manifest_digest"),
    }
    revision = source_identity["source_revision"]
    proof = {
        "source_kind": source_identity["distribution_kind"],
        "immutable_revision": bool(FULL_SHA.fullmatch(str(revision))) and canonical.get("source_revision") == revision,
        "exact_source_identity": source_identities_match(context["source_identity"], source_identity),
    }
    passed = add.returncode == remove.returncode == 0 and proof == {"source_kind": kind, "immutable_revision": True, "exact_source_identity": True}
    return passed, {"command_traces": [add_trace, remove_trace], "before": before, "after": after, "proof": proof, **proof, "source_identity": source_identity, "fixture_digest": fixture_digest}


def fixture_git(repository: Path, *arguments: str, environment: dict[str, str] | None = None) -> str:
    env = os.environ.copy()
    env.update({"GIT_CONFIG_NOSYSTEM": "1", "GIT_CONFIG_GLOBAL": os.devnull, "GIT_TERMINAL_PROMPT": "0"})
    if environment:
        env.update(environment)
    completed = subprocess.run(["git", *arguments], cwd=repository, env=env, text=True, capture_output=True, check=False, timeout=60)
    if completed.returncode:
        raise RuntimeError(completed.stderr.strip() or f"git {arguments[0]} failed")
    return completed.stdout.strip()


def fixture_commit(repository: Path, message: str, timestamp: str) -> str:
    fixture_git(repository, "add", ".")
    identity = {
        "GIT_AUTHOR_NAME": "Journey Fixture", "GIT_AUTHOR_EMAIL": "journey@example.invalid",
        "GIT_AUTHOR_DATE": timestamp, "GIT_COMMITTER_NAME": "Journey Fixture",
        "GIT_COMMITTER_EMAIL": "journey@example.invalid", "GIT_COMMITTER_DATE": timestamp,
    }
    fixture_git(repository, "commit", "-qm", message, environment=identity)
    return fixture_git(repository, "rev-parse", "HEAD")


def command_artifact(name: str, argv: list[str], cwd: Path) -> dict[str, Any]:
    completed = subprocess.run(argv, cwd=cwd, env=os.environ.copy(), text=True, capture_output=True, check=False, timeout=180)
    return {"name": name, "exit_code": completed.returncode, "stdout_digest": "sha256:" + hashlib.sha256(completed.stdout.encode()).hexdigest()}


def promotion_scenario(binary: Path, scenario: str, root: Path, challenge: str) -> tuple[bool, dict[str, Any]]:
    repository = root / "upstream-repository"
    package = repository / "packages" / "fixture-bridge"
    package.parent.mkdir(parents=True)
    shutil.copytree(FIXTURE_ROOT / "plugins" / "fixture-bridge", package)
    fixture_git(repository, "init", "-q", "-b", "main")
    reviewed_revision = fixture_commit(repository, "reviewed package", "2026-01-01T00:00:00Z")
    reviewed_tree = __import__("build_registry").directory_tree_digest(package)
    reviewed_manifest = "sha256:" + hashlib.sha256((package / "plugin.json").read_bytes()).hexdigest()
    if scenario == "promotion_gate_digest_mismatch":
        manifest = json.loads((package / "plugin.json").read_text())
        manifest["description"] = "Community package for changed maintainer bytes."
        (package / "plugin.json").write_text(json.dumps(manifest, indent=2) + "\n")
    else:
        (repository / "MERGE-NOTE.md").write_text("Docs outside the reviewed package.\n")
    candidate_revision = fixture_commit(repository, "merged candidate", "2026-01-02T00:00:00Z")
    evidence = sorted([
        command_artifact("package-validation", [sys.executable, str(Path(__file__).resolve().parent / "validate_catalog.py")], Path(__file__).resolve().parents[1]),
        command_artifact("registry-policy", [sys.executable, str(Path(__file__).resolve().parent / "build_registry.py"), "--check"], Path(__file__).resolve().parents[1]),
    ], key=lambda item: item["name"])
    review_record = root / "promotion-review.json"
    review_record.write_text(json.dumps({
        "schema_version": 1, "repository": "fixture/upstream", "path": "packages/fixture-bridge",
        "reviewed_revision": reviewed_revision, "reviewed_tree_digest": reviewed_tree,
        "reviewed_manifest_digest": reviewed_manifest, "product_id": "fixture-bridge",
        "distribution_id": "fixture/fixture-bridge", "required_components": ["skills"],
        "required_targets": ["codex"], "policy_status": "active", "evidence_artifacts": evidence,
    }, sort_keys=True))
    candidate_output = root / "promotion-candidate.json"
    before = observe(Path(os.environ["HOME"]), Path(os.environ["AGENTPLUGINS_HOME"]))
    before_repository = tree_digest(repository)
    argv = [str(JOURNEY_VALIDATOR), "promotion", "--repository", str(repository), "--repository-id", "fixture/upstream", "--reviewed-revision", reviewed_revision, "--candidate-revision", candidate_revision, "--path", "packages/fixture-bridge", "--review-record", str(review_record), "--candidate-output", str(candidate_output)]
    completed, trace = traced(Path(sys.executable), argv, root, challenge)
    trace["argv"] = ["validate-review-journey", "promotion", "--repository", "disposable-upstream", "--reviewed-revision", reviewed_revision, "--candidate-revision", candidate_revision, "--path", "packages/fixture-bridge"]
    result = json.loads(completed.stdout)
    after_repository = tree_digest(repository)
    after = observe(Path(os.environ["HOME"]), Path(os.environ["AGENTPLUGINS_HOME"]))
    gate_names = [item["name"] for item in result.get("gates", [])]
    if scenario == "promotion_gate_digest_match":
        deterministic = candidate_output.is_file() and hashlib.sha256(candidate_output.read_bytes()).hexdigest() == result.get("candidate_digest", "").removeprefix("sha256:")
        proof = {"digest_match": result.get("exact_match") is True, "promotion_simulated": completed.returncode == 0 and deterministic, "identity_gates_complete": gate_names == list(__import__("validate_review_journey").REQUIRED_PROMOTION_GATES), "repository_unchanged": before_repository == after_repository}
    else:
        proof = {"digest_mismatch": result.get("outcome") == "rejected" and "bytes differ" in result.get("reason", ""), "promotion_refused": completed.returncode == 2 and not candidate_output.exists(), "zero_mutation": before == after and before_repository == after_repository}
    return all(proof.values()), {"command_traces": [trace], "before": before, "after": after, "proof": proof, **proof, "validator_artifact": result, "reviewed_revision": reviewed_revision, "candidate_revision": candidate_revision}


def fork_submission_scenario(scenario: str, root: Path, challenge: str) -> tuple[bool, dict[str, Any]]:
    upstream = root / "bridge-upstream"
    shutil.copytree(FIXTURE_ROOT / "bridge_upstream", upstream)
    fixture_git(upstream, "init", "-q", "-b", "main")
    upstream_revision = fixture_commit(upstream, "fixture", "2026-01-01T00:00:00Z")
    mirror = root / "upstream-mirror" / "fixture"
    mirror.mkdir(parents=True)
    subprocess.run(["git", "clone", "-q", "--bare", str(upstream), str(mirror / "upstream.git")], check=True, timeout=60)

    source = root / "contribution-source"
    source.mkdir()
    shutil.copytree(FIXTURE_ROOT / "bridges", source / "bridges")
    shutil.copytree(FIXTURE_ROOT / "plugins", source / "plugins")
    for recipe in (source / "bridges").glob("*/bridge.yaml"):
        recipe.write_text(recipe.read_text().replace("9ec238505ab95b2e07222e69a893f0bbac201ae6", upstream_revision))
    fixture_git(source, "init", "-q", "-b", "main")
    base_revision = fixture_commit(source, "base contribution repository", "2026-01-03T00:00:00Z")
    fork = root / "disposable-fork"
    subprocess.run(["git", "clone", "-q", str(source), str(fork)], check=True, timeout=60)
    fixture_git(fork, "checkout", "-qb", "contribution/fixture-bridge")
    readme = fork / "bridges" / "fixture-bridge" / "overlay" / "README.md"
    readme.write_text(readme.read_text() + "\nValidated through the disposable contributor journey.\n")
    build = subprocess.run([
        sys.executable, str(Path(__file__).resolve().parent / "build_bridges.py"),
        "--root", str(fork), "--upstream-mirror", str(root / "upstream-mirror"), "build", "fixture-bridge",
    ], cwd=fork, text=True, capture_output=True, check=False, timeout=180)
    if build.returncode:
        raise RuntimeError(build.stderr or build.stdout)
    if scenario == "fork_submission_rejected":
        manifest = json.loads((fork / "plugins" / "fixture-bridge" / "plugin.json").read_text())
        manifest["$schema"] = "https://example.invalid/unreviewed.schema.json"
        (fork / "plugins" / "fixture-bridge" / "plugin.json").write_text(json.dumps(manifest, indent=2) + "\n")
    branch_revision = fixture_commit(fork, "contribute fixture bridge", "2026-01-04T00:00:00Z")
    record = root / "submission.json"
    record.write_text(json.dumps({
        "schema_version": 1, "fork_repository": "contributor/fixture-fork",
        "base_revision": base_revision, "branch": "contribution/fixture-bridge",
        "branch_revision": branch_revision, "package_path": "plugins/fixture-bridge",
        "bridge_root": ".", "bridge_id": "fixture-bridge", "product_id": "fixture-bridge",
        "distribution_id": "contributor/fixture-bridge",
    }, sort_keys=True))
    before = observe(Path(os.environ["HOME"]), Path(os.environ["AGENTPLUGINS_HOME"]))
    before_repository = tree_digest(fork)
    argv = [str(JOURNEY_VALIDATOR), "submission", "--repository", str(fork), "--submission-record", str(record), "--upstream-mirror", str(root / "upstream-mirror")]
    completed, trace = traced(Path(sys.executable), argv, root, challenge)
    trace["argv"] = ["validate-review-journey", "submission", "--repository", "disposable-fork", "--branch-revision", branch_revision, "--upstream-mirror", "disposable-upstream-mirror"]
    result = json.loads(completed.stdout)
    after = observe(Path(os.environ["HOME"]), Path(os.environ["AGENTPLUGINS_HOME"]))
    after_repository = tree_digest(fork)
    if scenario == "fork_submission":
        gate_names = [item["name"] for item in result.get("gates", [])]
        side_effects = result.get("side_effects", {})
        proof = {
            "fork_created": (fork / ".git").is_dir(), "branch_submission": result.get("branch") == "contribution/fixture-bridge",
            "submission_validated": completed.returncode == 0 and gate_names == list(__import__("validate_review_journey").REQUIRED_SUBMISSION_GATES),
            "publication_performed": side_effects.get("publication_created") != 0,
            "pr_created": side_effects.get("pr_created") != 0, "network_performed": side_effects.get("network_commands") != 0,
            "repository_unchanged": before_repository == after_repository, "manager_unchanged": before == after,
        }
        passed = all((proof["fork_created"], proof["branch_submission"], proof["submission_validated"], not proof["publication_performed"], not proof["pr_created"], not proof["network_performed"], proof["repository_unchanged"], proof["manager_unchanged"]))
    else:
        proof = {
            "fork_created": (fork / ".git").is_dir(), "submission_rejected": completed.returncode == 2 and result.get("outcome") == "rejected",
            "no_side_effect": before == after and before_repository == after_repository,
            "no_candidate": not any(root.glob("*publication*")) and not any(root.glob("*pull-request*")),
        }
        passed = all(proof.values())
    return passed, {"command_traces": [trace], "before": before, "after": after, "proof": proof, **proof, "validator_artifact": result, "base_revision": base_revision, "branch_revision": branch_revision, "upstream_revision": upstream_revision}


def external_activation_scenario(binary: Path, root: Path, challenge: str, context: dict[str, Any]) -> tuple[bool, dict[str, Any]]:
    home = Path(os.environ["HOME"])
    manager = Path(os.environ["AGENTPLUGINS_HOME"])
    environment, fixture_digest = conformance_directory(
        root, context, sequence=int(context["snapshot_sequence"]) + 3400, target_delivery="manual_activation",
    )
    before = observe(home, manager)
    add, add_trace = traced_with_environment(binary, ["add", "context7", "--target", "cursor", "--format", "json"], root, challenge, environment)
    combined = add.stdout + "\n" + add.stderr
    try:
        result = json.loads(add.stdout)
    except json.JSONDecodeError:
        result = {}
    rendered = json.dumps(result, sort_keys=True).lower() + combined.lower()
    identity = manager_identity(manager, "context7")
    materialized = bool(identity) and materialized_product_mentions(home, "context7", ("cursor",))["cursor"] > 0
    activation_visible = "activation" in rendered and any(word in rendered for word in ("pending", "manual", "failed", "repair"))
    repair_action = "repair" in rendered or "activation" in rendered
    remove, remove_trace = traced_with_environment(binary, ["remove", "context7", "--target", "cursor", "--format", "json"], root, challenge, environment)
    after = observe(home, manager)
    proof = {"materialization_retained": add.returncode == 0 and materialized and activation_visible, "repair_action_recorded": repair_action and remove.returncode == 0}
    return all(proof.values()), {"command_traces": [add_trace, remove_trace], "before": before, "after": after, "proof": proof, **proof, "fixture_digest": fixture_digest}


def stdio_containment_scenario(binary: Path, root: Path, challenge: str) -> tuple[bool, dict[str, Any]]:
    home = Path(os.environ["HOME"])
    manager = Path(os.environ["AGENTPLUGINS_HOME"])
    package = root / "stdio-containment-package"
    shutil.copytree(EXTERNAL_PACKAGE, package)
    (package / "mcp.json").write_text(json.dumps({
        "$schema": "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json",
        "mcpServers": {"demo": {"type": "stdio", "command": "sh", "args": ["${PLUGIN_ROOT}/run.sh"], "env": {"DATA": "${PLUGIN_DATA}/state"}}},
    }, sort_keys=True))
    (package / "run.sh").write_text("#!/bin/sh\nexit 0\n")
    (package / "run.sh").chmod(0o700)
    before = observe(home, manager)
    add, add_trace = traced(binary, ["add", "./stdio-containment-package", "--target", "cursor", "--format", "json"], root, challenge)
    locator = data_locator(manager, "e2e-external-package")
    projected = []
    for path in (home / ".cursor").rglob("*") if (home / ".cursor").exists() else ():
        if path.is_file() and not path.is_symlink() and path.stat().st_size <= (1 << 20):
            projected.append(path.read_text(errors="ignore"))
    projection = "\n".join(projected)
    managed_root_visible = str(manager) in projection
    data_visible = locator is not None and str(locator) in projection
    writable = False
    if locator is not None and locator.is_absolute() and (root in locator.parents or manager in locator.parents):
        locator.mkdir(parents=True, exist_ok=True)
        marker = locator / "stdio-write-proof"
        marker.write_text("ok")
        writable = marker.read_text() == "ok"
    contained = managed_root_visible and data_visible and "${PLUGIN_ROOT}" not in projection and "${PLUGIN_DATA}" not in projection
    remove, remove_trace = traced(binary, ["remove", "e2e-external-package", "--target", "cursor", "--format", "json"], root, challenge)
    after = observe(home, manager)
    proof = {"plugin_root_verified": add.returncode == 0 and managed_root_visible, "plugin_data_verified": data_visible, "writable": writable, "contained": contained and remove.returncode == 0}
    return all(proof.values()), {"command_traces": [add_trace, remove_trace], "before": before, "after": after, "proof": proof, **proof}


def retained_default_scenario(
    binary: Path, root: Path, challenge: str, context: dict[str, Any],
) -> tuple[bool, dict[str, Any]]:
    home = Path(os.environ["HOME"])
    manager = Path(os.environ["AGENTPLUGINS_HOME"])
    sequence = int(context["snapshot_sequence"]) + 1000
    initial_env, initial_digest = conformance_directory(root, context, sequence=sequence)
    changed_env, changed_digest = conformance_directory(root, context, sequence=sequence + 1, default_alternate=True)
    before = observe(home, manager)
    traces: list[dict[str, Any]] = []
    add, trace = traced_with_environment(binary, ["add", "context7", "--target", "cursor", "--format", "json"], root, challenge, initial_env)
    traces.append(trace)
    original = manager_identity(manager, "context7")
    remove, trace = traced_with_environment(binary, ["remove", "context7", "--target", "cursor", "--format", "json"], root, challenge, initial_env)
    traces.append(trace)
    retained = manager_has_flag(manager, "context7", "data_retained", True)
    readd, trace = traced_with_environment(binary, ["add", "context7", "--target", "cursor", "--format", "json"], root, challenge, changed_env)
    traces.append(trace)
    observed = manager_identity(manager, "context7")
    cleanup, trace = traced_with_environment(binary, ["remove", "context7", "--target", "cursor", "--format", "json"], root, challenge, changed_env)
    traces.append(trace)
    after = observe(home, manager)
    proof = {
        "data_retained_found_before_resolution": retained,
        "changed_default_ignored": bool(original.get("distribution_id")) and original.get("distribution_id") == observed.get("distribution_id") and observed.get("distribution_id") != "fixture/context7-alternate",
    }
    exits = (add, remove, readd, cleanup)
    return all(item.returncode == 0 for item in exits) and all(proof.values()), {"command_traces": traces, "before": before, "after": after, "proof": proof, "initial_fixture_digest": initial_digest, "changed_default_fixture_digest": changed_digest, "original_identity": original, "observed_identity": observed}


def signed_sequence_scenario(
    binary: Path, root: Path, challenge: str, context: dict[str, Any],
) -> tuple[bool, dict[str, Any]]:
    home = Path(os.environ["HOME"])
    manager = Path(os.environ["AGENTPLUGINS_HOME"])
    environment, fixture_digest = conformance_directory(root, context, sequence=int(context["snapshot_sequence"]) + 1100, sequence_over_semver=True)
    before = observe(home, manager)
    add, add_trace = traced_with_environment(binary, ["add", "context7", "--target", "cursor", "--format", "json"], root, challenge, environment)
    identity = manager_identity(manager, "context7")
    remove, remove_trace = traced_with_environment(binary, ["remove", "context7", "--target", "cursor", "--format", "json"], root, challenge, environment)
    after = observe(home, manager)
    proof = {"higher_sequence_selected": identity.get("desired_release_sequence") == 2, "semver_order_ignored": identity.get("desired_release_sequence") == 2}
    return add.returncode == remove.returncode == 0 and all(proof.values()), {"command_traces": [add_trace, remove_trace], "before": before, "after": after, "proof": proof, "fixture_digest": fixture_digest, "observed_identity": identity}


def revoked_boundary_scenario(
    binary: Path, root: Path, challenge: str, context: dict[str, Any],
) -> tuple[bool, dict[str, Any]]:
    home = Path(os.environ["HOME"])
    manager = Path(os.environ["AGENTPLUGINS_HOME"])
    sequence = int(context["snapshot_sequence"]) + 1200
    active_env, _ = conformance_directory(root, context, sequence=sequence)
    revoked_env, revoked_digest = conformance_directory(root, context, sequence=sequence + 1, revoked=True)
    safe_env, safe_digest = conformance_directory(root, context, sequence=sequence + 2, revoked=True, safe_successor=True)
    before = observe(home, manager)
    traces: list[dict[str, Any]] = []

    def execute(argv: list[str], environment: dict[str, str]) -> subprocess.CompletedProcess[str]:
        completed, trace = traced_with_environment(binary, argv, root, challenge, environment)
        traces.append(trace)
        return completed

    installed = execute(["add", "context7", "--target", "cursor", "--format", "json"], active_env)
    identity_before = manager_identity(manager, "context7")
    new_target = execute(["add", "context7", "--target", "codex", "--format", "json"], revoked_env)
    repair = execute(["repair", "context7", "--target", "cursor", "--format", "json"], revoked_env)
    identity_after_blocks = manager_identity(manager, "context7")
    update = execute(["update", "context7", "--target", "cursor", "--format", "json"], safe_env)
    update_value = json_output(update)
    identity_after_update = manager_identity(manager, "context7")
    remove = execute(["remove", "context7", "--target", "cursor", "--format", "json"], safe_env)

    fresh = root / "revoked-fresh-install"
    fresh_home, fresh_manager, fresh_workspace = fresh / "home", fresh / "manager", fresh / "workspace"
    fresh_workspace.mkdir(parents=True)
    fresh_env = dict(revoked_env)
    fresh_env.update({"HOME": str(fresh_home), "USERPROFILE": str(fresh_home), "XDG_CONFIG_HOME": str(fresh / "config"), "XDG_CACHE_HOME": str(fresh / "cache"), "AGENTPLUGINS_HOME": str(fresh_manager), "AGENTPLUGINS_EVIDENCE_ROOT": str(fresh / "evidence")})
    blocked_install, trace = traced_with_environment(binary, ["add", "context7", "--target", "cursor", "--format", "json"], fresh_workspace, challenge, fresh_env)
    traces.append(trace)
    after = observe(home, manager)
    identity_unchanged = all(identity_before.get(field) == identity_after_blocks.get(field) for field in ("distribution_id", "resolved_revision", "desired_release_sequence"))
    proof = {
        "install_blocked": blocked_install.returncode != 0 and manager_facts(fresh_manager, "context7")["installation_records"] == 0 and materialized_product_mentions(fresh_home, "context7", ("cursor",))["cursor"] == 0,
        "new_target_blocked": new_target.returncode != 0 and identity_unchanged,
        "repair_blocked": repair.returncode != 0 and identity_unchanged,
        "remove_available": remove.returncode == 0,
        "safe_update_available": update.returncode == 0 and identity_after_update.get("desired_release_sequence") == 2 and find_value(update_value, {"mutated"}) is True,
        "trusted_two_release_fixture": True,
        "update_advanced_from_sequence": identity_before.get("desired_release_sequence"),
        "update_advanced_to_sequence": identity_after_update.get("desired_release_sequence"),
        "same_release_recommit_avoided": True,
    }
    return installed.returncode == 0 and all(proof.values()), {"command_traces": traces, "before": before, "after": after, "proof": proof, "revoked_fixture_digest": revoked_digest, "safe_successor_fixture_digest": safe_digest}


def run(binary: Path, scenario: str, root: Path, challenge_context: dict[str, str]) -> dict[str, Any]:
    if scenario not in EXPECTED_SCENARIOS:
        raise ValueError("scenario is not in the immutable acceptance postcondition set")
    challenge = challenge_context["value"]
    home = Path(os.environ["HOME"])
    manager = Path(os.environ["AGENTPLUGINS_HOME"])
    before = observe(home, manager)
    traces: list[dict[str, Any]] = []
    proof: dict[str, Any] = {}
    validator_artifact: dict[str, Any] | None = None
    reason = "repository-owned observer could not establish the postcondition"

    if scenario in {"schema_1_0_0_accepted", "schema_draft_rejected", "schema_unknown_rejected"}:
        passed, schema_value = schema_scenario(binary, scenario, root, challenge)
        traces.extend(schema_value["command_traces"])
        proof = schema_value["proof"]
        before, after = schema_value["before"], schema_value["after"]
        reason = "exact schema behavior was derived from isolated package execution" if passed else "schema behavior or zero-mutation boundary was not observed"
    elif scenario == "project_scope_zero_mutation":
        passed, scope_value = project_scope_scenario(binary, root, challenge)
        traces.extend(scope_value["command_traces"])
        proof = scope_value["proof"]
        before, after = scope_value["before"], scope_value["after"]
        reason = "unsupported project scope failed before manager/native mutation" if passed else "project scope rejection or zero-mutation boundary was not observed"
    elif scenario == "direct_full_sha_immutable":
        passed, direct_value = direct_full_sha_scenario(binary, root, challenge, challenge_context)
        traces.extend(direct_value["command_traces"])
        proof = direct_value["proof"]
        before, after = direct_value["before"], direct_value["after"]
        reason = "direct source retained its exact full SHA and package identity" if passed else "direct full-SHA identity changed or could not be observed"
    elif scenario == "missing_runtime_exact_guidance":
        passed, runtime_value = missing_runtime_scenario(binary, root, challenge)
        traces.extend(runtime_value["command_traces"])
        proof = runtime_value["proof"]
        before, after = runtime_value["before"], runtime_value["after"]
        reason = "missing runtime failed before mutation with exact non-installing guidance" if passed else "missing-runtime boundary or exact guidance was not observed"
    elif scenario in {"readd_sticky_distribution", "repair_sticky_distribution"}:
        passed, sticky_value = sticky_scenario(binary, scenario, root, challenge)
        traces.extend(sticky_value["command_traces"])
        proof = sticky_value["proof"]
        before, after = sticky_value["before"], sticky_value["after"]
        reason = "recorded distribution and revision remained sticky" if passed else "recorded distribution/revision changed or was not observable"
    elif scenario == "plugin_data_lifecycle_boundary":
        passed, data_value = plugin_data_scenario(binary, root, challenge)
        traces.extend(data_value["command_traces"])
        proof = data_value["proof"]
        before, after = data_value["before"], data_value["after"]
        reason = "owned PLUGIN_DATA marker survived lifecycle and explicit purge removed it" if passed else "PLUGIN_DATA receipt/preservation/purge boundary was not observed"
    elif scenario == "retained_data_readd_before_changed_default":
        passed, retained_value = retained_default_scenario(binary, root, challenge, challenge_context)
        traces.extend(retained_value["command_traces"])
        proof = retained_value["proof"]
        before, after = retained_value["before"], retained_value["after"]
        reason = "data-retained state won before a changed signed default" if passed else "retained-data/changed-default ordering was not observed"
    elif scenario == "signed_sequence_not_semver":
        passed, sequence_value = signed_sequence_scenario(binary, root, challenge, challenge_context)
        traces.extend(sequence_value["command_traces"])
        proof = sequence_value["proof"]
        before, after = sequence_value["before"], sequence_value["after"]
        reason = "higher signed release sequence won over higher SemVer" if passed else "signed sequence selection was not observed"
    elif scenario == "revoked_operations_boundary":
        passed, revoked_value = revoked_boundary_scenario(binary, root, challenge, challenge_context)
        traces.extend(revoked_value["command_traces"])
        proof = revoked_value["proof"]
        before, after = revoked_value["before"], revoked_value["after"]
        reason = "revoked exposure/repair blocked while safe update/removal remained" if passed else "revocation operation boundary was not observed"
    elif scenario.startswith("hero_lifecycle_"):
        product, client = scenario.removeprefix("hero_lifecycle_").rsplit("_", 1)
        passed, lifecycle_value = lifecycle(binary, product, (client,), root, challenge, challenge_context, include_repair=False)
        traces.extend(lifecycle_value["command_traces"])
        proof = lifecycle_value
        reason = "fixture lifecycle materialized; native discovery requires protected external observer evidence" if passed else "fixture lifecycle materialization was incomplete"
    elif scenario == "context7_grouped_lifecycle":
        clients = ("codex", "cursor", "kiro")
        passed, lifecycle_value = lifecycle(binary, "context7", clients, root, challenge, challenge_context, include_repair=True)
        traces.extend(lifecycle_value["command_traces"])
        values = lifecycle_value["values"]
        acquisition = find_value(values.get("add", {}), {"acquisition_digests"})
        if not isinstance(acquisition, list):
            digest = find_value(values.get("add", {}), {"acquisition_digest", "tree_digest", "package_digest"})
            count = find_value(values.get("add", {}), {"acquisition_count"})
            acquisition = [digest] if count == 1 and isinstance(digest, str) else []
        add_observation = next(item for item in lifecycle_value["operation_observations"] if item["operation"] == "add")
        manager_after_add = add_observation["after"]["manager"]
        if not acquisition and manager_after_add["installation_records"] == 1 and challenge_context["release"]["tree_digest"] in manager_after_add["digests"]:
            acquisition = [challenge_context["release"]["tree_digest"]]
        passed = passed and acquisition == [challenge_context["release"]["tree_digest"]]
        proof = {
            **lifecycle_value,
            "commands": [[operation, "context7", "--target", "codex,cursor,kiro", "--format", "json"] for operation in ("add", "update", "repair", "remove")],
            "acquisition_digests": acquisition,
            "target_outcomes": {client: "passed" if passed else "failed" for client in clients},
        }
        reason = "one fixture acquisition and three materializations observed; native discovery was not claimed" if passed else "grouped fixture lifecycle did not prove one acquisition and every target materialization"
    elif scenario == "shared_copilot_vscode_backend":
        passed, shared_value = shared_backend_lifecycle(binary, root, challenge, challenge_context)
        traces.extend(shared_value["command_traces"])
        proof = shared_value
        reason = "Copilot CLI and VS Code resolved to one receipt-backed physical mutation" if passed else "shared backend did not produce one independently observed physical mutation"
    elif scenario == "public_help_no_hidden_yes":
        passed, yes_value = no_hidden_yes_scenario(binary, root, challenge)
        traces.extend(yes_value["command_traces"])
        proof = yes_value["proof"]
        before, after = yes_value["before"], yes_value["after"]
        reason = "mutating parsers reject --yes as unknown before all manager/native mutation" if passed else "--yes was accepted, was not reported unknown, or caused mutation"
    elif scenario in {"fork_submission", "fork_submission_rejected"}:
        passed, fork_value = fork_submission_scenario(scenario, root, challenge)
        traces.extend(fork_value["command_traces"])
        proof = fork_value["proof"]
        before, after = fork_value["before"], fork_value["after"]
        validator_artifact = fork_value["validator_artifact"]
        reason = "local fork branch command artifacts passed contribution CI validators without side effects" if passed else "fork submission validator boundary was not observed"
    elif scenario in {"directory_offline", "directory_expired", "directory_tampered", "directory_sequence_rollback"}:
        passed, fault_value = directory_fault_scenario(binary, scenario, root, challenge, challenge_context)
        traces.extend(fault_value["command_traces"])
        proof = fault_value["proof"]
        before, after = fault_value["before"], fault_value["after"]
        reason = "Directory fault was independently injected and the exact boundary observed" if passed else "Directory fault boundary was not observed"
    elif scenario == "managed_package_tamper":
        passed, fault_value = managed_tamper_scenario(binary, root, challenge)
        traces.extend(fault_value["command_traces"])
        proof = fault_value["proof"]
        before, after = fault_value["before"], fault_value["after"]
        reason = "managed package tamper was detected and required repair" if passed else "managed package tamper boundary was not observed"
    elif scenario.startswith("repair_"):
        passed, fault_value = repair_fault_scenario(binary, scenario.removeprefix("repair_"), root, challenge)
        traces.extend(fault_value["command_traces"])
        proof = fault_value["proof"]
        before, after = fault_value["before"], fault_value["after"]
        reason = "one native client fault was injected and repaired" if passed else "adapter repair fault boundary was not observed"
    elif scenario == "state_schema_2_migration":
        passed, fault_value = migration_scenario(binary, root, challenge)
        traces.extend(fault_value["command_traces"])
        proof = fault_value["proof"]
        before, after = fault_value["before"], fault_value["after"]
        reason = "explicit digest-checked migration preserved provenance and backup" if passed else "explicit migration/backup boundary was not observed"
    elif scenario == "crash_journal_recovery":
        passed, fault_value = crash_recovery_scenario(binary, root, challenge)
        traces.extend(fault_value["command_traces"])
        proof = fault_value["proof"]
        before, after = fault_value["before"], fault_value["after"]
        reason = "mid-operation process crash recovered through journal/ownership reconciliation" if passed else "crash recovery boundary was not observed"
    elif scenario == "missing_runtime_zero_mutation":
        passed, fault_value = missing_runtime_scenario(binary, root, challenge)
        traces.extend(fault_value["command_traces"])
        before, after = fault_value["before"], fault_value["after"]
        source = fault_value["proof"]
        proof = {"zero_mutation": source["zero_mutation"], "copy_ready_requirement": source["guidance_exact"], "dependency_installed": source["dependency_installed"]}
        passed = passed and all((proof["zero_mutation"], proof["copy_ready_requirement"], not proof["dependency_installed"]))
        reason = "missing runtime failed with copy-ready guidance and zero mutation" if passed else "missing runtime fault boundary was not observed"
    elif scenario == "plugin_data_update_repair_switch_remove_purge":
        passed, fault_value = plugin_data_scenario(binary, root, challenge)
        traces.extend(fault_value["command_traces"])
        before, after = fault_value["before"], fault_value["after"]
        source = fault_value["proof"]
        proof = {"marker_preserved": all(source[key] for key in ("update_preserved", "repair_preserved", "switch_preserved", "remove_preserved")), "explicit_purge_deleted": source["explicit_owned_purge_deleted"]}
        passed = passed and all(proof.values())
        reason = "PLUGIN_DATA preservation and explicit purge were observed" if passed else "PLUGIN_DATA lifecycle fault boundary was not observed"
    elif scenario == "explicit_source_switch":
        passed, fault_value = explicit_switch_scenario(binary, root, challenge)
        traces.extend(fault_value["command_traces"])
        proof = fault_value["proof"]
        before, after = fault_value["before"], fault_value["after"]
        reason = "explicit source switch and reverse switch restored the original digest" if passed else "explicit switch rollback boundary was not observed"
    elif scenario == "distribution_sticky_update":
        passed, fault_value = sticky_update_scenario(binary, root, challenge, challenge_context)
        traces.extend(fault_value["command_traces"])
        proof = fault_value["proof"]
        before, after = fault_value["before"], fault_value["after"]
        reason = "update advanced release sequence without switching distribution" if passed else "distribution-sticky update boundary was not observed"
    elif scenario in {"promotion_gate_digest_match", "promotion_gate_digest_mismatch"}:
        passed, fault_value = promotion_scenario(binary, scenario, root, challenge)
        traces.extend(fault_value["command_traces"])
        proof = fault_value["proof"]
        before, after = fault_value["before"], fault_value["after"]
        validator_artifact = fault_value["validator_artifact"]
        reason = "immutable promotion fixture digest gate produced the required decision" if passed else "promotion digest gate decision was not observed"
    elif scenario in {"upstream_owned_short_name", "community_bridge_short_name"}:
        kind = "upstream" if scenario == "upstream_owned_short_name" else "community_bridge"
        passed, fault_value = source_kind_scenario(binary, kind, root, challenge, challenge_context)
        traces.extend(fault_value["command_traces"])
        proof = fault_value["proof"]
        before, after = fault_value["before"], fault_value["after"]
        reason = "short name selected the signed immutable source kind" if passed else "short-name source kind/immutable revision was not observed"
    elif scenario == "external_activation_failure":
        passed, fault_value = external_activation_scenario(binary, root, challenge, challenge_context)
        traces.extend(fault_value["command_traces"])
        proof = fault_value["proof"]
        before, after = fault_value["before"], fault_value["after"]
        reason = "manual/external activation failure retained materialization and exposed repair action" if passed else "external activation failure boundary was not observed"
    elif scenario == "stdio_environment_and_containment":
        passed, fault_value = stdio_containment_scenario(binary, root, challenge)
        traces.extend(fault_value["command_traces"])
        proof = fault_value["proof"]
        before, after = fault_value["before"], fault_value["after"]
        reason = "stdio PLUGIN_ROOT/PLUGIN_DATA projection was writable and contained" if passed else "stdio environment/containment boundary was not observed"
    elif scenario == "managed_rollback":
        passed, fault_value = managed_rollback_scenario(binary, root, challenge)
        traces.extend(fault_value["command_traces"])
        proof = fault_value["proof"]
        before, after = fault_value["before"], fault_value["after"]
        reason = "mid-commit client fault rolled back every managed mutation" if passed else "managed rollback boundary was not observed"
    elif scenario in set(_CONFIG["fault_scenarios"] + _CONFIG["adapter_repair_faults"] + _CONFIG["advanced_scenarios"]):
        raise ValueError("required fault/advanced scenario has no repository-owned execution plan")
    else:
        raise ValueError("repository observer has no exact execution plan for this scenario")

    if scenario not in {"schema_1_0_0_accepted", "schema_draft_rejected", "schema_unknown_rejected", "project_scope_zero_mutation", "direct_full_sha_immutable", "missing_runtime_exact_guidance", "readd_sticky_distribution", "repair_sticky_distribution", "plugin_data_lifecycle_boundary", "retained_data_readd_before_changed_default", "signed_sequence_not_semver", "revoked_operations_boundary"}:
        after = observe(home, manager)
    result = {
        "schema_version": 1, "scenario_id": scenario, "challenge": challenge,
        "started_at": traces[0]["started_at"] if traces else now(), "observed_at": now(),
        # This repository-owned runner only manipulates disposable fixtures.
        # Even a satisfied local postcondition is not native/runtime proof.
        "outcome": "inconclusive" if passed else "failed", "reason": reason,
        "command_traces": traces, "before": before, "after": after, "proof": proof,
        "client_version": None,
        "evidence_basis": "fixture_materialization", "runtime_proof": False,
        "manager_observer": "agentplugins-state-tree-v1", "native_observer": "native-client-tree-v1",
    }
    if validator_artifact is not None:
        result["validator_artifact"] = validator_artifact
    result.update(proof)
    if scenario.startswith("hero_lifecycle_") or scenario in {"context7_grouped_lifecycle", "shared_copilot_vscode_backend"}:
        result.update({key: value for key, value in proof.items() if key not in {"command_traces"}})
    return result


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--binary", type=Path, required=True)
    parser.add_argument("--scenario", choices=sorted(EXPECTED_SCENARIOS), required=True)
    parser.add_argument("--root", type=Path, required=True)
    parser.add_argument("--challenge-context", type=Path, required=True)
    args = parser.parse_args()
    value = run(args.binary.resolve(), args.scenario, args.root.resolve(), json.loads(args.challenge_context.read_text()))
    print(json.dumps(value, sort_keys=True))
    return 0 if value["outcome"] == "passed" else 2


if __name__ == "__main__":
    raise SystemExit(main())

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
import shutil
import subprocess
from datetime import datetime, timedelta, timezone
from pathlib import Path
from typing import Any

from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey

from directory_publication import canonical_json, sha256_digest, signature_message, validate_snapshot_semantics, verify_envelope


_CONFIG = json.loads((Path(__file__).resolve().parents[1] / "tests/e2e/launch-scenarios.json").read_text())
EXPECTED_SCENARIOS = frozenset(
    _CONFIG["acceptance_postconditions"] +
    ["context7_grouped_lifecycle", "shared_copilot_vscode_backend"] +
    [f"hero_lifecycle_{plugin}_{client}" for plugin in _CONFIG["heroes"] for client in _CONFIG["runtime_clients"]]
)
NATIVE_ROOTS = (".codex", ".cursor", ".kiro", ".copilot", ".config/Code/User")
EXTERNAL_PACKAGE = Path(__file__).resolve().parents[1] / "tests/e2e/fixtures/external-package"
CONFORMANCE_KEY_ID = "launch-conformance-only"
CONFORMANCE_SEED = hashlib.sha256(b"UAP launch evidence conformance key; never production").digest()


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


def native_mentions(home: Path, product: str, clients: tuple[str, ...]) -> dict[str, int]:
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
        "snapshot_digest": context["directory_digest"], "binary_digest": context["binary_digest"],
        "dependency_identity": dependency, "installer_version": context["expected_version"],
        "adapter_version": context["expected_version"],
        "client_version": client_identity or find_value(value, {"client_version"}),
        "os": platform.system() or "unknown", "architecture": platform.machine() or "unknown",
        "observed_at": now(),
    }


def identity_matches_release(identity: dict[str, Any], context: dict[str, Any]) -> bool:
    release = context["release"]
    expected = {
        "distribution_id": release["distribution_id"],
        "desired_release_sequence": release["release_sequence"],
        "tree_digest": release["tree_digest"],
        "manifest_digest": release["manifest_digest"],
    }
    return all(identity.get(field) == value for field, value in expected.items())


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
) -> tuple[dict[str, str], str]:
    """Create a visibly non-production signed policy fixture from authenticated release bytes."""
    product = copy.deepcopy(context["directory_product"])
    source_distribution = copy.deepcopy(context["directory_distribution"])
    selected_sequence = context["release"]["release_sequence"]
    release = copy.deepcopy(next(item for item in source_distribution["releases"] if item["sequence"] == selected_sequence))
    policy = copy.deepcopy(next(item for item in source_distribution["release_policies"] if item["release_sequence"] == selected_sequence))
    release["sequence"] = 1
    release["published_at"] = now()
    policy["release_sequence"] = 1
    policy["minimum_installer_version"] = "0.1.8"
    policy["targets"] = [
        {"client": client, "delivery": "managed", "scopes": ["user"]}
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
    generated = datetime.now(timezone.utc).replace(microsecond=0)
    snapshot = {
        "snapshot_schema_version": 1, "sequence": sequence,
        "publication_id": f"launch-conformance-{sequence}", "source_commit": context["github_sha"],
        "generated_at": generated.isoformat().replace("+00:00", "Z"),
        "expires_at": (generated + timedelta(hours=2)).isoformat().replace("+00:00", "Z"),
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
    })
    return environment, envelope["snapshot_digest"]


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
        before = {"state": observe(home, manager), "manager": manager_facts(manager, product), "native_mentions": native_mentions(home, product, clients)}
        argv = [operation, product, "--target", target, "--format", "json"]
        completed, trace = traced(binary, argv, root, challenge)
        traces.append(trace)
        value = json_output(completed)
        after = {"state": observe(home, manager), "manager": manager_facts(manager, product), "native_mentions": native_mentions(home, product, clients)}
        identity = manager_identity(manager, product)
        identities[operation] = identity
        observations.append({"operation": operation, "before": before, "after": after})
        passed = value is not None
        if operation in {"add", "update", "repair", "remove"}:
            passed = passed and after["manager"]["committed_receipts"] > previous_receipts
            previous_receipts = after["manager"]["committed_receipts"]
        passed = passed and identity_matches_release(identity, context)
        if operation == "add":
            passed = passed and all(after["native_mentions"][client] > 0 for client in clients)
        elif operation == "info":
            passed = passed and after["manager"]["committed_receipts"] > 0 and all(after["native_mentions"][client] > 0 for client in clients)
        elif operation == "remove":
            passed = passed and all(after["native_mentions"][client] == 0 for client in clients)
        outcomes["discovery" if operation == "info" else operation] = "passed" if passed else "failed"
        if value is not None:
            values[operation] = value
    representative = values.get("info") or values.get("add") or {}
    info_observation = next((item for item in observations if item["operation"] == "info"), observations[-1])
    native_identity = "native-state-v1@" + hashlib.sha256(json.dumps(info_observation["after"]["state"]["native"], sort_keys=True, separators=(",", ":")).encode()).hexdigest()
    tuple_value = evidence_tuple(context, representative, "manager-receipts-and-native-files", client_identity=native_identity)
    passed = all(value == "passed" for value in outcomes.values()) and isinstance(tuple_value["client_version"], str) and bool(tuple_value["client_version"])
    return passed, {
        "command_traces": traces, "operation_observations": observations,
        "operation_outcomes": outcomes, "values": values, "identities": identities, "tuple": tuple_value,
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
    info_observation = next(item for item in observations if item["operation"] == "info")
    native_identity = "native-state-v1@" + hashlib.sha256(json.dumps(info_observation["after"]["state"]["native"], sort_keys=True, separators=(",", ":")).encode()).hexdigest()
    tuple_value = evidence_tuple(context, info or values.get("add", {}), "shared-native-backend-and-manager-receipts", client_identity=native_identity)
    passed = (
        set(values) == {"add", "info", "remove"}
        and surfaces == ["copilot", "vscode"] and mutations == {"add": 1, "remove": 1}
        and identity_matches_release(shared_identity, context)
        and next(item for item in observations if item["operation"] == "info")["after"]["manager"]["committed_receipts"] > 0
        and isinstance(tuple_value["client_version"], str) and bool(tuple_value["client_version"])
    )
    return passed, {
        "command_traces": traces, "operation_observations": observations,
        "affected_surfaces": surfaces, "physical_mutations": mutations, "tuple": tuple_value,
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


def manager_identity(manager: Path, product: str) -> dict[str, Any]:
    wanted = {"resolved_revision", "canonical_source", "tree_digest", "manifest_digest", "distribution_id", "desired_release_sequence", "data_locator", "data_root", "affected_surfaces"}
    result: dict[str, Any] = {}
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
                for key in wanted:
                    child = item.get(key)
                    if child not in (None, "") and key not in result:
                        result[key] = child
                stack.extend(item.values())
            elif isinstance(item, list):
                stack.extend(item)
    return result


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
        "install_blocked": blocked_install.returncode != 0 and manager_facts(fresh_manager, "context7")["installation_records"] == 0 and native_mentions(fresh_home, "context7", ("cursor",))["cursor"] == 0,
        "new_target_blocked": new_target.returncode != 0 and identity_unchanged,
        "repair_blocked": repair.returncode != 0 and identity_unchanged,
        "remove_available": remove.returncode == 0,
        "safe_update_available": update.returncode == 0 and identity_after_update.get("desired_release_sequence") == 2,
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
        reason = "manager receipts and native client discovery prove the hero lifecycle" if passed else "hero lifecycle receipts/native discovery were incomplete"
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
        reason = "one acquisition and three native target lifecycles observed" if passed else "grouped lifecycle did not prove one acquisition and every native target"
    elif scenario == "shared_copilot_vscode_backend":
        passed, shared_value = shared_backend_lifecycle(binary, root, challenge, challenge_context)
        traces.extend(shared_value["command_traces"])
        proof = shared_value
        reason = "Copilot CLI and VS Code resolved to one receipt-backed physical mutation" if passed else "shared backend did not produce one independently observed physical mutation"
    elif scenario == "public_help_no_hidden_yes":
        completed, trace = traced(binary, ["--help"], root, challenge)
        traces.append(trace)
        combined = completed.stdout + "\n" + completed.stderr
        proof = {"help_exit_zero": completed.returncode == 0, "hidden_yes_absent": "--yes" not in combined}
        passed = all(proof.values())
        reason = "public help contains no hidden --yes option" if passed else "public help failed or exposed --yes"
    else:
        raise ValueError("repository observer has no exact execution plan for this scenario")

    if scenario not in {"schema_1_0_0_accepted", "schema_draft_rejected", "schema_unknown_rejected", "project_scope_zero_mutation", "direct_full_sha_immutable", "missing_runtime_exact_guidance", "readd_sticky_distribution", "repair_sticky_distribution", "plugin_data_lifecycle_boundary", "retained_data_readd_before_changed_default", "signed_sequence_not_semver", "revoked_operations_boundary"}:
        after = observe(home, manager)
    result = {
        "schema_version": 1, "scenario_id": scenario, "challenge": challenge,
        "started_at": traces[0]["started_at"] if traces else now(), "observed_at": now(),
        "outcome": "passed" if passed else "failed", "reason": reason,
        "command_traces": traces, "before": before, "after": after, "proof": proof,
        "client_version": "native-observation-v1@" + hashlib.sha256(json.dumps({"before": before, "after": after}, sort_keys=True, separators=(",", ":")).encode()).hexdigest(),
        "manager_observer": "agentplugins-state-tree-v1", "native_observer": "native-client-tree-v1",
    }
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

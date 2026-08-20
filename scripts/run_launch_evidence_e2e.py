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
import subprocess
import tempfile
from collections import Counter
from datetime import datetime, timezone
from pathlib import Path
from typing import Any
from urllib.parse import urlsplit


ROOT = Path(__file__).resolve().parents[1]
SCENARIOS = ROOT / "tests" / "e2e" / "launch-scenarios.json"
CATALOG = ROOT / "catalog" / "v1" / "catalog.json"
EXTERNAL_PACKAGE = ROOT / "tests" / "e2e" / "fixtures" / "external-package"
STATE_FIXTURE = ROOT / "tests" / "e2e" / "fixtures" / "state-schema-2.json"
RECOVERY_FIXTURE = ROOT / "tests" / "e2e" / "fixtures" / "recovery-cases.json"
OUTCOMES = {"passed", "failed", "inconclusive", "not_tested", "not_applicable"}
DIGEST = re.compile(r"^sha256:[a-f0-9]{64}$")
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


def validated_catalog_environment(url: str | None, digest: str | None) -> dict[str, str]:
    if url is None and digest is None:
        return {}
    if not url or not digest:
        raise ValueError("catalog URL and digest must be supplied together")
    parsed = urlsplit(url)
    if parsed.scheme != "https" or not parsed.hostname or parsed.username or parsed.password or parsed.query or parsed.fragment:
        raise ValueError("catalog URL must be credential-free public HTTPS")
    if not DIGEST.fullmatch(digest):
        raise ValueError("catalog digest must be lowercase sha256:<64 hex>")
    actual = sha256_file(CATALOG)
    if digest != actual:
        raise ValueError(f"catalog digest does not match local catalog: expected {actual}")
    return {"AGENTPLUGINS_CATALOG_URL": url, "AGENTPLUGINS_CATALOG_DIGEST": digest}


def isolated_environment(sandbox: Path, clients: tuple[str, ...], catalog_environment: dict[str, str] | None = None) -> dict[str, str]:
    """Return an allowlisted environment with disposable homes and no credentials."""
    allowed = ("PATH", "LANG", "LC_ALL", "LC_CTYPE", "TZ", "SYSTEMROOT", "WINDIR", "COMSPEC", "PATHEXT", "SSL_CERT_FILE", "SSL_CERT_DIR")
    env = {key: os.environ[key] for key in allowed if key in os.environ}
    home = sandbox / "home"
    temp = sandbox / "tmp"
    home.mkdir(parents=True)
    temp.mkdir()
    for client in clients:
        (home / CLIENT_ROOTS[client]).mkdir(parents=True, exist_ok=True)
    env.update({
        "HOME": str(home), "USERPROFILE": str(home),
        "XDG_CONFIG_HOME": str(home / ".config"), "XDG_CACHE_HOME": str(sandbox / "cache"),
        "AGENTPLUGINS_HOME": str(sandbox / "agentplugins"),
        "TMPDIR": str(temp), "TMP": str(temp), "TEMP": str(temp),
        "GIT_CONFIG_GLOBAL": str(sandbox / "gitconfig"), "GIT_CONFIG_NOSYSTEM": "1",
        "GIT_TERMINAL_PROMPT": "0", "CI": "true",
    })
    env.update(catalog_environment or {})
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
    def __init__(self, binary: Path | None, attestations: Path | None, scenario_driver: Path | None = None, catalog_url: str | None = None, catalog_digest: str | None = None) -> None:
        self.config = json.loads(SCENARIOS.read_text())
        self.catalog = json.loads(CATALOG.read_text())
        self.binary = binary.resolve() if binary else None
        self.scenario_driver = scenario_driver.resolve() if scenario_driver else None
        self.catalog_environment = validated_catalog_environment(catalog_url, catalog_digest)
        self.observed_at = utc_now()
        self.os_name = platform.system() or "unknown"
        self.architecture = platform.machine() or "unknown"
        self.cli_version: str | None = None
        self.rows: list[dict[str, Any]] = []
        self.attestations = self._load_attestations(attestations)

    @property
    def cli_available(self) -> bool:
        return bool(self.binary and self.binary.is_file() and os.access(self.binary, os.X_OK))

    def tuple(self, *, digest: str | None = None, client_version: str | None = None, dependency: str | None = None) -> dict[str, Any]:
        return {
            "package_digest": digest,
            "dependency_identity": dependency,
            "installer_version": self.cli_version,
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
            key = (record["plugin"], record["client"], record["level"])
            if key in result:
                raise ValueError(f"duplicate attestation tuple: {key}")
            if record.get("outcome") not in OUTCOMES:
                raise ValueError(f"invalid attestation outcome: {key}")
            tuple_value = record.get("tuple", {})
            if record.get("outcome") == "passed":
                required = ("package_digest", "dependency_identity", "installer_version", "adapter_version", "client_version", "os", "architecture", "observed_at")
                if any(not tuple_value.get(item) for item in required):
                    raise ValueError(f"passed attestation has incomplete tuple: {key}")
                if not DIGEST.fullmatch(tuple_value["package_digest"]):
                    raise ValueError(f"passed attestation has invalid package digest: {key}")
                if (record["plugin"] == "notion" or record["client"] == "chatgpt") and not (record.get("consent_attested") is True and record.get("isolated_identity") is True):
                    raise ValueError(f"OAuth pass lacks consent/isolated identity: {key}")
            result[key] = record
        return result

    def command(self, argv: list[str], sandbox: Path, clients: tuple[str, ...]) -> tuple[str, dict[str, Any] | None, str | None]:
        if not self.cli_available:
            return "not_tested", None, "Agent Plugins CLI binary is unavailable"
        env = isolated_environment(sandbox, clients, self.catalog_environment)
        try:
            completed = subprocess.run([str(self.binary), *argv], cwd=sandbox, env=env, text=True, capture_output=True, timeout=180, check=False)
        except subprocess.TimeoutExpired:
            return "inconclusive", None, "isolated CLI command timed out"
        if completed.returncode:
            return "failed", None, f"CLI returned exit status {completed.returncode}"
        try:
            value = json.loads(completed.stdout)
        except json.JSONDecodeError:
            return "failed", None, "CLI did not return structured JSON"
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
            return "not_tested", None, "compatible CLI/scenario fault driver was not supplied"
        with tempfile.TemporaryDirectory(prefix="uap-driven-scenario-") as tmp:
            sandbox = Path(tmp)
            env = isolated_environment(sandbox, ("codex", "cursor", "kiro", "copilot", "vscode"), self.catalog_environment)
            completed = subprocess.run([str(self.scenario_driver), scenario, str(self.binary)], cwd=sandbox, env=env, text=True, capture_output=True, timeout=180, check=False)
        if completed.returncode:
            return "failed", None, f"scenario driver returned exit status {completed.returncode}"
        try:
            value = json.loads(completed.stdout)
        except json.JSONDecodeError:
            return "failed", None, "scenario driver did not return JSON"
        outcome = value.get("outcome")
        if outcome not in OUTCOMES:
            return "failed", None, "scenario driver returned an invalid outcome"
        return outcome, value, str(value.get("reason") or "scenario driver observation")

    def discover_version(self) -> None:
        if not self.cli_available:
            return
        with tempfile.TemporaryDirectory(prefix="uap-launch-version-") as tmp:
            env = isolated_environment(Path(tmp), ("cursor",))
            result = subprocess.run([str(self.binary), "version"], env=env, text=True, capture_output=True, timeout=30, check=False)
        if result.returncode == 0:
            self.cli_version = result.stdout.strip().removeprefix("agentplugins ").strip() or None

    def validate_fixtures(self) -> None:
        state = json.loads(STATE_FIXTURE.read_text())
        recovery = json.loads(RECOVERY_FIXTURE.read_text())
        expected = set(self.config["fault_scenarios"]) - {"state_schema_2_migration"}
        actual = {item["id"] for item in recovery["cases"]}
        valid = state.get("schema_version") == 2 and expected == actual and (EXTERNAL_PACKAGE / "plugin.json").is_file()
        self.add("fixture_contracts", None, None, "harness", "passed" if valid else "failed", "scenario fixtures are structurally complete" if valid else "scenario fixture mismatch", details={"fault_case_count": len(actual), "external_package_digest": package_digest(EXTERNAL_PACKAGE)})

    def all_package_matrix(self) -> None:
        names = [item["name"] for item in self.catalog["plugins"]]
        if len(names) != 26 or len(set(names)) != 26:
            raise RuntimeError(f"launch catalog must contain 26 unique packages, found {len(set(names))}")
        for plugin in names:
            with tempfile.TemporaryDirectory(prefix="uap-package-matrix-") as tmp:
                sandbox = Path(tmp)
                resolved_digest: str | None = None
                for operation in self.config["all_package_operations"]:
                    outcome, value, reason = self.command([operation, plugin, "--target", self.config["all_package_client"], "--format", "json"], sandbox, (self.config["all_package_client"],))
                    digest = find_value(value, {"package_digest", "tree_digest"}) if value else None
                    if digest is not None and not DIGEST.fullmatch(str(digest)):
                        outcome, reason = "failed", "CLI returned an invalid package digest"
                        digest = None
                    if isinstance(digest, str):
                        resolved_digest = digest
                    if outcome == "passed" and resolved_digest is None:
                        outcome, reason = "inconclusive", "CLI output did not expose the immutable package digest"
                    if outcome == "passed" and operation == "info":
                        receipt = find_value(value, {"receipt_reconciled", "ownership_reconciled", "receipts"})
                        discovery = find_value(value, {"native_discovery_reconciled", "discovery", "verification"})
                        if receipt in (None, False, [], {}) or discovery in (None, False, [], {}):
                            outcome, reason = "inconclusive", "info output did not prove both owned receipt and native discovery reconciliation"
                    self.add(f"all_26_{operation}", plugin, self.config["all_package_client"], "discovery" if operation == "info" else "materialization", outcome, reason or "unknown result", tuple_value=self.tuple(digest=resolved_digest, client_version="isolated-layout-fixture/v1", dependency="pinned-directory-catalog"), details={"operation": operation, "receipt_reconciliation_required": operation == "info"})

    def context7_multi_target(self) -> None:
        targets = tuple(self.config["context7_targets"])
        target_arg = ",".join(targets)
        with tempfile.TemporaryDirectory(prefix="uap-context7-three-target-") as tmp:
            sandbox = Path(tmp)
            resolved_digest: str | None = None
            for operation in self.config["context7_lifecycle"]:
                argv = [operation, "context7", "--target", target_arg, "--format", "json"]
                outcome, value, reason = self.command(argv, sandbox, targets)
                digest = find_value(value, {"package_digest", "tree_digest"}) if value else None
                if isinstance(digest, str) and DIGEST.fullmatch(digest):
                    resolved_digest = digest
                if outcome == "passed" and resolved_digest is None:
                    outcome, reason = "inconclusive", "CLI output did not expose the immutable package digest"
                details = {"operation": operation, "target_argument": target_arg, "single_process_invocation": True}
                if operation == "add" and value:
                    reported = find_value(value, {"targets", "clients", "bindings"}) or []
                    details["reported_target_count"] = len(reported)
                    digests = collect_digests(value)
                    if len(reported) != 3 or len(digests) > 1:
                        outcome, reason = "failed", "one-command add did not prove three bindings on one package digest"
                self.add(f"context7_three_target_{operation}", "context7", target_arg, "materialization", outcome, reason or "unknown result", tuple_value=self.tuple(digest=resolved_digest, client_version="isolated-layout-fixture/v1", dependency="pinned-directory-catalog"), details=details)

    def hero_runtime_matrix(self) -> None:
        for plugin in self.config["heroes"]:
            for client in self.config["runtime_clients"]:
                record = self.attestations.get((plugin, client, "runtime"))
                if record:
                    self.add("hero_5x3_runtime", plugin, client, "runtime", record["outcome"], record.get("reason", "explicit runtime attestation"), tuple_value=record.get("tuple"), details={"consent_attested": bool(record.get("consent_attested")), "isolated_identity": bool(record.get("isolated_identity"))})
                else:
                    reason = "runtime client/isolated identity attestation was not supplied" if plugin == "notion" else "client runtime attestation was not supplied"
                    self.add("hero_5x3_runtime", plugin, client, "runtime", "not_tested", reason)
        chatgpt = self.attestations.get(("cloudflare-docs", "chatgpt", "oauth"))
        if chatgpt:
            self.add("chatgpt_registered_binding", "cloudflare-docs", "chatgpt", "oauth", chatgpt["outcome"], chatgpt.get("reason", "explicit OAuth/runtime attestation"), tuple_value=chatgpt.get("tuple"), details={"consent_attested": bool(chatgpt.get("consent_attested")), "isolated_identity": bool(chatgpt.get("isolated_identity"))})
        else:
            self.add("chatgpt_registered_binding", "cloudflare-docs", "chatgpt", "oauth", "not_tested", "registered app binding and human UI consent attestation were not supplied")

    def hero_lifecycle_matrix(self) -> None:
        for plugin in self.config["heroes"]:
            for client in self.config["runtime_clients"]:
                with tempfile.TemporaryDirectory(prefix="uap-hero-lifecycle-") as tmp:
                    sandbox = Path(tmp)
                    outcomes: list[tuple[str, str, str]] = []
                    resolved_digest: str | None = None
                    for operation in ("add", "info", "update", "remove"):
                        outcome, value, reason = self.command([operation, plugin, "--target", client, "--format", "json"], sandbox, (client,))
                        digests = collect_digests(value) if value else set()
                        if len(digests) == 1:
                            resolved_digest = next(iter(digests))
                        if outcome == "passed" and resolved_digest is None:
                            outcome, reason = "inconclusive", "CLI output did not expose the immutable package digest"
                        if outcome == "passed" and operation == "info":
                            receipt = find_value(value, {"receipt_reconciled", "ownership_reconciled", "receipts"})
                            discovery = find_value(value, {"native_discovery_reconciled", "discovery", "verification"})
                            if receipt in (None, False, [], {}) or discovery in (None, False, [], {}):
                                outcome, reason = "inconclusive", "info omitted receipt/native discovery reconciliation"
                        outcomes.append((operation, outcome, reason or "unknown result"))
                first_nonpass = next((item for item in outcomes if item[1] != "passed"), None)
                overall = first_nonpass[1] if first_nonpass else "passed"
                reason = f"{first_nonpass[0]}: {first_nonpass[2]}" if first_nonpass else "add/info/update/remove completed in one disposable client home"
                self.add("hero_5x3_lifecycle", plugin, client, "discovery", overall, reason, tuple_value=self.tuple(digest=resolved_digest, client_version="isolated-layout-fixture/v1", dependency="pinned-directory-catalog"), details={"operations": [item[0] for item in outcomes], "operation_outcomes": {item[0]: item[1] for item in outcomes}})

    def shared_backend(self) -> None:
        targets = tuple(self.config["shared_backend_targets"])
        with tempfile.TemporaryDirectory(prefix="uap-shared-backend-") as tmp:
            sandbox = Path(tmp)
            add_outcome, value, add_reason = self.command(["add", "context7", "--target", ",".join(targets), "--format", "json"], sandbox, targets)
            physical = find_value(value, {"physical_artifacts", "physical_artifact_ids", "physical_artifact_id"}) if value else None
            if add_outcome == "passed" and physical is None:
                add_outcome, add_reason = "inconclusive", "add output omitted physical backend identity"
            if add_outcome == "passed" and isinstance(physical, list) and len(set(map(str, physical))) != 1:
                add_outcome, add_reason = "failed", "Copilot and VS Code did not collapse to one physical artifact"
            remove_outcome, _, remove_reason = self.command(["remove", "context7", "--target", ",".join(targets), "--format", "json"], sandbox, targets)
            outcome = add_outcome if add_outcome != "passed" else remove_outcome
            reason = add_reason if add_outcome != "passed" else remove_reason
            digests = collect_digests(value) if value else set()
            digest = next(iter(digests)) if len(digests) == 1 else None
            if outcome == "passed" and digest is None:
                outcome, reason = "inconclusive", "shared-backend output omitted the immutable package digest"
            self.add("shared_copilot_vscode_backend", "context7", "copilot,vscode", "materialization", outcome, reason or "unknown result", tuple_value=self.tuple(digest=digest, client_version="isolated-layout-fixture/v1", dependency="copilot-shared-backend"), details={"expected_physical_mutations_per_operation": 1, "operations": ["add", "remove"]})

    def fault_matrix(self) -> None:
        for scenario in (*self.config["fault_scenarios"], *self.config["adapter_repair_faults"], *self.config["advanced_scenarios"]):
            outcome, value, reason = self.driven_scenario(scenario)
            tuple_value = value.get("tuple") if value else None
            client = scenario.removeprefix("repair_") if scenario.startswith("repair_") else "cursor"
            self.add(scenario, "context7", client, "materialization", outcome, reason, tuple_value=tuple_value, details={"fixture_contract_present": scenario in self.config["fault_scenarios"], "scenario_driver_required": True})

    def journeys(self) -> None:
        digest = package_digest(EXTERNAL_PACKAGE)
        with tempfile.TemporaryDirectory(prefix="uap-direct-external-") as tmp:
            sandbox = Path(tmp)
            outcome, _, reason = self.command(["add", str(EXTERNAL_PACKAGE), "--target", "cursor", "--format", "json"], sandbox, ("cursor",))
        self.add("direct_external_package", "e2e-external-package", "cursor", "materialization", outcome, reason or "unknown result", tuple_value=self.tuple(digest=digest, client_version="isolated-layout-fixture/v1", dependency="direct-local-source"), details={"directory_submission_used": False, "source_locator": "fixture://external-package"})
        fork_outcome, fork_value, fork_reason = self.driven_scenario("fork_submission")
        self.add("fork_submission", "e2e-external-package", None, "harness", fork_outcome, fork_reason, tuple_value=fork_value.get("tuple") if fork_value else self.tuple(digest=digest), details={"publication_or_pr_created": fork_outcome == "passed"})

    def export(self) -> dict[str, Any]:
        self.discover_version()
        self.validate_fixtures()
        self.all_package_matrix()
        self.context7_multi_target()
        self.hero_lifecycle_matrix()
        self.hero_runtime_matrix()
        self.shared_backend()
        self.fault_matrix()
        self.journeys()
        counts = Counter(row["outcome"] for row in self.rows)
        required = [row for row in self.rows if row["scenario"] != "fixture_contracts"]
        complete = bool(required) and all(row["outcome"] in {"passed", "not_applicable"} for row in required)
        run_seed = json.dumps([self.observed_at, self.os_name, self.architecture, sha256_file(SCENARIOS)])
        return {
            "schema_version": 1,
            "run": {"id": hashlib.sha256(run_seed.encode()).hexdigest()[:16], "observed_at": self.observed_at, "platform": self.os_name, "architecture": self.architecture, "disposable": True, "cli": {"available": self.cli_available, "version": self.cli_version}},
            "matrix": self.rows,
            "summary": {**{name: counts[name] for name in ("passed", "failed", "inconclusive", "not_tested", "not_applicable")}, "required_gates_complete": complete},
            "privacy": {"redacted_export": True, "contains_absolute_home_paths": False, "contains_credentials": False, "real_user_project_used": False},
        }


def assert_redacted(value: dict[str, Any]) -> None:
    """Refuse evidence containing obvious credentials or absolute home paths."""
    for row in value.get("matrix", []):
        if row.get("outcome") != "passed" or row.get("level") == "harness":
            continue
        tuple_value = row.get("tuple", {})
        required = ("package_digest", "installer_version", "adapter_version", "client_version", "os", "architecture", "observed_at")
        if any(not tuple_value.get(field) for field in required):
            raise ValueError(f"passed evidence has an incomplete applicability tuple: {row.get('id')}")
        if not DIGEST.fullmatch(str(tuple_value["package_digest"])):
            raise ValueError(f"passed evidence has an invalid package digest: {row.get('id')}")
    body = json.dumps(value, sort_keys=True)
    if SECRET_NAME.search(body):
        # Schema field names describe privacy exclusions; only reject assignment-like values.
        if re.search(r'(?i)(token|secret|password|cookie|authorization|oauth[_-]?code)["\s]*[:=]["\s]+(?!false|null|not_tested)', body):
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
    parser.add_argument("--binary", type=Path, help="exact Agent Plugins CLI binary")
    parser.add_argument("--attestations", type=Path, help="reviewed runtime/OAuth attestation input")
    parser.add_argument("--scenario-driver", type=Path, help="isolated fault/fork driver implementing the harness JSON contract")
    parser.add_argument("--catalog-url", help="exact public HTTPS catalog URL")
    parser.add_argument("--catalog-digest", help="sha256 digest binding the local and remote catalog")
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--require-gates", action="store_true", help="return non-zero unless every launch gate is proved")
    args = parser.parse_args()
    evidence = LaunchHarness(args.binary, args.attestations, args.scenario_driver, args.catalog_url, args.catalog_digest).export()
    assert_redacted(evidence)
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(evidence, indent=2, sort_keys=True) + "\n")
    print(json.dumps(evidence["summary"], sort_keys=True))
    return 0 if not args.require_gates or evidence["summary"]["required_gates_complete"] else 2


if __name__ == "__main__":
    raise SystemExit(main())

from __future__ import annotations

import importlib.util
import json
import os
import tempfile
import unittest
from pathlib import Path
from unittest import mock


ROOT = Path(__file__).resolve().parents[1]
MODULE = ROOT / "scripts" / "run_launch_evidence_e2e.py"
SPEC = importlib.util.spec_from_file_location("run_launch_evidence_e2e", MODULE)
assert SPEC and SPEC.loader
e2e = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(e2e)


class LaunchEvidenceE2ETests(unittest.TestCase):
    def test_catalog_and_launch_matrix_have_fixed_dimensions(self) -> None:
        config = json.loads(e2e.SCENARIOS.read_text())
        catalog = json.loads(e2e.CATALOG.read_text())
        self.assertEqual(len(catalog["plugins"]), 26)
        self.assertEqual(len({item["name"] for item in catalog["plugins"]}), 26)
        self.assertEqual(len(config["heroes"]), 5)
        self.assertEqual(config["runtime_clients"], ["codex", "cursor", "kiro"])
        self.assertEqual(config["all_package_operations"], ["add", "info", "remove"])

    def test_unavailable_cli_is_never_reported_as_pass(self) -> None:
        harness = e2e.LaunchHarness(Path("/missing/agentplugins"), None)
        evidence = harness.export()
        cli_rows = [row for row in evidence["matrix"] if row["scenario"] != "fixture_contracts"]
        self.assertTrue(cli_rows)
        self.assertFalse(evidence["summary"]["required_gates_complete"])
        self.assertFalse(evidence["run"]["cli"]["available"])
        self.assertFalse(any(row["outcome"] == "passed" for row in cli_rows))
        package_rows = [row for row in cli_rows if row["scenario"].startswith("all_26_")]
        self.assertEqual(len(package_rows), 26 * 3)

    def test_context7_add_is_one_exact_three_target_invocation(self) -> None:
        harness = e2e.LaunchHarness(None, None)
        calls: list[list[str]] = []

        def capture(argv, _sandbox, _clients):
            calls.append(argv)
            return "not_tested", None, "fixture capture"

        with mock.patch.object(harness, "command", side_effect=capture):
            harness.context7_multi_target()
        self.assertEqual(calls[0], ["add", "context7", "--target", "codex,cursor,kiro", "--format", "json"])
        self.assertEqual([call[0] for call in calls], ["add", "update", "repair", "remove"])
        self.assertNotIn("--yes", calls[0])

    def test_isolated_environment_drops_credentials_and_uses_fake_homes(self) -> None:
        inherited = {"PATH": "/bin", "GITHUB_TOKEN": "secret", "AWS_SECRET_ACCESS_KEY": "secret", "HOME": "/real/home"}
        with tempfile.TemporaryDirectory() as tmp, mock.patch.dict(os.environ, inherited, clear=True):
            sandbox = Path(tmp)
            value = e2e.isolated_environment(sandbox, ("codex", "cursor", "kiro"))
            self.assertEqual(value["HOME"], str(sandbox / "home"))
            self.assertNotIn("GITHUB_TOKEN", value)
            self.assertNotIn("AWS_SECRET_ACCESS_KEY", value)
            for client in ("codex", "cursor", "kiro"):
                self.assertTrue((sandbox / "home" / e2e.CLIENT_ROOTS[client]).is_dir())

    def test_catalog_environment_is_https_and_digest_pinned(self) -> None:
        digest = e2e.sha256_file(e2e.CATALOG)
        url = "https://raw.githubusercontent.com/example/repository/" + "a" * 40 + "/catalog/v1/catalog.json"
        value = e2e.validated_catalog_environment(url, digest)
        self.assertEqual(value["AGENTPLUGINS_CATALOG_URL"], url)
        self.assertEqual(value["AGENTPLUGINS_CATALOG_DIGEST"], digest)
        with self.assertRaises(ValueError):
            e2e.validated_catalog_environment("https://user:secret@example.test/catalog.json", digest)
        with self.assertRaises(ValueError):
            e2e.validated_catalog_environment(url, "sha256:" + "0" * 64)

    def test_runtime_pass_requires_complete_tuple(self) -> None:
        value = {
            "schema_version": 1,
            "attestations": [{
                "plugin": "context7", "client": "codex", "level": "runtime",
                "outcome": "passed", "tuple": {"package_digest": "sha256:" + "a" * 64}
            }]
        }
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / "attestations.json"
            path.write_text(json.dumps(value))
            with self.assertRaisesRegex(ValueError, "incomplete tuple"):
                e2e.LaunchHarness(None, path)

    def test_oauth_pass_requires_consent_and_isolated_identity(self) -> None:
        tuple_value = {
            "package_digest": "sha256:" + "a" * 64,
            "dependency_identity": "remote:https://example.invalid/mcp",
            "installer_version": "1.0.0", "adapter_version": "1.0.0",
            "client_version": "1.0.0", "os": "Linux", "architecture": "x86_64",
            "observed_at": "2026-08-20T00:00:00Z"
        }
        value = {"schema_version": 1, "attestations": [{"plugin": "notion", "client": "codex", "level": "runtime", "outcome": "passed", "tuple": tuple_value}]}
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / "attestations.json"
            path.write_text(json.dumps(value))
            with self.assertRaisesRegex(ValueError, "lacks consent"):
                e2e.LaunchHarness(None, path)

    def test_evidence_has_per_tuple_fields_and_no_local_paths(self) -> None:
        evidence = e2e.LaunchHarness(None, None).export()
        required = {"package_digest", "dependency_identity", "installer_version", "adapter_version", "client_version", "os", "architecture", "observed_at"}
        self.assertTrue(all(set(row["tuple"]) == required for row in evidence["matrix"]))
        e2e.assert_redacted(evidence)
        body = json.dumps(evidence)
        self.assertNotIn(str(Path.home()), body)
        self.assertFalse(evidence["privacy"]["contains_credentials"])

    def test_redacted_export_rejects_a_pass_with_incomplete_tuple(self) -> None:
        evidence = e2e.LaunchHarness(None, None).export()
        row = next(item for item in evidence["matrix"] if item["level"] != "harness")
        row["outcome"] = "passed"
        with self.assertRaisesRegex(ValueError, "incomplete applicability tuple"):
            e2e.assert_redacted(evidence)

    def test_redacted_export_rejects_absolute_temporary_paths(self) -> None:
        evidence = e2e.LaunchHarness(None, None).export()
        evidence["matrix"][0]["reason"] = "driver wrote /tmp/private/result.json"
        with self.assertRaisesRegex(ValueError, "absolute local path"):
            e2e.assert_redacted(evidence)

    def test_fixture_contracts_cover_state_crash_offline_and_tamper(self) -> None:
        config = json.loads(e2e.SCENARIOS.read_text())
        recovery = json.loads(e2e.RECOVERY_FIXTURE.read_text())
        state = json.loads(e2e.STATE_FIXTURE.read_text())
        self.assertEqual(state["schema_version"], 2)
        fixture_cases = {item["id"] for item in recovery["cases"]}
        self.assertEqual(fixture_cases, set(config["fault_scenarios"]) - {"state_schema_2_migration"})
        self.assertTrue((e2e.EXTERNAL_PACKAGE / "plugin.json").is_file())


if __name__ == "__main__":
    unittest.main()

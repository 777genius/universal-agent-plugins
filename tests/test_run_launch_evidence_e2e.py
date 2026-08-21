from __future__ import annotations

import importlib.util
import json
import os
import tempfile
import unittest
from pathlib import Path
from unittest import mock

import jsonschema


ROOT = Path(__file__).resolve().parents[1]
MODULE = ROOT / "scripts" / "run_launch_evidence_e2e.py"
SPEC = importlib.util.spec_from_file_location("run_launch_evidence_e2e", MODULE)
assert SPEC and SPEC.loader
e2e = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(e2e)
CONSENT = ROOT / "tests/e2e/fixtures/fixture-only-consent.json"
PUBLICATION = ROOT / "tests/fixtures/directory-publication"


class LaunchEvidenceE2ETests(unittest.TestCase):
    def fixture_harness(self, root: Path | None = None, **kwargs):
        return e2e.LaunchHarness(
            None, None, mode="fixture-only", consent=CONSENT, run_root=root, **kwargs
        )

    def test_fixture_mode_is_explicitly_non_runtime(self) -> None:
        with tempfile.TemporaryDirectory() as tmp, mock.patch.object(e2e, "ROOT", Path("/opt/test-repository")):
            evidence = self.fixture_harness(Path(tmp) / "fresh").export()
        self.assertEqual(evidence["schema_version"], 2)
        self.assertEqual(evidence["run"]["mode"], "fixture-only")
        self.assertFalse(evidence["run"]["runtime_claims"])
        self.assertFalse(evidence["summary"]["required_gates_complete"])
        self.assertEqual(evidence["summary"]["hero_runtime_results"], 0)
        e2e.assert_redacted(evidence)

    def test_enforced_mode_refuses_missing_live_inputs_before_evidence(self) -> None:
        with self.assertRaisesRegex(ValueError, "missing required input"):
            e2e.LaunchHarness(None, None, mode="enforced", consent=CONSENT)

    def test_stable_version_floor(self) -> None:
        with self.assertRaisesRegex(ValueError, "0.1.8 or newer"):
            e2e.parse_stable_version("0.1.6")
        self.assertEqual(e2e.parse_stable_version("0.1.8"), (0, 1, 8))
        self.assertEqual(e2e.parse_stable_version("1.0.0"), (1, 0, 0))
        with self.assertRaisesRegex(ValueError, "exact semantic version"):
            e2e.parse_stable_version("latest")
        with self.assertRaisesRegex(ValueError, "exact semantic version"):
            e2e.parse_stable_version("0.1.8-rc.1")

    def test_signed_directory_fixture_binds_origin_digest_sequence_and_trust(self) -> None:
        env, snapshot, digest = e2e.validated_directory_environment(
            "https://directory.example.test/registry/",
            PUBLICATION / "snapshot.json",
            PUBLICATION / "envelope-current.json",
            PUBLICATION / "trusted-keys.json",
        )
        self.assertEqual(snapshot["sequence"], 7)
        self.assertEqual(digest, json.loads((PUBLICATION / "envelope-current.json").read_text())["snapshot_digest"])
        self.assertNotIn("CATALOG", " ".join(env))
        self.assertIn("AGENTPLUGINS_DIRECTORY_ORIGIN", env)

    def test_disposable_root_must_be_fresh(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            with self.assertRaisesRegex(ValueError, "must not already exist"):
                self.fixture_harness(Path(tmp))

    def test_isolated_environment_has_separate_roots_and_drops_auth(self) -> None:
        inherited = {"PATH": "/bin", "HOME": "/real/home", "GITHUB_TOKEN": "secret", "AWS_SECRET_ACCESS_KEY": "secret"}
        with tempfile.TemporaryDirectory() as tmp, mock.patch.dict(os.environ, inherited, clear=True):
            sandbox = Path(tmp) / "scenario"
            sandbox.mkdir()
            env = e2e.isolated_environment(sandbox, ("codex", "cursor", "kiro"))
            roots = {env[name] for name in ("HOME", "XDG_CONFIG_HOME", "XDG_CACHE_HOME", "AGENTPLUGINS_HOME", "AGENTPLUGINS_EVIDENCE_ROOT")}
            self.assertEqual(len(roots), 5)
            self.assertNotIn("GITHUB_TOKEN", env)
            self.assertNotIn("AWS_SECRET_ACCESS_KEY", env)
            self.assertTrue(all(Path(path).is_relative_to(sandbox) for path in roots))

    def test_driver_result_outside_disposable_root_is_rejected(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            sandbox = Path(tmp) / "scenario"
            sandbox.mkdir()
            with self.assertRaisesRegex(ValueError, "outside the disposable root"):
                e2e.LaunchHarness._assert_result_paths({"evidence_path": "/real/project/result.json"}, sandbox)

    def test_info_reconciliation_requires_exact_boolean_proofs(self) -> None:
        self.assertTrue(e2e.LaunchHarness.info_reconciled({"receipt_reconciled": True, "native_discovery_reconciled": True}))
        self.assertFalse(e2e.LaunchHarness.info_reconciled({"receipts": ["owned"], "discovery": {"state": "found"}}))
        self.assertFalse(e2e.LaunchHarness.info_reconciled({"receipt_reconciled": True, "native_discovery_reconciled": False}))

    def test_mutable_refs_and_unknown_outcomes_are_rejected(self) -> None:
        with self.assertRaisesRegex(ValueError, "mutable refs"):
            e2e.LaunchHarness._reject_mutable_refs({"ref": "main"})
        with tempfile.TemporaryDirectory() as tmp:
            attestation = Path(tmp) / "attestation.json"
            attestation.write_text(json.dumps({"schema_version": 1, "attestations": [{"plugin": "context7", "client": "codex", "level": "runtime", "outcome": "not_tested", "tuple": {}}]}))
            with self.assertRaisesRegex(ValueError, "invalid attestation outcome"):
                e2e.LaunchHarness(None, attestation, mode="fixture-only", consent=CONSENT)

    def test_duplicate_tuples_and_broad_chatgpt_claims_are_rejected(self) -> None:
        evidence = self.fixture_harness().export()
        evidence["matrix"].append(dict(evidence["matrix"][0]))
        with self.assertRaisesRegex(ValueError, "duplicate tuples"):
            e2e.assert_redacted(evidence)
        evidence = self.fixture_harness().export()
        evidence["matrix"].append({**evidence["matrix"][0], "id": "a" * 24, "scenario": "chatgpt", "plugin": "notion", "client": "chatgpt"})
        with self.assertRaisesRegex(ValueError, "ChatGPT"):
            e2e.assert_redacted(evidence)

    def test_launch_schema_rejects_unknown_outcome_and_mutable_ref(self) -> None:
        schema = json.loads((ROOT / "tests/e2e/schemas/launch-evidence.schema.json").read_text())
        evidence = self.fixture_harness().export()
        evidence["matrix"][0]["outcome"] = "not_tested"
        with self.assertRaises(jsonschema.ValidationError):
            jsonschema.Draft202012Validator(schema).validate(evidence)
        evidence = self.fixture_harness().export()
        evidence["matrix"][0]["details"]["ref"] = "main"
        with self.assertRaises(jsonschema.ValidationError):
            jsonschema.Draft202012Validator(schema).validate(evidence)

    def test_context7_contract_requires_one_three_target_grouped_lifecycle(self) -> None:
        harness = self.fixture_harness()
        harness.snapshot = {
            "sequence": 1,
            "products": [{"id": "context7", "default_distribution": "upstash/context7"}],
            "distributions": [{"id": "upstash/context7", "kind": "upstream", "release_policies": [{"release_sequence": 1, "status": "active", "targets": [{"client": "codex", "scopes": ["user"]}, {"client": "cursor", "scopes": ["user"]}, {"client": "kiro", "scopes": ["user"]}]}], "releases": [{"sequence": 1, "package_version": "1.0.0", "tree_digest": "sha256:" + "a" * 64, "manifest_digest": "sha256:" + "b" * 64}]}],
        }
        harness.snapshot_digest = "sha256:" + "c" * 64
        harness.binary_digest = "sha256:" + "d" * 64
        harness.expected_version = "0.1.8"
        commands = [[operation, "context7", "--target", "codex,cursor,kiro", "--format", "json"] for operation in ("add", "update", "repair", "remove")]
        value = {
            "commands": commands, "acquisition_digests": ["sha256:" + "a" * 64],
            "target_outcomes": {client: "passed" for client in ("codex", "cursor", "kiro")},
            "operation_outcomes": {operation: "passed" for operation in ("add", "update", "repair", "remove")},
            "tuple": harness.evidence_tuple("context7", client_version="driver", dependency="single-acquisition"),
        }
        with mock.patch.object(harness, "driven_scenario", return_value=("passed", value, "proved")):
            harness.context7_multi_target()
        self.assertEqual([row["outcome"] for row in harness.rows], ["passed"] * 4)
        self.assertTrue(all(row["details"]["target_argument"] == "codex,cursor,kiro" for row in harness.rows))
        self.assertNotIn("--yes", commands[0])

    def test_fixture_contracts_cover_required_fault_slots(self) -> None:
        config = json.loads(e2e.SCENARIOS.read_text())
        required = {
            "directory_offline", "directory_expired", "directory_tampered", "directory_sequence_rollback",
            "missing_runtime_zero_mutation", "plugin_data_update_repair_switch_remove_purge",
            "stdio_environment_and_containment", "promotion_gate_digest_mismatch",
            "cross_platform_binary_npm_install", "distribution_sticky_update",
        }
        observed = set(config["fault_scenarios"] + config["advanced_scenarios"])
        self.assertTrue(required.issubset(observed))
        for scenario in config["fault_scenarios"] + config["adapter_repair_faults"] + config["advanced_scenarios"]:
            with self.subTest(scenario=scenario):
                self.assertFalse(e2e.LaunchHarness.driver_proof_valid(scenario, {"outcome": "passed"}))

    def test_missing_runtime_proof_requires_zero_mutation_and_no_install(self) -> None:
        proof = {"zero_mutation": True, "copy_ready_requirement": True, "dependency_installed": False}
        self.assertTrue(e2e.LaunchHarness.driver_proof_valid("missing_runtime_zero_mutation", proof))
        self.assertFalse(e2e.LaunchHarness.driver_proof_valid("missing_runtime_zero_mutation", {**proof, "dependency_installed": True}))


if __name__ == "__main__":
    unittest.main()

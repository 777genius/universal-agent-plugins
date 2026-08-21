from __future__ import annotations

import base64
import json
import os
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

import yaml


ROOT = Path(__file__).resolve().parents[1]
SCRIPTS = ROOT / "scripts"
FIXTURES = ROOT / "tests" / "fixtures" / "directory-publication"
sys.path.insert(0, str(SCRIPTS))
import directory_publication as publication


class OpenSSLParityTests(unittest.TestCase):
    def test_system_openssl_signature_is_byte_identical_to_ed25519_contract(self) -> None:
        from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey

        seeds = json.loads((FIXTURES / "test-private-seeds.json").read_bytes())
        snapshot = (FIXTURES / "snapshot.json").read_bytes()
        message = publication.signature_message(snapshot)
        for key_id, encoded in seeds.items():
            with self.subTest(key_id=key_id):
                seed = base64.b64decode(encoded, validate=True)
                expected = Ed25519PrivateKey.from_private_bytes(seed).sign(message)
                actual = publication.ed25519_sign(seed, message)
                self.assertEqual(actual, expected)
                publication.ed25519_verify(publication.ed25519_public_bytes(seed), message, actual)


class LedgerFailureTests(unittest.TestCase):
    def run_signer(self, root: Path, publication_id: str, *ledger_args: str) -> subprocess.CompletedProcess[str]:
        candidate = root / "candidate.json"
        value = json.loads((FIXTURES / "candidate.json").read_bytes())
        value["publication_id"] = publication_id
        candidate.write_bytes(publication.canonical_json(value))
        seed = json.loads((FIXTURES / "test-private-seeds.json").read_bytes())["test-current"]
        environment = os.environ.copy()
        environment["DIRECTORY_ED25519_PRIVATE_KEY"] = seed
        day = {"run-1": "21", "run-2": "22", "run-3": "23"}[publication_id]
        return subprocess.run(
            [
                sys.executable, "-I", str(SCRIPTS / "sign_directory_publication.py"),
                "--candidate", str(candidate),
                "--candidate-digest", publication.candidate_digest(candidate.read_bytes()),
                "--ledger", str(root),
                "--trusted-keys", str(FIXTURES / "trusted-keys.json"),
                "--key-id", "test-current", "--now", f"2026-08-{day}T00:00:00Z",
                "--result", str(root / "result.json"), *ledger_args,
            ],
            cwd=ROOT, env=environment, text=True, stdout=subprocess.PIPE,
            stderr=subprocess.PIPE, check=False,
        )

    def test_initialization_is_explicit_exact_and_cannot_be_repeated(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            missing = self.run_signer(root, "run-1")
            self.assertNotEqual(missing.returncode, 0)
            self.assertIn("explicit initialization is required", missing.stderr)
            invalid = self.run_signer(root, "run-1", "--initialize-ledger", "--ledger-seed-commit", "not-a-sha")
            self.assertNotEqual(invalid.returncode, 0)
            initialized = self.run_signer(root, "run-1", "--initialize-ledger", "--ledger-seed-commit", "0" * 40)
            self.assertEqual(initialized.returncode, 0, initialized.stderr)
            repeated = self.run_signer(root, "run-2", "--initialize-ledger", "--ledger-seed-commit", "0" * 40)
            self.assertNotEqual(repeated.returncode, 0)
            self.assertIn("existing publication ledger", repeated.stderr)
            no_floor = self.run_signer(root, "run-2", "--ledger-seed-commit", "0" * 40)
            self.assertNotEqual(no_floor.returncode, 0)
            self.assertIn("immutable tag sequence floor", no_floor.stderr)

    def test_pointer_loss_floor_regression_and_rerun_fail_closed(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            first = self.run_signer(root, "run-1", "--initialize-ledger", "--ledger-seed-commit", "0" * 40)
            self.assertEqual(first.returncode, 0, first.stderr)
            retry = self.run_signer(root, "run-1", "--ledger-seed-commit", "0" * 40, "--ledger-sequence-floor", "1")
            self.assertEqual(retry.returncode, 0, retry.stderr)
            self.assertTrue(json.loads((root / "result.json").read_bytes())["reused"])

            second = self.run_signer(root, "run-2", "--ledger-seed-commit", "0" * 40, "--ledger-sequence-floor", "1")
            self.assertEqual(second.returncode, 0, second.stderr)
            regressed = self.run_signer(root, "run-3", "--ledger-seed-commit", "0" * 40, "--ledger-sequence-floor", "3")
            self.assertNotEqual(regressed.returncode, 0)
            self.assertIn("immutable tag floor", regressed.stderr)

            latest = root / "registry" / "schemas" / "1" / "latest.json"
            latest.unlink()
            lost = self.run_signer(root, "run-3", "--ledger-seed-commit", "0" * 40, "--ledger-sequence-floor", "2")
            self.assertNotEqual(lost.returncode, 0)
            self.assertIn("latest pointer is missing", lost.stderr)

    def test_contract_marker_loss_and_nonempty_reinitialization_fail_closed(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            first = self.run_signer(root, "run-1", "--initialize-ledger", "--ledger-seed-commit", "0" * 40)
            self.assertEqual(first.returncode, 0, first.stderr)
            (root / "registry" / "schemas" / "1" / publication.LEDGER_CONTRACT_NAME).unlink()
            lost = self.run_signer(root, "run-2", "--ledger-seed-commit", "0" * 40, "--ledger-sequence-floor", "1")
            self.assertNotEqual(lost.returncode, 0)
            self.assertIn("cannot read", lost.stderr)

        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            feed = root / "registry" / "schemas" / "1"
            feed.mkdir(parents=True)
            (feed / "unexpected").write_text("seed tree collision\n")
            rejected = self.run_signer(root, "run-1", "--initialize-ledger", "--ledger-seed-commit", "0" * 40)
            self.assertNotEqual(rejected.returncode, 0)
            self.assertIn("initial publication feed is not empty", rejected.stderr)


class WorkflowHardeningTests(unittest.TestCase):
    def test_only_app_tokens_write_ledger_and_floor_tags_are_atomic(self) -> None:
        text = (ROOT / ".github" / "workflows" / "directory-publication.yml").read_text()
        workflow = yaml.load(text, Loader=yaml.BaseLoader)
        self.assertNotIn("github.token", text)
        self.assertNotIn("GH_TOKEN", text)
        self.assertEqual(workflow["jobs"]["sign"]["permissions"]["contents"], "read")
        self.assertEqual(workflow["jobs"]["materialize_site"]["permissions"]["contents"], "read")
        self.assertEqual(workflow["jobs"]["sign"]["environment"], "directory-publication")
        self.assertEqual(workflow["jobs"]["materialize_site"]["environment"], "directory-publication-materialization")
        self.assertEqual(text.count("actions/create-github-app-token@bcd2ba49218906704ab6c1aa796996da409d3eb1"), 2)
        self.assertIn("push --atomic", text)
        self.assertIn("refs/tags/${tag_name}:refs/tags/${tag_name}", text)
        self.assertIn("merge-base --is-ancestor", text)
        self.assertEqual(workflow["concurrency"], {
            "group": "directory-publication-schema-1",
            "cancel-in-progress": "false",
        })
        self.assertGreaterEqual(text.count("merge-base --is-ancestor"), 3)
        self.assertIn('merge-base --is-ancestor "${seed_commit}" HEAD', text)
        self.assertIn('merge-base --is-ancestor "refs/tags/${tag}" HEAD', text)
        self.assertIn('test "${tag_sequence}" -eq "$((sequence_floor + 1))"', text)
        self.assertIn("contract_status=", text)
        self.assertIn("test -z \"${contract_status}\"", text)

    def test_privileged_signer_has_no_downloaded_dependency_install(self) -> None:
        workflow = yaml.load((ROOT / ".github" / "workflows" / "directory-publication.yml").read_text(), Loader=yaml.BaseLoader)
        signer = workflow["jobs"]["sign"]
        commands = "\n".join(step.get("run", "") for step in signer["steps"] if isinstance(step, dict))
        self.assertNotIn("pip install", commands)
        self.assertNotIn("setup-python", json.dumps(signer))
        self.assertIn("/usr/bin/openssl", commands)
        self.assertIn("dpkg --verify", commands)
        self.assertNotIn("jsonschema", commands)
        signer_source = (SCRIPTS / "sign_directory_publication.py").read_text()
        self.assertNotIn("validate_with_schema", signer_source)
        self.assertNotIn("jsonschema", signer_source)

    def test_legacy_pages_is_pull_request_validation_only_and_all_workflows_are_owned(self) -> None:
        pages_text = (ROOT / ".github" / "workflows" / "pages.yml").read_text()
        pages = yaml.load(pages_text, Loader=yaml.BaseLoader)
        self.assertEqual(set(pages["on"]), {"pull_request"})
        self.assertNotIn("deploy", pages["jobs"])
        self.assertNotIn("deploy-pages", pages_text)
        self.assertNotIn("pages: write", pages_text)
        codeowners = (ROOT / ".github" / "CODEOWNERS").read_text()
        self.assertIn("/.github/workflows/ @777genius", codeowners)

    def test_documented_rulesets_leave_no_destructive_bypass(self) -> None:
        documentation = (ROOT / "registry" / "publication" / "README.md").read_text()
        self.assertIn("four active repository rulesets", documentation)
        self.assertEqual(documentation.count("has **no bypass actors**"), 2)
        self.assertIn("even the publisher cannot reset the branch", documentation)
        self.assertIn("administrators", documentation)
        self.assertIn("deploy keys", documentation)
        self.assertIn("generic Actions", documentation)


if __name__ == "__main__":
    unittest.main()

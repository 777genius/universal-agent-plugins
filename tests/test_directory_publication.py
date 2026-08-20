from __future__ import annotations

import copy
import importlib.util
import json
import os
import shutil
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
import prepare_directory_publication as prepare


def fixture(name: str) -> bytes:
    return (FIXTURES / name).read_bytes()


def fixture_json(name: str):  # type: ignore[no-untyped-def]
    return json.loads(fixture(name))


def run_script(name: str, *arguments: str, env: dict[str, str] | None = None) -> subprocess.CompletedProcess[str]:
    merged = os.environ.copy()
    if env:
        merged.update(env)
    return subprocess.run(
        [sys.executable, str(SCRIPTS / name), *arguments],
        cwd=ROOT,
        env=merged,
        text=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        check=False,
    )


def install_fixture_feed(root: Path, envelope: str = "envelope-current.json") -> Path:
    feed = root / "registry" / "schemas" / "1"
    snapshots = feed / "snapshots"
    snapshots.mkdir(parents=True)
    (feed / "latest.json").write_bytes(fixture("latest.json"))
    (snapshots / "00000000000000000007.json").write_bytes(fixture("snapshot.json"))
    (snapshots / "00000000000000000007.envelope.json").write_bytes(fixture(envelope))
    return feed


class CanonicalAndSignatureTests(unittest.TestCase):
    def test_all_contract_fixtures_are_canonical_and_schema_valid(self) -> None:
        schemas = {
            "candidate.json": publication.CANDIDATE_SCHEMA,
            "snapshot.json": publication.SNAPSHOT_SCHEMA,
            "envelope-current.json": publication.ENVELOPE_SCHEMA,
            "envelope-next.json": publication.ENVELOPE_SCHEMA,
            "latest.json": publication.LATEST_SCHEMA,
        }
        for name, schema in schemas.items():
            with self.subTest(name=name):
                value = fixture_json(name)
                self.assertEqual(fixture(name), publication.canonical_json(value))
                publication.validate_with_schema(value, schema)

        malformed = fixture_json("candidate.json")
        malformed["products"][0]["distributions"][0]["releases"][0]["unexpected"] = True
        with self.assertRaises(publication.PublicationError):
            publication.validate_with_schema(malformed, publication.CANDIDATE_SCHEMA)

        snapshot_schema = json.loads(publication.SNAPSHOT_SCHEMA.read_bytes())
        candidate_schema = json.loads(publication.CANDIDATE_SCHEMA.read_bytes())
        candidate_defs = copy.deepcopy(candidate_schema["$defs"])
        candidate_defs["release"]["properties"]["published_at"] = snapshot_schema["$defs"]["release"]["properties"]["published_at"]
        self.assertEqual(candidate_defs, snapshot_schema["$defs"])

    def test_signature_domain_digest_and_two_key_overlap(self) -> None:
        snapshot = fixture("snapshot.json")
        keys = publication.load_public_keys(FIXTURES / "trusted-keys.json")
        for envelope_name in ("envelope-current.json", "envelope-next.json"):
            with self.subTest(envelope=envelope_name):
                publication.verify_envelope(snapshot, fixture_json(envelope_name), keys)

        current_only = {"test-current": keys["test-current"]}
        with self.assertRaisesRegex(publication.PublicationError, "unknown signing key"):
            publication.verify_envelope(snapshot, fixture_json("envelope-next.json"), current_only)

    def test_tamper_and_detached_digest_mismatch_fail_closed(self) -> None:
        keys = publication.load_public_keys(FIXTURES / "trusted-keys.json")
        envelope = fixture_json("envelope-current.json")
        tampered = fixture("snapshot.json").replace(b'"Demo"', b'"Demu"', 1)
        with self.assertRaisesRegex(publication.PublicationError, "digest mismatch"):
            publication.verify_envelope(tampered, envelope, keys)
        envelope["snapshot_digest"] = "sha256:" + "0" * 64
        with self.assertRaisesRegex(publication.PublicationError, "digest mismatch"):
            publication.verify_envelope(fixture("snapshot.json"), envelope, keys)
        envelope = fixture_json("envelope-current.json")
        envelope["signature"] = "A" * 86 + "=="
        with self.assertRaisesRegex(publication.PublicationError, "invalid Ed25519"):
            publication.verify_envelope(fixture("snapshot.json"), envelope, keys)

    def test_canonicalization_rejects_floats_and_normalization_collisions(self) -> None:
        with self.assertRaises(publication.PublicationError):
            publication.canonical_json({"value": 1.5})
        with self.assertRaises(publication.PublicationError):
            publication.parse_json_bytes(b'{"A":1,"a":2}', "collision", max_bytes=100)


class ClientContractTests(unittest.TestCase):
    def test_valid_floor_rollback_and_expiry(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            feed = install_fixture_feed(Path(tmp))
            common = ("--feed", str(feed), "--trusted-keys", str(FIXTURES / "trusted-keys.json"))
            valid = run_script("verify_directory_publication.py", *common, "--now", "2026-08-21T00:00:00Z", "--minimum-sequence", "7")
            self.assertEqual(valid.returncode, 0, valid.stderr)
            rollback = run_script("verify_directory_publication.py", *common, "--now", "2026-08-21T00:00:00Z", "--minimum-sequence", "8")
            self.assertNotEqual(rollback.returncode, 0)
            self.assertIn("below local floor", rollback.stderr)
            expired = run_script("verify_directory_publication.py", *common, "--now", "2026-09-20T00:00:00Z")
            self.assertNotEqual(expired.returncode, 0)
            self.assertIn("expired", expired.stderr)
            recovery = run_script("verify_directory_publication.py", *common, "--now", "2026-09-20T00:00:00Z", "--allow-expired-ledger")
            self.assertEqual(recovery.returncode, 0, recovery.stderr)

    def test_latest_pointer_is_strictly_relative_and_bounded(self) -> None:
        latest = fixture_json("latest.json")
        publication.validate_latest(latest)
        for unsafe in ("https://evil.example/snapshot.json", "/snapshot.json", "../snapshot.json"):
            changed = copy.deepcopy(latest)
            changed["snapshot_path"] = unsafe
            with self.subTest(path=unsafe), self.assertRaises(publication.PublicationError):
                publication.validate_latest(changed)
        self.assertFalse(latest["fetch_contract"]["forward_credentials_on_redirect"])
        self.assertEqual(latest["fetch_contract"]["max_redirects"], 2)

    def test_oversized_snapshot_is_rejected_before_signature_processing(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            feed = install_fixture_feed(Path(tmp))
            snapshot_path = feed / "snapshots" / "00000000000000000007.json"
            snapshot_path.write_bytes(b"x" * (publication.MAX_SNAPSHOT_BYTES + 1))
            result = run_script(
                "verify_directory_publication.py",
                "--feed", str(feed),
                "--trusted-keys", str(FIXTURES / "trusted-keys.json"),
                "--now", "2026-08-21T00:00:00Z",
            )
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("exceeds 4194304 bytes", result.stderr)


class PublicationLifecycleTests(unittest.TestCase):
    def signer(self, root: Path, candidate: Path, publication_id: str, now: str) -> subprocess.CompletedProcess[str]:
        value = json.loads(candidate.read_bytes())
        value["publication_id"] = publication_id
        candidate.write_bytes(publication.canonical_json(value))
        digest = publication.candidate_digest(candidate.read_bytes())
        seed = fixture_json("test-private-seeds.json")["test-current"]
        return run_script(
            "sign_directory_publication.py",
            "--candidate", str(candidate),
            "--candidate-digest", digest,
            "--ledger", str(root),
            "--trusted-keys", str(FIXTURES / "trusted-keys.json"),
            "--key-id", "test-current",
            "--now", now,
            "--result", str(root / "result.json"),
            env={"DIRECTORY_ED25519_PRIVATE_KEY": seed},
        )

    def test_weekly_evidence_only_refresh_and_retry_do_not_duplicate_release_or_sequence(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            candidate = root / "candidate.json"
            candidate.write_bytes(fixture("candidate.json"))
            first = self.signer(root, candidate, "run-100", "2026-08-20T00:00:00Z")
            self.assertEqual(first.returncode, 0, first.stderr)
            feed = root / "registry" / "schemas" / "1"
            first_snapshot = json.loads((feed / "snapshots" / "00000000000000000001.json").read_bytes())

            value = json.loads(candidate.read_bytes())
            evidence = value["products"][0]["distributions"][0]["releases"][0]["policy"]["current_evidence"][0]
            evidence["evidence_id"] = "runtime-demo-codex-retest"
            evidence["digest"] = "sha256:" + "6" * 64
            evidence["outcome"] = "inconclusive"
            candidate.write_bytes(publication.canonical_json(value))
            second = self.signer(root, candidate, "run-101", "2026-08-27T00:00:00Z")
            self.assertEqual(second.returncode, 0, second.stderr)
            second_snapshot = json.loads((feed / "snapshots" / "00000000000000000002.json").read_bytes())
            old_release = first_snapshot["products"][0]["distributions"][0]["releases"][0]
            new_release = second_snapshot["products"][0]["distributions"][0]["releases"][0]
            self.assertEqual(old_release["sequence"], new_release["sequence"])
            self.assertEqual(old_release["package_source"], new_release["package_source"])
            self.assertEqual(old_release["published_at"], new_release["published_at"])
            self.assertNotEqual(old_release["policy"]["current_evidence"], new_release["policy"]["current_evidence"])

            original_first_artifact = (feed / "snapshots" / "00000000000000000001.json").read_bytes()
            weekly = self.signer(root, candidate, "run-102", "2026-09-03T00:00:00Z")
            self.assertEqual(weekly.returncode, 0, weekly.stderr)
            weekly_snapshot = json.loads((feed / "snapshots" / "00000000000000000003.json").read_bytes())
            weekly_release = weekly_snapshot["products"][0]["distributions"][0]["releases"][0]
            self.assertEqual(weekly_snapshot["expires_at"], "2026-10-03T00:00:00Z")
            self.assertEqual(weekly_release["sequence"], new_release["sequence"])
            self.assertEqual(weekly_release["package_source"], new_release["package_source"])
            self.assertEqual(weekly_release["published_at"], new_release["published_at"])
            self.assertEqual(
                (feed / "snapshots" / "00000000000000000001.json").read_bytes(),
                original_first_artifact,
            )

            retry = self.signer(root, candidate, "run-102", "2026-09-04T00:00:00Z")
            self.assertEqual(retry.returncode, 0, retry.stderr)
            self.assertFalse((feed / "snapshots" / "00000000000000000004.json").exists())
            self.assertEqual(json.loads((root / "result.json").read_bytes())["reused"], True)

            recycled = json.loads(candidate.read_bytes())
            recycled_evidence = recycled["products"][0]["distributions"][0]["releases"][0]["policy"]["current_evidence"][0]
            recycled_evidence["evidence_id"] = "runtime-demo-codex"
            recycled_evidence["digest"] = "sha256:" + "3" * 64
            recycled_evidence["outcome"] = "inconclusive"
            candidate.write_bytes(publication.canonical_json(recycled))
            recycled_result = self.signer(root, candidate, "run-103", "2026-09-10T00:00:00Z")
            self.assertNotEqual(recycled_result.returncode, 0)
            self.assertIn("immutable evidence runtime-demo-codex changed", recycled_result.stderr)
            self.assertFalse((feed / "snapshots" / "00000000000000000004.json").exists())

            candidate.write_bytes(publication.canonical_json(value))
            (feed / "snapshots" / "00000000000000000001.json").unlink()
            broken_history = self.signer(root, candidate, "run-104", "2026-09-17T00:00:00Z")
            self.assertNotEqual(broken_history.returncode, 0)
            self.assertIn("sequence 1 is incomplete", broken_history.stderr)
            self.assertFalse((feed / "snapshots" / "00000000000000000004.json").exists())

    def test_terminal_revocation_and_historical_removal_fail(self) -> None:
        previous = fixture_json("snapshot.json")
        changed_evidence = copy.deepcopy(previous)
        changed_evidence["sequence"] = 8
        changed_evidence["publication_id"] = "fixture-evidence-tamper"
        changed_evidence["generated_at"] = "2026-08-27T00:00:00Z"
        changed_evidence["expires_at"] = "2026-09-26T00:00:00Z"
        changed_evidence["products"][0]["distributions"][0]["releases"][0]["policy"]["current_evidence"][0]["outcome"] = "inconclusive"
        with self.assertRaisesRegex(publication.PublicationError, "immutable evidence"):
            publication.validate_snapshot_semantics(changed_evidence, previous)

        newer = copy.deepcopy(previous)
        newer["sequence"] = 8
        newer["publication_id"] = "fixture-2"
        newer["generated_at"] = "2026-08-27T00:00:00Z"
        newer["expires_at"] = "2026-09-26T00:00:00Z"
        revoked = newer["products"][0]["distributions"][1]["releases"][0]
        revoked["policy"]["status"] = "active"
        with self.assertRaisesRegex(publication.PublicationError, "cannot be restored"):
            publication.validate_snapshot_semantics(newer, previous)
        removed = copy.deepcopy(newer)
        removed["products"][0]["distributions"].pop()
        removed["products"][0]["default_distribution"] = "example/demo"
        with self.assertRaisesRegex(publication.PublicationError, "was removed"):
            publication.validate_snapshot_semantics(removed, previous)

    def test_post_merge_sha_binding_and_unchanged_release_reuse(self) -> None:
        index = json.loads((ROOT / "registry" / "index.json").read_bytes())
        config = prepare.load_config(ROOT / "registry" / "publication" / "config.json")
        source_commit = subprocess.check_output(["/usr/bin/git", "rev-parse", "HEAD"], cwd=ROOT, text=True).strip()
        first = prepare.build_candidate(index, config, source_commit, "prepare-1", None)
        for product in first["products"]:
            release = product["distributions"][0]["releases"][0]
            self.assertEqual(release["package_source"]["revision"], source_commit)
            self.assertIsNone(release["published_at"])
        signed_products = copy.deepcopy(first["products"])
        for product in signed_products:
            for distribution in product["distributions"]:
                for release in distribution["releases"]:
                    release["published_at"] = "2026-08-20T00:00:00Z"
        previous = {
            "products": signed_products,
        }
        later_commit = "e" * 40
        second = prepare.build_candidate(index, config, later_commit, "prepare-2", previous)
        for product in second["products"]:
            release = product["distributions"][0]["releases"][0]
            self.assertEqual(release["package_source"]["revision"], source_commit)
            self.assertEqual(release["published_at"], "2026-08-20T00:00:00Z")

        with tempfile.TemporaryDirectory() as tmp:
            rejected = run_script(
                "prepare_directory_publication.py",
                "--index", str(ROOT / "registry" / "index.json"),
                "--config", str(ROOT / "registry" / "publication" / "config.json"),
                "--source-commit", "f" * 40,
                "--publication-id", "wrong-head",
                "--output", str(Path(tmp) / "candidate.json"),
                "--digest-output", str(Path(tmp) / "candidate.digest"),
            )
            self.assertNotEqual(rejected.returncode, 0)
            self.assertIn("does not match --source-commit", rejected.stderr)


class PublicationWorkflowTests(unittest.TestCase):
    def test_workflow_security_and_exact_tree_contract(self) -> None:
        path = ROOT / ".github" / "workflows" / "directory-publication.yml"
        text = path.read_text()
        workflow = yaml.load(text, Loader=yaml.BaseLoader)
        self.assertNotIn("pull_request_target", workflow["on"])
        self.assertEqual(workflow["concurrency"]["cancel-in-progress"], "false")
        self.assertIn("schedule", workflow["on"])
        prepare_job = workflow["jobs"]["prepare"]
        signer = workflow["jobs"]["sign"]
        self.assertEqual(prepare_job["permissions"], {"contents": "read"})
        self.assertNotIn("secrets.", json.dumps(prepare_job))
        self.assertEqual(signer["environment"], "directory-publication")
        self.assertEqual(signer["if"], "github.ref == 'refs/heads/main'")
        signer_commands = "\n".join(step.get("run", "") for step in signer["steps"] if isinstance(step, dict))
        self.assertNotIn("build_registry.py", signer_commands)
        self.assertNotIn("plugins/", signer_commands)
        self.assertIn("for attempt in 1 2 3", signer_commands)
        self.assertIn("git diff --name-status", signer_commands)
        deploy_commands = "\n".join(step.get("run", "") for step in workflow["jobs"]["deploy"]["steps"] if isinstance(step, dict))
        self.assertIn("needs.sign.outputs.ledger_commit", text)
        self.assertIn("git -C exact-pages-tree rev-parse HEAD", deploy_commands)
        for match in __import__("re").findall(r"uses:\s+([^\s]+)", text):
            self.assertRegex(match, r"@[0-9a-f]{40}$")

    def test_all_workflow_yaml_parses(self) -> None:
        for path in (ROOT / ".github" / "workflows").glob("*.yml"):
            with self.subTest(path=path.name):
                self.assertIsInstance(yaml.safe_load(path.read_text()), dict)


if __name__ == "__main__":
    unittest.main()

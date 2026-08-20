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
        malformed["distributions"][0]["releases"][0]["unexpected"] = True
        with self.assertRaises(publication.PublicationError):
            publication.validate_with_schema(malformed, publication.CANDIDATE_SCHEMA)

        signed_with_null_time = fixture_json("snapshot.json")
        signed_with_null_time["distributions"][0]["releases"][0]["published_at"] = None
        with self.assertRaises(publication.PublicationError):
            publication.validate_with_schema(signed_with_null_time, publication.SNAPSHOT_SCHEMA)

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
            evidence = value["evidence"][0]
            evidence["id"] = "runtime-demo-codex-retest"
            evidence["artifact"]["digest"] = "sha256:" + "6" * 64
            evidence["outcome"] = "inconclusive"
            value["distributions"][0]["release_policies"][0]["current_evidence"] = [evidence["id"]]
            candidate.write_bytes(publication.canonical_json(value))
            second = self.signer(root, candidate, "run-101", "2026-08-27T00:00:00Z")
            self.assertEqual(second.returncode, 0, second.stderr)
            second_snapshot = json.loads((feed / "snapshots" / "00000000000000000002.json").read_bytes())
            old_release = first_snapshot["distributions"][0]["releases"][0]
            new_release = second_snapshot["distributions"][0]["releases"][0]
            self.assertEqual(old_release["sequence"], new_release["sequence"])
            self.assertEqual(old_release["package_source"], new_release["package_source"])
            self.assertEqual(old_release["published_at"], new_release["published_at"])
            self.assertNotEqual(first_snapshot["evidence"], second_snapshot["evidence"])
            self.assertEqual(len(second_snapshot["products"][0]["distributions"]), 2)
            self.assertEqual(len(second_snapshot["distributions"]), 2)

            original_first_artifact = (feed / "snapshots" / "00000000000000000001.json").read_bytes()
            weekly = self.signer(root, candidate, "run-102", "2026-09-03T00:00:00Z")
            self.assertEqual(weekly.returncode, 0, weekly.stderr)
            weekly_snapshot = json.loads((feed / "snapshots" / "00000000000000000003.json").read_bytes())
            weekly_release = weekly_snapshot["distributions"][0]["releases"][0]
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
            recycled_evidence = recycled["evidence"][0]
            recycled_evidence["id"] = "runtime-demo-codex"
            recycled_evidence["artifact"]["digest"] = "sha256:" + "3" * 64
            recycled_evidence["outcome"] = "inconclusive"
            recycled["distributions"][0]["release_policies"][0]["current_evidence"] = [recycled_evidence["id"]]
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
        changed_evidence["evidence"][0]["outcome"] = "inconclusive"
        with self.assertRaisesRegex(publication.PublicationError, "immutable evidence"):
            publication.validate_snapshot_semantics(changed_evidence, previous)

        newer = copy.deepcopy(previous)
        newer["sequence"] = 8
        newer["publication_id"] = "fixture-2"
        newer["generated_at"] = "2026-08-27T00:00:00Z"
        newer["expires_at"] = "2026-09-26T00:00:00Z"
        newer["distributions"][1]["release_policies"][0]["status"] = "active"
        newer["revocations"] = []
        with self.assertRaisesRegex(publication.PublicationError, "cannot be restored"):
            publication.validate_snapshot_semantics(newer, previous)
        removed = copy.deepcopy(newer)
        removed["distributions"].pop()
        removed["products"][0]["distributions"].pop()
        removed["products"][0]["default_distribution"] = "example/demo"
        with self.assertRaisesRegex(publication.PublicationError, "was removed"):
            publication.validate_snapshot_semantics(removed, previous)

    def test_post_merge_sha_binding_and_unchanged_release_reuse(self) -> None:
        config = prepare.load_config(ROOT / "registry" / "publication" / "config.json")
        source_commit = subprocess.check_output(["/usr/bin/git", "rev-parse", "HEAD"], cwd=ROOT, text=True).strip()
        with tempfile.TemporaryDirectory() as tmp:
            package = Path(tmp) / "plugins" / "demo"
            package.mkdir(parents=True)
            (package / "plugin.json").write_text('{"name":"demo"}\n')
            tree = prepare.package_tree_digest(package)
            manifest = "sha256:" + __import__("hashlib").sha256((package / "plugin.json").read_bytes()).hexdigest()
            source = {
                "schema_version": 1,
                "products": [{"schema_version": 1, "id": "demo", "display_name": "Demo", "description": "Demo package.", "manifest_name": "demo", "aliases": ["demo"], "reserved_aliases": ["demo"], "categories": ["demo"], "minimum_capabilities": {"skills": "optional", "mcp": "required"}, "default_distribution": "777genius/demo", "distributions": ["777genius/demo"]}],
                "distributions": [{"schema_version": 1, "id": "777genius/demo", "product_id": "demo", "kind": "community", "status": "active", "packager": "777genius", "releases": [{"sequence": 1, "package_version": "1.0.0", "manifest_name": "demo", "agent_plugins_schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json", "package_source": {"repository": config["repository"], "revision": None, "path": "plugins/demo"}, "tree_digest_algorithm": "uap-tree-sha256-v1", "tree_digest": tree, "manifest_digest": manifest, "components": ["mcp"]}], "release_policies": [{"release_sequence": 1, "status": "active", "minimum_installer_version": "0.1.6", "targets": [{"client": "codex", "scopes": ["user"], "delivery": "managed"}], "current_evidence": []}]}],
                "evidence": [],
            }
            first = prepare.build_candidate(source, config, source_commit, "prepare-1", None, repository_root=Path(tmp))
            release = first["distributions"][0]["releases"][0]
            self.assertEqual(release["package_source"]["revision"], source_commit)
            self.assertIsNone(release["published_at"])
            previous = {"products": first["products"], "distributions": copy.deepcopy(first["distributions"]), "evidence": [], "revocations": []}
            previous["distributions"][0]["releases"][0]["published_at"] = "2026-08-20T00:00:00Z"
            second = prepare.build_candidate(source, config, "e" * 40, "prepare-2", previous, repository_root=Path(tmp))
            release = second["distributions"][0]["releases"][0]
            self.assertEqual(release["package_source"]["revision"], source_commit)
            self.assertEqual(release["published_at"], "2026-08-20T00:00:00Z")

        with tempfile.TemporaryDirectory() as tmp:
            rejected = run_script(
                "prepare_directory_publication.py",
                "--directory", str(ROOT / "registry" / "directory.json"),
                "--config", str(ROOT / "registry" / "publication" / "config.json"),
                "--source-commit", "f" * 40,
                "--publication-id", "wrong-head",
                "--output", str(Path(tmp) / "candidate.json"),
                "--digest-output", str(Path(tmp) / "candidate.digest"),
            )
            self.assertNotEqual(rejected.returncode, 0)
            self.assertIn("does not match --source-commit", rejected.stderr)

    def test_external_reacquisition_mismatch_fails_before_output_mutation(self) -> None:
        config = prepare.load_config(ROOT / "registry" / "publication" / "config.json")
        source_commit = subprocess.check_output(["/usr/bin/git", "rev-parse", "HEAD"], cwd=ROOT, text=True).strip()
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            external = root / "external"
            package = external / "plugins" / "demo"
            package.mkdir(parents=True)
            (package / "plugin.json").write_text('{"name":"demo"}\n')
            subprocess.run(["git", "init", "-q", str(external)], check=True)
            subprocess.run(["git", "-C", str(external), "add", "."], check=True)
            subprocess.run(["git", "-C", str(external), "-c", "user.name=Fixture", "-c", "user.email=fixture@example.test", "commit", "-qm", "fixture"], check=True)
            revision = subprocess.check_output(["git", "-C", str(external), "rev-parse", "HEAD"], text=True).strip()
            source = {
                "schema_version": 1,
                "products": [{"schema_version": 1, "id": "demo", "display_name": "Demo", "description": "External demo package.", "manifest_name": "demo", "aliases": ["demo"], "reserved_aliases": ["demo"], "categories": ["demo"], "minimum_capabilities": {"skills": "optional", "mcp": "required"}, "default_distribution": "example/demo", "distributions": ["example/demo"]}],
                "distributions": [{"schema_version": 1, "id": "example/demo", "product_id": "demo", "kind": "upstream", "status": "active", "packager": "example", "releases": [{"sequence": 1, "package_version": "1.0.0", "manifest_name": "demo", "agent_plugins_schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json", "package_source": {"repository": "example/external", "revision": revision, "path": "plugins/demo"}, "tree_digest_algorithm": "uap-tree-sha256-v1", "tree_digest": "sha256:" + "0" * 64, "manifest_digest": "sha256:" + "0" * 64, "components": ["mcp"]}], "release_policies": [{"release_sequence": 1, "status": "active", "minimum_installer_version": "0.1.6", "targets": [{"client": "codex", "scopes": ["user"], "delivery": "managed"}], "current_evidence": []}]}],
                "evidence": [],
            }
            source_path = root / "directory.json"
            source_path.write_text(json.dumps(source))
            output = root / "candidate.json"
            digest_output = root / "candidate.digest"
            output.write_text("unchanged-candidate")
            digest_output.write_text("unchanged-digest")
            result = run_script(
                "prepare_directory_publication.py",
                "--directory", str(source_path),
                "--config", str(ROOT / "registry" / "publication" / "config.json"),
                "--source-commit", source_commit,
                "--publication-id", "external-mismatch",
                "--external-repository", f"example/external={external}",
                "--output", str(output),
                "--digest-output", str(digest_output),
            )
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("reacquired tree digest differs", result.stderr)
            self.assertEqual(output.read_text(), "unchanged-candidate")
            self.assertEqual(digest_output.read_text(), "unchanged-digest")

            release = source["distributions"][0]["releases"][0]
            release["tree_digest"] = prepare.package_tree_digest(package)
            release["manifest_digest"] = "sha256:" + __import__("hashlib").sha256((package / "plugin.json").read_bytes()).hexdigest()
            first = prepare.build_candidate(source, config, source_commit, "external-first", None, external_overrides={"example/external": external})
            previous = {"products": first["products"], "distributions": copy.deepcopy(first["distributions"]), "evidence": [], "revocations": []}
            previous_release = previous["distributions"][0]["releases"][0]
            previous_release["published_at"] = "2026-08-20T00:00:00Z"
            missing = root / "unavailable"
            unchanged = prepare.build_candidate(source, config, "f" * 40, "external-refresh", previous, external_overrides={"example/external": missing})
            unchanged_release = unchanged["distributions"][0]["releases"][0]
            self.assertEqual(unchanged_release["package_source"]["revision"], revision)
            self.assertEqual(unchanged_release["published_at"], "2026-08-20T00:00:00Z")
            broadened = copy.deepcopy(source)
            broadened["distributions"][0]["release_policies"][0]["targets"].append({"client": "cursor", "scopes": ["user"], "delivery": "managed"})
            with self.assertRaisesRegex(publication.PublicationError, "reacquisition failed"):
                prepare.build_candidate(broadened, config, "f" * 40, "external-broadened", previous, external_overrides={"example/external": missing})


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
        site_job = workflow["jobs"]["materialize_site"]
        site_commands = "\n".join(step.get("run", "") for step in site_job["steps"] if isinstance(step, dict))
        self.assertIn("UAP_SIGNED_SNAPSHOT_PATH", site_commands)
        self.assertIn("git -C ledger diff --exit-code -- registry", site_commands)
        deploy_commands = "\n".join(step.get("run", "") for step in workflow["jobs"]["deploy"]["steps"] if isinstance(step, dict))
        self.assertIn("needs.materialize_site.outputs.ledger_commit", text)
        self.assertIn("git -C exact-pages-tree rev-parse HEAD", deploy_commands)
        for match in __import__("re").findall(r"uses:\s+([^\s]+)", text):
            self.assertRegex(match, r"@[0-9a-f]{40}$")

    def test_all_workflow_yaml_parses(self) -> None:
        for path in (ROOT / ".github" / "workflows").glob("*.yml"):
            with self.subTest(path=path.name):
                self.assertIsInstance(yaml.safe_load(path.read_text()), dict)


if __name__ == "__main__":
    unittest.main()

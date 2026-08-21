from __future__ import annotations

import base64
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
    def run_signer(self, root: Path, publication_id: str, *ledger_args: str, mutate=None, key_id: str = "test-current", trusted_keys: Path | None = None) -> subprocess.CompletedProcess[str]:
        candidate = root / "candidate.json"
        value = json.loads((FIXTURES / "candidate.json").read_bytes())
        value["publication_id"] = publication_id
        if mutate is not None:
            mutate(value)
        candidate.write_bytes(publication.canonical_json(value))
        seed = json.loads((FIXTURES / "test-private-seeds.json").read_bytes())[key_id]
        environment = os.environ.copy()
        environment["DIRECTORY_ED25519_PRIVATE_KEY"] = seed
        day = {"50": "20", "100": "22", "200": "23", "run-1": "21", "run-2": "22", "run-3": "23"}.get(publication_id, "24")
        return subprocess.run(
            [
                sys.executable, "-I", str(SCRIPTS / "sign_directory_publication.py"),
                "--candidate", str(candidate),
                "--candidate-digest", publication.candidate_digest(candidate.read_bytes()),
                "--ledger", str(root),
                "--trusted-keys", str(trusted_keys or FIXTURES / "trusted-keys.json"),
                "--key-id", key_id, "--now", f"2026-08-{day}T00:00:00Z",
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
            self.assertIn("sequence-1 tag/transaction identity is invalid", repeated.stderr)
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

    def test_initialize_rerun_reuses_exact_sequence_one_after_atomic_publication(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            subprocess.run(["/usr/bin/git", "init", "-q"], cwd=root, check=True)
            subprocess.run(["/usr/bin/git", "config", "user.name", "test"], cwd=root, check=True)
            subprocess.run(["/usr/bin/git", "config", "user.email", "test@example.com"], cwd=root, check=True)
            subprocess.run(["/usr/bin/git", "commit", "--allow-empty", "-qm", "ledger seed"], cwd=root, check=True)
            seed_commit = subprocess.check_output(["/usr/bin/git", "rev-parse", "HEAD"], cwd=root, text=True).strip()
            first = self.run_signer(root, "run-1", "--initialize-ledger", "--ledger-seed-commit", seed_commit)
            self.assertEqual(first.returncode, 0, first.stderr)
            subprocess.run(["/usr/bin/git", "add", "registry/schemas/1"], cwd=root, check=True)
            subprocess.run(["/usr/bin/git", "commit", "-qm", "publish sequence 1"], cwd=root, check=True)
            sequence_tag = "directory-publication-schema-1-sequence-00000000000000000001"
            subprocess.run(["/usr/bin/git", "tag", sequence_tag, "HEAD"], cwd=root, check=True)
            published_commit = subprocess.check_output(["/usr/bin/git", "rev-parse", f"refs/tags/{sequence_tag}^{{commit}}"], cwd=root, text=True).strip()
            (root / "index.html").write_text("materialized site child\n")
            subprocess.run(["/usr/bin/git", "add", "index.html"], cwd=root, check=True)
            subprocess.run(["/usr/bin/git", "commit", "-qm", "materialize site"], cwd=root, check=True)
            site_commit = subprocess.check_output(["/usr/bin/git", "rev-parse", "HEAD"], cwd=root, text=True).strip()
            self.assertNotEqual(site_commit, published_commit)
            feed = root / "registry" / "schemas" / "1"
            before = {path.relative_to(feed): path.read_bytes() for path in feed.rglob("*") if path.is_file()}

            retry = self.run_signer(root, "run-1", "--initialize-ledger", "--ledger-seed-commit", seed_commit)
            self.assertEqual(retry.returncode, 0, retry.stderr)
            self.assertTrue(json.loads((root / "result.json").read_bytes())["reused"])
            after = {path.relative_to(feed): path.read_bytes() for path in feed.rglob("*") if path.is_file()}
            self.assertEqual(after, before)
            self.assertEqual(subprocess.check_output(["/usr/bin/git", "rev-parse", f"refs/tags/{sequence_tag}^{{commit}}"], cwd=root, text=True).strip(), published_commit)

            wrong_seed = self.run_signer(root, "run-1", "--initialize-ledger", "--ledger-seed-commit", "1" * 40)
            self.assertNotEqual(wrong_seed.returncode, 0)
            self.assertEqual({path.relative_to(feed): path.read_bytes() for path in feed.rglob("*") if path.is_file()}, before)

            wrong_content = self.run_signer(
                root, "run-1", "--initialize-ledger", "--ledger-seed-commit", seed_commit,
                mutate=lambda value: value["products"][0].update({"description": "different reviewed content"}),
            )
            self.assertNotEqual(wrong_content.returncode, 0)
            wrong_key = self.run_signer(
                root, "run-1", "--initialize-ledger", "--ledger-seed-commit", seed_commit,
                key_id="test-next",
            )
            self.assertNotEqual(wrong_key.returncode, 0)
            self.assertIn("another signer key", wrong_key.stderr)
            self.assertEqual({path.relative_to(feed): path.read_bytes() for path in feed.rglob("*") if path.is_file()}, before)

            for label, publication_id, mutation in (
                ("source", "run-1", lambda value: value.update({"source_commit": "1" * 40})),
                ("publication ID", "run-2", None),
                ("lifetime", "run-1", lambda value: value.update({"lifetime_days": 29})),
            ):
                with self.subTest(changed=label):
                    changed = self.run_signer(
                        root, publication_id, "--initialize-ledger", "--ledger-seed-commit", seed_commit,
                        mutate=mutation,
                    )
                    self.assertNotEqual(changed.returncode, 0)
                    self.assertEqual({path.relative_to(feed): path.read_bytes() for path in feed.rglob("*") if path.is_file()}, before)

            subprocess.run(["/usr/bin/git", "tag", "-f", sequence_tag, "HEAD"], cwd=root, check=True, stdout=subprocess.DEVNULL)
            moved_tag = self.run_signer(root, "run-1", "--initialize-ledger", "--ledger-seed-commit", seed_commit)
            self.assertNotEqual(moved_tag.returncode, 0)
            subprocess.run(["/usr/bin/git", "tag", "-f", sequence_tag, published_commit], cwd=root, check=True, stdout=subprocess.DEVNULL)

            subprocess.run(["/usr/bin/git", "checkout", "-q", "--detach", seed_commit], cwd=root, check=True)
            subprocess.run(["/usr/bin/git", "checkout", sequence_tag, "--", "registry/schemas/1"], cwd=root, check=True)
            subprocess.run(["/usr/bin/git", "commit", "-qm", "divergent sequence copy"], cwd=root, check=True)
            divergent = self.run_signer(root, "run-1", "--initialize-ledger", "--ledger-seed-commit", seed_commit)
            self.assertNotEqual(divergent.returncode, 0)
            self.assertIn("sequence-1 tag/transaction identity is invalid", divergent.stderr)

    def test_historical_publication_id_and_old_run_after_new_run_fail_without_mutation(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            first = self.run_signer(root, "run-1", "--initialize-ledger", "--ledger-seed-commit", "0" * 40)
            self.assertEqual(first.returncode, 0, first.stderr)
            second = self.run_signer(root, "run-2", "--ledger-seed-commit", "0" * 40, "--ledger-sequence-floor", "1")
            self.assertEqual(second.returncode, 0, second.stderr)
            feed = root / "registry" / "schemas" / "1"
            before = {path.relative_to(feed): path.read_bytes() for path in feed.rglob("*") if path.is_file()}

            old = self.run_signer(root, "run-1", "--ledger-seed-commit", "0" * 40, "--ledger-sequence-floor", "2")
            self.assertNotEqual(old.returncode, 0)
            self.assertIn("historical publication", old.stderr)
            self.assertEqual({path.relative_to(feed): path.read_bytes() for path in feed.rglob("*") if path.is_file()}, before)

        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            initial = self.run_signer(root, "50", "--initialize-ledger", "--ledger-seed-commit", "0" * 40)
            self.assertEqual(initial.returncode, 0, initial.stderr)
            newer = self.run_signer(root, "200", "--ledger-seed-commit", "0" * 40, "--ledger-sequence-floor", "1")
            self.assertEqual(newer.returncode, 0, newer.stderr)
            feed = root / "registry" / "schemas" / "1"
            before = {path.relative_to(feed): path.read_bytes() for path in feed.rglob("*") if path.is_file()}
            reordered = self.run_signer(root, "100", "--ledger-seed-commit", "0" * 40, "--ledger-sequence-floor", "2")
            self.assertNotEqual(reordered.returncode, 0)
            self.assertIn("older than current latest", reordered.stderr)
            self.assertEqual({path.relative_to(feed): path.read_bytes() for path in feed.rglob("*") if path.is_file()}, before)

    def test_actual_signer_rejects_nested_schema_violations_without_ledger_mutation(self) -> None:
        mutations = {
            "true candidate version": lambda value: value.update({"candidate_schema_version": True}),
            "false snapshot version": lambda value: value.update({"snapshot_schema_version": False}),
            "true lifetime": lambda value: value.update({"lifetime_days": True}),
            "false lifetime": lambda value: value.update({"lifetime_days": False}),
            "forbidden product field": lambda value: value["products"][0].update({"forbidden": True}),
            "true product version": lambda value: value["products"][0].update({"schema_version": True}),
            "missing product field": lambda value: value["products"][0].pop("description"),
            "malformed distribution": lambda value: value["distributions"][0].update({"status": "candidate"}),
            "false distribution version": lambda value: value["distributions"][0].update({"schema_version": False}),
            "malformed release": lambda value: value["distributions"][0]["releases"][0].update({"sequence": True}),
            "malformed release digest": lambda value: value["distributions"][0]["releases"][0].update({"tree_digest": "sha256:nope"}),
            "malformed policy": lambda value: value["distributions"][0]["release_policies"][0].update({"release_sequence": 0}),
            "false policy sequence": lambda value: value["distributions"][0]["release_policies"][0].update({"release_sequence": False}),
            "malformed policy target": lambda value: value["distributions"][0]["release_policies"][0]["targets"][0].update({"app_binding": {"app_key": "x", "id": "x", "mcp_server": "x"}}),
            "malformed evidence artifact": lambda value: value["evidence"][0]["artifact"].update({"unknown": "x"}),
            "unsafe evidence artifact path": lambda value: value["evidence"][0]["artifact"].update({"path": "../invented.json"}),
            "true evidence version": lambda value: value["evidence"][0].update({"schema_version": True}),
            "false evidence sequence": lambda value: value["evidence"][0].update({"release_sequence": False}),
            "missing evidence field": lambda value: value["evidence"][0].pop("outcome"),
            "malformed revocation": lambda value: value["revocations"].append({"distribution_id": "INVALID", "release_sequence": 1}),
            "true revocation sequence": lambda value: value["revocations"].append({"distribution_id": value["distributions"][0]["id"], "release_sequence": True}),
        }
        for label, mutation in mutations.items():
            with self.subTest(label=label), tempfile.TemporaryDirectory() as temporary:
                root = Path(temporary)
                rejected = self.run_signer(root, "run-1", "--initialize-ledger", "--ledger-seed-commit", "0" * 40, mutate=mutation)
                self.assertNotEqual(rejected.returncode, 0)
                self.assertFalse((root / "registry").exists())

    def test_actual_signer_rejects_boolean_integer_contracts_in_trust_and_history(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            base = Path(temporary) / "base"
            base.mkdir()
            initialized = self.run_signer(base, "run-1", "--initialize-ledger", "--ledger-seed-commit", "0" * 40)
            self.assertEqual(initialized.returncode, 0, initialized.stderr)

            trusted = Path(temporary) / "trusted.json"
            trusted_value = json.loads((FIXTURES / "trusted-keys.json").read_bytes())
            trusted_value["schema_version"] = True
            trusted.write_bytes(publication.canonical_json(trusted_value))
            trust_root = Path(temporary) / "trust-root"
            trust_root.mkdir()
            rejected_trust = self.run_signer(
                trust_root, "run-1", "--initialize-ledger", "--ledger-seed-commit", "0" * 40,
                trusted_keys=trusted,
            )
            self.assertNotEqual(rejected_trust.returncode, 0)
            self.assertFalse((trust_root / "registry").exists())

            mutations = {
                "true contract version": ("contract", lambda value: value.update({"contract_version": True})),
                "false contract initial sequence": ("contract", lambda value: value.update({"initial_sequence": False})),
                "true pointer version": ("latest", lambda value: value.update({"pointer_schema_version": True})),
                "false pointer snapshot version": ("latest", lambda value: value.update({"snapshot_schema_version": False})),
                "true pointer sequence": ("latest", lambda value: value.update({"sequence": True})),
                "false pointer size": ("latest", lambda value: value["fetch_contract"].update({"latest_max_bytes": False})),
                "true envelope version": ("envelope", lambda value: value.update({"envelope_schema_version": True})),
                "false envelope snapshot version": ("envelope", lambda value: value.update({"snapshot_schema_version": False})),
                "true envelope sequence": ("envelope", lambda value: value.update({"sequence": True})),
                "true snapshot version": ("snapshot", lambda value: value.update({"snapshot_schema_version": True})),
                "false nested product version": ("snapshot", lambda value: value["products"][0].update({"schema_version": False})),
                "true nested release sequence": ("snapshot", lambda value: value["distributions"][0]["releases"][0].update({"sequence": True})),
                "false nested policy sequence": ("snapshot", lambda value: value["distributions"][0]["release_policies"][0].update({"release_sequence": False})),
                "true nested evidence version": ("snapshot", lambda value: value["evidence"][0].update({"schema_version": True})),
            }
            for index, (label, (artifact, mutation)) in enumerate(mutations.items()):
                with self.subTest(label=label):
                    root = Path(temporary) / f"case-{index}"
                    shutil.copytree(base, root)
                    feed = root / "registry" / "schemas" / "1"
                    paths = {
                        "contract": feed / publication.LEDGER_CONTRACT_NAME,
                        "latest": feed / "latest.json",
                        "snapshot": feed / "snapshots" / "00000000000000000001.json",
                        "envelope": feed / "snapshots" / "00000000000000000001.envelope.json",
                    }
                    value = json.loads(paths[artifact].read_bytes())
                    mutation(value)
                    paths[artifact].write_bytes(publication.canonical_json(value))
                    if artifact == "snapshot":
                        snapshot_body = paths["snapshot"].read_bytes()
                        envelope = json.loads(paths["envelope"].read_bytes())
                        envelope["snapshot_digest"] = publication.sha256_digest(snapshot_body)
                        seed = json.loads((FIXTURES / "test-private-seeds.json").read_bytes())["test-current"]
                        envelope["signature"] = base64.b64encode(publication.ed25519_sign(publication.ed25519_private_key(seed), publication.signature_message(snapshot_body))).decode("ascii")
                        paths["envelope"].write_bytes(publication.canonical_json(envelope))
                    before = {path.relative_to(feed): path.read_bytes() for path in feed.rglob("*") if path.is_file()}
                    rejected = self.run_signer(root, "run-2", "--ledger-seed-commit", "0" * 40, "--ledger-sequence-floor", "1")
                    self.assertNotEqual(rejected.returncode, 0)
                    self.assertEqual({path.relative_to(feed): path.read_bytes() for path in feed.rglob("*") if path.is_file()}, before)

    def test_actual_signer_rejects_malformed_existing_snapshot_and_envelope_without_mutation(self) -> None:
        for artifact in ("snapshot", "envelope"):
            with self.subTest(artifact=artifact), tempfile.TemporaryDirectory() as temporary:
                root = Path(temporary)
                initialized = self.run_signer(root, "run-1", "--initialize-ledger", "--ledger-seed-commit", "0" * 40)
                self.assertEqual(initialized.returncode, 0, initialized.stderr)
                snapshots = root / "registry" / "schemas" / "1" / "snapshots"
                snapshot_path = snapshots / "00000000000000000001.json"
                envelope_path = snapshots / "00000000000000000001.envelope.json"
                if artifact == "envelope":
                    envelope = json.loads(envelope_path.read_bytes())
                    envelope["forbidden"] = True
                    envelope_path.write_bytes(publication.canonical_json(envelope))
                else:
                    snapshot = json.loads(snapshot_path.read_bytes())
                    snapshot["products"][0]["forbidden"] = True
                    snapshot_body = publication.canonical_json(snapshot)
                    snapshot_path.write_bytes(snapshot_body)
                    envelope = json.loads(envelope_path.read_bytes())
                    envelope["snapshot_digest"] = publication.sha256_digest(snapshot_body)
                    seed = json.loads((FIXTURES / "test-private-seeds.json").read_bytes())["test-current"]
                    signature = publication.ed25519_sign(publication.ed25519_private_key(seed), publication.signature_message(snapshot_body))
                    envelope["signature"] = base64.b64encode(signature).decode("ascii")
                    envelope_path.write_bytes(publication.canonical_json(envelope))
                feed = root / "registry" / "schemas" / "1"
                before = {path.relative_to(feed): path.read_bytes() for path in feed.rglob("*") if path.is_file()}
                rejected = self.run_signer(root, "run-2", "--ledger-seed-commit", "0" * 40, "--ledger-sequence-floor", "1")
                self.assertNotEqual(rejected.returncode, 0)
                self.assertEqual({path.relative_to(feed): path.read_bytes() for path in feed.rglob("*") if path.is_file()}, before)


class WorkflowHardeningTests(unittest.TestCase):
    def test_reordered_workflow_rechecks_protected_main_before_candidate_sign_and_push(self) -> None:
        workflow = yaml.load((ROOT / ".github" / "workflows" / "directory-publication.yml").read_text(), Loader=yaml.BaseLoader)
        prepare_steps = workflow["jobs"]["prepare"]["steps"]
        prepare_guard = next(step["run"] for step in prepare_steps if step.get("name") == "Require event source to remain protected main")
        freshness = next(step for step in workflow["jobs"]["sign"]["steps"] if step.get("id") == "freshness")
        signer = next(step["run"] for step in workflow["jobs"]["sign"]["steps"] if step.get("id") == "signed")
        publisher = next(step["run"] for step in workflow["jobs"]["sign"]["steps"] if step.get("id") == "commit")
        for commands in (prepare_guard, freshness["run"], publisher):
            self.assertIn("refs/heads/main:refs/remotes/origin/main", commands)
            self.assertIn("rev-parse refs/remotes/origin/main", commands)
        self.assertLess(prepare_guard.index("git fetch"), prepare_guard.index("rev-parse"))
        self.assertNotIn("DIRECTORY_ED25519_PRIVATE_KEY", freshness.get("env", {}))
        self.assertNotIn("fetch", signer)
        self.assertNotIn("refs/remotes/origin", signer)
        self.assertIn("OBSERVED_SOURCE_COMMIT", signer)
        self.assertNotIn("DIRECTORY_ED25519_PRIVATE_KEY", json.dumps(freshness))
        self.assertLess(publisher.rindex("rev-parse refs/remotes/origin/main"), publisher.index("directory_publication_cas.py publish"))
        self.assertIn("--source \"${EVENT_SOURCE_COMMIT}\"", publisher)
        self.assertIn("--marker \"${MARKER_COMMIT}\"", publisher)

    def test_only_app_tokens_write_ledger_and_floor_tags_are_atomic(self) -> None:
        text = (ROOT / ".github" / "workflows" / "directory-publication.yml").read_text()
        workflow = yaml.load(text, Loader=yaml.BaseLoader)
        self.assertNotIn("github.token", text)
        self.assertNotIn("GH_TOKEN", text)
        self.assertEqual(workflow["jobs"]["sign"]["permissions"]["contents"], "read")
        self.assertEqual(workflow["jobs"]["materialize_site"]["permissions"]["contents"], "read")
        self.assertEqual(workflow["jobs"]["sign"]["environment"], "directory-publication")
        self.assertEqual(workflow["jobs"]["materialize_site"]["environment"], "directory-publication-materialization")
        self.assertEqual(text.count("actions/create-github-app-token@bcd2ba49218906704ab6c1aa796996da409d3eb1"), 3)
        cas_helper = (SCRIPTS / "directory_publication_cas.py").read_text()
        self.assertIn('"push", "--atomic"', cas_helper)
        self.assertIn('f"--force-with-lease={main_ref}:{source}"', cas_helper)
        self.assertIn('f"--force-with-lease={ledger_ref}:{ledger_old}"', cas_helper)
        self.assertIn('f"--force-with-lease={sequence_tag}:"', cas_helper)
        self.assertIn('f"{ledger_new}:{sequence_tag}"', cas_helper)
        self.assertIn('--force-with-lease="${ledger_ref}:${EXPECTED_LEDGER_COMMIT}"', text)
        self.assertIn('f"--force-with-lease={ledger_ref}:{ledger_old}"', cas_helper)
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
        self.assertGreaterEqual(text.count("refs/heads/main:refs/remotes/origin/main"), 3)
        self.assertGreaterEqual(text.count('rev-parse refs/remotes/origin/main'), 3)
        self.assertIn('main_ref = "refs/heads/main"', cas_helper)
        self.assertIn('= "${EVENT_SOURCE_COMMIT}"', text)
        self.assertIn('sequence_one_tag="${tag_prefix}00000000000000000001"', text)
        self.assertGreaterEqual(text.count('merge-base --is-ancestor "${sequence_one_commit}" HEAD'), 2)
        self.assertGreaterEqual(text.count('diff --exit-code "${sequence_one_commit}" HEAD -- registry/schemas/1'), 2)

    def test_privileged_signer_has_no_downloaded_dependency_install(self) -> None:
        workflow = yaml.load((ROOT / ".github" / "workflows" / "directory-publication.yml").read_text(), Loader=yaml.BaseLoader)
        signer = workflow["jobs"]["sign"]
        commands = "\n".join(step.get("run", "") for step in signer["steps"] if isinstance(step, dict))
        self.assertNotIn("pip install", commands)
        self.assertNotIn("setup-python", json.dumps(signer))
        self.assertIn("/usr/bin/openssl", commands)
        self.assertIn("/usr/bin/git", commands)
        self.assertIn("dpkg --verify", commands)
        self.assertNotIn("jsonschema", commands)
        signer_source = (SCRIPTS / "sign_directory_publication.py").read_text()
        self.assertNotIn("validate_with_schema", signer_source)
        self.assertNotIn("jsonschema", signer_source)

    def test_evidence_attestation_uses_verified_system_tools_without_signing_seed(self) -> None:
        workflow = yaml.load((ROOT / ".github" / "workflows" / "directory-publication.yml").read_text(), Loader=yaml.BaseLoader)
        candidate = next(step for step in workflow["jobs"]["prepare"]["steps"] if step.get("id") == "candidate")
        self.assertIn("dpkg-query --show git gh", candidate["run"])
        self.assertIn("dpkg --verify git gh", candidate["run"])
        self.assertNotIn("DIRECTORY_ED25519_PRIVATE_KEY", json.dumps(workflow["jobs"]["prepare"]))
        preparer = (SCRIPTS / "prepare_directory_publication.py").read_text()
        self.assertIn('GH = "/usr/bin/gh"', preparer)
        self.assertIn('GH, "attestation", "verify"', preparer)
        self.assertIn('"--source-ref"', preparer)
        self.assertIn('"--source-digest"', preparer)
        self.assertIn('"--deny-self-hosted-runners"', preparer)

    def test_signing_seed_is_scoped_only_to_the_signer_step(self) -> None:
        workflow = yaml.load((ROOT / ".github" / "workflows" / "directory-publication.yml").read_text(), Loader=yaml.BaseLoader)
        sign_steps = workflow["jobs"]["sign"]["steps"]
        seed_steps = [step for step in sign_steps if "DIRECTORY_ED25519_PRIVATE_KEY" in json.dumps(step)]
        self.assertEqual(len(seed_steps), 1)
        self.assertEqual(seed_steps[0].get("id"), "signed")
        marker = next(step for step in sign_steps if step.get("id") == "marker")
        publisher = next(step for step in sign_steps if step.get("id") == "publisher")
        self.assertNotIn("DIRECTORY_ED25519_PRIVATE_KEY", json.dumps(marker))
        self.assertNotIn("DIRECTORY_ED25519_PRIVATE_KEY", json.dumps(publisher))
        for job_name in ("prepare", "build_site", "materialize_site", "deploy"):
            self.assertNotIn("DIRECTORY_ED25519_PRIVATE_KEY", json.dumps(workflow["jobs"][job_name]))

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
        self.assertIn("`bcd2ba49218906704ab6c1aa796996da409d3eb1` (`v3.2.0`)", documentation)
        self.assertNotIn("a8d616148505b5069dccd32f177bb87d7f39123b", documentation)
        self.assertIn("eight active repository rulesets", documentation)
        self.assertEqual(documentation.count("**no bypass actors**"), 4)
        self.assertIn("even the publisher cannot reset the branch", documentation)
        self.assertIn("administrators", documentation)
        self.assertIn("deploy keys", documentation)
        self.assertIn("generic Actions", documentation)


if __name__ == "__main__":
    unittest.main()

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
from unittest import mock

import yaml


ROOT = Path(__file__).resolve().parents[1]
SCRIPTS = ROOT / "scripts"
FIXTURES = ROOT / "tests" / "fixtures" / "directory-publication"
sys.path.insert(0, str(SCRIPTS))
import directory_publication as publication
import directory_publication_cas as publication_cas
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


def write_valid_package(
    package: Path, *, name: str = "demo", version: str = "1.0.0",
    repository: str = "example/external",
) -> None:
    package.mkdir(parents=True, exist_ok=True)
    (package / "plugin.json").write_text(json.dumps({
        "$schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json",
        "name": name,
        "version": version,
        "description": "Deterministic publication validation fixture.",
        "author": {"name": "Fixture"},
        "license": "MIT",
        "keywords": ["fixture"],
        "repository": f"https://github.com/{repository}",
    }, sort_keys=True) + "\n")
    (package / "README.md").write_text("# Fixture\n")
    (package / "mcp.json").write_text(json.dumps({
        "$schema": "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json",
        "mcpServers": {
            "demo": {"type": "streamable-http", "url": "https://example.test/mcp"},
        },
    }, sort_keys=True) + "\n")


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

        uppercase_security_identity = fixture_json("candidate.json")
        uppercase_security_identity["products"][0]["id"] = "Demo"
        with self.assertRaises(publication.PublicationError):
            publication.validate_with_schema(uppercase_security_identity, publication.CANDIDATE_SCHEMA)

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
    def test_initial_inactive_bridge_is_reproduced_before_its_first_signed_binding(self) -> None:
        import build_bridges

        source = json.loads((ROOT / "registry" / "directory.json").read_bytes())
        reproduced: list[str] = []

        def assemble(repository_root, bridge_id, destination, _cache):  # type: ignore[no-untyped-def]
            reproduced.append(bridge_id)
            shutil.copytree(
                repository_root / "plugins" / bridge_id,
                destination,
                dirs_exist_ok=True,
            )
            return {"package_path": f"plugins/{bridge_id}"}

        with mock.patch.object(build_bridges, "assemble", side_effect=assemble):
            prepare.validate_reproduced_bridges(
                source, ROOT, "777genius/universal-agent-plugins", None,
            )

        self.assertIn("chrome-devtools", reproduced)

    def test_publication_rebinds_bridge_recipe_before_accepting_prebuilt_bytes(self) -> None:
        source = json.loads((ROOT / "registry" / "directory.json").read_bytes())
        bridge = next(item for item in source["distributions"] if item["kind"] == "community_bridge")
        # The package and recipe remain fixed; only the contributor-authored
        # provenance claim is changed.
        bridge["releases"][0]["build_provenance"]["upstream_revision"] = "0" * 40
        config = prepare.load_config(ROOT / "registry" / "publication" / "config.json")
        with self.assertRaisesRegex(publication.PublicationError, "build provenance.*canonical recipe upstream"):
            prepare.build_candidate(source, config, "a" * 40, "bridge-provenance", None)

        import build_bridges

        source = json.loads((ROOT / "registry" / "directory.json").read_bytes())
        previous = copy.deepcopy(source)
        previous["distributions"] = [
            distribution for distribution in previous["distributions"]
            if distribution["status"] != "active"
        ]
        inactive = next(
            distribution for distribution in source["distributions"]
            if distribution["kind"] == "community_bridge" and distribution["status"] != "active"
        )
        inactive["releases"][-1]["package_source"]["revision"] = "a" * 40
        previous_inactive = next(item for item in previous["distributions"] if item["id"] == inactive["id"])
        previous_inactive["releases"][-1]["package_source"]["revision"] = "a" * 40
        with mock.patch.object(build_bridges, "assemble", side_effect=build_bridges.BridgeError("upstream unavailable")) as reproduce, self.assertRaisesRegex(
            publication.PublicationError, "bridge reproduction failed: upstream unavailable",
        ):
            prepare.validate_reproduced_bridges(source, ROOT, config["repository"], previous)
        self.assertTrue(reproduce.called)

    def test_emergency_bridge_revocation_reuses_signed_binding_but_broadening_reproduces(self) -> None:
        import build_bridges

        source = json.loads((ROOT / "registry" / "directory.json").read_bytes())
        for distribution in source["distributions"]:
            if distribution["kind"] == "community_bridge":
                for release in distribution["releases"]:
                    if release["package_source"]["revision"] is None:
                        release["package_source"]["revision"] = "a" * 40
                    release.setdefault("published_at", "2026-08-20T00:00:00Z")
        previous = copy.deepcopy(source)
        previous["revocations"] = []
        for distribution in previous["distributions"]:
            policies = {
                policy["release_sequence"]: policy
                for policy in distribution["release_policies"]
            }
            for release in distribution["releases"]:
                release.setdefault("published_at", "2026-08-20T00:00:00Z")
                if policies[release["sequence"]]["status"] == "revoked":
                    previous["revocations"].append({
                        "distribution_id": distribution["id"],
                        "release_sequence": release["sequence"],
                    })
        for distribution in source["distributions"]:
            if distribution["kind"] == "community_bridge":
                distribution["status"] = "suspended"
                for policy in distribution["release_policies"]:
                    policy["status"] = "revoked"

        with mock.patch.object(build_bridges, "assemble", side_effect=AssertionError("revoked upstream was fetched")) as assemble:
            prepare.validate_reproduced_bridges(
                source, ROOT, "777genius/universal-agent-plugins", previous,
            )
        assemble.assert_not_called()

        config = prepare.load_config(ROOT / "registry" / "publication" / "config.json")
        with mock.patch.object(build_bridges, "assemble", side_effect=AssertionError("revoked upstream was fetched")) as assemble:
            candidate = prepare.build_candidate(
                source, config, "c" * 40, "emergency-bridge-revocation", previous,
            )
        assemble.assert_not_called()
        self.assertTrue(any(
            distribution["kind"] == "community_bridge"
            and distribution["status"] == "suspended"
            and all(policy["status"] == "revoked" for policy in distribution["release_policies"])
            for distribution in candidate["distributions"]
        ))

        broadened = copy.deepcopy(previous)
        bridge = next(
            item for item in broadened["distributions"]
            if item["kind"] == "community_bridge" and item["status"] == "active"
        )
        bridge["release_policies"][-1]["targets"].append({
            "client": "chatgpt", "scopes": ["user"], "delivery": "manual_activation",
            "authentication": "required",
            "app_binding": {"app_key": "fixture", "id": "fixture", "mcp_server": "fixture"},
        })
        with mock.patch.object(build_bridges, "assemble", side_effect=build_bridges.BridgeError("upstream unavailable")) as assemble, self.assertRaisesRegex(
            publication.PublicationError, "bridge reproduction failed: upstream unavailable",
        ):
            prepare.validate_reproduced_bridges(
                broadened, ROOT, "777genius/universal-agent-plugins", previous,
            )
        assemble.assert_called_once()

    def test_broadened_active_historical_bridge_fails_closed_when_newer_release_is_revoked(self) -> None:
        import build_bridges

        source = json.loads((ROOT / "registry" / "directory.json").read_bytes())
        bridge = next(
            item for item in source["distributions"]
            if item["kind"] == "community_bridge" and item["status"] == "active"
        )
        release_one = bridge["releases"][0]
        release_one["package_source"]["revision"] = "a" * 40
        release_one["published_at"] = "2026-08-20T00:00:00Z"
        release_two = copy.deepcopy(release_one)
        release_two["sequence"] = 2
        bridge["releases"].append(release_two)
        policy_two = copy.deepcopy(bridge["release_policies"][0])
        policy_two["release_sequence"] = 2
        bridge["release_policies"].append(policy_two)
        previous = copy.deepcopy(source)

        bridge["release_policies"][1]["status"] = "revoked"
        revocation_only = copy.deepcopy(source)
        with mock.patch.object(
            build_bridges, "assemble", side_effect=AssertionError("revoked upstream was fetched"),
        ) as assemble:
            prepare.validate_reproduced_bridges(
                revocation_only, ROOT, "777genius/universal-agent-plugins", previous,
            )
        assemble.assert_not_called()

        bridge["release_policies"][0]["targets"].append({
            "client": "chatgpt", "scopes": ["user"], "delivery": "manual_activation",
            "authentication": "required",
            "app_binding": {"app_key": "fixture", "id": "fixture", "mcp_server": "fixture"},
        })

        with mock.patch.object(
            build_bridges, "assemble", side_effect=AssertionError("historical release used current recipe"),
        ) as assemble, self.assertRaisesRegex(
            publication.PublicationError,
            r"@1: active historical bridge requires reproduction.*canonical recipe represents release 2.*versioned historical reproduction inputs are unavailable",
        ):
            prepare.validate_reproduced_bridges(
                source, ROOT, "777genius/universal-agent-plugins", previous,
            )
        assemble.assert_not_called()

    def test_capability_relaxation_reproduces_current_bridge_and_rejects_historical_bridge(self) -> None:
        import build_bridges

        source = json.loads((ROOT / "registry" / "directory.json").read_bytes())
        bridge = next(
            item for item in source["distributions"]
            if item["kind"] == "community_bridge" and item["status"] == "active"
        )
        for release in bridge["releases"]:
            release["package_source"]["revision"] = release["package_source"]["revision"] or "a" * 40
            release.setdefault("published_at", "2026-08-20T00:00:00Z")
        previous = copy.deepcopy(source)
        old_product = next(item for item in previous["products"] if item["id"] == bridge["product_id"])
        old_product["minimum_capabilities"]["skills"] = "required"

        with mock.patch.object(
            build_bridges, "assemble", side_effect=build_bridges.BridgeError("upstream unavailable"),
        ) as assemble, self.assertRaisesRegex(publication.PublicationError, "bridge reproduction failed: upstream unavailable"):
            prepare.validate_reproduced_bridges(
                source, ROOT, "777genius/universal-agent-plugins", previous,
            )
        assemble.assert_called_once()

        historical = copy.deepcopy(source)
        historical_bridge = next(item for item in historical["distributions"] if item["id"] == bridge["id"])
        newer = copy.deepcopy(historical_bridge["releases"][-1])
        newer["sequence"] += 1
        historical_bridge["releases"].append(newer)
        newer_policy = copy.deepcopy(historical_bridge["release_policies"][-1])
        newer_policy["release_sequence"] = newer["sequence"]
        historical_bridge["release_policies"].append(newer_policy)
        with self.assertRaisesRegex(
            publication.PublicationError,
            r"active historical bridge requires reproduction.*versioned historical reproduction inputs are unavailable",
        ):
            prepare.validate_reproduced_bridges(
                historical, ROOT, "777genius/universal-agent-plugins", previous,
            )

    def test_publication_upstream_positive_evidence_binds_complete_release_tuple(self) -> None:
        product = {
            "id": "demo", "default_distribution": "upstream/demo",
            "minimum_capabilities": {"mcp": "required", "skills": "optional"},
        }
        release = {"sequence": 7, "components": ["mcp"], "tree_digest": "sha256:" + "1" * 64}
        policy = {
            "release_sequence": 7, "status": "active",
            "targets": [{"client": "codex"}], "current_evidence": ["materialized"],
        }
        distribution = {
            "id": "upstream/demo", "kind": "upstream", "status": "active",
            "releases": [release], "release_policies": [policy],
        }
        observation = {
            "id": "materialized", "distribution_id": "upstream/demo", "release_sequence": 7,
            "package_tree_digest": release["tree_digest"], "client": "codex",
            "level": "materialization", "outcome": "passed",
        }
        prepare.validate_upstream_default_evidence([product], [distribution], [observation])
        for field, value in (
            ("distribution_id", "other/demo"),
            ("release_sequence", 8),
            ("package_tree_digest", "sha256:" + "2" * 64),
            ("client", "cursor"),
            ("level", "runtime"),
            ("outcome", "failed"),
        ):
            changed = copy.deepcopy(observation)
            changed[field] = value
            with self.subTest(field=field), self.assertRaisesRegex(publication.PublicationError, "lacks exact passed materialization evidence for codex"):
                prepare.validate_upstream_default_evidence([product], [distribution], [changed])

    def signer(self, root: Path, candidate: Path, publication_id: str, now: str) -> subprocess.CompletedProcess[str]:
        value = json.loads(candidate.read_bytes())
        value["publication_id"] = publication_id
        candidate.write_bytes(publication.canonical_json(value))
        digest = publication.candidate_digest(candidate.read_bytes())
        seed = fixture_json("test-private-seeds.json")["test-current"]
        latest = root / "registry" / "schemas" / "1" / "latest.json"
        ledger_arguments = ["--ledger-seed-commit", "0" * 40]
        if latest.exists():
            ledger_arguments.extend(("--ledger-sequence-floor", str(json.loads(latest.read_bytes())["sequence"])))
        else:
            ledger_arguments.append("--initialize-ledger")
        return run_script(
            "sign_directory_publication.py",
            "--candidate", str(candidate),
            "--candidate-digest", digest,
            "--ledger", str(root),
            "--trusted-keys", str(FIXTURES / "trusted-keys.json"),
            "--key-id", "test-current",
            "--now", now,
            "--result", str(root / "result.json"),
            *ledger_arguments,
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

    def test_higher_sequence_preserves_product_distribution_and_alias_identity(self) -> None:
        previous = fixture_json("snapshot.json")

        def changed() -> dict:  # type: ignore[type-arg]
            value = copy.deepcopy(previous)
            value["sequence"] += 1
            value["publication_id"] = "identity-change"
            value["generated_at"] = "2026-08-27T00:00:00Z"
            value["expires_at"] = "2026-09-26T00:00:00Z"
            return value

        cases = []
        relabel = changed()
        relabel["distributions"][0]["kind"] = "community"
        cases.append(("upstream to community relabel", relabel))
        packager = changed()
        packager["distributions"][0]["packager"] = "another"
        cases.append(("packager rewrite", packager))
        product = changed()
        product["distributions"][0]["product_id"] = "other"
        cases.append(("distribution product rewrite", product))
        manifest = changed()
        manifest["products"][0]["manifest_name"] = "renamed"
        cases.append(("manifest rename", manifest))
        reserved = changed()
        reserved["products"][0]["aliases"] = ["replacement"]
        reserved["products"][0]["reserved_aliases"] = ["replacement"]
        cases.append(("reserved alias removal", reserved))
        removed = changed()
        removed["products"][0]["distributions"].remove("example/demo-bridge")
        removed["distributions"].pop(1)
        cases.append(("distribution removal", removed))

        for label, value in cases:
            with self.subTest(label), self.assertRaises(publication.PublicationError):
                publication.validate_snapshot_semantics(value, previous)

        reused = changed()
        reused["products"][0]["aliases"] = ["replacement"]
        reused["products"][0]["reserved_aliases"] = ["replacement"]
        reused["products"].append({
            "schema_version": 1, "id": "other", "display_name": "Other", "description": "Other product.",
            "manifest_name": "other", "aliases": ["demo"], "reserved_aliases": ["demo"], "categories": ["other"],
            "minimum_capabilities": {"skills": "optional", "mcp": "required"},
            "default_distribution": "example/demo", "distributions": ["example/demo"],
        })
        with self.assertRaises(publication.PublicationError):
            publication.validate_snapshot_semantics(reused, previous)

    def test_case_sensitive_repository_spelling_is_preserved_by_source_and_candidate_schemas(self) -> None:
        candidate = fixture_json("candidate.json")
        candidate["distributions"][0]["releases"][0]["package_source"]["repository"] = "ChromeDevTools/chrome-devtools-mcp"
        candidate["evidence"][0]["artifact"]["repository"] = "ChromeDevTools/chrome-devtools-mcp"
        publication.validate_with_schema(candidate, publication.CANDIDATE_SCHEMA)
        encoded = publication.canonical_json(candidate)
        self.assertIn(b"ChromeDevTools/chrome-devtools-mcp", encoded)
        self.assertEqual(json.loads(encoded)["distributions"][0]["releases"][0]["package_source"]["repository"], "ChromeDevTools/chrome-devtools-mcp")

        source = fixture_json("initial-source.json")
        source["distributions"][0]["releases"][0]["package_source"]["repository"] = "ChromeDevTools/chrome-devtools-mcp"
        publication.validate_with_schema(source, prepare.SOURCE_SCHEMA)

    def test_initial_publication_preserves_historical_binding_and_binds_only_unresolved_release(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            repository = Path(tmp) / "repository"
            package = repository / "plugins" / "demo"
            write_valid_package(package, version="1.0.0", repository="example/uap")
            subprocess.run(["/usr/bin/git", "init", "-q", str(repository)], check=True)
            subprocess.run(["/usr/bin/git", "-C", str(repository), "config", "uploadpack.allowFilter", "true"], check=True)
            subprocess.run(["/usr/bin/git", "-C", str(repository), "add", "."], check=True)
            subprocess.run(["/usr/bin/git", "-C", str(repository), "-c", "user.name=Fixture", "-c", "user.email=fixture@example.test", "commit", "-qm", "old"], check=True)
            old_revision = subprocess.check_output(["/usr/bin/git", "-C", str(repository), "rev-parse", "HEAD"], text=True).strip()
            old_tree = prepare.package_tree_digest(package)
            old_manifest = prepare.manifest_digest(package)

            write_valid_package(package, version="2.0.0", repository="example/uap")
            subprocess.run(["/usr/bin/git", "-C", str(repository), "add", "."], check=True)
            subprocess.run(["/usr/bin/git", "-C", str(repository), "-c", "user.name=Fixture", "-c", "user.email=fixture@example.test", "commit", "-qm", "new"], check=True)
            current_revision = subprocess.check_output(["/usr/bin/git", "-C", str(repository), "rev-parse", "HEAD"], text=True).strip()
            current_tree = prepare.package_tree_digest(package)
            current_manifest = prepare.manifest_digest(package)

            source = fixture_json("initial-source.json")
            old, new = source["distributions"][0]["releases"]
            old["package_source"]["revision"] = old_revision
            old["tree_digest"], old["manifest_digest"] = old_tree, old_manifest
            new["tree_digest"], new["manifest_digest"] = current_tree, current_manifest
            config = {"schema_version": 1, "repository": "example/uap", "snapshot_lifetime_days": 30}
            candidate = prepare.build_candidate(source, config, current_revision, "initial", None, repository_root=repository)
            bound_old, bound_new = candidate["distributions"][0]["releases"]
            self.assertEqual(bound_old["package_source"]["revision"], old_revision)
            self.assertEqual(bound_old["published_at"], "2025-01-02T03:04:05Z")
            self.assertEqual(bound_new["package_source"]["revision"], current_revision)
            self.assertIsNone(bound_new["published_at"])

    def test_external_acquisition_is_sparse_and_refuses_lfs_and_submodules(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            repository = root / "monorepo"
            package = repository / "packages" / "demo"
            unrelated = repository / "unrelated"
            package.mkdir(parents=True)
            unrelated.mkdir()
            (package / "plugin.json").write_text('{"name":"demo"}\n')
            (unrelated / "large.bin").write_bytes(b"x" * (2 << 20))
            subprocess.run(["/usr/bin/git", "init", "-q", str(repository)], check=True)
            subprocess.run(["/usr/bin/git", "-C", str(repository), "config", "uploadpack.allowFilter", "true"], check=True)
            subprocess.run(["/usr/bin/git", "-C", str(repository), "add", "."], check=True)
            subprocess.run(["/usr/bin/git", "-C", str(repository), "-c", "user.name=Fixture", "-c", "user.email=fixture@example.test", "commit", "-qm", "monorepo"], check=True)
            revision = subprocess.check_output(["/usr/bin/git", "-C", str(repository), "rev-parse", "HEAD"], text=True).strip()
            unrelated_oid = subprocess.check_output(["/usr/bin/git", "-C", str(repository), "rev-parse", "HEAD:unrelated/large.bin"], text=True).strip()
            acquired = prepare.acquire_external("Example/Monorepo", revision, "packages/demo", repository)
            try:
                checkout = Path(acquired.name) / "checkout"
                missing_env = os.environ.copy()
                missing_env["GIT_NO_LAZY_FETCH"] = "1"
                objects = subprocess.check_output(["/usr/bin/git", "-C", str(checkout), "rev-list", "--objects", "--missing=print", "HEAD"], text=True, env=missing_env)
                self.assertIn("?" + unrelated_oid, objects)
                self.assertFalse((checkout / "unrelated" / "large.bin").exists())
                self.assertEqual(prepare.package_tree_digest(checkout / "packages" / "demo"), prepare.package_tree_digest(package))
            finally:
                acquired.cleanup()

            (package / "payload.bin").write_text("version https://git-lfs.github.com/spec/v1\noid sha256:" + "0" * 64 + "\nsize 1\n")
            subprocess.run(["/usr/bin/git", "-C", str(repository), "add", "."], check=True)
            subprocess.run(["/usr/bin/git", "-C", str(repository), "-c", "user.name=Fixture", "-c", "user.email=fixture@example.test", "commit", "-qm", "lfs"], check=True)
            lfs_revision = subprocess.check_output(["/usr/bin/git", "-C", str(repository), "rev-parse", "HEAD"], text=True).strip()
            with self.assertRaisesRegex(publication.PublicationError, "Git LFS pointer"):
                prepare.acquire_external("Example/Monorepo", lfs_revision, "packages/demo", repository)

            child = root / "child"
            child.mkdir()
            (child / "README").write_text("child\n")
            subprocess.run(["/usr/bin/git", "init", "-q", str(child)], check=True)
            subprocess.run(["/usr/bin/git", "-C", str(child), "add", "."], check=True)
            subprocess.run(["/usr/bin/git", "-C", str(child), "-c", "user.name=Fixture", "-c", "user.email=fixture@example.test", "commit", "-qm", "child"], check=True)
            child_revision = subprocess.check_output(["/usr/bin/git", "-C", str(child), "rev-parse", "HEAD"], text=True).strip()
            subprocess.run(["/usr/bin/git", "-C", str(repository), "update-index", "--add", "--cacheinfo", f"160000,{child_revision},packages/demo/vendor"], check=True)
            subprocess.run(["/usr/bin/git", "-C", str(repository), "-c", "user.name=Fixture", "-c", "user.email=fixture@example.test", "commit", "-qm", "submodule"], check=True)
            submodule_revision = subprocess.check_output(["/usr/bin/git", "-C", str(repository), "rev-parse", "HEAD"], text=True).strip()
            with self.assertRaisesRegex(publication.PublicationError, "Git submodule"):
                prepare.acquire_external("Example/Monorepo", submodule_revision, "packages/demo", repository)

    def test_acquisition_environment_and_plugin_root_limits_fail_closed(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            environment = prepare.acquisition_environment(root)
            self.assertEqual(environment["GIT_TERMINAL_PROMPT"], "0")
            self.assertEqual(environment["GIT_LFS_SKIP_SMUDGE"], "1")
            self.assertEqual(environment["GIT_CONFIG_GLOBAL"], os.devnull)
            self.assertNotIn("GITHUB_TOKEN", environment)
            self.assertNotIn("DIRECTORY_ED25519_PRIVATE_KEY", environment)

            package = root / "package"
            package.mkdir()
            (package / "plugin.json").write_text('{"name":"demo"}\n')
            (package / "second.txt").symlink_to("plugin.json")
            original_limit = prepare.MAX_PLUGIN_FILES
            prepare.MAX_PLUGIN_FILES = 1
            try:
                with self.assertRaisesRegex(publication.PublicationError, "exceeds 1 files"):
                    prepare.inspect_plugin_root(package, "fixture")
            finally:
                prepare.MAX_PLUGIN_FILES = original_limit

    def test_post_merge_sha_binding_and_unchanged_release_reuse(self) -> None:
        config = prepare.load_config(ROOT / "registry" / "publication" / "config.json")
        with tempfile.TemporaryDirectory() as tmp:
            package = Path(tmp) / "plugins" / "demo"
            write_valid_package(
                package, repository=config["repository"],
            )
            subprocess.run(["/usr/bin/git", "init", "-q", tmp], check=True)
            subprocess.run(["/usr/bin/git", "-C", tmp, "add", "."], check=True)
            subprocess.run(["/usr/bin/git", "-C", tmp, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.test", "commit", "-qm", "source"], check=True)
            source_tree_commit = subprocess.check_output(["/usr/bin/git", "-C", tmp, "rev-parse", "HEAD"], text=True).strip()
            source_commit = publication_cas.create_marker(Path(tmp), source_tree_commit, "prepare-1")
            tree = prepare.package_tree_digest(package)
            manifest = "sha256:" + __import__("hashlib").sha256((package / "plugin.json").read_bytes()).hexdigest()
            source = {
                "schema_version": 1,
                "products": [{"schema_version": 1, "id": "demo", "display_name": "Demo", "description": "Demo package.", "manifest_name": "demo", "aliases": ["demo"], "reserved_aliases": ["demo"], "categories": ["demo"], "minimum_capabilities": {"skills": "optional", "mcp": "required"}, "default_distribution": "777genius/demo", "distributions": ["777genius/demo"]}],
                "distributions": [{"schema_version": 1, "id": "777genius/demo", "product_id": "demo", "kind": "community", "status": "active", "packager": "777genius", "releases": [{"sequence": 1, "package_version": "1.0.0", "manifest_name": "demo", "agent_plugins_schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json", "package_source": {"repository": config["repository"], "revision": None, "path": "plugins/demo"}, "tree_digest_algorithm": "agentplugins-tree-sha256-v1", "tree_digest": tree, "manifest_digest": manifest, "components": ["mcp"]}], "release_policies": [{"release_sequence": 1, "status": "active", "minimum_installer_version": "0.1.6", "targets": [{"client": "codex", "scopes": ["user"], "delivery": "managed", "authentication": "unknown"}], "current_evidence": []}]}],
                "evidence": [],
            }
            first = prepare.build_candidate(source, config, source_commit, "prepare-1", None, repository_root=Path(tmp))
            self.assertEqual(first["source_commit"], source_commit)
            release = first["distributions"][0]["releases"][0]
            self.assertEqual(release["package_source"]["revision"], source_commit)
            self.assertIsNone(release["published_at"])

            artifact = {
                "repository": "example/evidence", "revision": "b" * 40,
                "path": "evidence.json", "digest": "sha256:" + "3" * 64,
            }
            evidence = {
                "schema_version": 1, "id": "schema-demo", "product_id": "demo",
                "distribution_id": "777genius/demo", "release_sequence": 1,
                "package_tree_digest": tree, "manifest_digest": manifest,
                "source_repository": config["repository"], "source_revision": source_commit,
                "source_path": "plugins/demo", "level": "schema", "outcome": "passed",
                "artifact": artifact,
            }
            source["evidence"] = [{**evidence, "trust": {"kind": "reviewed_external"}}]
            source["distributions"][0]["release_policies"][0]["current_evidence"] = ["schema-demo"]
            with mock.patch.object(prepare, "verified_evidence", return_value=evidence):
                evidenced = prepare.build_candidate(
                    source, config, source_commit, "prepare-evidence", None,
                    repository_root=Path(tmp),
                )
            self.assertEqual(evidenced["evidence"], [evidence])
            self.assertEqual(
                evidenced["distributions"][0]["releases"][0]["package_source"]["revision"],
                evidenced["evidence"][0]["source_revision"],
            )

            source["evidence"] = []
            source["distributions"][0]["release_policies"][0]["current_evidence"] = []
            previous = {"products": first["products"], "distributions": copy.deepcopy(first["distributions"]), "evidence": [], "revocations": []}
            previous["distributions"][0]["releases"][0]["published_at"] = "2026-08-20T00:00:00Z"
            second = prepare.build_candidate(source, config, "e" * 40, "prepare-2", previous, repository_root=Path(tmp))
            release = second["distributions"][0]["releases"][0]
            self.assertEqual(release["package_source"]["revision"], source_commit)
            self.assertEqual(release["published_at"], "2026-08-20T00:00:00Z")

    def test_safety_publication_preserves_an_ineligible_default(self) -> None:
        config = prepare.load_config(ROOT / "registry" / "publication" / "config.json")
        with tempfile.TemporaryDirectory() as tmp:
            package = Path(tmp) / "plugins" / "demo"
            write_valid_package(package, repository=config["repository"])
            tree = prepare.package_tree_digest(package)
            manifest = prepare.manifest_digest(package)
            distribution = {
                "schema_version": 1, "id": "777genius/demo", "product_id": "demo",
                "kind": "community", "status": "suspended", "packager": "777genius",
                "releases": [{
                    "sequence": 1, "package_version": "1.0.0", "manifest_name": "demo",
                    "agent_plugins_schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json",
                    "package_source": {"repository": config["repository"], "revision": None, "path": "plugins/demo"},
                    "tree_digest_algorithm": "agentplugins-tree-sha256-v1", "tree_digest": tree,
                    "manifest_digest": manifest, "components": ["mcp"],
                }],
                "release_policies": [{
                    "release_sequence": 1, "status": "revoked", "minimum_installer_version": "0.1.6",
                    "targets": [{"client": "codex", "scopes": ["user"], "delivery": "managed", "authentication": "unknown"}],
                    "current_evidence": [],
                }],
            }
            source = {
                "schema_version": 1,
                "products": [{
                    "schema_version": 1, "id": "demo", "display_name": "Demo", "description": "Demo.",
                    "manifest_name": "demo", "aliases": ["demo"], "reserved_aliases": ["demo"],
                    "categories": ["demo"], "minimum_capabilities": {"skills": "optional", "mcp": "required"},
                    "default_distribution": "777genius/demo", "distributions": ["777genius/demo"],
                }],
                "distributions": [distribution], "evidence": [],
            }
            candidate = prepare.build_candidate(
                source, config, "c" * 40, "emergency-revocation", None,
                repository_root=Path(tmp),
            )
            self.assertEqual(candidate["products"][0]["default_distribution"], "777genius/demo")
            self.assertEqual(candidate["distributions"][0]["status"], "suspended")
            self.assertEqual(candidate["distributions"][0]["release_policies"][0]["status"], "revoked")
            self.assertEqual(candidate["revocations"], [{"distribution_id": "777genius/demo", "release_sequence": 1}])

            (package / "mcp.json").write_text(json.dumps({
                "$schema": "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json",
                "mcpServers": {"demo": {"type": "stdio", "command": "npx", "args": ["demo@1.0.0"]}},
            }))
            source["distributions"][0]["releases"][0]["tree_digest"] = prepare.package_tree_digest(package)
            revoked = prepare.build_candidate(
                source, config, "d" * 40, "revoked-runtime", None,
                repository_root=Path(tmp),
            )
            self.assertEqual(
                revoked["distributions"][0]["releases"][0]["package_source"]["revision"],
                "d" * 40,
            )

            source["distributions"][0]["status"] = "active"
            source["distributions"][0]["release_policies"][0]["status"] = "active"
            with self.assertRaisesRegex(publication.PublicationError, "content-addressed runtime closure"):
                prepare.build_candidate(
                    source, config, "d" * 40, "unsafe-runtime", None,
                    repository_root=Path(tmp),
                )

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
            write_valid_package(package)
            subprocess.run(["/usr/bin/git", "init", "-q", str(external)], check=True)
            subprocess.run(["/usr/bin/git", "-C", str(external), "add", "."], check=True)
            subprocess.run(["/usr/bin/git", "-C", str(external), "-c", "user.name=Fixture", "-c", "user.email=fixture@example.test", "commit", "-qm", "fixture"], check=True)
            revision = subprocess.check_output(["/usr/bin/git", "-C", str(external), "rev-parse", "HEAD"], text=True).strip()
            source = {
                "schema_version": 1,
                "products": [{"schema_version": 1, "id": "demo", "display_name": "Demo", "description": "External demo package.", "manifest_name": "demo", "aliases": ["demo"], "reserved_aliases": ["demo"], "categories": ["demo"], "minimum_capabilities": {"skills": "optional", "mcp": "required"}, "default_distribution": "example/demo", "distributions": ["example/demo"]}],
                "distributions": [{"schema_version": 1, "id": "example/demo", "product_id": "demo", "kind": "upstream", "status": "active", "packager": "example", "releases": [{"sequence": 1, "package_version": "1.0.0", "manifest_name": "demo", "agent_plugins_schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json", "package_source": {"repository": "example/external", "revision": revision, "path": "plugins/demo"}, "tree_digest_algorithm": "agentplugins-tree-sha256-v1", "tree_digest": "sha256:" + "0" * 64, "manifest_digest": "sha256:" + "0" * 64, "components": ["mcp"]}], "release_policies": [{"release_sequence": 1, "status": "active", "minimum_installer_version": "0.1.6", "targets": [{"client": "codex", "scopes": ["user"], "delivery": "managed", "authentication": "unknown"}], "current_evidence": []}]}],
                "evidence": [],
            }
            # This test isolates external byte reacquisition. Upstream-default
            # publication additionally requires trusted positive materialization.
            source["distributions"][0]["kind"] = "community"
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
            broadened["distributions"][0]["release_policies"][0]["targets"].append({"client": "cursor", "scopes": ["user"], "delivery": "managed", "authentication": "unknown"})
            with self.assertRaisesRegex(publication.PublicationError, "reacquisition failed"):
                prepare.build_candidate(broadened, config, "f" * 40, "external-broadened", previous, external_overrides={"example/external": missing})

            capability_previous = copy.deepcopy(previous)
            capability_previous["products"][0]["minimum_capabilities"]["skills"] = "required"
            with self.assertRaisesRegex(publication.PublicationError, "reacquisition failed"):
                prepare.build_candidate(
                    source, config, "f" * 40, "external-capability-broadened",
                    capability_previous,
                    external_overrides={"example/external": missing},
                )

            metadata_only = copy.deepcopy(source)
            metadata_only["products"][0]["description"] = "Updated card copy only."
            offline = prepare.build_candidate(
                metadata_only, config, "f" * 40, "external-metadata-only",
                previous, external_overrides={"example/external": missing},
            )
            self.assertEqual(
                offline["distributions"][0]["releases"][0]["package_source"],
                previous_release["package_source"],
            )

    def test_newly_eligible_local_binding_uses_pinned_bytes_and_unchanged_binding_is_offline(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            repository = Path(tmp) / "repository"
            package = repository / "plugins" / "demo"
            write_valid_package(package, version="1.0.0", repository="example/local")
            subprocess.run(["/usr/bin/git", "init", "-q", str(repository)], check=True)
            subprocess.run(["/usr/bin/git", "-C", str(repository), "add", "."], check=True)
            subprocess.run(["/usr/bin/git", "-C", str(repository), "-c", "user.name=Fixture", "-c", "user.email=fixture@example.test", "commit", "-qm", "historical"], check=True)
            historical_revision = subprocess.check_output(["/usr/bin/git", "-C", str(repository), "rev-parse", "HEAD"], text=True).strip()
            release = {
                "sequence": 1, "package_version": "1.0.0", "manifest_name": "demo",
                "agent_plugins_schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json",
                "package_source": {"repository": "example/local", "revision": historical_revision, "path": "plugins/demo"},
                "tree_digest_algorithm": "agentplugins-tree-sha256-v1",
                "tree_digest": prepare.package_tree_digest(package),
                "manifest_digest": prepare.manifest_digest(package), "components": ["mcp"],
                "published_at": "2026-08-20T00:00:00Z",
            }
            source = {
                "schema_version": 1,
                "products": [{"schema_version": 1, "id": "demo", "display_name": "Demo", "description": "Demo package.", "manifest_name": "demo", "aliases": ["demo"], "reserved_aliases": ["demo"], "categories": ["demo"], "minimum_capabilities": {"skills": "optional", "mcp": "required"}, "default_distribution": "example/demo", "distributions": ["example/demo"]}],
                "distributions": [{"schema_version": 1, "id": "example/demo", "product_id": "demo", "kind": "community", "status": "active", "packager": "example", "releases": [release], "release_policies": [{"release_sequence": 1, "status": "active", "minimum_installer_version": "0.1.6", "targets": [{"client": "codex", "scopes": ["user"], "delivery": "managed", "authentication": "unknown"}], "current_evidence": []}]}],
                "evidence": [],
            }
            previous = copy.deepcopy(source)
            previous["products"][0]["minimum_capabilities"]["skills"] = "required"
            write_valid_package(package, version="2.0.0", repository="example/local")
            subprocess.run(["/usr/bin/git", "-C", str(repository), "add", "."], check=True)
            subprocess.run(["/usr/bin/git", "-C", str(repository), "-c", "user.name=Fixture", "-c", "user.email=fixture@example.test", "commit", "-qm", "new working tree"], check=True)
            current_revision = subprocess.check_output(["/usr/bin/git", "-C", str(repository), "rev-parse", "HEAD"], text=True).strip()
            config = {"schema_version": 1, "repository": "example/local", "snapshot_lifetime_days": 30}

            candidate = prepare.build_candidate(
                source, config, current_revision, "local-capability-broadened",
                previous, repository_root=repository,
            )
            self.assertEqual(candidate["distributions"][0]["releases"][0]["package_version"], "1.0.0")

            unavailable = Path(tmp) / "unavailable"
            with self.assertRaisesRegex(publication.PublicationError, "reacquisition failed"):
                prepare.build_candidate(
                    source, config, current_revision, "local-capability-offline",
                    previous, repository_root=unavailable,
                )
            unchanged = prepare.build_candidate(
                source, config, current_revision, "local-emergency-offline",
                copy.deepcopy(source), repository_root=unavailable,
            )
            self.assertEqual(
                unchanged["distributions"][0]["releases"][0]["package_source"]["revision"],
                historical_revision,
            )

    def test_external_package_validation_derives_identity_version_and_components(self) -> None:
        config = prepare.load_config(ROOT / "registry" / "publication" / "config.json")
        source_commit = subprocess.check_output(["/usr/bin/git", "rev-parse", "HEAD"], cwd=ROOT, text=True).strip()
        with tempfile.TemporaryDirectory() as tmp:
            external = Path(tmp) / "external"
            package = external / "plugins" / "demo"
            write_valid_package(package)
            subprocess.run(["/usr/bin/git", "init", "-q", str(external)], check=True)
            subprocess.run(["/usr/bin/git", "-C", str(external), "add", "."], check=True)
            subprocess.run(["/usr/bin/git", "-C", str(external), "-c", "user.name=Fixture", "-c", "user.email=fixture@example.test", "commit", "-qm", "valid package"], check=True)
            revision = subprocess.check_output(["/usr/bin/git", "-C", str(external), "rev-parse", "HEAD"], text=True).strip()
            source = {
                "schema_version": 1,
                "products": [{"schema_version": 1, "id": "demo", "display_name": "Demo", "description": "External demo package.", "manifest_name": "demo", "aliases": ["demo"], "reserved_aliases": ["demo"], "categories": ["demo"], "minimum_capabilities": {"skills": "optional", "mcp": "required"}, "default_distribution": "example/demo", "distributions": ["example/demo"]}],
                "distributions": [{"schema_version": 1, "id": "example/demo", "product_id": "demo", "kind": "community", "status": "active", "packager": "example", "releases": [{"sequence": 1, "package_version": "1.0.0", "manifest_name": "demo", "agent_plugins_schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json", "package_source": {"repository": "example/external", "revision": revision, "path": "plugins/demo"}, "tree_digest_algorithm": "agentplugins-tree-sha256-v1", "tree_digest": prepare.package_tree_digest(package), "manifest_digest": prepare.manifest_digest(package), "components": ["mcp"]}], "release_policies": [{"release_sequence": 1, "status": "active", "minimum_installer_version": "0.1.6", "targets": [{"client": "codex", "scopes": ["user"], "delivery": "managed", "authentication": "unknown"}], "current_evidence": []}]}],
                "evidence": [],
            }
            candidate = prepare.build_candidate(source, config, source_commit, "valid-external", None, external_overrides={"example/external": external})
            self.assertEqual(candidate["distributions"][0]["releases"][0]["components"], ["mcp"])

            release = source["distributions"][0]["releases"][0]
            substitutions = (
                ("manifest_name", "substituted", "manifest identity"),
                ("package_version", "9.9.9", "package version"),
                ("components", ["mcp", "skills"], "components differs"),
            )
            for field, substituted, message in substitutions:
                with self.subTest(field=field):
                    changed = copy.deepcopy(source)
                    changed["distributions"][0]["releases"][0][field] = substituted
                    if field == "manifest_name":
                        changed["products"][0]["manifest_name"] = substituted
                    with self.assertRaisesRegex(publication.PublicationError, message):
                        prepare.build_candidate(changed, config, source_commit, "substituted", None, external_overrides={"example/external": external})

            manifest_value = json.loads((package / "plugin.json").read_text())
            manifest_value["repository"] = "https://github.com/attacker/substitute"
            (package / "plugin.json").write_text(json.dumps(manifest_value, sort_keys=True) + "\n")
            subprocess.run(["/usr/bin/git", "-C", str(external), "add", "."], check=True)
            subprocess.run(["/usr/bin/git", "-C", str(external), "-c", "user.name=Fixture", "-c", "user.email=fixture@example.test", "commit", "-qm", "substitute repository"], check=True)
            substituted_repository = copy.deepcopy(source)
            substituted_release = substituted_repository["distributions"][0]["releases"][0]
            substituted_release["package_source"]["revision"] = subprocess.check_output(["/usr/bin/git", "-C", str(external), "rev-parse", "HEAD"], text=True).strip()
            substituted_release["tree_digest"] = prepare.package_tree_digest(package)
            substituted_release["manifest_digest"] = prepare.manifest_digest(package)
            with self.assertRaisesRegex(publication.PublicationError, "manifest repository differs from package source repository"):
                prepare.build_candidate(
                    substituted_repository, config, source_commit, "repository-substitution", None,
                    external_overrides={"example/external": external},
                )

            (package / "plugin.json").write_text('{"name":"demo"}\n')
            subprocess.run(["/usr/bin/git", "-C", str(external), "add", "."], check=True)
            subprocess.run(["/usr/bin/git", "-C", str(external), "-c", "user.name=Fixture", "-c", "user.email=fixture@example.test", "commit", "-qm", "malformed package"], check=True)
            malformed = copy.deepcopy(source)
            malformed_release = malformed["distributions"][0]["releases"][0]
            malformed_release["package_source"]["revision"] = subprocess.check_output(["/usr/bin/git", "-C", str(external), "rev-parse", "HEAD"], text=True).strip()
            malformed_release["tree_digest"] = prepare.package_tree_digest(package)
            malformed_release["manifest_digest"] = prepare.manifest_digest(package)
            with self.assertRaisesRegex(publication.PublicationError, "Agent Plugins 1.0 schema error"):
                prepare.build_candidate(malformed, config, source_commit, "malformed", None, external_overrides={"example/external": external})

    def test_evidence_is_reacquired_verified_and_derived_before_signing(self) -> None:
        payload = {
            "schema_version": 1,
            "id": "runtime-demo-codex",
            "product_id": "demo",
            "distribution_id": "example/demo",
            "release_sequence": 1,
            "package_tree_digest": "sha256:" + "1" * 64,
            "manifest_digest": "sha256:" + "2" * 64,
            "source_repository": "example/plugins",
            "source_revision": "a" * 40,
            "source_path": "plugins/demo",
            "level": "runtime",
            "outcome": "passed",
            "client": "codex",
            "client_version": "0.200.0",
            "installer_version": "0.1.6",
            "os": "linux",
            "architecture": "amd64",
            "observed_at": "2026-08-19T00:00:00Z",
        }

        def source_for(pointer):  # type: ignore[no-untyped-def]
            reviewed = {**payload, **pointer}
            return {
                "evidence": [reviewed],
                "distributions": [{
                    "id": "example/demo",
                    "product_id": payload["product_id"],
                    "releases": [{
                        "sequence": 1,
                        "tree_digest": payload["package_tree_digest"],
                        "manifest_digest": payload["manifest_digest"],
                        "package_source": {
                            "repository": payload["source_repository"],
                            "revision": payload["source_revision"],
                            "path": payload["source_path"],
                        },
                    }],
                    "release_policies": [{"release_sequence": 1, "current_evidence": [reviewed["id"]]}],
                }],
            }

        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            repository = root / "evidence"
            repository.mkdir()
            subprocess.run(["/usr/bin/git", "init", "-q", str(repository)], check=True)
            artifact_path = repository / "evidence.json"
            artifact_body = publication.canonical_json(payload)
            artifact_path.write_bytes(artifact_body)
            subprocess.run(["/usr/bin/git", "-C", str(repository), "add", "evidence.json"], check=True)
            subprocess.run(["/usr/bin/git", "-C", str(repository), "-c", "user.name=Fixture", "-c", "user.email=fixture@example.test", "commit", "-qm", "evidence"], check=True)
            revision = subprocess.check_output(["/usr/bin/git", "-C", str(repository), "rev-parse", "HEAD"], text=True).strip()
            locator = {
                "repository": "example/evidence",
                "revision": revision,
                "path": "evidence.json",
                "digest": publication.sha256_digest(artifact_body),
            }
            pointer = {"id": "runtime-demo-codex", "artifact": locator, "trust": {"kind": "reviewed_external"}}
            config = prepare.load_config(ROOT / "registry" / "publication" / "config.json")
            config["trusted_external_evidence"] = [copy.deepcopy(locator)]
            selected = prepare.selected_evidence(source_for(pointer), {"example/demo"}, config, {"example/evidence": repository})
            self.assertEqual(selected, [{**payload, "artifact": locator}])

            invented_summary = {**pointer, "outcome": "failed", "repository": "invented/repository"}
            derived = prepare.selected_evidence(source_for(invented_summary), {"example/demo"}, config, {"example/evidence": repository})
            self.assertEqual(derived[0]["outcome"], "passed")
            self.assertNotIn("repository", derived[0])

            cases = []
            nonexistent = copy.deepcopy(pointer)
            nonexistent["artifact"]["path"] = "missing.json"
            cases.append(("nonexistent", nonexistent, config, "unavailable"))
            mismatched = copy.deepcopy(pointer)
            mismatched["artifact"]["digest"] = "sha256:" + "0" * 64
            mismatched_config = copy.deepcopy(config)
            mismatched_config["trusted_external_evidence"] = [copy.deepcopy(mismatched["artifact"])]
            cases.append(("digest", mismatched, mismatched_config, "digest mismatch"))
            untrusted = copy.deepcopy(config)
            untrusted["trusted_external_evidence"] = []
            cases.append(("untrusted", pointer, untrusted, "not explicitly trusted"))
            untrusted_workflow = {**pointer, "trust": {"kind": "github_actions", "workflow": "example/evidence/.github/workflows/evidence.yml"}}
            cases.append(("untrusted workflow", untrusted_workflow, config, "no reviewed trust policy"))
            for label, changed_pointer, changed_config, error in cases:
                with self.subTest(label=label), self.assertRaisesRegex(publication.PublicationError, error):
                    prepare.selected_evidence(source_for(changed_pointer), {"example/demo"}, changed_config, {"example/evidence": repository})

            artifact_path.write_bytes(b"{malformed\n")
            subprocess.run(["/usr/bin/git", "-C", str(repository), "add", "evidence.json"], check=True)
            subprocess.run(["/usr/bin/git", "-C", str(repository), "-c", "user.name=Fixture", "-c", "user.email=fixture@example.test", "commit", "-qm", "malformed"], check=True)
            malformed_revision = subprocess.check_output(["/usr/bin/git", "-C", str(repository), "rev-parse", "HEAD"], text=True).strip()
            malformed_locator = {**locator, "revision": malformed_revision, "digest": publication.sha256_digest(b"{malformed\n")}
            malformed_pointer = {**pointer, "artifact": malformed_locator}
            malformed_config = copy.deepcopy(config)
            malformed_config["trusted_external_evidence"] = [copy.deepcopy(malformed_locator)]
            with self.assertRaisesRegex(publication.PublicationError, "invalid UTF-8 JSON"):
                prepare.selected_evidence(source_for(malformed_pointer), {"example/demo"}, malformed_config, {"example/evidence": repository})

    def test_github_evidence_policy_binds_ref_digest_and_runner_environment(self) -> None:
        revision = "a" * 40
        workflow = "example/evidence/.github/workflows/evidence.yml"
        pointer = {
            "id": "runtime-demo-codex",
            "artifact": {
                "repository": "example/evidence", "revision": revision,
                "path": "evidence.json", "digest": "sha256:" + "1" * 64,
            },
            "trust": {
                "kind": "github_actions", "workflow": workflow,
                "source_ref": "refs/heads/main", "source_digest": revision,
            },
        }
        policy = {
            "workflow": workflow,
            "protected_source_ref": "refs/heads/main",
            "source_digest_policy": "artifact_revision",
            "allow_self_hosted_runners": False,
        }
        config = {"trusted_evidence_workflows": [policy]}

        with tempfile.TemporaryDirectory() as temporary, mock.patch.object(
            prepare.subprocess, "run", return_value=subprocess.CompletedProcess([], 0, b"", b""),
        ) as run:
            prepare.verify_evidence_trust(pointer, config, Path(temporary), b"{}")
            command = run.call_args.args[0]
            self.assertIn("--source-ref", command)
            self.assertEqual(command[command.index("--source-ref") + 1], "refs/heads/main")
            self.assertIn("--source-digest", command)
            self.assertEqual(command[command.index("--source-digest") + 1], revision)
            self.assertIn("--deny-self-hosted-runners", command)

        wrong_ref = copy.deepcopy(pointer)
        wrong_ref["trust"]["source_ref"] = "refs/heads/unprotected"
        with tempfile.TemporaryDirectory() as temporary, self.assertRaisesRegex(publication.PublicationError, "source ref is not trusted"):
            prepare.verify_evidence_trust(wrong_ref, config, Path(temporary), b"{}")

        wrong_digest = copy.deepcopy(pointer)
        wrong_digest["trust"]["source_digest"] = "b" * 40
        with tempfile.TemporaryDirectory() as temporary, self.assertRaisesRegex(publication.PublicationError, "source digest does not match"):
            prepare.verify_evidence_trust(wrong_digest, config, Path(temporary), b"{}")

        with tempfile.TemporaryDirectory() as temporary, self.assertRaisesRegex(publication.PublicationError, "no reviewed trust policy"):
            prepare.verify_evidence_trust(pointer, {"trusted_evidence_workflows": []}, Path(temporary), b"{}")

        def reject_self_hosted(command, **_kwargs):  # type: ignore[no-untyped-def]
            return subprocess.CompletedProcess(command, 1 if "--deny-self-hosted-runners" in command else 0, b"", b"")

        with tempfile.TemporaryDirectory() as temporary, mock.patch.object(prepare.subprocess, "run", side_effect=reject_self_hosted), self.assertRaisesRegex(
            publication.PublicationError, "attestation verification failed",
        ):
            prepare.verify_evidence_trust(pointer, config, Path(temporary), b"{}")

        reviewed_self_hosted = copy.deepcopy(config)
        reviewed_self_hosted["trusted_evidence_workflows"][0]["allow_self_hosted_runners"] = True
        with tempfile.TemporaryDirectory() as temporary, mock.patch.object(prepare.subprocess, "run", side_effect=reject_self_hosted) as run:
            prepare.verify_evidence_trust(pointer, reviewed_self_hosted, Path(temporary), b"{}")
            self.assertNotIn("--deny-self-hosted-runners", run.call_args.args[0])


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
        cas_helper = (SCRIPTS / "directory_publication_cas.py").read_text()
        self.assertIn("range(attempts)", cas_helper)
        self.assertIn("git diff --name-status", signer_commands)
        build_job = workflow["jobs"]["build_site"]
        build_commands = "\n".join(step.get("run", "") for step in build_job["steps"] if isinstance(step, dict))
        self.assertEqual(build_job["permissions"], {"contents": "read"})
        self.assertNotIn("secrets.", json.dumps(build_job))
        self.assertIn("pnpm generate", build_commands)
        self.assertIn("pnpm check:generated", build_commands)
        self.assertIn('rm -rf "${GITHUB_WORKSPACE}/ledger"', build_commands)
        self.assertIn('test ! -e "../../ledger"', build_commands)
        self.assertIn("site.files.sha256", build_commands)
        self.assertIn("site.tar.sha256", build_commands)
        self.assertIn("test \"${total_bytes}\" -le 104857600", build_commands)
        self.assertIn('test "${snapshot_digest}" = "${EXPECTED_SNAPSHOT_DIGEST}"', build_commands)
        self.assertIn('rm -rf "${RUNNER_TEMP}/site-artifact"', build_commands)
        site_job = workflow["jobs"]["materialize_site"]
        site_commands = "\n".join(step.get("run", "") for step in site_job["steps"] if isinstance(step, dict))
        self.assertEqual(site_job["permissions"], {"actions": "read", "contents": "read"})
        self.assertEqual(
            {
                step["uses"]
                for step in site_job["steps"]
                if isinstance(step, dict) and "uses" in step
            },
            {
                "actions/download-artifact@b14cf4c92620c250e1c0745ffefa574f1c4531a9",
                "actions/create-github-app-token@bcd2ba49218906704ab6c1aa796996da409d3eb1",
            },
        )
        step_names = [step.get("name", "") for step in site_job["steps"] if isinstance(step, dict)]
        self.assertLess(
            step_names.index("Reject unsafe archive entries and verify the artifact"),
            step_names.index("Fetch the exact ledger commit without credentials"),
        )
        for forbidden in ("npm", "pnpm", "node ", "python", "trusted-source"):
            with self.subTest(forbidden=forbidden):
                self.assertNotIn(forbidden, site_commands.lower())
        self.assertIn("EXPECTED_ARCHIVE_DIGEST", json.dumps(site_job))
        self.assertIn("EXPECTED_MANIFEST_DIGEST", json.dumps(site_job))
        self.assertIn("EXPECTED_SNAPSHOT_DIGEST", json.dumps(site_job))
        self.assertIn("expected-artifact.paths", site_commands)
        self.assertIn("-links +1", site_commands)
        self.assertIn("--full-time -tvf", site_commands)
        self.assertIn('substr($0, 1, 1) != "-"', site_commands)
        self.assertIn(".git[^\\/]*", site_commands)
        self.assertIn("sha256sum --check", site_commands)
        self.assertIn("manifest.paths", site_commands)
        self.assertIn("core.hooksPath=/dev/null commit", site_commands)
        self.assertIn("git -C ledger diff --exit-code -- registry/schemas/1/snapshots", site_commands)
        self.assertNotIn("github.token", text)
        self.assertNotIn("GH_TOKEN", text)
        self.assertEqual(signer["permissions"], {"actions": "read", "contents": "read"})
        self.assertEqual(site_job["environment"], "directory-publication-materialization")
        deploy_commands = "\n".join(step.get("run", "") for step in workflow["jobs"]["deploy"]["steps"] if isinstance(step, dict))
        self.assertIn("needs.materialize_site.outputs.ledger_commit", text)
        self.assertIn("git -C exact-pages-tree rev-parse HEAD", deploy_commands)
        exact_gate = workflow["jobs"]["gate_exact_staged_publication"]
        gate_step = next(step for step in exact_gate["steps"] if step.get("name") == "Verify staged bytes and immutable identity before promotion")
        self.assertEqual(exact_gate["needs"], ["sign", "materialize_site"])
        self.assertEqual(gate_step["env"]["EXPECTED_SEQUENCE"], "${{ needs.sign.outputs.sequence }}")
        self.assertEqual(gate_step["env"]["EXPECTED_SNAPSHOT_DIGEST"], "${{ needs.sign.outputs.snapshot_digest }}")
        self.assertEqual(gate_step["env"]["EXPECTED_PUBLICATION_ID"], "${{ needs.sign.outputs.publication_id }}")
        self.assertEqual(gate_step["env"]["EXPECTED_SOURCE_COMMIT"], "${{ needs.sign.outputs.marker_commit }}")
        self.assertEqual(gate_step["env"]["EXPECTED_SIGNED_LEDGER_COMMIT"], "${{ needs.sign.outputs.ledger_commit }}")
        self.assertIn("raw.githubusercontent.com", gate_step["run"])
        self.assertIn('cmp --silent "${feed}/${relative}"', gate_step["run"])
        self.assertIn("gate_launch_approval", workflow["jobs"]["deploy"]["needs"])
        self.assertIn("gate_exact_staged_publication", workflow["jobs"]["deploy"]["needs"])
        self.assertIn("sign", workflow["jobs"]["deploy"]["needs"])
        self.assertEqual(
            set(workflow["jobs"]["required_stable_launch_evidence"]["needs"]),
            {"sign", "materialize_site", "gate_exact_staged_publication"},
        )
        launch_gate = workflow["jobs"]["required_stable_launch_evidence"]
        self.assertIn("needs.sign.outputs.sequence == '1'", launch_gate["if"])
        marker_gate = workflow["jobs"]["gate_launch_approval"]
        self.assertIn("needs.sign.outputs.sequence > 1", marker_gate["if"])
        self.assertIn("needs.required_stable_launch_evidence.result == 'success'", marker_gate["if"])
        self.assertIn("needs.required_stable_launch_evidence.result == 'skipped'", marker_gate["if"])
        deploy_if = workflow["jobs"]["deploy"]["if"]
        self.assertIn("always()", deploy_if)
        self.assertIn("needs.gate_launch_approval.result == 'success'", deploy_if)
        production_observation = workflow["jobs"]["observe_production_latest"]
        self.assertIn("deploy", production_observation["needs"])
        self.assertEqual(production_observation["permissions"], {"contents": "read"})
        self.assertIn("EXISTING_MATERIALIZED_COMMIT", site_commands)
        self.assertIn("rerun site tree differs", site_commands)
        self.assertIn('--materialized-output ../materialized-ledger.commit', signer_commands)
        for match in __import__("re").findall(r"uses:\s+([^\s]+)", text):
            if match.startswith("./"):
                self.assertEqual(match, "./.github/workflows/live-e2e.yml")
            else:
                self.assertRegex(match, r"@[0-9a-f]{40}$")

    def test_no_site_or_package_execution_in_contents_write_jobs(self) -> None:
        path = ROOT / ".github" / "workflows" / "directory-publication.yml"
        workflow = yaml.load(path.read_text(), Loader=yaml.BaseLoader)
        for name, job in workflow["jobs"].items():
            if job.get("permissions", {}).get("contents") != "write":
                continue
            commands = "\n".join(
                step.get("run", "")
                for step in job["steps"]
                if isinstance(step, dict)
            ).lower()
            with self.subTest(job=name):
                self.assertNotIn("pnpm", commands)
                self.assertNotIn("npm ", commands)
                self.assertNotIn("node ", commands)
                self.assertFalse(
                    any(
                        "setup-node" in step.get("uses", "")
                        or "pnpm/action-setup" in step.get("uses", "")
                        for step in job["steps"]
                        if isinstance(step, dict)
                    )
                )

    def test_security_sensitive_publication_inputs_are_code_owned(self) -> None:
        codeowners = (ROOT / ".github" / "CODEOWNERS").read_text().splitlines()
        patterns = {line.split()[0] for line in codeowners if line.strip()}
        self.assertTrue(
            {
                "/.github/workflows/directory-publication.yml",
                "/.github/workflows/validate.yml",
                "/registry/directory.json",
                "/registry/publication/",
                "/plugins/",
                "/bridges/",
                "/site/",
                "/scripts/*directory_publication.py",
                "/scripts/build-bridges",
                "/scripts/build_bridges.py",
                "/scripts/build_registry.py",
            }.issubset(patterns)
        )

    def test_all_workflow_yaml_parses(self) -> None:
        for path in (ROOT / ".github" / "workflows").glob("*.yml"):
            with self.subTest(path=path.name):
                self.assertIsInstance(yaml.safe_load(path.read_text()), dict)


if __name__ == "__main__":
    unittest.main()

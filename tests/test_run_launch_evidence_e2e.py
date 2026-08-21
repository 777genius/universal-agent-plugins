from __future__ import annotations

import importlib.util
import base64
import json
import os
import tempfile
import unittest
from datetime import datetime, timedelta, timezone
from pathlib import Path
from unittest import mock

import jsonschema
from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey


ROOT = Path(__file__).resolve().parents[1]
MODULE = ROOT / "scripts" / "run_launch_evidence_e2e.py"
SPEC = importlib.util.spec_from_file_location("run_launch_evidence_e2e", MODULE)
assert SPEC and SPEC.loader
e2e = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(e2e)
import launch_observer_signatures as observer_signatures
OBSERVER_SPEC = importlib.util.spec_from_file_location("observe_launch_scenario", ROOT / "scripts" / "observe_launch_scenario.py")
assert OBSERVER_SPEC and OBSERVER_SPEC.loader
observer = importlib.util.module_from_spec(OBSERVER_SPEC)
OBSERVER_SPEC.loader.exec_module(observer)
FACADE_SPEC = importlib.util.spec_from_file_location("observe_release_facade", ROOT / "scripts" / "observe_release_facade.py")
assert FACADE_SPEC and FACADE_SPEC.loader
facade = importlib.util.module_from_spec(FACADE_SPEC)
FACADE_SPEC.loader.exec_module(facade)
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
        self.assertEqual(evidence["schema_version"], 3)
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

    def test_challenge_binds_github_release_directory_and_root(self) -> None:
        with tempfile.TemporaryDirectory() as tmp, mock.patch.object(e2e.secrets, "token_hex", return_value="ab" * 32):
            first = e2e.make_challenge("a" * 40, "12", "3", "sha256:" + "b" * 64, "sha256:" + "c" * 64, Path(tmp))
            changed = e2e.make_challenge("a" * 40, "12", "3", "sha256:" + "d" * 64, "sha256:" + "c" * 64, Path(tmp))
        self.assertNotEqual(first["value"], changed["value"])
        self.assertEqual(first["github_sha"], "a" * 40)
        self.assertEqual(first["root_id"], e2e.hashlib.sha256(str(Path(tmp).resolve()).encode()).hexdigest())
        self.assertTrue(e2e.challenge_context_valid(first))
        self.assertFalse(e2e.challenge_context_valid({**first, "directory_digest": "sha256:" + "d" * 64}))

    def test_live_artifacts_require_fresh_challenge_bound_ed25519_bundle(self) -> None:
        private_key = Ed25519PrivateKey.generate()
        public_key = private_key.public_key().public_bytes(
            encoding=serialization.Encoding.Raw,
            format=serialization.PublicFormat.Raw,
        )
        challenge = "a" * 64
        artifacts = {
            "runtime-attestations.json": {"schema_version": 1, "attestations": []},
            "notion-oauth-attestations.json": {"schema_version": 1, "attestations": []},
            "chatgpt-cloudflare-attestation.json": {"schema_version": 1, "attestations": []},
            "consent.json": {"schema_version": 1, "purpose": "stable-launch-e2e", "consent": True, "disposable_only": True},
        }
        now = datetime.now(timezone.utc).replace(microsecond=0)
        bundle = {
            "schema_version": 1, "challenge": challenge,
            "signed_at": now.isoformat().replace("+00:00", "Z"),
            "key_id": "stable-observer-2026", "artifacts": artifacts,
        }
        bundle["signature"] = base64.b64encode(private_key.sign(observer_signatures.signed_payload(bundle))).decode()
        encoded_key = base64.b64encode(public_key).decode()
        self.assertEqual(
            observer_signatures.verify_observer_bundle(
                bundle, challenge=challenge, public_key_base64=encoded_key,
                expected_key_id="stable-observer-2026", now=now,
            ), artifacts,
        )
        with self.assertRaisesRegex(ValueError, "signature is invalid"):
            observer_signatures.verify_observer_bundle(
                {**bundle, "artifacts": {**artifacts, "consent.json": {"consent": False}}},
                challenge=challenge, public_key_base64=encoded_key,
                expected_key_id="stable-observer-2026", now=now,
            )
        stale = {**bundle, "signed_at": (now - timedelta(hours=1)).isoformat().replace("+00:00", "Z")}
        stale["signature"] = base64.b64encode(private_key.sign(observer_signatures.signed_payload(stale))).decode()
        with self.assertRaisesRegex(ValueError, "stale"):
            observer_signatures.verify_observer_bundle(
                stale, challenge=challenge, public_key_base64=encoded_key,
                expected_key_id="stable-observer-2026", now=now,
            )

    def test_release_manifest_requires_every_native_slot_and_exact_identity(self) -> None:
        assets = {
            key: {"file": f"agentplugins_1.2.3_{os_name}_{arch}{suffix}", "sha256": f"{index + 1:064x}", "size": 1}
            for index, (key, os_name, arch, suffix) in enumerate((
                ("darwin-amd64", "darwin", "amd64", ""), ("darwin-arm64", "darwin", "arm64", ""),
                ("linux-amd64", "linux", "amd64", ""), ("linux-arm64", "linux", "arm64", ""),
                ("windows-amd64", "windows", "amd64", ".exe"), ("windows-arm64", "windows", "arm64", ".exe"),
            ))
        }
        value = {"schema_version": 2, "tag": "agentplugins-v1.2.3", "commit": "a" * 40, "version": "1.2.3", "assets": assets}
        e2e.validate_release_manifest(value, repository=e2e.TRUSTED_CLI_RELEASE_REPOSITORY, tag="agentplugins-v1.2.3", tag_commit="a" * 40)
        with self.assertRaisesRegex(ValueError, "omits a required"):
            e2e.validate_release_manifest({**value, "assets": dict(list(assets.items())[:-1])}, repository=e2e.TRUSTED_CLI_RELEASE_REPOSITORY, tag="agentplugins-v1.2.3", tag_commit="a" * 40)

    def test_release_resolution_binds_github_tag_manifest_and_asset_bytes(self) -> None:
        selected = b"native-binary"
        slots = (
            ("darwin-amd64", "agentplugins_1.2.3_darwin_amd64", b"1"),
            ("darwin-arm64", "agentplugins_1.2.3_darwin_arm64", b"2"),
            ("linux-amd64", "agentplugins_1.2.3_linux_amd64", selected),
            ("linux-arm64", "agentplugins_1.2.3_linux_arm64", b"4"),
            ("windows-amd64", "agentplugins_1.2.3_windows_amd64.exe", b"5"),
            ("windows-arm64", "agentplugins_1.2.3_windows_arm64.exe", b"6"),
        )
        manifest = {
            "schema_version": 2, "tag": "agentplugins-v1.2.3", "commit": "a" * 40, "version": "1.2.3",
            "assets": {key: {"file": name, "sha256": e2e.hashlib.sha256(body).hexdigest(), "size": len(body)} for key, name, body in slots},
        }
        manifest_body = json.dumps(manifest, sort_keys=True, separators=(",", ":")).encode()
        checksum_bodies = [(name, body) for _, name, body in slots] + [(e2e.RELEASE_MANIFEST_NAME, manifest_body)]
        checksums_body = "".join(
            f"{e2e.hashlib.sha256(body).hexdigest()}  {name}\n" for name, body in checksum_bodies
        ).encode()
        api_assets = [{"name": e2e.RELEASE_MANIFEST_NAME, "url": "https://api.github.test/manifest", "size": len(manifest_body)}]
        api_assets.append({"name": e2e.RELEASE_CHECKSUMS_NAME, "url": "https://api.github.test/checksums", "size": len(checksums_body)})
        api_assets += [{"name": name, "url": f"https://api.github.test/{name}", "size": len(body)} for _, name, body in slots]
        release = {"id": 123, "draft": False, "prerelease": False, "immutable": True, "tag_name": "agentplugins-v1.2.3", "assets": api_assets}
        bodies = {
            "https://api.github.test/manifest": manifest_body,
            "https://api.github.test/checksums": checksums_body,
            **{f"https://api.github.test/{name}": body for _, name, body in slots},
        }

        with tempfile.TemporaryDirectory() as tmp, mock.patch.object(e2e, "github_json", return_value=release), mock.patch.object(e2e, "resolve_tag_commit", return_value="a" * 40):
            destination, resolved, digest = e2e.resolve_github_release(
                e2e.TRUSTED_CLI_RELEASE_REPOSITORY, "agentplugins-v1.2.3", Path(tmp) / "agentplugins",
                asset_name="agentplugins_1.2.3_linux_amd64",
                fixture_fetch=lambda url, _limit, _accept: bodies[url],
                attestation_verifier=lambda path, repo, workflow, tag, commit, digest: {"repository": repo, "workflow": workflow, "tag": tag, "tag_commit": commit, "asset_name": path.name, "asset_digest": digest, "verified": True},
            )
            self.assertEqual(destination.read_bytes(), selected)
            self.assertEqual(resolved, manifest)
            self.assertEqual(digest, "sha256:" + e2e.hashlib.sha256(manifest_body).hexdigest())
            self.assertEqual((destination.parent / e2e.RELEASE_CHECKSUMS_NAME).read_bytes(), checksums_body)

            tampered = {**bodies, "https://api.github.test/agentplugins_1.2.3_linux_amd64": b"tampered-bytes"}
            with self.assertRaisesRegex(ValueError, "digest disagrees"):
                e2e.resolve_github_release(
                    e2e.TRUSTED_CLI_RELEASE_REPOSITORY, "agentplugins-v1.2.3", Path(tmp) / "tampered",
                    asset_name="agentplugins_1.2.3_linux_amd64",
                    fixture_fetch=lambda url, _limit, _accept: tampered[url],
                    attestation_verifier=lambda *_args: {},
                )

            tampered_checksums = {**bodies, "https://api.github.test/checksums": checksums_body.replace(b"  release-manifest.json", b"  renamed-manifest.json")}
            with self.assertRaisesRegex(ValueError, "exact manifest asset set"):
                e2e.resolve_github_release(
                    e2e.TRUSTED_CLI_RELEASE_REPOSITORY, "agentplugins-v1.2.3", Path(tmp) / "bad-checksums",
                    asset_name="agentplugins_1.2.3_linux_amd64",
                    fixture_fetch=lambda url, _limit, _accept: tampered_checksums[url],
                    attestation_verifier=lambda *_args: {},
                )

            with mock.patch.object(e2e, "github_json", return_value={**release, "immutable": False}), self.assertRaisesRegex(ValueError, "mutable"):
                e2e.resolve_github_release(
                    e2e.TRUSTED_CLI_RELEASE_REPOSITORY, "agentplugins-v1.2.3", Path(tmp) / "mutable",
                    asset_name="agentplugins_1.2.3_linux_amd64", fixture_fetch=lambda url, _limit, _accept: bodies[url],
                    attestation_verifier=lambda *_args: {},
                )

    def test_github_attestation_rejects_missing_or_wrong_subject(self) -> None:
        verified = mock.Mock(returncode=0, stdout="[]", stderr="")
        with tempfile.TemporaryDirectory() as tmp, mock.patch.object(e2e.subprocess, "run", return_value=verified):
            asset = Path(tmp) / "agentplugins_0.1.8_linux_amd64"
            asset.write_bytes(b"native")
            with self.assertRaisesRegex(ValueError, "no verified"):
                e2e.verify_github_asset_attestation(asset, e2e.TRUSTED_CLI_RELEASE_REPOSITORY, e2e.TRUSTED_CLI_RELEASE_WORKFLOW, e2e.TRUSTED_CLI_RELEASE_TAG, "a" * 40, "sha256:" + e2e.hashlib.sha256(b"native").hexdigest())
            wrong = [{"verificationResult": {"statement": {"subject": [{"name": "wrong", "digest": {"sha256": "0" * 64}}]}}}]
            verified.stdout = json.dumps(wrong)
            with self.assertRaisesRegex(ValueError, "subject name/digest"):
                e2e.verify_github_asset_attestation(asset, e2e.TRUSTED_CLI_RELEASE_REPOSITORY, e2e.TRUSTED_CLI_RELEASE_WORKFLOW, e2e.TRUSTED_CLI_RELEASE_TAG, "a" * 40, "sha256:" + e2e.hashlib.sha256(b"native").hexdigest())

    def test_npm_installed_executable_must_equal_authenticated_native_asset(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            executable = Path(tmp) / "agentplugins"
            executable.write_bytes(b"prints-correct-version-but-is-not-release-binary")
            executable.chmod(0o700)
            native = {"sha256": e2e.hashlib.sha256(b"real-release-binary").hexdigest(), "size": len(b"real-release-binary")}
            with self.assertRaisesRegex(RuntimeError, "does not match"):
                facade.verify_installed_npm_payload(Path(tmp), native)

    def test_npm_resolution_binds_exact_registry_integrity_and_tarball(self) -> None:
        body = b"exact npm tarball"
        integrity = "sha512-" + base64.b64encode(e2e.hashlib.sha512(body).digest()).decode()
        metadata_url = "https://registry.npmjs.org/universal-agent-plugins/0.1.8"
        tarball_url = "https://registry.npmjs.org/universal-agent-plugins/-/universal-agent-plugins-0.1.8.tgz"
        provenance_url = "https://registry.npmjs.org/-/npm/v1/attestations/universal-agent-plugins@0.1.8"
        metadata = json.dumps({"name": "universal-agent-plugins", "version": "0.1.8", "dist": {"integrity": integrity, "tarball": tarball_url, "attestations": {"url": provenance_url, "provenance": {"predicateType": "https://slsa.dev/provenance/v1"}}}}).encode()
        bodies = {metadata_url: metadata, tarball_url: body}
        with tempfile.TemporaryDirectory() as tmp:
            path, identity = e2e.resolve_npm_package(
                "universal-agent-plugins", "0.1.8", Path(tmp) / "package.tgz",
                fixture_fetch=lambda url, _limit, _accept: bodies[url],
            )
            self.assertEqual(path.read_bytes(), body)
            self.assertEqual(identity["integrity"], integrity)
            self.assertEqual(identity["provenance_url"], provenance_url)
            with self.assertRaisesRegex(ValueError, "dist.integrity"):
                e2e.resolve_npm_package(
                    "universal-agent-plugins", "0.1.8", Path(tmp) / "tampered.tgz",
                    fixture_fetch=lambda url, _limit, _accept: b"tampered" if url == tarball_url else metadata,
                )
            without_provenance = json.dumps({"name": "universal-agent-plugins", "version": "0.1.8", "dist": {"integrity": integrity, "tarball": tarball_url}}).encode()
            with self.assertRaisesRegex(ValueError, "provenance"):
                e2e.resolve_npm_package(
                    "universal-agent-plugins", "0.1.8", Path(tmp) / "no-provenance.tgz",
                    fixture_fetch=lambda url, _limit, _accept: body if url == tarball_url else without_provenance,
                )

    def test_production_identity_is_fixed_cross_repository_configuration(self) -> None:
        config = e2e.read_production_config()
        self.assertEqual(config["catalog_repository"], "777genius/universal-agent-plugins")
        self.assertEqual(config["cli_release_repository"], "777genius/plugin-kit-ai")
        self.assertEqual(config["cli_release_tag"], "agentplugins-v0.1.8")
        self.assertEqual(config["cli_release_workflow"], "777genius/plugin-kit-ai/.github/workflows/agentplugins-release.yml")
        schema = json.loads((ROOT / "tests/e2e/schemas/native-release-observation.schema.json").read_text())
        self.assertEqual(
            schema["properties"]["github_asset_attestation"]["properties"]["workflow"]["const"],
            e2e.TRUSTED_CLI_RELEASE_WORKFLOW,
        )
        self.assertNotIn("repository", config)

    def test_production_identity_rejects_configured_repository_or_tag_changes(self) -> None:
        original = json.loads(e2e.PRODUCTION_CONFIG.read_text())
        for field, changed in (
            ("catalog_repository", "attacker/catalog"),
            ("cli_release_repository", "attacker/binaries"),
            ("cli_release_tag", "agentplugins-v0.1.9"),
            ("cli_release_workflow", "attacker/repo/.github/workflows/agentplugins-release.yml"),
        ):
            with self.subTest(field=field), tempfile.TemporaryDirectory() as tmp:
                path = Path(tmp) / "production-launch.json"
                path.write_text(json.dumps({**original, field: changed}))
                with mock.patch.object(e2e, "PRODUCTION_CONFIG", path), self.assertRaisesRegex(ValueError, "configuration is invalid"):
                    e2e.read_production_config()

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
        self.assertEqual(set(env), e2e.DIRECTORY_INPUT_ENVIRONMENT_KEYS)

    def test_real_binary_directory_environment_has_exact_conformance_tuple(self) -> None:
        directory_environment = {
            "AGENTPLUGINS_DIRECTORY_ORIGIN": "https://directory.example.test/registry/",
            "AGENTPLUGINS_DIRECTORY_SNAPSHOT": str(PUBLICATION / "snapshot.json"),
            "AGENTPLUGINS_DIRECTORY_ENVELOPE": str(PUBLICATION / "envelope-current.json"),
            "AGENTPLUGINS_DIRECTORY_TRUST": str(PUBLICATION / "trusted-keys.json"),
        }
        with tempfile.TemporaryDirectory() as tmp:
            sandbox = Path(tmp) / "scenario"
            sandbox.mkdir()
            env = e2e.isolated_environment(sandbox, ("cursor",), directory_environment)
        directory_keys = {key for key in env if key.startswith("AGENTPLUGINS_DIRECTORY_")}
        self.assertEqual(directory_keys, e2e.DIRECTORY_LAUNCH_ENVIRONMENT_KEYS)
        self.assertEqual(env["AGENTPLUGINS_DIRECTORY_CONFORMANCE_ONLY"], "1")

    def test_partial_real_binary_directory_environment_is_rejected(self) -> None:
        partial = {
            "AGENTPLUGINS_DIRECTORY_ORIGIN": "https://directory.example.test/registry/",
            "AGENTPLUGINS_DIRECTORY_SNAPSHOT": str(PUBLICATION / "snapshot.json"),
        }
        with tempfile.TemporaryDirectory() as tmp:
            sandbox = Path(tmp) / "scenario"
            sandbox.mkdir()
            with self.assertRaisesRegex(ValueError, "complete origin/snapshot/envelope/trust tuple"):
                e2e.isolated_environment(sandbox, ("cursor",), partial)

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

    def test_repository_observer_derives_grouped_lifecycle_from_receipts_and_native_files(self) -> None:
        fake_binary = '''#!/usr/bin/python3
import json, os, pathlib, sys
operation, product = sys.argv[1], sys.argv[2]
target = sys.argv[sys.argv.index("--target") + 1]
home = pathlib.Path(os.environ["HOME"])
manager = pathlib.Path(os.environ["AGENTPLUGINS_HOME"])
state_path = manager / "state.json"
state = json.loads(state_path.read_text()) if state_path.exists() else {"product": product, "receipts": [], "directory": {"distribution_id": "upstash/context7", "desired_release_sequence": 1}, "package": {"tree_digest": "sha256:" + "a" * 64, "manifest_digest": "sha256:" + "e" * 64}}
roots = {"codex": home / ".codex", "cursor": home / ".cursor", "kiro": home / ".kiro"}
if operation in {"add", "update", "repair", "remove"}:
    state["receipts"].append({"phase": "committed", "operation": operation})
    manager.mkdir(parents=True, exist_ok=True)
    state_path.write_text(json.dumps(state))
if operation == "add":
    for client in target.split(","):
        roots[client].mkdir(parents=True, exist_ok=True)
        (roots[client] / (product + ".json")).write_text(json.dumps({"product": product}))
if operation == "remove":
    for client in target.split(","):
        path = roots[client] / (product + ".json")
        if path.exists(): path.unlink()
value = {"client_version": "isolated-native-config-v1", "receipt_reconciled": True, "native_discovery_reconciled": True}
if operation == "add":
    value.update({"acquisition_count": 1, "tree_digest": "sha256:" + "a" * 64})
print(json.dumps(value))
'''
        context = {
            "value": "c" * 64, "directory_digest": "sha256:" + "d" * 64,
            "binary_digest": "sha256:" + "b" * 64, "expected_version": "0.1.8",
            "snapshot_sequence": 7,
            "release": {
                "product_id": "context7", "tree_digest": "sha256:" + "a" * 64,
                "manifest_digest": "sha256:" + "e" * 64, "distribution_id": "upstash/context7",
                "distribution_kind": "upstream", "release_sequence": 1, "package_version": "1.0.0",
            },
        }
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            binary = root / "agentplugins"
            binary.write_text(fake_binary)
            binary.chmod(0o700)
            home = root / "home"
            manager = root / "manager"
            workspace = root / "workspace"
            workspace.mkdir()
            with mock.patch.dict(os.environ, {"HOME": str(home), "AGENTPLUGINS_HOME": str(manager)}, clear=False):
                value = observer.run(binary, "context7_grouped_lifecycle", workspace, context)
        self.assertEqual(value["outcome"], "passed")
        self.assertEqual(value["acquisition_digests"], ["sha256:" + "a" * 64])
        self.assertEqual(set(value["target_outcomes"]), {"codex", "cursor", "kiro"})
        self.assertTrue(all(item["after"]["manager"]["committed_receipts"] >= item["before"]["manager"]["committed_receipts"] for item in value["operation_observations"]))
        self.assertEqual(value["operation_observations"][-1]["after"]["native_mentions"], {"codex": 0, "cursor": 0, "kiro": 0})

    def test_policy_conformance_directory_is_test_signed_and_never_the_production_root(self) -> None:
        snapshot = json.loads((PUBLICATION / "snapshot.json").read_text())
        distribution = snapshot["distributions"][0]
        release = distribution["releases"][0]
        context = {
            "github_sha": "a" * 40,
            "directory_product": snapshot["products"][0],
            "directory_distribution": distribution,
            "release": {
                "release_sequence": release["sequence"],
                "product_id": snapshot["products"][0]["id"],
                "distribution_id": distribution["id"],
                "tree_digest": release["tree_digest"],
            },
        }
        with tempfile.TemporaryDirectory() as tmp:
            environment, digest = observer.conformance_directory(
                Path(tmp), context, sequence=1007, sequence_over_semver=True,
            )
            trust = json.loads(Path(environment["AGENTPLUGINS_DIRECTORY_TRUST"]).read_text())
            generated = json.loads(Path(environment["AGENTPLUGINS_DIRECTORY_SNAPSHOT"]).read_text())
        self.assertTrue(digest.startswith("sha256:"))
        self.assertEqual(trust["keys"][0]["key_id"], "launch-conformance-only")
        self.assertNotEqual(trust, json.loads(e2e.PRODUCTION_DIRECTORY_TRUST.read_text()))
        self.assertEqual([item["sequence"] for item in generated["distributions"][0]["releases"]], [1, 2])
        self.assertEqual([item["package_version"] for item in generated["distributions"][0]["releases"]], ["9.0.0", "1.0.0"])

    def test_fixture_contracts_cover_required_fault_slots(self) -> None:
        config = json.loads(e2e.SCENARIOS.read_text())
        required = {
            "directory_offline", "directory_expired", "directory_tampered", "directory_sequence_rollback",
            "missing_runtime_zero_mutation", "plugin_data_update_repair_switch_remove_purge",
            "stdio_environment_and_containment", "promotion_gate_digest_mismatch",
            "distribution_sticky_update", "managed_rollback",
        }
        observed = set(config["fault_scenarios"] + config["advanced_scenarios"])
        self.assertTrue(required.issubset(observed))
        for scenario in config["fault_scenarios"] + config["adapter_repair_faults"] + config["advanced_scenarios"]:
            with self.subTest(scenario=scenario):
                self.assertFalse(e2e.LaunchHarness.driver_proof_valid(scenario, {"outcome": "passed"}))

    def test_promotion_and_fork_observers_execute_exact_local_validators(self) -> None:
        scenarios = (
            "promotion_gate_digest_match", "promotion_gate_digest_mismatch",
            "fork_submission", "fork_submission_rejected",
        )
        match_candidate_digest = None
        with tempfile.TemporaryDirectory() as tmp:
            parent = Path(tmp)
            for scenario in scenarios:
                with self.subTest(scenario=scenario):
                    root = parent / scenario
                    root.mkdir()
                    environment = {"HOME": str(root / "home"), "AGENTPLUGINS_HOME": str(root / "manager")}
                    with mock.patch.dict(os.environ, environment, clear=False):
                        if scenario.startswith("promotion_"):
                            passed, value = observer.promotion_scenario(Path("/not-used"), scenario, root, "a" * 64)
                        else:
                            passed, value = observer.fork_submission_scenario(scenario, root, "a" * 64)
                    self.assertTrue(passed, value)
                    artifact = value["validator_artifact"]
                    if scenario.endswith("mismatch") or scenario.endswith("rejected"):
                        self.assertEqual(artifact["outcome"], "rejected")
                    else:
                        self.assertEqual(artifact["outcome"], "accepted")
                        self.assertTrue(artifact["gates"])
                    if scenario == "promotion_gate_digest_match":
                        match_candidate_digest = artifact["candidate_digest"]
            repeat = parent / "promotion_gate_digest_match_repeat"
            repeat.mkdir()
            with mock.patch.dict(os.environ, {"HOME": str(repeat / "home"), "AGENTPLUGINS_HOME": str(repeat / "manager")}, clear=False):
                passed, value = observer.promotion_scenario(Path("/not-used"), "promotion_gate_digest_match", repeat, "a" * 64)
            self.assertTrue(passed, value)
            self.assertEqual(value["validator_artifact"]["candidate_digest"], match_candidate_digest)

    def test_journey_aggregation_requires_accepted_and_rejected_fork_artifacts(self) -> None:
        harness = self.fixture_harness()
        harness.cli_version = "0.1.8"
        accepted = {
            "fork_created": True, "branch_submission": True, "submission_validated": True,
            "publication_performed": False, "pr_created": False, "network_performed": False,
            "client_version": "fixture-validator-v1",
        }
        rejected = {
            "fork_created": True, "submission_rejected": True, "no_side_effect": True,
            "no_candidate": True, "client_version": "fixture-validator-v1",
        }
        with mock.patch.object(harness, "command", return_value=("failed", None, "not under test")), mock.patch.object(
            harness, "driven_scenario", side_effect=[("passed", accepted, "accepted"), ("passed", rejected, "rejected")],
        ):
            harness.journeys()
        rows = {row["scenario"]: row for row in harness.rows}
        self.assertEqual(rows["fork_submission"]["outcome"], "passed")
        self.assertEqual(rows["fork_submission_rejected"]["outcome"], "passed")

    def test_missing_runtime_proof_requires_zero_mutation_and_no_install(self) -> None:
        proof = {"zero_mutation": True, "copy_ready_requirement": True, "dependency_installed": False}
        self.assertTrue(e2e.LaunchHarness.driver_proof_valid("missing_runtime_zero_mutation", proof))
        self.assertFalse(e2e.LaunchHarness.driver_proof_valid("missing_runtime_zero_mutation", {**proof, "dependency_installed": True}))

    def test_dead_required_scenario_omission_is_rejected(self) -> None:
        config = json.loads(e2e.SCENARIOS.read_text())
        required = config["fault_scenarios"] + config["adapter_repair_faults"] + config["advanced_scenarios"] + config["acceptance_postconditions"] + config["journeys"] + ["shared_copilot_vscode_backend"]
        rows = [{"scenario": scenario} for scenario in required]
        e2e.validate_enforced_scenario_coverage(rows, config)
        with self.assertRaisesRegex(ValueError, "omitted or duplicated"):
            e2e.validate_enforced_scenario_coverage(rows[1:], config)

    def test_fixture_only_claim_escalation_is_rejected(self) -> None:
        evidence = self.fixture_harness().export()
        evidence["run"]["runtime_claims"] = True
        with self.assertRaisesRegex(ValueError, "cannot escalate"):
            e2e.assert_redacted(evidence)

    def test_hidden_yes_acceptance_or_mutation_fails_public_scenario(self) -> None:
        fake = '''#!/usr/bin/python3
import os, pathlib, sys
if sys.argv[1:] == ["--help"]:
    print("help")
    raise SystemExit(0)
path = pathlib.Path(os.environ["AGENTPLUGINS_HOME"]) / "state"
path.parent.mkdir(parents=True, exist_ok=True)
path.write_text("mutated")
print("accepted")
'''
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            binary = root / "agentplugins"
            binary.write_text(fake)
            binary.chmod(0o700)
            workspace = root / "workspace"
            workspace.mkdir()
            with mock.patch.dict(os.environ, {"HOME": str(root / "home"), "AGENTPLUGINS_HOME": str(root / "manager")}, clear=False):
                passed, value = observer.no_hidden_yes_scenario(binary, workspace, "a" * 64)
        self.assertFalse(passed)
        self.assertFalse(value["proof"]["manager_unchanged"])
        self.assertFalse(value["proof"]["unknown_option_reported"])

    def test_stale_public_pointer_is_rejected_against_caller_identity(self) -> None:
        latest = (PUBLICATION / "latest.json").read_bytes()
        with tempfile.TemporaryDirectory() as tmp, mock.patch.object(e2e, "bounded_https_get", return_value=latest):
            with self.assertRaisesRegex(ValueError, "exact caller publication identity"):
                e2e.fetch_production_directory(
                    Path(tmp) / "directory", expected_publication_id="fixture-1", expected_sequence=8,
                    expected_snapshot_digest="sha256:" + "b" * 64, expected_source_commit="d" * 40,
                )

    def test_production_n_minus_one_does_not_block_valid_staged_n(self) -> None:
        latest = json.loads((PUBLICATION / "latest.json").read_bytes())
        production_n_minus_one = e2e.canonical_json({
            **latest,
            "sequence": 6,
            "snapshot_path": "snapshots/00000000000000000006.json",
            "envelope_path": "snapshots/00000000000000000006.envelope.json",
        })
        digest = json.loads((PUBLICATION / "envelope-current.json").read_text())["snapshot_digest"]
        ledger_commit = "e" * 40
        staged_origin = f"https://raw.githubusercontent.com/{e2e.TRUSTED_CATALOG_REPOSITORY}/{ledger_commit}/registry/schemas/1/"
        staged_bodies = {
            staged_origin + "latest.json": (PUBLICATION / "latest.json").read_bytes(),
            staged_origin + latest["snapshot_path"]: (PUBLICATION / "snapshot.json").read_bytes(),
            staged_origin + latest["envelope_path"]: (PUBLICATION / "envelope-current.json").read_bytes(),
        }
        with tempfile.TemporaryDirectory() as tmp, mock.patch.object(
            e2e, "PRODUCTION_DIRECTORY_TRUST", PUBLICATION / "trusted-keys.json"
        ), mock.patch.object(e2e, "bounded_https_get", return_value=production_n_minus_one):
            with self.assertRaisesRegex(ValueError, "exact caller publication identity"):
                e2e.fetch_production_directory(
                    Path(tmp) / "production", expected_publication_id="fixture-1", expected_sequence=7,
                    expected_snapshot_digest=digest, expected_source_commit="d" * 40,
                )
            environment, snapshot, staged_digest = e2e.fetch_staged_directory(
                Path(tmp) / "staged", repository=e2e.TRUSTED_CATALOG_REPOSITORY,
                ledger_commit=ledger_commit, expected_publication_id="fixture-1",
                expected_sequence=7, expected_snapshot_digest=digest,
                expected_source_commit="d" * 40,
                fixture_fetch=lambda url, _maximum, _accept: staged_bodies[url],
            )
            self.assertEqual(snapshot["sequence"], 7)
            self.assertEqual(staged_digest, digest)
            self.assertEqual(environment["AGENTPLUGINS_DIRECTORY_ORIGIN"], staged_origin)
            with self.assertRaisesRegex(ValueError, "differs from the exact caller publication identity"):
                e2e.fetch_staged_directory(
                    Path(tmp) / "mismatched-staged", repository=e2e.TRUSTED_CATALOG_REPOSITORY,
                    ledger_commit=ledger_commit, expected_publication_id="wrong-publication",
                    expected_sequence=7, expected_snapshot_digest=digest,
                    expected_source_commit="d" * 40,
                    fixture_fetch=lambda url, _maximum, _accept: staged_bodies[url],
                )


if __name__ == "__main__":
    unittest.main()

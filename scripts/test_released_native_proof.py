"""Offline release/native identity boundaries; these are not vendor native passes."""
import argparse
import importlib.util
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch

SOURCE = Path(__file__).resolve().parents[1]
SPEC = importlib.util.spec_from_file_location("proof", SOURCE / "scripts/e2e/released-native-proof.py")
proof = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(proof)


class ReleasedNativeProofTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="TEST-release-native-")
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name).resolve()
        self.source = self.root / "source"
        self.source.mkdir()
        self.env = {**os.environ, "GITHUB_REPOSITORY": proof.REPOSITORY,
                    "GIT_CONFIG_NOSYSTEM": "1", "GIT_CONFIG_GLOBAL": os.devnull,
                    "GIT_AUTHOR_NAME": "iliya", "GIT_AUTHOR_EMAIL": "iliyazelenkog@gmail.com",
                    "GIT_COMMITTER_NAME": "iliya", "GIT_COMMITTER_EMAIL": "iliyazelenkog@gmail.com"}
        self.environment = patch.dict(os.environ, self.env, clear=True)
        self.environment.start()
        self.addCleanup(self.environment.stop)
        self.git("init")
        self.git("config", "core.autocrlf", "false")
        policy = self.source / "npm/agentplugins/scripts/release-assets.js"
        policy.parent.mkdir(parents=True)
        shutil.copyfile(SOURCE / "npm/agentplugins/scripts/release-assets.js", policy)
        (policy.parent.parent / "THIRD_PARTY_NOTICES.txt").write_text("synthetic notices\n")
        for client in ("hermes", "antigravity"):
            for target in proof.TARGETS:
                for name in proof.harness_files(client, target):
                    file = self.source / name
                    file.parent.mkdir(parents=True, exist_ok=True)
                    file.write_text("synthetic fixed harness for identity tests\n")
        self.git("add", ".")
        self.git("commit", "-m", "test: synthetic frozen source")
        commit = self.git("rev-parse", "HEAD")
        self.assets = self.root / "assets"
        self.assets.mkdir()
        for target in ("darwin-amd64", "darwin-arm64", "linux-amd64", "linux-arm64",
                       "windows-amd64", "windows-arm64"):
            name = "agentplugins_1.2.3_" + target.replace("-", "_")
            if target.startswith("windows"):
                name += ".exe"
            (self.assets / name).write_text("synthetic executable bytes " + target)
        subprocess.check_output(["node", str(policy), "prepare", str(self.assets),
                                 "agentplugins-v1.2.3", commit], env=self.env)
        self.args = argparse.Namespace(client="antigravity", target="linux-amd64",
                                       tag="agentplugins-v1.2.3", commit=commit,
                                       assets_artifact="agentplugins-draft-assets-" + commit,
                                       checksums_sha256=proof.sha(self.assets / "checksums.txt"),
                                       run_id=42, run_attempt=2, source=str(self.source),
                                       assets=str(self.assets), proofs=str(self.root / "proofs"))
        self.vendor = {"version_output": "agy 1.5.3", "executable_sha256": "a" * 64,
                       "runtime_sha256": "a" * 64}

    def git(self, *args):
        return subprocess.check_output(["git", "-C", str(self.source), *args], env=self.env,
                                       stderr=subprocess.PIPE).decode().strip()

    def receipt(self):
        return Path(self.args.proofs) / "receipt.json"

    def invoke(self, execute=None, version="agentplugins 1.2.3", vendor_samples=None):
        real_output = proof.output

        def output(command, env):
            if command == [str(self.assets / "agentplugins_1.2.3_linux_amd64"), "version"]:
                return version
            return real_output(command, env)

        def success(command, env, source, transcript):
            expected = ([sys.executable, str(self.source / "scripts/e2e/hermes-native.py"), "run",
                         str(self.source / "scripts/e2e/hermes-native-lifecycle.py")]
                        if self.args.client == "hermes" else
                        ["bash", str(self.source / "scripts/e2e/antigravity-native.sh")])
            self.assertEqual(command, expected)
            self.assertEqual(env["UAP_BIN"], str(self.assets / "agentplugins_1.2.3_linux_amd64"))
            self.assertEqual(source, str(self.source))
            self.assertFalse({"GH_TOKEN", "GITHUB_TOKEN", "NPM_TOKEN", "NODE_AUTH_TOKEN",
                              "NPM_CONFIG_USERCONFIG", "npm_config_registry"} & set(env))
            transcript.write_text("synthetic harness transcript\n")
            return 0

        with patch.object(proof, "output", side_effect=output), \
             patch.object(proof, "host_target", return_value="linux-amd64"), \
             patch.object(proof, "vendor_identity", side_effect=vendor_samples or
                          [dict(self.vendor), dict(self.vendor)]) as observe_vendor, \
             patch.object(proof, "execute", side_effect=execute or success):
            self.vendor_observer = observe_vendor
            proof.run(self.args)
        return observe_vendor.call_count

    def test_pass_only_receipt_uses_exact_binary_and_fixed_harness_without_credentials(self):
        with patch.dict(os.environ, {"GH_TOKEN": "secret", "GITHUB_TOKEN": "secret",
                                    "NPM_TOKEN": "secret", "NODE_AUTH_TOKEN": "secret",
                                    "NPM_CONFIG_USERCONFIG": "private", "npm_config_registry": "private"}):
            self.invoke()
        receipt = json.loads(self.receipt().read_text())
        self.assertEqual(receipt["status"], "pass")
        self.assertEqual(receipt["binary_sha256_before"], receipt["binary_sha256_after"])
        self.assertEqual(receipt["transcript_sha256"], proof.sha(self.receipt().with_name("transcript.log")))
        with self.assertRaisesRegex(ValueError, "prior pass receipt"):
            self.invoke()

    def test_hermes_uses_existing_fixed_lifecycle_and_mcp_driver(self):
        self.args.client = "hermes"
        self.vendor["version_output"] = "Hermes Agent v0.23.0"
        self.invoke()
        receipt = json.loads(self.receipt().read_text())
        self.assertEqual(receipt["vendor_pin"], proof.HERMES_PIN)
        self.assertEqual(set(receipt["harness"]), set(proof.harness_files("hermes", "linux-amd64")))

    def vendor_diagnosis(self):
        transcript = self.receipt().with_name("transcript.log")
        return json.loads(transcript.read_text().splitlines()[-1])

    def test_hermes_advisory_drift_retains_raw_samples_and_exact_hashes(self):
        self.args.client = "hermes"
        before = {**self.vendor, "version_output": "Hermes Agent v0.23.0 (2026.9.24)\nUp to date"}
        after = {**before, "version_output": "Hermes Agent v0.23.0 (2026.9.24)\nUpdate available"}
        self.assertEqual(self.invoke(vendor_samples=[before, after]), 2)
        self.assertIsNot(before, after)
        self.assertEqual(self.vendor_diagnosis(), {"vendor_before": before, "vendor_after": after,
                                                  "changed_fields": ["version_output"]})
        receipt = json.loads(self.receipt().read_text())
        self.assertEqual(receipt["vendor"], before)
        self.assertEqual(receipt["transcript_sha256"], proof.sha(self.receipt().with_name("transcript.log")))

    def test_vendor_hash_drift_is_rejected_with_or_without_advisory_drift(self):
        for client in ("hermes", "antigravity"):
            self.args.client = client
            before = dict(self.vendor)
            if client == "hermes":
                before["version_output"] = "Hermes Agent v0.23.0\nUp to date"
            for key in ("executable_sha256", "runtime_sha256"):
                for advisory in (False, True):
                    with self.subTest(client=client, key=key, advisory=advisory):
                        after = {**before, key: "b" * 64}
                        if advisory:
                            after["version_output"] += "\nUpdate available"
                        with self.assertRaisesRegex(ValueError, "native vendor changed.*" + key):
                            self.invoke(vendor_samples=[dict(before), after])
                        self.assertEqual(self.vendor_observer.call_count, 2)
                        expected = sorted([key] + (["version_output"] if advisory else []))
                        self.assertEqual(self.vendor_diagnosis(), {
                            "vendor_before": before, "vendor_after": after, "changed_fields": expected})
                        self.assertFalse(self.receipt().exists())

    def test_antigravity_raw_advisory_drift_stays_strict(self):
        before, after = dict(self.vendor), {**self.vendor, "version_output": "agy 1.5.3\nUpdate available"}
        with self.assertRaisesRegex(ValueError, "native vendor changed.*version_output"):
            self.invoke(vendor_samples=[before, after])
        self.assertEqual(self.vendor_observer.call_count, 2)
        self.assertEqual(self.vendor_diagnosis()["changed_fields"], ["version_output"])
        self.assertFalse(self.receipt().exists())

    def test_after_observation_error_retains_before_and_never_writes_pass(self):
        self.args.client = "hermes"
        before = {**self.vendor, "version_output": "Hermes Agent v0.23.0"}
        for message in ("native vendor version differs from pinned release", "Hermes source pin mismatch"):
            with self.subTest(message=message):
                with self.assertRaisesRegex(ValueError, message):
                    self.invoke(vendor_samples=[dict(before), ValueError(message)])
                self.assertEqual(self.vendor_observer.call_count, 2)
                self.assertEqual(self.vendor_diagnosis(), {"vendor_before": before,
                                                          "vendor_after_error": "ValueError: " + message})
                self.assertFalse(self.receipt().exists())

    def test_hermes_primary_version_validation_also_applies_to_aggregate(self):
        self.args.client = "hermes"
        self.vendor["version_output"] = "Hermes Agent v0.23.0 (2026.9.24) · stable · local 2bd378ed1b8d"
        paths = self.make_receipts()
        proof.aggregate(self.args)
        original = json.loads(paths[0].read_text())
        for version in ("Hermes Agent v0.23.1\nUpdate available: 0.23.0", "notice 0.23.0",
                        "Hermes Agent v0.23.00", "Hermes Agent v0.23.0.dev1", "Hermes Agent v0.23.0-rc1",
                        "notice\nHermes Agent v0.23.0", "Hermes Agent v0.23.0+local", "Hermes Agent v0.23.0fake"):
            with self.subTest(version=version):
                with self.assertRaisesRegex(ValueError, "vendor version"):
                    proof.pinned_vendor_version("hermes", version)
                after = {**self.vendor, "version_output": version}
                with self.assertRaisesRegex(ValueError, "vendor version"):
                    self.invoke(vendor_samples=[dict(self.vendor), after])
                self.assertEqual(self.vendor_observer.call_count, 2)
                self.assertEqual(self.vendor_diagnosis()["vendor_after"], after)
                self.assertFalse(self.receipt().exists())
                self.receipt().with_name("transcript.log").unlink()
                data = {**original, "vendor": {**self.vendor, "version_output": version}}
                paths[0].write_text(json.dumps(data))
                with self.assertRaisesRegex(ValueError, "vendor version"):
                    proof.aggregate(self.args)

    def test_wrong_actual_version_and_runtime_error_never_write_pass(self):
        with self.assertRaisesRegex(ValueError, "actual UAP version"):
            self.invoke(version="agentplugins 1.2.4")
        self.assertFalse(self.receipt().exists())
        for code in (1, 2, -9):
            with self.subTest(exit=code), self.assertRaisesRegex(ValueError, "native harness exited"):
                self.invoke(execute=lambda *args: code)
            self.assertFalse(self.receipt().exists())
        with self.assertRaises(subprocess.TimeoutExpired):
            self.invoke(execute=lambda *args: (_ for _ in ()).throw(subprocess.TimeoutExpired("fixture", 1)))
        self.assertFalse(self.receipt().exists())

    def test_changed_binary_during_harness_rejected(self):
        def mutate(command, env, source, transcript):
            Path(env["UAP_BIN"]).write_text("changed after version check")
            return 0
        with self.assertRaisesRegex(ValueError, "binary changed"):
            self.invoke(execute=mutate)
        self.assertFalse(self.receipt().exists())

    def test_wrong_hash_source_run_identity_host_or_incomplete_bundle_rejected(self):
        for name, value, message in (
            ("checksums_sha256", "f" * 64, "checksums digest"),
            ("commit", "b" * 40, "same-run release assets"),
            ("run_attempt", 0, "producer run identity"),
            ("assets_artifact", "other-run-assets", "same-run release assets"),
            ("target", "windows-amd64", "native host")):
            with self.subTest(field=name):
                original = getattr(self.args, name)
                setattr(self.args, name, value)
                with self.assertRaisesRegex(ValueError, message):
                    self.invoke()
                setattr(self.args, name, original)
                self.assertFalse(self.receipt().exists())
        original = (self.source / "scripts/e2e/antigravity-native.sh").read_text()
        (self.source / "scripts/e2e/antigravity-native.sh").write_text("changed harness")
        with self.assertRaisesRegex(ValueError, "checkout differs"):
            self.invoke()
        (self.source / "scripts/e2e/antigravity-native.sh").write_text(original)
        (self.assets / "THIRD_PARTY_NOTICES.txt").unlink()
        with self.assertRaisesRegex(ValueError, "nine-asset bundle"):
            self.invoke()

    def make_receipts(self):
        common, verified = proof.identity(self.args, proof.client_environment())
        receipts = []
        for target in proof.TARGETS:
            folder = Path(self.args.proofs) / target
            folder.mkdir(parents=True)
            transcript = folder / "transcript.log"
            transcript.write_text("synthetic " + target)
            data = proof.target_identity(common, verified, self.source, target)
            data.update(status="pass", exit_code=0, uap_version_output="agentplugins 1.2.3",
                        binary_sha256_before=data["asset"]["sha256"],
                        binary_sha256_after=data["asset"]["sha256"], vendor=self.vendor,
                        transcript_sha256=proof.sha(transcript))
            path = folder / "receipt.json"
            path.write_text(json.dumps(data))
            receipts.append(path)
        return receipts

    def test_exact_three_platform_set_passes(self):
        self.make_receipts()
        proof.aggregate(self.args)

    def test_absent_and_duplicate_platform_sets_rejected(self):
        with self.assertRaisesRegex(ValueError, "three native platform receipts"):
            proof.aggregate(self.args)
        paths = self.make_receipts()
        data = json.loads(paths[0].read_text())
        paths[1].write_text(json.dumps(data))
        paths[1].with_name("transcript.log").write_bytes(paths[0].with_name("transcript.log").read_bytes())
        with self.assertRaisesRegex(ValueError, "duplicate native target"):
            proof.aggregate(self.args)
        paths[2].unlink()
        with self.assertRaisesRegex(ValueError, "three native platform receipts"):
            proof.aggregate(self.args)

    def test_mixed_or_modified_receipt_identity_hash_version_and_transcript_rejected(self):
        paths = self.make_receipts()
        original = json.loads(paths[0].read_text())
        for key, value in (("client", "hermes"), ("producer_run_id", 43),
                           ("producer_run_attempt", 3), ("tag", "agentplugins-v1.2.4"),
                           ("source_commit", "f" * 40), ("checksums_sha256", "f" * 64),
                           ("manifest_sha256", "f" * 64), ("harness", {}),
                           ("asset", {**original["asset"], "size": 1}),
                           ("binary_sha256_after", "f" * 64),
                           ("uap_version_output", "agentplugins 1.2.4"),
                           ("exit_code", True), ("status", "failure"),
                           ("vendor", {**self.vendor, "version_output": "agy 1.5.4"})):
            with self.subTest(field=key):
                paths[0].write_text(json.dumps({**original, key: value}))
                with self.assertRaises(ValueError):
                    proof.aggregate(self.args)
        paths[0].write_text(json.dumps(original))
        paths[0].with_name("transcript.log").write_text("changed transcript")
        with self.assertRaisesRegex(ValueError, "exact binary and transcript"):
            proof.aggregate(self.args)

    def test_full_bundle_authority_rejects_modified_other_platform(self):
        self.make_receipts()
        (self.assets / "agentplugins_1.2.3_windows_arm64.exe").write_text("changed unexecuted asset")
        with self.assertRaises(subprocess.CalledProcessError):
            proof.aggregate(self.args)

    def test_vendor_version_and_runtime_pin_refusals(self):
        binary = self.root / "vendor"
        binary.write_text("synthetic vendor")
        env = {"AGY_BIN": str(binary)}
        with patch.object(proof, "output", return_value="agy 1.5.3"):
            self.assertEqual(proof.vendor_identity("antigravity", env)["runtime_sha256"], proof.sha(binary))
        for version in ("agy 1.5.30", "agy 1.5.4", "no version"):
            with patch.object(proof, "output", return_value=version), \
                 self.assertRaisesRegex(ValueError, "vendor version"):
                proof.vendor_identity("antigravity", env)
        env = {"HERMES_BIN": str(binary), "RUNNER_TEMP": str(self.root)}
        with patch.object(proof, "output", side_effect=["Hermes Agent v0.23.0", "wrong-pin"]), \
             self.assertRaisesRegex(ValueError, "Hermes source pin"):
            proof.vendor_identity("hermes", env)
        runtime_file = self.root / "runtime-command.json"
        env["HERMES_NATIVE_RUNTIME_COMMAND_FILE"] = str(runtime_file)
        runtime_file.write_text(json.dumps([str(binary), "-I", "-c", "pass"]))
        with patch.object(proof, "output", side_effect=["Hermes Agent v0.23.0", proof.HERMES_PIN]):
            self.assertEqual(proof.vendor_identity("hermes", env)["runtime_sha256"], proof.sha(binary))
        runtime_file.write_text(json.dumps([str(binary), "unisolated-runtime"]))
        with patch.object(proof, "output", side_effect=["Hermes Agent v0.23.0", proof.HERMES_PIN]), \
             self.assertRaisesRegex(ValueError, "runtime command is invalid"):
            proof.vendor_identity("hermes", env)


if __name__ == "__main__":
    unittest.main()

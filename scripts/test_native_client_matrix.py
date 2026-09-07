"""Bootstrap security tests. Never downloads or executes a native client."""
import base64
import hashlib
import importlib.util
import io
import json
from pathlib import Path
import tarfile
import tempfile
import sys
import unittest
from unittest.mock import patch
import zipfile

SPEC = importlib.util.spec_from_file_location("native_matrix", Path(__file__).with_name("run-native-client-matrix.py"))
matrix = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(matrix)


class NativeMatrixTests(unittest.TestCase):
    def test_release_identity_rejected_before_network(self):
        for tag, commit, repo in [("latest", "a" * 40, "777genius/universal-agent-plugins"), ("agentplugins-v1.2.3", "main", "777genius/universal-agent-plugins"), ("agentplugins-v1.2.3", "a" * 40, "other/repo"), ("agentplugins-v1.2.3", "a" * 40, "777genius/plugin-kit-ai")]:
            with patch.object(matrix.subprocess, "check_output") as network:
                with self.assertRaises(ValueError):
                    matrix.provision_release(Path("."), Path("."), "darwin-arm64", tag, commit, repo)
                network.assert_not_called()

    def test_release_requires_public_stable_and_exact_producer_commit(self):
        good = {"tag_name": "agentplugins-v1.2.3", "draft": False, "prerelease": False}
        for responses in [[dict(good, draft=True)], [dict(good, prerelease=True)], [good, {"sha": "b" * 40}]]:
            with patch.object(matrix.subprocess, "check_output", side_effect=[json.dumps(r) for r in responses]), patch.object(matrix.subprocess, "run") as download:
                with self.assertRaises(ValueError):
                    matrix.provision_release(Path("."), Path("."), "darwin-arm64", good["tag_name"], "a" * 40, "777genius/universal-agent-plugins")
                download.assert_not_called()

    def test_released_installer_requires_all_three_attestations(self):
        with tempfile.TemporaryDirectory() as temp:
            directory = Path(temp)
            asset = "agentplugins_1.2.3_darwin_arm64"
            def download(*args, **kwargs):
                (directory / "release-assets" / asset).write_bytes(b"installer")
                (directory / "release-assets/checksums.txt").write_bytes(b"checksums")
            responses = [
                {"tag_name": "agentplugins-v1.2.3", "draft": False, "prerelease": False},
                {"sha": "a" * 40, "commit": {"tree": {"sha": "b" * 40}}},
                {"version": "1.2.3", "manifest_sha256": "c" * 64, "assets": {"darwin-arm64": {"file": asset, "sha256": "d" * 64, "size": 9}}}, [], [], []]
            # Simulate the Windows ANSI locale while real child processes emit
            # UTF-8 JSON. Cyrillic я contains 0x8f, undefined in cp1252.
            payloads = iter(json.dumps({**r, "unicode_note": "я 😀"} if isinstance(r, dict) else r,
                                       ensure_ascii=False).encode("utf-8") for r in responses)
            real_run = matrix.subprocess.run
            def child_output(command, **kwargs):
                payload = next(payloads)
                return real_run([sys.executable, "-c", "import sys; sys.stdout.buffer.write(" + repr(payload) + ")"],
                                stdout=matrix.subprocess.PIPE, check=True, **kwargs).stdout
            with patch.object(matrix.subprocess, "_text_encoding", return_value="cp1252"), patch.object(matrix.subprocess, "check_output", side_effect=child_output) as check, patch.object(matrix.subprocess, "run", side_effect=download):
                installer, evidence = matrix.provision_release(Path("."), directory, "darwin-arm64", "agentplugins-v1.2.3", "a" * 40, "777genius/universal-agent-plugins")
            self.assertEqual(installer.read_bytes(), b"installer")
            self.assertEqual(evidence["commit"], "a" * 40)
            self.assertEqual(evidence["tree"], "b" * 40)
            for call in check.call_args_list[-3:]:
                command = call.args[0]
                self.assertIn("--deny-self-hosted-runners", command)
                self.assertEqual(command[command.index("--source-digest") + 1], "a" * 40)
                self.assertIn("github.com/777genius/universal-agent-plugins/.github/workflows/agentplugins-release.yml", command)
            self.assertEqual(set(evidence["attestations"]), {asset, "checksums.txt", "release-manifest.json"})

    def test_release_json_invalid_utf8_fails_before_download(self):
        real_run = matrix.subprocess.run
        def child_output(command, **kwargs):
            return real_run([sys.executable, "-c", "import sys; sys.stdout.buffer.write(b'\\xff')"],
                            stdout=matrix.subprocess.PIPE, check=True, **kwargs).stdout
        with patch.object(matrix.subprocess, "_text_encoding", return_value="cp1252"), patch.object(matrix.subprocess, "check_output", side_effect=child_output), patch.object(matrix.subprocess, "run") as download:
            with self.assertRaises(UnicodeDecodeError):
                matrix.provision_release(Path("."), Path("."), "darwin-arm64", "agentplugins-v1.2.3", "a" * 40, "777genius/universal-agent-plugins")
            download.assert_not_called()

    def test_git_bash_discovery_supports_runner_git_layouts(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp).resolve()
            (root / "bin").mkdir()
            bash = root / "bin/bash.exe"
            bash.write_bytes(b"fixture")
            for relative in ["bin/git.exe", "cmd/git.exe", "mingw64/bin/git.exe"]:
                self.assertEqual(matrix.find_git_bash(root / relative), bash)
            self.assertIsNone(matrix.find_git_bash(root / "unrelated/deep/tree/git.exe"))

    def test_extended_runtime_is_required_for_opencode_proof(self):
        self.assertIn("TestAgentpluginsOpenCodeNativeRuntimeExtended", matrix.REQUIRED_TESTS["opencode"])

    def test_runtime_rejects_ambient_and_self_hosted_machine(self):
        for env in [{}, {"GITHUB_ACTIONS": "true", "RUNNER_ENVIRONMENT": "self-hosted", "AGENTPLUGINS_NATIVE_DISPOSABLE_HOSTED": "1"}]:
            with patch.dict(matrix.os.environ, env, clear=True):
                with self.assertRaises(RuntimeError):
                    matrix.require_hosted("darwin-arm64")

    def test_target_must_be_native(self):
        env = {"GITHUB_ACTIONS": "true", "RUNNER_ENVIRONMENT": "github-hosted", "AGENTPLUGINS_NATIVE_DISPOSABLE_HOSTED": "1"}
        with patch.dict(matrix.os.environ, env, clear=True), patch.object(matrix.platform, "system", return_value="Darwin"), patch.object(matrix.platform, "machine", return_value="arm64"):
            matrix.require_hosted("darwin-arm64")
            with self.assertRaises(RuntimeError):
                matrix.require_hosted("windows-amd64")

    def test_integrity_checks_exact_archive(self):
        body = b"native-client-fixture"
        for pin in ["sha256:" + hashlib.sha256(body).hexdigest(), "sha512-" + base64.b64encode(hashlib.sha512(body).digest()).decode()]:
            matrix.verify_digest(body, pin)
            with self.assertRaises(ValueError):
                matrix.verify_digest(body + b"changed", pin)

    def test_tar_selects_only_regular_exact_binary(self):
        archive = io.BytesIO()
        with tarfile.open(fileobj=archive, mode="w:gz") as out:
            link = tarfile.TarInfo("client")
            link.type = tarfile.SYMTYPE
            link.linkname = "/etc/passwd"
            out.addfile(link)
        with self.assertRaises(ValueError):
            matrix.extract_binary(archive.getvalue(), "client.tgz", "client")

    def test_duplicate_binaries_rejected(self):
        archive = io.BytesIO()
        with zipfile.ZipFile(archive, "w") as out:
            out.writestr("one/client.exe", b"one")
            out.writestr("two/client.exe", b"two")
        with self.assertRaises(ValueError):
            matrix.extract_binary(archive.getvalue(), "client.zip", "client.exe")

    def test_archive_paths_never_materialized(self):
        archive = io.BytesIO()
        with zipfile.ZipFile(archive, "w") as out:
            out.writestr("../../client.exe", b"binary")
            out.writestr("postinstall.sh", b"must never run")
        self.assertEqual(matrix.extract_binary(archive.getvalue(), "client.zip", "client.exe"), b"binary")


if __name__ == "__main__":
    unittest.main()

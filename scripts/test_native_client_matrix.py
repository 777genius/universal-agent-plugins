"""Bootstrap security tests. Never downloads or executes a native client."""
import base64
import hashlib
import importlib.util
import io
from pathlib import Path
import tarfile
import unittest
from unittest.mock import patch
import zipfile

SPEC = importlib.util.spec_from_file_location("native_matrix", Path(__file__).with_name("run-native-client-matrix.py"))
matrix = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(matrix)


class NativeMatrixTests(unittest.TestCase):
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

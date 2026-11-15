"""Inert fixture preparation/preservation checks; no native client execution."""
import os
from pathlib import Path
import runpy
import stat
import sys
import tempfile
import unittest
from unittest.mock import patch


runner = runpy.run_path(str(Path(__file__).parent / "e2e/grok-enable-readonly.py"))


@unittest.skipUnless(sys.platform == "linux", "Linux exclusive fixture-lock contract")
class ReadonlyFixtureTests(unittest.TestCase):
    def test_lock_is_exclusively_created_before_complete_snapshot(self):
        with tempfile.TemporaryDirectory() as temporary:
            config = Path(temporary) / "config.toml"
            config.write_bytes(b"inert config")
            runner["prepare_writer_lock"](config)
            lock = config.with_name("config.toml.agentplugins.lock")
            before = lock.lstat()
            self.assertTrue(stat.S_ISREG(before.st_mode))
            self.assertEqual((stat.S_IMODE(before.st_mode), before.st_uid, before.st_gid,
                              before.st_nlink, before.st_size), (0o600, os.getuid(), os.getgid(), 1, 0))
            snapshot = runner["snapshot"](config.parent)
            with self.assertRaises(FileExistsError):
                runner["prepare_writer_lock"](config)
            self.assertEqual(snapshot, runner["snapshot"](config.parent))
            self.assertEqual(before.st_ino, lock.lstat().st_ino)

    def test_preexisting_regular_or_symlink_lock_is_never_overwritten(self):
        for symlink in (False, True):
            with self.subTest(symlink=symlink), tempfile.TemporaryDirectory() as temporary:
                root = Path(temporary)
                config, lock, foreign = root / "config.toml", root / "config.toml.agentplugins.lock", root / "foreign"
                foreign.write_bytes(b"foreign sentinel")
                if symlink:
                    lock.symlink_to(foreign)
                else:
                    lock.write_bytes(b"existing lock")
                before = lock.lstat()
                with self.assertRaises(FileExistsError):
                    runner["prepare_writer_lock"](config)
                self.assertEqual((before.st_ino, before.st_mode), (lock.lstat().st_ino, lock.lstat().st_mode))
                self.assertEqual(foreign.read_bytes(), b"foreign sentinel")
                if not symlink:
                    self.assertEqual(lock.read_bytes(), b"existing lock")

    def test_same_metadata_named_replacement_refuses_original_identity(self):
        with tempfile.TemporaryDirectory() as temporary:
            config = Path(temporary) / "config.toml"
            lock = config.with_name("config.toml.agentplugins.lock")
            original_stat, replaced = os.stat, []

            def substitute(path, **kwargs):
                if path == lock.name and kwargs.get("dir_fd") is not None and not replaced:
                    replaced.append(original_stat(path, **kwargs).st_ino)
                    lock.rename(lock.with_name("retained-original-lock"))
                    lock.write_bytes(b"")
                    lock.chmod(0o600)
                return original_stat(path, **kwargs)

            with patch("os.stat", side_effect=substitute):
                with self.assertRaisesRegex(ValueError, "unsafe metadata or identity"):
                    runner["prepare_writer_lock"](config)
            self.assertNotEqual(replaced[0], lock.lstat().st_ino)


    def test_temporary_fixture_cleanup_error_cannot_report_success(self):
        # Fully inert success-shaped run with a deliberate post-body cleanup error.
        # The previous launcher wrote passed:true/returned 0 before __exit__ ran.
        import contextlib
        import gzip
        import hashlib
        import io
        import json
        from types import SimpleNamespace

        binary = b"inert test executable"
        archive = gzip.compress(binary, mtime=0)
        real_tempdir = tempfile.TemporaryDirectory

        class CleanupFailure(real_tempdir):
            def __exit__(self, exc_type, exc, tb):
                super().__exit__(exc_type, exc, tb)
                if exc_type is None:
                    raise OSError("simulated fixture cleanup failure")

        calls = []

        def fake_run(argv, **kwargs):
            calls.append(list(argv))
            if len(calls) == 1:
                output = b"grok 1.0.70 (f50904eaee6c)\n"
            elif len(calls) == 2:
                output = b""
            elif len(calls) == 3:
                lock = Path(kwargs["env"]["HOME"]) / ".grok/.config-init.lock"
                lock.write_bytes(b"")
                lock.chmod(0o600)
                output = b""
            elif len(calls) == 4:
                output = b"GROK_READONLY_NATIVE_INVENTORY_PASS\n"
            elif len(calls) == 5:
                output = b"GROK_PUBLIC_NATIVE_SAVED_LIFECYCLE_PASS\n"
            else:
                raise AssertionError("unexpected process call")
            return SimpleNamespace(returncode=0, stdout=output, stderr=b"")

        with real_tempdir() as temporary:
            output_dir = Path(temporary) / "evidence"
            arguments = ["grok-enable-readonly.py", "--test", str(Path(temporary) / "inert-helper"),
                         "--public-test", str(Path(temporary) / "inert-public-helper"),
                         "--output", str(output_dir)]
            constants = {"ARCHIVE": hashlib.sha256(archive).hexdigest(),
                         "BINARY": hashlib.sha256(binary).hexdigest()}
            with patch.dict(runner["main"].__globals__, constants), \
                 patch("urllib.request.urlopen", return_value=io.BytesIO(archive)), \
                 patch("subprocess.run", side_effect=fake_run), \
                 patch("tempfile.TemporaryDirectory", CleanupFailure), \
                 patch.object(sys, "argv", arguments), \
                 contextlib.redirect_stdout(io.StringIO()):
                status = runner["main"]()

            evidence = json.loads((output_dir / "evidence.json").read_text())
            self.assertEqual(len(calls), 5)
            self.assertIn("-test.run=^TestProjectGrokNativeSavedEnablementLifecycle$", calls[4])
            self.assertTrue(evidence["public_native_saved_lifecycle_passed"])
            self.assertEqual(status, 1)
            self.assertFalse(evidence["passed"])
            self.assertNotIn("fixture_unchanged", evidence)
            self.assertNotIn("archive_sha256", evidence)
            self.assertNotIn("binary_sha256", evidence)
            self.assertIn("simulated fixture cleanup failure", evidence["error"])

if __name__ == "__main__":
    unittest.main()

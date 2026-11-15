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


if __name__ == "__main__":
    unittest.main()

"""Platform admission tests; no vendor process or platform success claim."""
import ctypes
import os
from pathlib import Path
import sys
import tempfile
from types import SimpleNamespace
import unittest
from unittest.mock import Mock, patch
import zipfile
import json

import test_native_client_project_artifacts as base_tests

artifacts = base_tests.artifacts


def win_info(info, held=False, **changes):
    values = {name: getattr(info, name) for name in (
        "st_dev", "st_ino", "st_mode", "st_nlink", "st_size", "st_mtime_ns", "st_ctime_ns")}
    values.update(st_birthtime_ns=1234567, st_file_attributes=0, st_reparse_tag=0)
    if held:
        values["st_ctime_ns"] += 100000
    values.update(changes)
    return SimpleNamespace(**values)


class PlatformIdentityTests(unittest.TestCase):
    fixture = base_tests.ProjectEnvelopeTests.fixture

    def test_separate_path_and_held_ctime_domains_still_require_stability(self):
        with tempfile.TemporaryDirectory() as temp:
            source = self.fixture(Path(temp).resolve())
            real_lstat, real_fstat, real_open = Path.lstat, os.fstat, artifacts.open_member
            path_stat = lambda path: win_info(real_lstat(path))
            held_stat = lambda fd: win_info(real_fstat(fd), held=True)
            with patch.object(artifacts, "WINDOWS", True), patch.object(Path, "lstat", path_stat), patch.object(artifacts.os, "fstat", held_stat), patch.object(artifacts, "open_member", real_open):
                # open_member sees WINDOWS at call time; use the explicit POSIX
                # opener for this in-memory Windows metadata simulation only.
                with patch.object(artifacts, "open_member", lambda p: os.open(p, os.O_RDONLY)):
                    bodies, packet = artifacts.read_packet(source)
                    self.assertIn("one.stdout.log", bodies)
                    artifacts.confirm_paths(packet)
                    first = packet[2][0][0]
                    def changed_path(path):
                        value = path_stat(path)
                        if path == first:
                            value.st_ctime_ns += 1
                        return value
                    with patch.object(Path, "lstat", changed_path):
                        with self.assertRaisesRegex(ValueError, "member replaced"):
                            artifacts.confirm_paths(packet)
            counter = {}
            def changed_held(fd):
                value = held_stat(fd)
                counter[fd] = counter.get(fd, 0) + 1
                if counter[fd] % 2 == 0:
                    value.st_ctime_ns += 1
                return value
            with patch.object(artifacts, "WINDOWS", True), patch.object(Path, "lstat", path_stat), patch.object(artifacts.os, "fstat", changed_held), patch.object(artifacts, "open_member", lambda p: os.open(p, os.O_RDONLY)):
                with self.assertRaisesRegex(ValueError, "held identity changed"):
                    artifacts.read_packet(source)

    def test_positive_cross_identity_fields_and_reparse_refusal(self):
        with tempfile.TemporaryDirectory() as temp:
            info = win_info(Path(temp).stat())
            with patch.object(artifacts, "WINDOWS", True):
                held = SimpleNamespace(**vars(info))
                held.st_ctime_ns += 100
                self.assertTrue(artifacts.same_opened(info, held))
                for field in ("st_dev", "st_ino", "st_mode", "st_nlink", "st_size", "st_mtime_ns", "st_birthtime_ns", "st_file_attributes"):
                    with self.subTest(field=field):
                        other = SimpleNamespace(**vars(held))
                        setattr(other, field, getattr(other, field) + 1)
                        self.assertFalse(artifacts.same_opened(info, other))
                for changes in ({"st_dev": 0}, {"st_ino": 0}, {"st_birthtime_ns": None}, {"st_file_attributes": 0x400}, {"st_reparse_tag": 1}):
                    with self.subTest(changes=changes), self.assertRaises(ValueError):
                        artifacts.windows_identity(SimpleNamespace(**(vars(info) | changes)))

    def test_root_member_and_held_reparse_are_refused(self):
        for where in ("root", "member", "held"):
            with self.subTest(where=where), tempfile.TemporaryDirectory() as temp:
                source = self.fixture(Path(temp).resolve())
                real_lstat, real_fstat = Path.lstat, os.fstat
                opened = []
                def path_stat(path):
                    value = win_info(real_lstat(path))
                    if (where == "root" and path == source) or (where == "member" and path.name == "one.stdout.log"):
                        value.st_file_attributes = 0x400
                    return value
                def held_stat(fd):
                    return win_info(real_fstat(fd), held=True, st_reparse_tag=1 if where == "held" else 0)
                def open_file(path):
                    descriptor = os.open(path, os.O_RDONLY)
                    opened.append(descriptor)
                    return descriptor
                real_close = os.close
                closed = []
                def close_file(fd):
                    closed.append(fd)
                    real_close(fd)
                with patch.object(artifacts, "WINDOWS", True), patch.object(Path, "lstat", path_stat), patch.object(artifacts.os, "fstat", held_stat), patch.object(artifacts, "open_member", open_file), patch.object(artifacts.os, "close", close_file):
                    with self.assertRaises(ValueError):
                        artifacts.read_packet(source)
                self.assertEqual(opened, closed)
                if where == "root":
                    self.assertEqual(opened, [])

    def test_binary_bytes_and_post_zip_path_recheck(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp).resolve()
            source = self.fixture(root)
            binary = b"CR\r\nLF\x1a\x00"
            # Preserve the exact command receipt when replacing fixture bytes.
            (source / "one.stdout.log").write_bytes(binary)
            summary_path = source / "opencode-project-evidence.json"
            summary = json.loads(summary_path.read_text())
            import hashlib
            summary["commands"][0]["stdout_bytes"] = len(binary)
            summary["commands"][0]["stdout_sha256"] = hashlib.sha256(binary).hexdigest()
            summary_path.write_text(json.dumps(summary))
            destination = root / "admitted.zip"
            self.assertTrue(artifacts.write_archive(source, destination))
            with zipfile.ZipFile(destination) as archive:
                self.assertEqual(archive.read("one.stdout.log"), binary)
            real_write = artifacts.write_zip
            def changed_after_zip(pending, bodies):
                real_write(pending, bodies)
                (source / "one.stdout.log").write_bytes(b"replaced after capture")
            with patch.object(artifacts, "write_zip", changed_after_zip):
                self.assertFalse(artifacts.write_archive(source, destination))
            with zipfile.ZipFile(destination) as archive:
                self.assertEqual(archive.namelist(), ["opencode-project-evidence.json"])
                failure = json.loads(archive.read(archive.namelist()[0]))
                self.assertFalse(failure["success"])
                self.assertIn("version", failure["artifact_python_runtime"])
                self.assertIn("member replaced", failure["artifact_admission_error"])

    def test_typed_windows_open_binary_transfer_and_exact_close_ownership(self):
        create, close = Mock(return_value=123), Mock(return_value=1)
        kernel = SimpleNamespace(CreateFileW=create, CloseHandle=close)
        transfer = Mock(return_value=17)
        msvcrt = SimpleNamespace(open_osfhandle=transfer)
        def win_error(code):
            return OSError(code, "inert Windows error")
        with patch.object(artifacts, "WINDOWS", True), patch.object(ctypes, "WinDLL", return_value=kernel, create=True), patch.object(ctypes, "WinError", side_effect=win_error, create=True), patch.object(ctypes, "get_last_error", return_value=5, create=True), patch.dict(sys.modules, {"msvcrt": msvcrt}), patch.object(os, "O_BINARY", 0x8000, create=True):
            self.assertEqual(artifacts.open_member(Path("inert.json")), 17)
            self.assertIs(create.restype, ctypes.c_void_p)
            self.assertEqual(create.call_args.args, ("inert.json", 0x80000000, 7, None, 3, 0x00200000, None))
            transfer.assert_called_once_with(123, os.O_RDONLY | 0x8000)
            close.assert_not_called()
            for error in (OSError("transfer failed"), None):
                transfer.side_effect, transfer.return_value = error, -1
                close.reset_mock()
                with self.assertRaises(OSError):
                    artifacts.open_member(Path("inert.json"))
                close.assert_called_once_with(123)
            close.return_value = 0
            with self.assertRaises(OSError) as raised:
                artifacts.open_member(Path("inert.json"))
            self.assertEqual(raised.exception.errno, 5)
            close.return_value = 1
            create.return_value = ctypes.c_void_p(-1).value
            close.reset_mock()
            transfer.reset_mock()
            with self.assertRaises(OSError):
                artifacts.open_member(Path("inert.json"))
            close.assert_not_called()
            transfer.assert_not_called()


if __name__ == "__main__":
    unittest.main()

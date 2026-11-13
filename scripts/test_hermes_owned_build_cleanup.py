"""Actual owned local inode tests; no vendor execution."""
import os
import io
import json
from contextlib import redirect_stdout
from pathlib import Path
import tempfile
import time
import types
import unittest
from unittest import mock

import hermes_owned_build_cleanup as cleanup


class OwnedBuildCleanupTests(unittest.TestCase):
    def fixture(self, temporary):
        base = Path(temporary).resolve()
        roots = {label: base / label for label in ("payload", "work", "cache")}
        for root in roots.values():
            root.mkdir(mode=0o700)
        return roots

    def test_each_owned_domain_has_its_own_finite_byte_budget(self):
        with tempfile.TemporaryDirectory() as temporary:
            roots = self.fixture(temporary)
            for root in roots.values():
                (root / "member").write_bytes(b"123456")
            owned = cleanup.OwnedRoots(roots)
            try:
                output = io.StringIO()
                with mock.patch.object(cleanup, "BYTES", 8), mock.patch.object(cleanup, "UNION_BYTES", 24), redirect_stdout(output):
                    with mock.patch.object(os, "read", side_effect=AssertionError("regular body read")):
                        records = cleanup.census(owned, time.monotonic() + 5)
                fact = json.loads(output.getvalue().removeprefix("HERMES_BUILD_CENSUS="))
                self.assertEqual(len(records), 6)
                self.assertEqual(fact["regular_bytes"], 18)
                self.assertEqual(fact["status"], "complete")
                self.assertEqual({r["regular_bytes"] for r in fact["domains"].values()}, {6})
                self.assertEqual(cleanup.API["PAYLOAD_BYTES"], 8 << 30)
            finally:
                owned.close()

    def test_domain_or_union_overflow_refuses_before_any_unlink(self):
        for domain_overflow in (False, True):
            with self.subTest(domain_overflow=domain_overflow), tempfile.TemporaryDirectory() as temporary:
                roots = self.fixture(temporary)
                for label, root in roots.items():
                    (root / "member").write_bytes(b"123456789" if label == "cache" and domain_overflow else b"123456")
                owned = cleanup.OwnedRoots(roots)
                try:
                    output = io.StringIO()
                    with mock.patch.object(cleanup, "BYTES", 8), mock.patch.object(cleanup, "UNION_BYTES", 24 if domain_overflow else 12), redirect_stdout(output):
                        with mock.patch.object(os, "unlink", side_effect=AssertionError("unlink before full union")), mock.patch.object(os, "read", side_effect=AssertionError("regular body read")):
                            with self.assertRaisesRegex(RuntimeError, "owned build byte bound"):
                                cleanup.detach(owned, time.monotonic() + 5)
                    fact = json.loads(output.getvalue().removeprefix("HERMES_BUILD_CENSUS="))
                    self.assertEqual((fact["status"], fact["role"]), ("refused", "cache"))
                    self.assertEqual(set(fact["domains"]), {"payload", "work", "cache"})
                    self.assertNotIn(str(roots["cache"]), output.getvalue())
                    self.assertTrue(all((root / "member").exists() for root in roots.values()))
                finally:
                    owned.close()

    def test_domain_membership_bound_cannot_borrow_sibling_budget(self):
        with tempfile.TemporaryDirectory() as temporary:
            roots = self.fixture(temporary)
            (roots["payload"] / "member").write_bytes(b"owned")
            owned = cleanup.OwnedRoots(roots)
            try:
                with mock.patch.object(cleanup, "DOMAIN_MEMBERS", 1), redirect_stdout(io.StringIO()):
                    with self.assertRaisesRegex(RuntimeError, "owned build census bound"):
                        cleanup.census(owned, time.monotonic() + 5)
            finally:
                owned.close()

    def test_complete_alias_union_detaches_only_external_names(self):
        with tempfile.TemporaryDirectory() as temporary:
            roots = self.fixture(temporary)
            original = roots["payload"] / "member"
            original.write_bytes(b"immutable complete payload")
            for path in (roots["payload"] / "internal", roots["cache"] / "copy", roots["work"] / "copy"):
                os.link(original, path)
            (roots["cache"] / "foreign-control").write_bytes(b"retained")
            owned = cleanup.OwnedRoots(roots)
            try:
                self.assertEqual(cleanup.detach(owned, time.monotonic() + 5), 2)
                self.assertEqual(original.read_bytes(), b"immutable complete payload")
                self.assertEqual(original.stat().st_nlink, 2)
                self.assertTrue((roots["payload"] / "internal").exists())
                self.assertEqual((roots["cache"] / "foreign-control").read_bytes(), b"retained")
            finally:
                owned.close()

    def test_unknown_external_alias_refuses_before_any_unlink(self):
        with tempfile.TemporaryDirectory() as temporary:
            roots = self.fixture(temporary)
            member = roots["payload"] / "member"
            member.write_bytes(b"payload")
            os.link(member, roots["cache"] / "known")
            os.link(member, Path(temporary) / "outside")
            owned = cleanup.OwnedRoots(roots)
            try:
                with mock.patch.object(os, "unlink", side_effect=AssertionError("unexpected unlink")):
                    with self.assertRaisesRegex(RuntimeError, "unknown external alias"):
                        cleanup.detach(owned, time.monotonic() + 5)
            finally:
                owned.close()

    def test_new_alias_after_census_refuses_before_unlink(self):
        with tempfile.TemporaryDirectory() as temporary:
            roots = self.fixture(temporary)
            member = roots["payload"] / "member"
            member.write_bytes(b"payload")
            os.link(member, roots["cache"] / "known")
            owned = cleanup.OwnedRoots(roots)
            original = cleanup.alias_plan
            def changed(records):
                selected = original(records)
                os.link(member, Path(temporary) / "late-outside")
                return selected
            try:
                with mock.patch.object(cleanup, "alias_plan", side_effect=changed), mock.patch.object(os, "unlink", side_effect=AssertionError("unlink")):
                    with self.assertRaisesRegex(RuntimeError, "delete member changed"):
                        cleanup.detach(owned, time.monotonic() + 5)
            finally:
                owned.close()

    def test_shared_deadline_stops_between_domains_without_unlink(self):
        with tempfile.TemporaryDirectory() as temporary:
            roots = self.fixture(temporary)
            owned = cleanup.OwnedRoots(roots)
            try:
                with mock.patch.object(cleanup.time, "monotonic", side_effect=[1, 9]), mock.patch.object(os, "unlink", side_effect=AssertionError("unlink")):
                    with self.assertRaisesRegex(RuntimeError, "deadline"):
                        cleanup.detach(owned, 5)
            finally:
                owned.close()

    def test_byte_bound_refuses_before_read_or_unlink(self):
        with tempfile.TemporaryDirectory() as temporary:
            roots = self.fixture(temporary)
            (roots["payload"] / "large").write_bytes(b"12345")
            owned = cleanup.OwnedRoots(roots)
            try:
                with mock.patch.object(cleanup, "BYTES", 4), mock.patch.object(os, "read", side_effect=AssertionError("read")):
                    with self.assertRaisesRegex(RuntimeError, "byte bound"):
                        cleanup.detach(owned, time.monotonic() + 5)
            finally:
                owned.close()

    def test_replacement_after_census_preserves_new_object(self):
        with tempfile.TemporaryDirectory() as temporary:
            roots = self.fixture(temporary)
            member = roots["payload"] / "member"
            member.write_bytes(b"payload")
            target = roots["cache"] / "known"
            os.link(member, target)
            owned = cleanup.OwnedRoots(roots)
            original = cleanup.alias_plan
            def swapped(records):
                plan = original(records)
                target.unlink()
                target.write_bytes(b"foreign replacement")
                return plan
            try:
                with mock.patch.object(cleanup, "alias_plan", side_effect=swapped):
                    with self.assertRaisesRegex(RuntimeError, "delete member changed"):
                        cleanup.detach(owned, time.monotonic() + 5)
                self.assertEqual(target.read_bytes(), b"foreign replacement")
            finally:
                owned.close()

    @unittest.skipIf(os.name == "nt", "POSIX moved held-directory regression")
    def test_moved_descendant_parent_refuses_before_unlink(self):
        with tempfile.TemporaryDirectory() as temporary:
            roots = self.fixture(temporary)
            member = roots["payload"] / "member"
            member.write_bytes(b"payload")
            sub = roots["cache"] / "sub"
            sub.mkdir()
            os.link(member, sub / "alias")
            outside = Path(temporary) / "moved-outside"
            owned = cleanup.OwnedRoots(roots)
            original_open = os.open
            def swapped(path, flags, *args, **kwargs):
                if path == "alias" and "dir_fd" in kwargs:
                    sub.rename(outside)
                    sub.mkdir()
                return original_open(path, flags, *args, **kwargs)
            try:
                with mock.patch.object(os, "open", side_effect=swapped):
                    with self.assertRaisesRegex(RuntimeError, "ancestor link changed"):
                        cleanup.detach(owned, time.monotonic() + 5)
                self.assertEqual((outside / "alias").read_bytes(), b"payload")
                self.assertEqual(member.stat().st_nlink, 2)
            finally:
                owned.close()

    @unittest.skipIf(os.name == "nt", "POSIX observed post-unlink ancestor drift")
    def test_parent_move_after_unlink_cannot_report_success(self):
        with tempfile.TemporaryDirectory() as temporary:
            roots = self.fixture(temporary)
            member = roots["payload"] / "member"
            member.write_bytes(b"payload")
            sub = roots["cache"] / "sub"
            sub.mkdir()
            os.link(member, sub / "alias")
            owned = cleanup.OwnedRoots(roots)
            original_unlink = os.unlink
            def moved(path, *args, **kwargs):
                result = original_unlink(path, *args, **kwargs)
                if path == "alias":
                    sub.rename(Path(temporary) / "moved")
                    sub.mkdir()
                return result
            try:
                with mock.patch.object(os, "unlink", side_effect=moved):
                    with self.assertRaisesRegex(RuntimeError, "ancestor link changed"):
                        cleanup.detach(owned, time.monotonic() + 5)
                self.assertEqual(member.read_bytes(), b"payload")
            finally:
                owned.close()

    def test_cancel_and_membership_bound_do_not_unlink_or_read_contents(self):
        with tempfile.TemporaryDirectory() as temporary:
            roots = self.fixture(temporary)
            (roots["payload"] / "member").write_bytes(b"payload")
            owned = cleanup.OwnedRoots(roots)
            try:
                with mock.patch.object(os, "read", side_effect=AssertionError("content read")), mock.patch.object(os, "unlink", side_effect=AssertionError("unlink")):
                    with self.assertRaisesRegex(RuntimeError, "deadline"):
                        cleanup.detach(owned, time.monotonic() - 1)
                    with mock.patch.object(cleanup, "LIMIT", 1):
                        with self.assertRaisesRegex(RuntimeError, "census bound"):
                            cleanup.detach(owned, time.monotonic() + 5)
            finally:
                owned.close()

    @unittest.skipIf(os.name == "nt", "POSIX symlink fixture")
    def test_symlink_is_not_followed_and_root_replacement_refuses(self):
        with tempfile.TemporaryDirectory() as temporary:
            roots = self.fixture(temporary)
            outside = Path(temporary) / "outside"
            outside.mkdir()
            (outside / "sentinel").write_bytes(b"private")
            (roots["cache"] / "link").symlink_to(outside, target_is_directory=True)
            owned = cleanup.OwnedRoots(roots)
            try:
                records = cleanup.census(owned, time.monotonic() + 5)
                self.assertNotIn(("cache", "link/sentinel"), records)
                roots["cache"].rename(Path(temporary) / "old")
                roots["cache"].mkdir(mode=0o700)
                with self.assertRaisesRegex(RuntimeError, "root replaced"):
                    cleanup.detach(owned, time.monotonic() + 5)
            finally:
                owned.close()

    @unittest.skipIf(os.name == "nt", "inert projected WinAPI contract on POSIX")
    def test_windows_handle_disposition_arguments_and_identity_gate(self):
        import ctypes
        with tempfile.TemporaryDirectory() as temporary:
            path = Path(temporary) / "owned"
            path.write_bytes(b"payload")
            expected = path.lstat()
            kernel = types.SimpleNamespace(CreateFileW=mock.Mock(return_value=123),
                                           CloseHandle=mock.Mock(return_value=True),
                                           SetFileInformationByHandle=mock.Mock(return_value=True))
            runtime = types.SimpleNamespace(open_osfhandle=lambda handle, flags: os.open(path, os.O_RDONLY))
            with mock.patch.object(ctypes, "WinDLL", return_value=kernel, create=True), mock.patch.object(os, "O_BINARY", 0, create=True), mock.patch.dict("sys.modules", {"msvcrt": runtime}):
                cleanup.windows_delete(path, expected)
                self.assertEqual(kernel.CreateFileW.call_args.args[1:6], (0x80010000, 3, None, 3, 0x00200000))
                call = kernel.SetFileInformationByHandle.call_args.args
                self.assertEqual((call[0], call[1], call[3]), (123, 4, 1))
                self.assertEqual(ctypes.cast(call[2], ctypes.POINTER(ctypes.c_ubyte)).contents.value, 1)
                kernel.SetFileInformationByHandle.reset_mock()
                path.unlink()
                path.write_bytes(b"foreign replacement")
                with self.assertRaisesRegex(RuntimeError, "delete leaf changed"):
                    cleanup.windows_delete(path, expected)
                kernel.SetFileInformationByHandle.assert_not_called()
                self.assertEqual(path.read_bytes(), b"foreign replacement")


if __name__ == "__main__":
    unittest.main()

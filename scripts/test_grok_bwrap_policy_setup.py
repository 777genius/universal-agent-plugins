#!/usr/bin/env python3
"""Inert directory-authority checks; never install or execute a host policy."""
import os
from pathlib import Path
import stat
import sys
import tempfile
import unittest
from unittest import mock

import grok_bwrap_policy_setup as policy


@unittest.skipUnless(sys.platform == "linux", "Linux directory descriptor contract")
class PolicyDirectoryAuthorityTests(unittest.TestCase):
    def test_owned_entry_creation_keeps_original_authority(self):
        with tempfile.TemporaryDirectory(prefix="grok-policy-directory-test-") as temporary:
            directory = Path(temporary) / "policy"
            directory.mkdir(mode=0o755)
            directory.chmod(0o755)
            before = directory.lstat()
            fd, original = policy.open_policy_directory(directory)
            try:
                self.assertEqual(policy.identity(original), policy.identity(before))
                (directory / "owned-entry").write_bytes(b"inert")
                policy.verify_policy_directory(fd, original, directory)
                self.assertEqual(policy.identity(directory.lstat())[:5], policy.identity(before)[:5])
            finally:
                os.close(fd)

    def test_same_inode_permission_change_refuses(self):
        for mode in (0o777, 0o750):
            with self.subTest(mode=oct(mode)):
                with tempfile.TemporaryDirectory(prefix="grok-policy-directory-test-") as temporary:
                    directory = Path(temporary) / "policy"
                    directory.mkdir(mode=0o755)
                    directory.chmod(0o755)
                    fd, original = policy.open_policy_directory(directory)
                    try:
                        directory.chmod(mode)
                        held, named = os.fstat(fd), directory.lstat()
                        self.assertEqual(policy.identity(held)[:5], policy.identity(named)[:5])
                        self.assertEqual((held.st_dev, held.st_ino), (original.st_dev, original.st_ino))
                        self.assertEqual(stat.S_IMODE(held.st_mode), mode)
                        with self.assertRaisesRegex(ValueError, "^policy directory authority changed$"):
                            policy.verify_policy_directory(fd, original, directory)
                    finally:
                        directory.chmod(0o755)
                        os.close(fd)

    def test_original_is_retained_across_acquisition_return(self):
        with tempfile.TemporaryDirectory(prefix="grok-policy-directory-test-") as temporary:
            directory = Path(temporary) / "policy"
            directory.mkdir(mode=0o755)
            directory.chmod(0o755)
            before = directory.lstat()
            original_fstat = os.fstat

            def read_then_change_mode(fd):
                result = original_fstat(fd)
                directory.chmod(0o750)
                return result

            with mock.patch.object(policy.os, "fstat", side_effect=read_then_change_mode):
                fd, original = policy.open_policy_directory(directory)
            try:
                self.assertEqual(policy.identity(original), policy.identity(before))
                self.assertEqual(stat.S_IMODE(os.fstat(fd).st_mode), 0o750)
                with self.assertRaisesRegex(ValueError, "^policy directory authority changed$"):
                    policy.verify_policy_directory(fd, original, directory)
            finally:
                directory.chmod(0o755)
                os.close(fd)

    def test_replaced_named_directory_refuses(self):
        with tempfile.TemporaryDirectory(prefix="grok-policy-directory-test-") as temporary:
            parent = Path(temporary)
            directory = parent / "policy"
            directory.mkdir(mode=0o755)
            directory.chmod(0o755)
            fd, original = policy.open_policy_directory(directory)
            try:
                directory.rename(parent / "retained-policy")
                directory.mkdir(mode=0o755)
                directory.chmod(0o755)
                self.assertNotEqual(directory.lstat().st_ino, original.st_ino)
                with self.assertRaisesRegex(ValueError, "^policy directory authority changed$"):
                    policy.verify_policy_directory(fd, original, directory)
            finally:
                os.close(fd)


if __name__ == "__main__":
    unittest.main()

#!/usr/bin/env python3
"""Inert directory-authority checks; never install or execute a host policy."""
import os
from pathlib import Path
import stat
import sys
import tempfile
from types import SimpleNamespace
import unittest
from unittest import mock

import devin_bwrap_policy_setup as devin_policy
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


@unittest.skipUnless(sys.platform == "linux", "Linux kernel feature descriptor contract")
class KernelABIContracts(unittest.TestCase):
    @staticmethod
    def fixture(parent):
        root = parent / "features"
        root.mkdir(mode=0o755)
        for name in ("io_uring", "namespaces", "domain"):
            (root / name).mkdir(mode=0o755)
        for name, body in (("io_uring/mask", b"sqpoll override_creds\n"),
                           ("namespaces/mask", b"userns_create\n"),
                           ("domain/stack", b"yes\n"), ("domain/post_nnp_subset", b"yes\n")):
            (root / name).write_bytes(body)
            (root / name).chmod(0o444)
        return root

    def assert_closed(self, descriptors):
        for fd in descriptors:
            with self.assertRaises(OSError):
                os.fstat(fd)

    def test_required_features_support_zero_size_securityfs_files_and_repeated_reads(self):
        fields = ("st_dev", "st_ino", "st_mode", "st_uid", "st_gid", "st_nlink",
                  "st_size", "st_mtime_ns", "st_ctime_ns")
        real_stat, real_fstat = os.stat, os.fstat
        def securityfs_size(info):
            if stat.S_ISREG(info.st_mode):
                values = {name: getattr(info, name) for name in fields}
                values["st_size"] = 0
                return SimpleNamespace(**values)
            return info
        for module in (policy, devin_policy):
            with self.subTest(module=module.__name__), tempfile.TemporaryDirectory() as temporary:
                root = self.fixture(Path(temporary))
                with mock.patch.object(module, "KERNEL_FEATURE_DIRECTORY", root), \
                     mock.patch.object(os, "stat", side_effect=lambda *a, **kw: securityfs_size(real_stat(*a, **kw))), \
                     mock.patch.object(os, "fstat", side_effect=lambda fd: securityfs_size(real_fstat(fd))):
                    deadline = module.time.monotonic() + 10
                    held = module.open_kernel_features(deadline)
                    try:
                        self.assertEqual(len(held), 8)
                        self.assertEqual(held["io_uring/mask"][1].st_size, 0)
                        self.assertEqual(held["io_uring/mask"][2], b"sqpoll override_creds\n")
                        module.verify_kernel_features(held, deadline)
                        module.verify_kernel_features(held, deadline)
                    finally:
                        module.close_kernel_features(held)
                self.assert_closed([row[0] for row in held.values()])

    def test_missing_mediation_and_unsafe_feature_objects_refuse_before_export(self):
        changes = ("missing", "sqpoll", "override_creds", "userns", "stack", "post_nnp",
                   "symlink", "hardlink", "writable", "overbound")
        for module in (policy, devin_policy):
            for change in changes:
                with self.subTest(module=module.__name__, change=change), tempfile.TemporaryDirectory() as temporary:
                    root = self.fixture(Path(temporary))
                    leaf = root / "io_uring/mask"
                    if change == "missing":
                        leaf.unlink()
                    elif change in ("sqpoll", "override_creds", "userns", "stack", "post_nnp", "overbound"):
                        target, body = {
                            "sqpoll": (leaf, b"override_creds\n"),
                            "override_creds": (leaf, b"sqpoll\n"),
                            "userns": (root / "namespaces/mask", b"other\n"),
                            "stack": (root / "domain/stack", b"no\n"),
                            "post_nnp": (root / "domain/post_nnp_subset", b"no\n"),
                            "overbound": (leaf, b"x" * 4097),
                        }[change]
                        target.chmod(0o600); target.write_bytes(body); target.chmod(0o444)
                    elif change == "symlink":
                        leaf.rename(root.parent / "original-mask")
                        leaf.symlink_to(root.parent / "original-mask")
                    elif change == "hardlink":
                        os.link(leaf, root.parent / "mask-alias")
                    else:
                        leaf.chmod(0o666)
                    opened, real_open = [], os.open
                    def remember(*args, **kwargs):
                        fd = real_open(*args, **kwargs); opened.append(fd); return fd
                    with mock.patch.object(module, "KERNEL_FEATURE_DIRECTORY", root), \
                         mock.patch.object(os, "open", side_effect=remember), \
                         mock.patch.object(module, "command") as command:
                        with self.assertRaises((OSError, ValueError)):
                            module.open_kernel_features(module.time.monotonic() + 10)
                        command.assert_not_called()
                    self.assert_closed(opened)

    def test_same_bytes_replacements_and_same_metadata_byte_drift_keep_original_authority(self):
        for module in (policy, devin_policy):
            for change in ("file", "directory", "bytes"):
                with self.subTest(module=module.__name__, change=change), tempfile.TemporaryDirectory() as temporary:
                    root = self.fixture(Path(temporary))
                    with mock.patch.object(module, "KERNEL_FEATURE_DIRECTORY", root):
                        deadline = module.time.monotonic() + 10
                        held = module.open_kernel_features(deadline)
                        try:
                            leaf = root / "io_uring/mask"
                            if change == "file":
                                leaf.rename(root.parent / "original-mask")
                                leaf.write_bytes(b"sqpoll override_creds\n"); leaf.chmod(0o444)
                            elif change == "directory":
                                root.rename(root.parent / "original-features")
                                self.fixture(root.parent)
                            else:
                                original = held["io_uring/mask"][1]
                                leaf.chmod(0o600); leaf.write_bytes(b"override_creds sqpoll\n"); leaf.chmod(0o444)
                                real_stat, real_fstat = os.stat, os.fstat
                                def stable_metadata(info):
                                    return original if (info.st_dev, info.st_ino) == (original.st_dev, original.st_ino) else info
                                with mock.patch.object(os, "stat", side_effect=lambda *a, **kw: stable_metadata(real_stat(*a, **kw))), \
                                     mock.patch.object(os, "fstat", side_effect=lambda fd: stable_metadata(real_fstat(fd))):
                                    with self.assertRaisesRegex(ValueError, "kernel feature bytes changed"):
                                        module.verify_kernel_features(held, deadline)
                            if change != "bytes":
                                with self.assertRaisesRegex(ValueError, "original policy kernel feature changed"):
                                    module.verify_kernel_features(held, deadline)
                        finally:
                            module.close_kernel_features(held)
                    self.assert_closed([row[0] for row in held.values()])

    def test_last_feature_read_revalidates_earlier_original_identities(self):
        for change in ("early-feature", "feature-root"):
            with self.subTest(change=change), tempfile.TemporaryDirectory() as temporary:
                parent = Path(temporary)
                root = self.fixture(parent)
                with mock.patch.object(policy, "KERNEL_FEATURE_DIRECTORY", root):
                    deadline = policy.time.monotonic() + 10
                    held = policy.open_kernel_features(deadline)
                    try:
                        final_fd = held["domain/post_nnp_subset"][0]
                        original_read = policy.read_kernel_feature
                        triggered = [False]

                        def replace_after_final_read(fd, value):
                            body = original_read(fd, value)
                            if fd == final_fd and not triggered[0]:
                                triggered[0] = True
                                if change == "early-feature":
                                    leaf = root / "io_uring/mask"
                                    leaf.rename(parent / "retained-mask")
                                    leaf.write_bytes(b"sqpoll override_creds\n")
                                    leaf.chmod(0o444)
                                else:
                                    root.rename(parent / "retained-features")
                                    self.fixture(parent)
                            return body

                        with mock.patch.object(policy, "read_kernel_feature", side_effect=replace_after_final_read):
                            with self.assertRaisesRegex(ValueError, "^original policy kernel feature changed$"):
                                policy.verify_kernel_features(held, deadline)
                        self.assertTrue(triggered[0])
                    finally:
                        policy.close_kernel_features(held)
                self.assert_closed([row[0] for row in held.values()])

    def test_full_actual_abi_and_original_deadline_survive_export(self):
        for module in (policy, devin_policy):
            for change in ("same", "other-feature", "oversize", "binary", "late-export", "late-eof"):
                with self.subTest(module=module.__name__, change=change), tempfile.TemporaryDirectory() as temporary:
                    root = self.fixture(Path(temporary))
                    clock, records, observed = [0.0], [], []
                    original = b"inert complete kernel feature export\n"
                    def command(directory, label, argv, deadline, output, limit):
                        observed.append(label)
                        self.assertEqual((directory, deadline, output, limit), (root.parent, 1.0, records, 65536))
                        self.assertEqual(argv, ["/usr/bin/aa-features-abi", "--extract", "--stdout"])
                        if label.endswith("-before"):
                            return original
                        if change == "late-export": clock[0] = 1.0
                        return {"other-feature": original + b"another feature\n",
                                "oversize": b"x" * 65537, "binary": b"bad\x00export"}.get(change, original)
                    with mock.patch.object(module, "KERNEL_FEATURE_DIRECTORY", root), \
                         mock.patch.object(module.time, "monotonic", side_effect=lambda: clock[0]), \
                         mock.patch.object(module, "command", side_effect=command):
                        held = module.open_kernel_features(1.0)
                        try:
                            actual = module.capture_kernel_abi(root.parent, held, 1.0, records)
                            self.assertEqual(actual, original)
                            if change == "same":
                                self.assertEqual(module.capture_kernel_abi(root.parent, held, 1.0, records, actual), original)
                            elif change == "late-eof":
                                real_read = os.read
                                def expire_after_eof(fd, count):
                                    body = real_read(fd, count)
                                    if not body: clock[0] = 1.0
                                    return body
                                with mock.patch.object(os, "read", side_effect=expire_after_eof), \
                                     self.assertRaisesRegex(ValueError, "setup deadline"):
                                    module.capture_kernel_abi(root.parent, held, 1.0, records, actual)
                                self.assertEqual(observed, ["policy-kernel-abi-before"])
                            else:
                                with self.assertRaises(ValueError):
                                    module.capture_kernel_abi(root.parent, held, 1.0, records, actual)
                        finally:
                            module.close_kernel_features(held)
                    self.assert_closed([row[0] for row in held.values()])


    def test_grok_setup_selects_actual_abi_and_rejoins_after_existing_control(self):
        for changed in (False, True):
            with self.subTest(control_replaces_feature=changed), tempfile.TemporaryDirectory() as temporary:
                parent = Path(temporary)
                features = self.fixture(parent)
                download, policies = parent / "download", parent / "policies"
                download.mkdir(mode=0o700); policies.mkdir(mode=0o700)
                deb, vendor, derived = b"inert archive", b"inert vendor", b"inert derived policy"
                initial, records, observed, held_rows = {"existing (enforce)"}, [], [], []
                deadline = policy.time.monotonic() + 10
                real_open, real_verify = policy.open_policy_directory, policy.verify_policy_directory
                real_features = policy.open_kernel_features
                def features_open(value):
                    result = real_features(value); held_rows.extend(result.values()); return result
                def command(directory, label, argv, value, output, limit=policy.LIMIT):
                    self.assertEqual((directory, value, output), (download, deadline, records))
                    observed.append(label)
                    records.append({"probe": label, "argv": argv, "error": None, "exit": 0})
                    if label == "policy-download":
                        (download / policy.DEB_NAME).write_bytes(deb)
                        (download / policy.DEB_NAME).chmod(0o600)
                    if label in ("policy-kernel-abi-before", "policy-kernel-abi-after"):
                        self.assertEqual(argv, ["/usr/bin/aa-features-abi", "--extract", "--stdout"])
                        return b"inert complete actual kernel ABI\n"
                    if label == "policy-add":
                        self.assertEqual(argv, ["/usr/sbin/apparmor_parser", "--config-file=/dev/null",
                            "--add", "--skip-cache", "--jobs=1", "--override-policy-abi=<kernel>",
                            "--warn=rule-not-enforced", "--Werror=rule-not-enforced",
                            str(policies / policy.PROFILE_NAME)])
                        self.assertEqual((policies / policy.PROFILE_NAME).read_bytes(), derived)
                    return b""
                def control(*args):
                    self.assertEqual(args, (download, deadline, parent / "runtime", parent / "inputs",
                                             parent / "output", 1000, 1000, records))
                    observed.append("original-runtime-control")
                    if changed:
                        leaf = features / "io_uring/mask"
                        leaf.rename(parent / "retained-mask")
                        leaf.write_bytes(b"sqpoll override_creds\n"); leaf.chmod(0o444)
                    return {"native_client_executed": False, "qualification": False}
                with mock.patch.object(policy, "POLICY_DIRECTORY", policies), \
                     mock.patch.object(policy, "KERNEL_FEATURE_DIRECTORY", features), \
                     mock.patch.object(policy, "open_policy_directory", side_effect=lambda: real_open(policies)), \
                     mock.patch.object(policy, "verify_policy_directory", side_effect=lambda fd, original: real_verify(fd, original, policies)), \
                     mock.patch.object(policy, "open_kernel_features", side_effect=features_open), \
                     mock.patch.object(policy, "apt_metadata", return_value={"fixed": "inert package"}), \
                     mock.patch.object(policy, "DEB_BYTES", len(deb)), \
                     mock.patch.object(policy, "DEB_SHA256", policy.sha(deb)), \
                     mock.patch.object(policy, "profile_from_tar", return_value=vendor), \
                     mock.patch.object(policy, "derive_profile", return_value=derived) as derive, \
                     mock.patch.object(policy, "command", side_effect=command), \
                     mock.patch.object(policy, "prove_runtime", side_effect=control), \
                     mock.patch.object(policy, "loaded_profiles", side_effect=[initial, initial | policy.EXPECTED_LOADED]), \
                     mock.patch.object(policy, "kernel_read", return_value=b"1\n"):
                    if changed:
                        with self.assertRaisesRegex(ValueError, "original policy kernel feature changed"):
                            policy.setup(download, deadline, parent / "runtime", parent / "inputs",
                                         parent / "output", 1000, 1000, records)
                    else:
                        result = policy.setup(download, deadline, parent / "runtime", parent / "inputs",
                                              parent / "output", 1000, 1000, records)
                        self.assertFalse(result["native_client_executed"])
                        self.assertFalse(result["qualification"])
                        self.assertEqual(result["vendor_profile_sha256"], policy.sha(vendor))
                        self.assertEqual(result["profile"]["sha256"], policy.sha(derived))
                    derive.assert_called_once_with(vendor)
                self.assertEqual(observed, ["policy-metadata", "policy-download", "policy-package-data",
                    "policy-kernel-abi-before", "policy-add", "original-runtime-control"]
                    + ([] if changed else ["policy-kernel-abi-after"]))
                self.assert_closed([row[0] for row in held_rows])


if __name__ == "__main__":
    unittest.main()
